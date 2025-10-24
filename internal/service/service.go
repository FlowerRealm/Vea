package service

import (
	"context"
	"math/rand"
	"time"

	"vea/internal/domain"
	"vea/internal/store"
)

type Service struct {
	store *store.MemoryStore
	tasks []Task
}

type Task interface {
	Start(ctx context.Context)
}

func NewService(store *store.MemoryStore, tasks ...Task) *Service {
	return &Service{store: store, tasks: tasks}
}

func (s *Service) AttachTasks(tasks ...Task) {
	s.tasks = append(s.tasks, tasks...)
}

func (s *Service) Start(ctx context.Context) {
	for _, task := range s.tasks {
		go task.Start(ctx)
	}
}

func (s *Service) ListNodes() []domain.Node {
	return s.store.ListNodes()
}

func (s *Service) CreateNode(node domain.Node) domain.Node {
	return s.store.CreateNode(node)
}

func (s *Service) UpdateNode(id string, mutate func(domain.Node) (domain.Node, error)) (domain.Node, error) {
	return s.store.UpdateNode(id, mutate)
}

func (s *Service) DeleteNode(id string) error {
	return s.store.DeleteNode(id)
}

func (s *Service) ResetNodeTraffic(id string) (domain.Node, error) {
	return s.store.ResetNodeTraffic(id)
}

func (s *Service) IncrementNodeTraffic(id string, up, down int64) (domain.Node, error) {
	return s.store.IncrementNodeTraffic(id, up, down)
}

func (s *Service) ProbeLatency(id string) (domain.Node, error) {
	return s.store.UpdateNode(id, func(node domain.Node) (domain.Node, error) {
		node.LastLatencyMS = int64(50 + rand.Intn(200))
		return node, nil
	})
}

func (s *Service) ProbeSpeed(id string) (domain.Node, error) {
	return s.store.UpdateNode(id, func(node domain.Node) (domain.Node, error) {
		node.LastSpeedMbps = 10 + rand.Float64()*90
		return node, nil
	})
}

func (s *Service) ListConfigs() []domain.Config {
	return s.store.ListConfigs()
}

func (s *Service) CreateConfig(cfg domain.Config) domain.Config {
	return s.store.CreateConfig(cfg)
}

func (s *Service) UpdateConfig(id string, mutate func(domain.Config) (domain.Config, error)) (domain.Config, error) {
	return s.store.UpdateConfig(id, mutate)
}

func (s *Service) DeleteConfig(id string) error {
	return s.store.DeleteConfig(id)
}

func (s *Service) IncrementConfigTraffic(id string, up, down int64) (domain.Config, error) {
	return s.store.IncrementConfigTraffic(id, up, down)
}

func (s *Service) RefreshConfig(id string) (domain.Config, error) {
	return s.store.UpdateConfig(id, func(cfg domain.Config) (domain.Config, error) {
		cfg.LastSyncedAt = time.Now()
		return cfg, nil
	})
}

func (s *Service) AutoUpdateConfigs() {
	for _, cfg := range s.ListConfigs() {
		interval := cfg.AutoUpdateInterval
		if interval <= 0 {
			interval = time.Hour
		}
		if time.Since(cfg.LastSyncedAt) >= interval {
			_, _ = s.RefreshConfig(cfg.ID)
		}
	}
}

func (s *Service) ListGeo() []domain.GeoResource {
	return s.store.ListGeo()
}

func (s *Service) UpsertGeo(res domain.GeoResource) domain.GeoResource {
	return s.store.UpsertGeo(res)
}

func (s *Service) DeleteGeo(id string) error {
	return s.store.DeleteGeo(id)
}

func (s *Service) SyncGeoResources() {
	for _, res := range s.ListGeo() {
		s.store.UpsertGeo(domain.GeoResource{
			ID:         res.ID,
			Name:       res.Name,
			Type:       res.Type,
			SourceURL:  res.SourceURL,
			Checksum:   res.Checksum,
			Version:    res.Version,
			LastSynced: time.Now(),
		})
	}
}

func (s *Service) RefreshGeo(id string) (domain.GeoResource, error) {
	res, err := s.store.GetGeo(id)
	if err != nil {
		return domain.GeoResource{}, err
	}
	res.LastSynced = time.Now()
	return s.store.UpsertGeo(res), nil
}

func (s *Service) ListTrafficRules() []domain.TrafficRule {
	return s.store.ListTrafficRules()
}

func (s *Service) CreateTrafficRule(rule domain.TrafficRule) domain.TrafficRule {
	return s.store.CreateTrafficRule(rule)
}

func (s *Service) UpdateTrafficRule(id string, mutate func(domain.TrafficRule) (domain.TrafficRule, error)) (domain.TrafficRule, error) {
	return s.store.UpdateTrafficRule(id, mutate)
}

func (s *Service) DeleteTrafficRule(id string) error {
	return s.store.DeleteTrafficRule(id)
}

func (s *Service) UpdateTrafficProfile(mutator func(domain.TrafficProfile) (domain.TrafficProfile, error)) (domain.TrafficProfile, error) {
	return s.store.UpdateTrafficProfile(mutator)
}

func (s *Service) GetTrafficProfile() domain.TrafficProfile {
	return s.store.GetTrafficProfile()
}

func (s *Service) Snapshot() domain.ServiceState {
	return s.store.Snapshot()
}

func (s *Service) Errors() (error, error, error, error) {
	return s.store.Errors()
}
