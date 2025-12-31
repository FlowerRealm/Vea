package frouter

import (
	"context"
	"errors"
	"log"
	"sync"

	"vea/backend/domain"
	"vea/backend/repository"
)

// 错误定义
var (
	ErrFRouterNotFound = errors.New("frouter not found")
)

// Measurer FRouter 测量接口
type Measurer interface {
	MeasureSpeed(frouter domain.FRouter, nodes []domain.Node, onProgress func(speedMbps float64)) (float64, error)
	MeasureLatency(frouter domain.FRouter, nodes []domain.Node) (int64, error)
}

// Service FRouter 服务
type Service struct {
	repo     repository.FRouterRepository
	nodeRepo repository.NodeRepository

	measurer Measurer

	mu           sync.Mutex
	speedQueue   chan string
	latencyQueue chan string
	speedJobs    map[string]struct{}
	latencyJobs  map[string]struct{}
	stopCh       chan struct{}
}

// NewService 创建 FRouter 服务
func NewService(repo repository.FRouterRepository, nodeRepo repository.NodeRepository) *Service {
	s := &Service{
		repo:         repo,
		nodeRepo:     nodeRepo,
		speedQueue:   make(chan string, 32),
		latencyQueue: make(chan string, 32),
		speedJobs:    make(map[string]struct{}),
		latencyJobs:  make(map[string]struct{}),
		stopCh:       make(chan struct{}),
	}
	go s.speedWorker()
	go s.latencyWorker()
	return s
}

// SetMeasurer 设置测速器
func (s *Service) SetMeasurer(measurer Measurer) {
	s.measurer = measurer
}

// List 列出所有 FRouter
func (s *Service) List(ctx context.Context) ([]domain.FRouter, error) {
	return s.repo.List(ctx)
}

// Get 获取 FRouter
func (s *Service) Get(ctx context.Context, id string) (domain.FRouter, error) {
	return s.repo.Get(ctx, id)
}

// Create 创建 FRouter
func (s *Service) Create(ctx context.Context, frouter domain.FRouter) (domain.FRouter, error) {
	return s.repo.Create(ctx, frouter)
}

// Update 更新 FRouter
func (s *Service) Update(ctx context.Context, id string, frouter domain.FRouter) (domain.FRouter, error) {
	return s.repo.Update(ctx, id, frouter)
}

// Delete 删除 FRouter
func (s *Service) Delete(ctx context.Context, id string) error {
	return s.repo.Delete(ctx, id)
}

// ProbeLatencyAsync 异步测延迟
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

// ProbeSpeedAsync 异步测速
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

func (s *Service) doProbeSpeed(id string) {
	ctx := context.Background()
	frouter, err := s.repo.Get(ctx, id)
	if err != nil {
		log.Printf("[FRouterSpeed] 获取 FRouter 失败 %s: %v", id, err)
		return
	}

	if s.measurer == nil {
		log.Printf("[FRouterSpeed] 测速器未设置，跳过 FRouter %s", id)
		_ = s.repo.UpdateSpeed(ctx, id, 0, "测速器未初始化")
		return
	}

	nodes := []domain.Node(nil)
	if s.nodeRepo != nil {
		nodes, _ = s.nodeRepo.List(ctx)
	}

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

	mbps, err := s.measurer.MeasureSpeed(frouter, nodes, onProgress)
	if err != nil {
		log.Printf("[FRouterSpeed] 测速失败 %s: %v", id, err)
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
	ctx := context.Background()
	frouter, err := s.repo.Get(ctx, id)
	if err != nil {
		log.Printf("[FRouterLatency] 获取 FRouter 失败 %s: %v", id, err)
		return
	}
	if s.measurer == nil {
		log.Printf("[FRouterLatency] 测速器未设置，跳过 FRouter %s", id)
		_ = s.repo.UpdateLatency(ctx, id, 0, "测速器未初始化")
		return
	}
	nodes := []domain.Node(nil)
	if s.nodeRepo != nil {
		nodes, _ = s.nodeRepo.List(ctx)
	}
	latency, err := s.measurer.MeasureLatency(frouter, nodes)
	if err != nil {
		log.Printf("[FRouterLatency] 测延迟失败 %s: %v", id, err)
		_ = s.repo.UpdateLatency(ctx, id, 0, err.Error())
		return
	}
	_ = s.repo.UpdateLatency(ctx, id, latency, "")
}
