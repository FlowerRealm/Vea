package memory

import (
	"testing"

	"vea/backend/domain"
)

func TestStore_DefaultProxyConfigInboundPortIs31346(t *testing.T) {
	t.Parallel()

	store := NewStore(nil)
	store.RLock()
	cfg := store.GetProxyConfig()
	store.RUnlock()

	if cfg.InboundPort != 31346 {
		t.Fatalf("expected default proxyConfig.inboundPort=31346, got %d", cfg.InboundPort)
	}
}

func TestStore_LoadState_FallbackProxyConfigInboundPortIs31346(t *testing.T) {
	t.Parallel()

	store := NewStore(nil)
	store.LoadState(domain.ServiceState{
		ProxyConfig: domain.ProxyConfig{
			InboundMode: domain.InboundMixed,
			InboundPort: 0,
		},
	})

	store.RLock()
	cfg := store.GetProxyConfig()
	store.RUnlock()

	if cfg.InboundPort != 31346 {
		t.Fatalf("expected fallback proxyConfig.inboundPort=31346, got %d", cfg.InboundPort)
	}
}
