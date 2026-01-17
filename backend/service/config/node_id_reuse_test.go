package config

import (
	"testing"

	"vea/backend/domain"
)

func TestReuseNodeIDs_IdentityConflict_ResolvesByName(t *testing.T) {
	t.Parallel()

	existing := []domain.Node{
		{
			ID:       "id-a",
			Name:     "A",
			Protocol: domain.ProtocolVLESS,
			Address:  "example.com",
			Port:     443,
			Security: &domain.NodeSecurity{UUID: "11111111-1111-1111-1111-111111111111"},
			Transport: &domain.NodeTransport{
				Type: "ws",
				Host: "h1.example.com",
				Path: "/path-1",
			},
			TLS: &domain.NodeTLS{
				Enabled:    true,
				Type:       "tls",
				ServerName: "sni-1.example.com",
			},
		},
		{
			ID:       "id-b",
			Name:     "B",
			Protocol: domain.ProtocolVLESS,
			Address:  "example.com",
			Port:     443,
			Security: &domain.NodeSecurity{UUID: "11111111-1111-1111-1111-111111111111"},
			Transport: &domain.NodeTransport{
				Type: "ws",
				Host: "h2.example.com",
				Path: "/path-2",
			},
			TLS: &domain.NodeTLS{
				Enabled:    true,
				Type:       "tls",
				ServerName: "sni-2.example.com",
			},
		},
	}

	index := buildExistingNodeIDIndex(existing)

	parsed := []domain.Node{
		{
			ID:       "parsed-a",
			Name:     "A",
			Protocol: domain.ProtocolVLESS,
			Address:  "example.com",
			Port:     443,
			Security: &domain.NodeSecurity{UUID: "11111111-1111-1111-1111-111111111111"},
			// transport/tls 细节变化使 fingerprintKey 不再匹配任一历史节点，identityKey 也因冲突而不可用。
			Transport: &domain.NodeTransport{
				Type: "ws",
				Host: "h1-new.example.com",
				Path: "/path-1-new",
			},
			TLS: &domain.NodeTLS{
				Enabled:    true,
				Type:       "tls",
				ServerName: "sni-1-new.example.com",
			},
		},
	}

	got, idMap := reuseNodeIDs(index, parsed)
	if len(got) != 1 {
		t.Fatalf("expected 1 node, got %d", len(got))
	}
	if got[0].ID != "id-a" {
		t.Fatalf("expected reused id %q, got %q", "id-a", got[0].ID)
	}
	if idMap == nil || idMap["parsed-a"] != "id-a" {
		t.Fatalf("expected idMap[parsed-a]=%q, got %#v", "id-a", idMap)
	}
}
