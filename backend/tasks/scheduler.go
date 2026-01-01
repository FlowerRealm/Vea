package tasks

import (
	"context"
	"log"
	"time"

	"vea/backend/service/component"
	configsvc "vea/backend/service/config"
	"vea/backend/service/geo"
)

type Scheduler struct {
	config    *configsvc.Service
	geo       *geo.Service
	component *component.Service
}

func NewScheduler(configSvc *configsvc.Service, geoSvc *geo.Service, componentSvc *component.Service) *Scheduler {
	return &Scheduler{
		config:    configSvc,
		geo:       geoSvc,
		component: componentSvc,
	}
}

func (s *Scheduler) Start(ctx context.Context) {
	if s == nil {
		return
	}

	if s.config != nil {
		go runWithTicker(ctx, time.Minute, "config sync", func(ctx context.Context) {
			s.config.SyncAll(ctx)
		})
	}
	if s.geo != nil {
		go runWithTicker(ctx, 6*time.Hour, "geo sync", func(ctx context.Context) {
			s.geo.SyncAll(ctx)
		})
	}
	if s.component != nil {
		go runWithTicker(ctx, time.Hour, "component update", func(ctx context.Context) {
			s.component.CheckUpdates(ctx)
		})
	}
}

func runWithTicker(ctx context.Context, interval time.Duration, name string, fn func(context.Context)) {
	if interval <= 0 {
		interval = time.Minute
	}

	// 启动后先跑一次，避免“等待一个周期才生效”。
	safeRun(ctx, name, fn)

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			safeRun(ctx, name, fn)
		}
	}
}

func safeRun(ctx context.Context, name string, fn func(context.Context)) {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("[tasks] %s panicked: %v", name, r)
		}
	}()
	fn(ctx)
}
