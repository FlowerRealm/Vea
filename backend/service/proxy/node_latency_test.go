package proxy

import (
	"net"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"vea/backend/domain"
)

func TestSpeedMeasurer_MeasureLatency_NodeDirectTCP(t *testing.T) {
	t.Parallel()

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}

	done := make(chan struct{})
	go func() {
		defer close(done)
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			_ = conn.Close()
		}
	}()
	t.Cleanup(func() {
		_ = ln.Close()
		<-done
	})

	host, portStr, err := net.SplitHostPort(ln.Addr().String())
	if err != nil {
		t.Fatalf("split host port: %v", err)
	}
	port, err := strconv.Atoi(portStr)
	if err != nil {
		t.Fatalf("parse port: %v", err)
	}

	node := domain.Node{
		ID:       "n1",
		Name:     "n1",
		Address:  host,
		Port:     port,
		Protocol: domain.ProtocolShadowsocks,
	}
	frouter := domain.FRouter{
		ID:   "fr1",
		Name: "fr1",
		ChainProxy: domain.ChainProxySettings{
			Edges: []domain.ProxyEdge{
				{ID: "e1", From: domain.EdgeNodeLocal, To: node.ID, Enabled: true},
			},
		},
	}

	// 关键点：不依赖 core 组件/引擎进程。以前的实现会在这里因为 components=nil 直接失败。
	m := &SpeedMeasurer{}
	latency, err := m.MeasureLatency(frouter, []domain.Node{node})
	if err != nil {
		t.Fatalf("MeasureLatency() error: %v", err)
	}
	if latency <= 0 {
		t.Fatalf("expected latency > 0, got %d", latency)
	}
}

func TestSpeedMeasurer_MeasureLatency_NodeTLSHandshake(t *testing.T) {
	t.Parallel()

	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(srv.Close)

	host, portStr, err := net.SplitHostPort(srv.Listener.Addr().String())
	if err != nil {
		t.Fatalf("split host port: %v", err)
	}
	port, err := strconv.Atoi(portStr)
	if err != nil {
		t.Fatalf("parse port: %v", err)
	}

	node := domain.Node{
		ID:       "n1",
		Name:     "n1",
		Address:  host,
		Port:     port,
		Protocol: domain.ProtocolTrojan,
		TLS: &domain.NodeTLS{
			Enabled:  true,
			Insecure: true,
		},
	}
	frouter := domain.FRouter{
		ID:   "fr1",
		Name: "fr1",
		ChainProxy: domain.ChainProxySettings{
			Edges: []domain.ProxyEdge{
				{ID: "e1", From: domain.EdgeNodeLocal, To: node.ID, Enabled: true},
			},
		},
	}

	m := &SpeedMeasurer{}
	latency, err := m.MeasureLatency(frouter, []domain.Node{node})
	if err != nil {
		t.Fatalf("MeasureLatency() error: %v", err)
	}
	if latency <= 0 {
		t.Fatalf("expected latency > 0, got %d", latency)
	}
}
