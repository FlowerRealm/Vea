package config

import (
	"testing"

	"vea/backend/domain"
)

func TestBuildSubscriptionNodeIDRewriteMap_RewritesByStableKey_WhenNameAmbiguous(t *testing.T) {
	t.Parallel()

	existing := []domain.Node{
		{ID: "old-a", Name: "dup", Address: "a.example.com", Port: 443, Protocol: domain.ProtocolVLESS},
		{ID: "old-b", Name: "dup", Address: "b.example.com", Port: 443, Protocol: domain.ProtocolVLESS},
	}
	next := []domain.Node{
		{ID: "new-a", Name: "dup", SourceKey: "dup", Address: "a.example.com", Port: 443, Protocol: domain.ProtocolVLESS},
		{ID: "new-b", Name: "dup", SourceKey: "dup", Address: "b.example.com", Port: 443, Protocol: domain.ProtocolVLESS},
	}

	got := buildSubscriptionNodeIDRewriteMap(existing, next)
	if got == nil {
		t.Fatalf("expected rewrite map, got nil")
	}
	if got["old-a"] != "new-a" || got["old-b"] != "new-b" {
		t.Fatalf("unexpected rewrite map: %#v", got)
	}
}

func TestBuildSubscriptionNodeIDRewriteMap_DoesNotRewriteWhenStableKeyAmbiguous(t *testing.T) {
	t.Parallel()

	existing := []domain.Node{
		{ID: "old-a", Name: "dup", Address: "example.com", Port: 443, Protocol: domain.ProtocolVLESS},
		{ID: "old-b", Name: "dup", Address: "example.com", Port: 443, Protocol: domain.ProtocolVLESS},
	}
	next := []domain.Node{
		{ID: "new-a", Name: "dup", SourceKey: "dup", Address: "example.com", Port: 443, Protocol: domain.ProtocolVLESS},
		{ID: "new-b", Name: "dup", SourceKey: "dup", Address: "example.com", Port: 443, Protocol: domain.ProtocolVLESS},
	}

	if got := buildSubscriptionNodeIDRewriteMap(existing, next); got != nil {
		t.Fatalf("expected no rewrite map, got %#v", got)
	}
}
