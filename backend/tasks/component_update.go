package tasks

import (
	"context"
	"time"

	"vea/backend/service"
)

type ComponentUpdate struct {
	Service  *service.Service
	Interval time.Duration
}

func (t *ComponentUpdate) Start(ctx context.Context) {
	if t.Service == nil {
		return
	}
	interval := t.Interval
	if interval <= 0 {
		interval = 12 * time.Hour
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			t.Service.AutoUpdateComponents()
		}
	}
}
