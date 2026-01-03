package frouter

import (
	"context"
	"testing"

	"vea/backend/domain"
)

type recordingRepo struct {
	frouter domain.FRouter

	speedCalls   []updateSpeedCall
	latencyCalls []updateLatencyCall
}

type updateSpeedCall struct {
	id    string
	speed float64
	err   string
}

type updateLatencyCall struct {
	id      string
	latency int64
	err     string
}

func (r *recordingRepo) Get(context.Context, string) (domain.FRouter, error) { return r.frouter, nil }
func (r *recordingRepo) List(context.Context) ([]domain.FRouter, error)      { return nil, nil }
func (r *recordingRepo) Create(context.Context, domain.FRouter) (domain.FRouter, error) {
	return domain.FRouter{}, nil
}
func (r *recordingRepo) Update(context.Context, string, domain.FRouter) (domain.FRouter, error) {
	return domain.FRouter{}, nil
}
func (r *recordingRepo) Delete(context.Context, string) error { return nil }
func (r *recordingRepo) UpdateLatency(_ context.Context, id string, latencyMS int64, latencyErr string) error {
	r.latencyCalls = append(r.latencyCalls, updateLatencyCall{id: id, latency: latencyMS, err: latencyErr})
	return nil
}
func (r *recordingRepo) UpdateSpeed(_ context.Context, id string, speedMbps float64, speedErr string) error {
	r.speedCalls = append(r.speedCalls, updateSpeedCall{id: id, speed: speedMbps, err: speedErr})
	return nil
}

func TestService_doProbeSpeed_NoMeasurer_SetsError(t *testing.T) {
	t.Parallel()

	repo := &recordingRepo{frouter: domain.FRouter{ID: "fr-1"}}
	svc := NewService(repo, nil)
	t.Cleanup(func() { close(svc.stopCh) })

	svc.doProbeSpeed("fr-1")

	if len(repo.speedCalls) != 1 {
		t.Fatalf("expected 1 UpdateSpeed call, got %d", len(repo.speedCalls))
	}
	call := repo.speedCalls[0]
	if call.id != "fr-1" || call.speed != 0 || call.err != "测速器未初始化" {
		t.Fatalf("unexpected UpdateSpeed call: %+v", call)
	}
}

func TestService_doProbeLatency_NoMeasurer_SetsError(t *testing.T) {
	t.Parallel()

	repo := &recordingRepo{frouter: domain.FRouter{ID: "fr-1"}}
	svc := NewService(repo, nil)
	t.Cleanup(func() { close(svc.stopCh) })

	svc.doProbeLatency("fr-1")

	if len(repo.latencyCalls) != 1 {
		t.Fatalf("expected 1 UpdateLatency call, got %d", len(repo.latencyCalls))
	}
	call := repo.latencyCalls[0]
	if call.id != "fr-1" || call.latency != 0 || call.err != "测速器未初始化" {
		t.Fatalf("unexpected UpdateLatency call: %+v", call)
	}
}
