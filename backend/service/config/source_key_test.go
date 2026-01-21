package config

import (
	"testing"

	"vea/backend/domain"
)

func TestBuildSubscriptionNodeIDRewriteMap_DoesNotRewriteWhenAmbiguous(t *testing.T) {
	t.Parallel()

	existing := []domain.Node{
		{ID: "old-a", Name: "dup", Address: "a.example.com", Port: 443, Protocol: domain.ProtocolVLESS},
		{ID: "old-b", Name: "dup", Address: "b.example.com", Port: 443, Protocol: domain.ProtocolVLESS},
	}
	next := []domain.Node{
		{ID: "new-a", Name: "dup", SourceKey: "dup", Address: "a.example.com", Port: 443, Protocol: domain.ProtocolVLESS},
		{ID: "new-b", Name: "dup", SourceKey: "dup", Address: "b.example.com", Port: 443, Protocol: domain.ProtocolVLESS},
	}

	if got := buildSubscriptionNodeIDRewriteMap(existing, next); got != nil {
		t.Fatalf("expected no rewrite map for ambiguous name, got %#v", got)
	}
}
