package nodes

import (
	"context"
	"errors"
	"log"
	"sync"

	"github.com/google/uuid"

	"vea/backend/domain"
	"vea/backend/repository"
)

var (
	ErrNodeNotFound = errors.New("node not found")
)

const (
	nodeSpeedWorkers   = 4
	nodeLatencyWorkers = 4
)

// Measurer 复用测速器接口（通过构造临时 FRouter 来测单节点）
type Measurer interface {
	MeasureSpeed(frouter domain.FRouter, nodes []domain.Node, onProgress func(speedMbps float64)) (float64, error)
	MeasureLatency(frouter domain.FRouter, nodes []domain.Node) (int64, error)
}

type Service struct {
	repo repository.NodeRepository

	bgCtx context.Context

	measurer Measurer

	mu           sync.Mutex
	speedQueue   chan string
	latencyQueue chan string
	speedJobs    map[string]struct{}
	latencyJobs  map[string]struct{}
	stopCh       chan struct{}
}

func NewService(bgCtx context.Context, repo repository.NodeRepository) *Service {
	if bgCtx == nil {
		bgCtx = context.Background()
	}
	s := &Service{
		repo:         repo,
		bgCtx:        bgCtx,
		speedQueue:   make(chan string, 1024),
		latencyQueue: make(chan string, 1024),
		speedJobs:    make(map[string]struct{}),
		latencyJobs:  make(map[string]struct{}),
		stopCh:       make(chan struct{}),
	}
	for i := 0; i < nodeSpeedWorkers; i++ {
		go s.speedWorker()
	}
	for i := 0; i < nodeLatencyWorkers; i++ {
		go s.latencyWorker()
	}
	return s
}

func (s *Service) SetMeasurer(measurer Measurer) {
	s.measurer = measurer
}

func (s *Service) List(ctx context.Context) ([]domain.Node, error) {
	return s.repo.List(ctx)
}

func (s *Service) Get(ctx context.Context, id string) (domain.Node, error) {
	return s.repo.Get(ctx, id)
}

func (s *Service) Create(ctx context.Context, node domain.Node) (domain.Node, error) {
	return s.repo.Create(ctx, node)
}

func (s *Service) Update(ctx context.Context, id string, node domain.Node) (domain.Node, error) {
	return s.repo.Update(ctx, id, node)
}

func (s *Service) ReplaceNodesForConfig(ctx context.Context, configID string, nodes []domain.Node) ([]domain.Node, error) {
	return s.repo.ReplaceNodesForConfig(ctx, configID, nodes)
}

func (s *Service) ProbeLatencyAsync(id string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.latencyJobs[id]; ok {
		return
	}
	s.latencyJobs[id] = struct{}{}
	select {
	case s.latencyQueue <- id:
	default:
		delete(s.latencyJobs, id)
	}
}

func (s *Service) ProbeSpeedAsync(id string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.speedJobs[id]; ok {
		return
	}
	s.speedJobs[id] = struct{}{}
	select {
	case s.speedQueue <- id:
	default:
		delete(s.speedJobs, id)
	}
}

func (s *Service) speedWorker() {
	for {
		select {
		case <-s.bgCtx.Done():
			return
		case <-s.stopCh:
			return
		case id := <-s.speedQueue:
			s.doProbeSpeed(id)
			s.mu.Lock()
			delete(s.speedJobs, id)
			s.mu.Unlock()
		}
	}
}

func (s *Service) latencyWorker() {
	for {
		select {
		case <-s.bgCtx.Done():
			return
		case <-s.stopCh:
			return
		case id := <-s.latencyQueue:
			s.doProbeLatency(id)
			s.mu.Lock()
			delete(s.latencyJobs, id)
			s.mu.Unlock()
		}
	}
}

func syntheticFRouterForNode(nodeID string) domain.FRouter {
	return domain.FRouter{
		ID:   "node-probe-" + uuid.NewString(),
		Name: "node-probe",
		ChainProxy: domain.ChainProxySettings{
			Edges: []domain.ProxyEdge{
				{
					ID:       uuid.NewString(),
					From:     domain.EdgeNodeLocal,
					To:       nodeID,
					Priority: 0,
					Enabled:  true,
				},
			},
		},
	}
}

func (s *Service) doProbeSpeed(id string) {
	ctx := s.bgCtx
	node, err := s.repo.Get(ctx, id)
	if err != nil {
		log.Printf("[NodeSpeed] 获取节点失败 %s: %v", id, err)
		return
	}

	if s.measurer == nil {
		log.Printf("[NodeSpeed] 测速器未设置，跳过节点 %s", id)
		_ = s.repo.UpdateSpeed(ctx, id, 0, "测速器未初始化")
		return
	}

	// 仅测该节点：构造临时 FRouter
	frouter := syntheticFRouterForNode(node.ID)

	var lastReported float64
	onProgress := func(speedMbps float64) {
		if speedMbps <= 0 {
			return
		}
		smoothed := speedMbps
		if lastReported > 0 {
			if speedMbps > lastReported {
				smoothed = lastReported + (speedMbps-lastReported)*0.4
			} else {
				smoothed = lastReported*0.6 + speedMbps*0.4
			}
		}
		lastReported = smoothed
		_ = s.repo.UpdateSpeed(ctx, id, smoothed, "")
	}

	mbps, err := s.measurer.MeasureSpeed(frouter, []domain.Node{node}, onProgress)
	if err != nil {
		log.Printf("[NodeSpeed] 测速失败 %s: %v", id, err)
		_ = s.repo.UpdateSpeed(ctx, id, 0, err.Error())
		return
	}

	finalSpeed := mbps
	if lastReported > 0 && finalSpeed < lastReported {
		finalSpeed = lastReported
	}
	_ = s.repo.UpdateSpeed(ctx, id, finalSpeed, "")
}

func (s *Service) doProbeLatency(id string) {
	ctx := s.bgCtx
	node, err := s.repo.Get(ctx, id)
	if err != nil {
		log.Printf("[NodeLatency] 获取节点失败 %s: %v", id, err)
		return
	}

	if s.measurer == nil {
		log.Printf("[NodeLatency] 测速器未设置，跳过节点 %s", id)
		_ = s.repo.UpdateLatency(ctx, id, 0, "测速器未初始化")
		return
	}

	frouter := syntheticFRouterForNode(node.ID)

	latency, err := s.measurer.MeasureLatency(frouter, []domain.Node{node})
	if err != nil {
		log.Printf("[NodeLatency] 测延迟失败 %s: %v", id, err)
		_ = s.repo.UpdateLatency(ctx, id, 0, err.Error())
		return
	}
	_ = s.repo.UpdateLatency(ctx, id, latency, "")
}
