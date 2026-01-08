package node

import (
	"testing"

	"vea/backend/domain"
)

func TestParseShareLink_VLESS(t *testing.T) {
	t.Parallel()

	link := "vless://11111111-1111-1111-1111-111111111111@example.com:8443?type=tcp&security=tls&sni=example.com#node-1"
	parsed, err := ParseShareLink(link)
	if err != nil {
		t.Fatalf("parse share link: %v", err)
	}
	if parsed.Protocol != domain.ProtocolVLESS {
		t.Fatalf("expected protocol %q, got %q", domain.ProtocolVLESS, parsed.Protocol)
	}
	if parsed.Address != "example.com" {
		t.Fatalf("expected address %q, got %q", "example.com", parsed.Address)
	}
	if parsed.Port != 8443 {
		t.Fatalf("expected port %d, got %d", 8443, parsed.Port)
	}
	if parsed.Security == nil || parsed.Security.UUID != "11111111-1111-1111-1111-111111111111" {
		t.Fatalf("expected uuid to be set, got %+v", parsed.Security)
	}
	if parsed.Transport == nil || parsed.Transport.Type != "tcp" {
		t.Fatalf("expected transport type tcp, got %+v", parsed.Transport)
	}
	if parsed.TLS == nil || !parsed.TLS.Enabled || parsed.TLS.Type != "tls" || parsed.TLS.ServerName != "example.com" {
		t.Fatalf("expected tls enabled with sni, got %+v", parsed.TLS)
	}
	if parsed.Name != "node-1" {
		t.Fatalf("expected name %q, got %q", "node-1", parsed.Name)
	}
}

func TestParseMultipleLinks_CollectsErrorsAndKeepsValidNodes(t *testing.T) {
	t.Parallel()

	links := "vless://%zz\nvless://11111111-1111-1111-1111-111111111111@example.com:443#ok\n"
	nodes, errs := ParseMultipleLinks(links)
	if len(nodes) != 1 {
		t.Fatalf("expected 1 node, got %d", len(nodes))
	}
	if nodes[0].Protocol != domain.ProtocolVLESS {
		t.Fatalf("expected protocol %q, got %q", domain.ProtocolVLESS, nodes[0].Protocol)
	}
	if len(errs) == 0 {
		t.Fatalf("expected parse errors to be collected")
	}
}

func TestParseMultipleLinks_FiltersSubscriptionInfoNodes(t *testing.T) {
	t.Parallel()

	links := "" +
		"vless://11111111-1111-1111-1111-111111111111@127.0.0.1:1080#version\n" +
		"vless://11111111-1111-1111-1111-111111111111@example.com:443#ok\n"
	nodes, errs := ParseMultipleLinks(links)
	if len(errs) != 0 {
		t.Fatalf("expected no errors, got %d", len(errs))
	}
	if len(nodes) != 1 {
		t.Fatalf("expected 1 node after filtering, got %d", len(nodes))
	}
	if nodes[0].Address != "example.com" {
		t.Fatalf("expected node address %q, got %q", "example.com", nodes[0].Address)
	}
}

func TestParseShareLink_Shadowsocks_ObfsAliasNormalizesToObfsLocal(t *testing.T) {
	t.Parallel()

	link := "ss://YWVzLTEyOC1nY206cGFzcw==@example.com:8388/?plugin=obfs%3Bmode%3Dtls%3Bhost%3Dobfs.example.com#ss-obfs"
	parsed, err := ParseShareLink(link)
	if err != nil {
		t.Fatalf("parse share link: %v", err)
	}
	if parsed.Protocol != domain.ProtocolShadowsocks {
		t.Fatalf("expected protocol %q, got %q", domain.ProtocolShadowsocks, parsed.Protocol)
	}
	if parsed.Security == nil {
		t.Fatalf("expected security to be set")
	}
	if parsed.Security.Plugin != "obfs-local" {
		t.Fatalf("expected plugin=obfs-local, got %q", parsed.Security.Plugin)
	}
	if parsed.Security.PluginOpts != "obfs=tls;obfs-host=obfs.example.com" {
		t.Fatalf("expected pluginOpts normalized, got %q", parsed.Security.PluginOpts)
	}
}
