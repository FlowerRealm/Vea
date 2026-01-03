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

func TestParseMultipleLinks_ClashYAML(t *testing.T) {
	t.Parallel()

	yamlPayload := `
proxies:
  - name: test-vless
    type: vless
    server: example.com
    port: 443
    uuid: 11111111-1111-1111-1111-111111111111
    tls: true
    servername: sni.example.com
  - name: test-vmess
    type: vmess
    server: example.com
    port: 443
    uuid: 22222222-2222-2222-2222-222222222222
    alterId: 0
    cipher: auto
    network: ws
    ws-opts:
      path: /ws
      headers:
        Host: ws.example.com
  - name: test-trojan
    type: trojan
    server: example.com
    port: 443
    password: pass
    tls: true
    sni: trojan.example.com
  - name: test-ss
    type: ss
    server: example.com
    port: 8388
    cipher: aes-128-gcm
    password: ss-pass
`

	nodes, errs := ParseMultipleLinks(yamlPayload)
	if len(errs) != 0 {
		t.Fatalf("expected no errors, got %d", len(errs))
	}
	if len(nodes) != 4 {
		t.Fatalf("expected 4 nodes, got %d", len(nodes))
	}
	if nodes[0].Protocol == "" || nodes[0].Address == "" || nodes[0].Port == 0 {
		t.Fatalf("expected node fields to be set, got %+v", nodes[0])
	}
}

func TestParseMultipleLinks_FiltersSubscriptionInfoNodes(t *testing.T) {
	t.Parallel()

	links := "vless://11111111-1111-1111-1111-111111111111@127.0.0.1:1080#剩余流量:1GB\n" +
		"vless://11111111-1111-1111-1111-111111111111@example.com:443#ok\n"
	nodes, _ := ParseMultipleLinks(links)
	if len(nodes) != 1 {
		t.Fatalf("expected 1 node, got %d", len(nodes))
	}
	if nodes[0].Name != "ok" {
		t.Fatalf("expected remaining node to be ok, got %q", nodes[0].Name)
	}
}
