package tasks

import (
	"context"
	"time"

	"vea/backend/service"
)

type GeoSync struct {
	Service  *service.Service
	Interval time.Duration
}

func (t *GeoSync) Start(ctx context.Context) {
	if t.Service == nil {
		return
	}
	interval := t.Interval
	if interval <= 0 {
		interval = 6 * time.Hour
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			t.Service.SyncGeoResources()
		}
	}
}
