package nodes

import (
	"context"
	"testing"

	"vea/backend/domain"
)

type noopNodeRepo struct{}

func (r *noopNodeRepo) Get(context.Context, string) (domain.Node, error) { return domain.Node{}, nil }
func (r *noopNodeRepo) List(context.Context) ([]domain.Node, error)      { return nil, nil }
func (r *noopNodeRepo) Create(context.Context, domain.Node) (domain.Node, error) {
	return domain.Node{}, nil
}
func (r *noopNodeRepo) Update(context.Context, string, domain.Node) (domain.Node, error) {
	return domain.Node{}, nil
}
func (r *noopNodeRepo) Delete(context.Context, string) error { return nil }
func (r *noopNodeRepo) ListByConfigID(context.Context, string) ([]domain.Node, error) {
	return nil, nil
}
func (r *noopNodeRepo) ReplaceNodesForConfig(context.Context, string, []domain.Node) ([]domain.Node, error) {
	return nil, nil
}
func (r *noopNodeRepo) UpdateLatency(context.Context, string, int64, string) error { return nil }
func (r *noopNodeRepo) UpdateSpeed(context.Context, string, float64, string) error { return nil }

func TestService_ProbeLatencyAsync_Deduplicates(t *testing.T) {
	t.Parallel()

	svc := NewService(&noopNodeRepo{})
	close(svc.stopCh) // 停止 worker，避免消费队列影响断言

	svc.ProbeLatencyAsync("n1")
	svc.ProbeLatencyAsync("n1")

	svc.mu.Lock()
	defer svc.mu.Unlock()

	if _, ok := svc.latencyJobs["n1"]; !ok {
		t.Fatalf("expected latency job n1 to exist")
	}
	if len(svc.latencyJobs) != 1 {
		t.Fatalf("expected 1 latency job, got %d", len(svc.latencyJobs))
	}
}
