package tasks

import (
	"context"
	"log"
	"time"

	"vea/internal/service"
)

type NodeProbe struct {
	Service  *service.Service
	Interval time.Duration
}

func (t *NodeProbe) Start(ctx context.Context) {
	if t.Service == nil {
		return
	}
	interval := t.Interval
	if interval <= 0 {
		interval = 30 * time.Second
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			for _, node := range t.Service.ListNodes() {
				if _, err := t.Service.ProbeLatency(node.ID); err != nil {
					log.Printf("node probe latency failed for %s: %v", node.ID, err)
				}
			}
		}
	}
}
