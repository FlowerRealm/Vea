package tasks

import (
	"context"
	"time"

	"vea/internal/service"
)

type ConfigSync struct {
	Service  *service.Service
	Interval time.Duration
}

func (t *ConfigSync) Start(ctx context.Context) {
	if t.Service == nil {
		return
	}
	interval := t.Interval
	if interval <= 0 {
		interval = time.Minute
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			t.Service.AutoUpdateConfigs()
		}
	}
}
