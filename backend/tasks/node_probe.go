package tasks

import (
	"context"
	"log"
	"sync"
	"time"

	"vea/backend/service"
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
			t.probeAllNodesConcurrent(ctx)
		}
	}
}

// probeAllNodesConcurrent 并发测速所有节点，最多同时测5个节点
func (t *NodeProbe) probeAllNodesConcurrent(ctx context.Context) {
	nodes := t.Service.ListNodes()
	if len(nodes) == 0 {
		return
	}

	// 使用semaphore限制并发数为5
	const maxConcurrent = 5
	sem := make(chan struct{}, maxConcurrent)
	var wg sync.WaitGroup

	for _, node := range nodes {
		wg.Add(1)
		go func(nodeID string) {
			defer wg.Done()

			// 获取semaphore
			select {
			case sem <- struct{}{}:
				defer func() { <-sem }()
			case <-ctx.Done():
				return
			}

			// 执行测速
			if _, err := t.Service.ProbeLatency(nodeID); err != nil {
				log.Printf("node probe latency failed for %s: %v", nodeID, err)
			}
		}(node.ID)
	}

	wg.Wait()
}
