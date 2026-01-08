package config

import (
	"strings"
	"testing"

	"vea/backend/domain"
)

func TestParseClashProxyToNode_ShadowsocksObfsPluginOpts_NormalizesToObfsLocal(t *testing.T) {
	t.Parallel()

	node, proxyName, err := parseClashProxyToNode(map[string]interface{}{
		"name":     " ss-obfs ",
		"type":     "ss",
		"server":   "example.com",
		"port":     443,
		"cipher":   "aes-128-gcm",
		"password": "pass",
		"plugin":   "obfs",
		"plugin-opts": map[string]interface{}{
			"mode": "tls",
			"host": "obfs.example.com",
		},
	})
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if proxyName != "ss-obfs" {
		t.Fatalf("expected proxyName=ss-obfs, got %q", proxyName)
	}
	if node.Protocol != domain.ProtocolShadowsocks {
		t.Fatalf("expected protocol=%s, got %s", domain.ProtocolShadowsocks, node.Protocol)
	}
	if node.Security == nil {
		t.Fatalf("expected security to be set")
	}
	if node.Security.Plugin != "obfs-local" {
		t.Fatalf("expected plugin=obfs-local, got %q", node.Security.Plugin)
	}
	if node.Security.PluginOpts != "obfs=tls;obfs-host=obfs.example.com" {
		t.Fatalf("expected pluginOpts normalized, got %q", node.Security.PluginOpts)
	}
}

func TestParseClashProxyToNode_Servername_EnablesTLS(t *testing.T) {
	t.Parallel()

	node, _, err := parseClashProxyToNode(map[string]interface{}{
		"name":       "n1",
		"type":       "vmess",
		"server":     "example.com",
		"port":       443,
		"uuid":       "11111111-1111-1111-1111-111111111111",
		"alterId":    0,
		"cipher":     "auto",
		"servername": "sni.example.com",
	})
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if node.TLS == nil || !node.TLS.Enabled {
		t.Fatalf("expected tls enabled, got %+v", node.TLS)
	}
	if node.TLS.Type != "tls" {
		t.Fatalf("expected tls type=tls, got %q", node.TLS.Type)
	}
	if node.TLS.ServerName != "sni.example.com" {
		t.Fatalf("expected serverName=sni.example.com, got %q", node.TLS.ServerName)
	}
}

func TestCompactClashSelectionEdges_MergesOnlyAdjacentSameTarget(t *testing.T) {
	t.Parallel()

	slotID := "slot-1"
	edges := []domain.ProxyEdge{
		{
			ID:       "e1",
			From:     domain.EdgeNodeLocal,
			To:       slotID,
			Enabled:  true,
			RuleType: domain.EdgeRuleRoute,
			RouteRule: &domain.RouteMatchRule{
				Domains: []string{"domain:a.com"},
			},
		},
		{
			ID:       "e2",
			From:     domain.EdgeNodeLocal,
			To:       slotID,
			Enabled:  true,
			RuleType: domain.EdgeRuleRoute,
			RouteRule: &domain.RouteMatchRule{
				Domains: []string{"domain:a.com", "domain:b.com"},
			},
		},
		{
			ID:       "e3",
			From:     domain.EdgeNodeLocal,
			To:       domain.EdgeNodeDirect,
			Enabled:  true,
			RuleType: domain.EdgeRuleRoute,
			RouteRule: &domain.RouteMatchRule{
				Domains: []string{"domain:c.com"},
			},
		},
		{
			ID:       "e4",
			From:     domain.EdgeNodeLocal,
			To:       slotID,
			Enabled:  true,
			RuleType: domain.EdgeRuleRoute,
			RouteRule: &domain.RouteMatchRule{
				Domains: []string{"domain:d.com"},
			},
		},
	}

	out := compactClashSelectionEdges(edges)
	if len(out) != 3 {
		t.Fatalf("expected edges=3 after compaction, got %d", len(out))
	}
	if out[0].To != slotID || out[0].RouteRule == nil {
		t.Fatalf("expected first edge to be merged slot edge, got %+v", out[0])
	}
	if got := strings.Join(out[0].RouteRule.Domains, ","); got != "domain:a.com,domain:b.com" {
		t.Fatalf("expected merged domains preserved order & deduped, got %q", got)
	}
	if out[1].To != domain.EdgeNodeDirect {
		t.Fatalf("expected middle edge to be direct, got %+v", out[1])
	}
	if out[2].To != slotID || out[2].RouteRule == nil || len(out[2].RouteRule.Domains) != 1 || out[2].RouteRule.Domains[0] != "domain:d.com" {
		t.Fatalf("expected last edge to remain separate, got %+v", out[2])
	}
}

func TestParseClashSubscription_DuplicateProxyName_AddsWarning(t *testing.T) {
	t.Parallel()

	const payload = `
proxies:
  - name: same
    type: vmess
    server: a.example.com
    port: 443
    uuid: 11111111-1111-1111-1111-111111111111
    alterId: 0
    cipher: auto
  - name: same
    type: vmess
    server: b.example.com
    port: 443
    uuid: 22222222-2222-2222-2222-222222222222
    alterId: 0
    cipher: auto
rules:
  - MATCH,same
`

	result, err := parseClashSubscription("cfg-1", payload)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	found := false
	for _, w := range result.Warnings {
		if strings.Contains(w, "duplicate proxy name") {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected duplicate proxy name warning, got warnings=%v", result.Warnings)
	}
}
