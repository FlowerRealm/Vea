package adapters

import (
	"runtime"
	"testing"

	"vea/backend/domain"
	"vea/backend/service/nodegroup"

	"gopkg.in/yaml.v3"
)

func TestClashAdapter_TUNDNSListenAvoidsPort53(t *testing.T) {
	frouter := domain.FRouter{
		ID:   "fr1",
		Name: "test",
		ChainProxy: domain.ChainProxySettings{
			Edges: []domain.ProxyEdge{
				{
					ID:       "e1",
					From:     domain.EdgeNodeLocal,
					To:       domain.EdgeNodeDirect,
					Priority: 0,
					Enabled:  true,
				},
			},
		},
	}

	cfg := domain.ProxyConfig{
		InboundMode:     domain.InboundTUN,
		PreferredEngine: domain.EngineClash,
		FRouterID:       frouter.ID,
		TUNSettings: &domain.TUNConfiguration{
			InterfaceName: "tun0",
			MTU:           9000,
			Address:       []string{"198.18.0.1/30"},
			AutoRoute:     true,
			StrictRoute:   true,
			Stack:         "mixed",
			DNSHijack:     true,
		},
	}

	plan, err := nodegroup.CompileProxyPlan(domain.EngineClash, cfg, frouter, nil)
	if err != nil {
		t.Fatalf("CompileProxyPlan: %v", err)
	}

	out, err := (&ClashAdapter{}).BuildConfig(plan, GeoFiles{})
	if err != nil {
		t.Fatalf("BuildConfig: %v", err)
	}

	var m map[string]interface{}
	if err := yaml.Unmarshal(out, &m); err != nil {
		t.Fatalf("yaml.Unmarshal: %v", err)
	}

	dns, _ := m["dns"].(map[string]interface{})
	if dns == nil {
		t.Fatalf("expected dns config")
	}
	if got, _ := dns["listen"].(string); got != "0.0.0.0:1053" {
		t.Fatalf("expected dns.listen=0.0.0.0:1053, got %q", got)
	}
	if got, _ := dns["fake-ip-range"].(string); got != "198.18.0.1/16" {
		t.Fatalf("expected dns.fake-ip-range=198.18.0.1/16, got %q", got)
	}
	if _, ok := dns["proxy-server-nameserver"]; !ok {
		t.Fatalf("expected dns.proxy-server-nameserver to be set")
	}

	tun, _ := m["tun"].(map[string]interface{})
	if tun == nil {
		t.Fatalf("expected tun config")
	}
	// dns-hijack 固定劫持 53（UDP+TCP）
	hijack, _ := tun["dns-hijack"].([]interface{})
	if len(hijack) == 0 {
		t.Fatalf("expected tun.dns-hijack to be set")
	}

	// strict-route：没有排除地址时不应开启（否则容易全网断开）
	if _, ok := tun["strict-route"]; ok {
		t.Fatalf("expected tun.strict-route to be omitted when route-exclude-address is empty")
	}
}

func TestClashAdapter_TUNIncludesMixedPort(t *testing.T) {
	frouter := domain.FRouter{
		ID:   "fr1",
		Name: "test",
		ChainProxy: domain.ChainProxySettings{
			Edges: []domain.ProxyEdge{
				{
					ID:       "e1",
					From:     domain.EdgeNodeLocal,
					To:       domain.EdgeNodeDirect,
					Priority: 0,
					Enabled:  true,
				},
			},
		},
	}

	cfg := domain.ProxyConfig{
		InboundMode:     domain.InboundTUN,
		InboundPort:     23456,
		PreferredEngine: domain.EngineClash,
		FRouterID:       frouter.ID,
		TUNSettings: &domain.TUNConfiguration{
			InterfaceName: "tun0",
			MTU:           9000,
			Address:       []string{"198.18.0.1/30"},
			AutoRoute:     true,
			StrictRoute:   true,
			Stack:         "mixed",
			DNSHijack:     true,
		},
	}

	plan, err := nodegroup.CompileProxyPlan(domain.EngineClash, cfg, frouter, nil)
	if err != nil {
		t.Fatalf("CompileProxyPlan: %v", err)
	}

	out, err := (&ClashAdapter{}).BuildConfig(plan, GeoFiles{})
	if err != nil {
		t.Fatalf("BuildConfig: %v", err)
	}

	var m map[string]interface{}
	if err := yaml.Unmarshal(out, &m); err != nil {
		t.Fatalf("yaml.Unmarshal: %v", err)
	}

	got, ok := m["mixed-port"]
	if !ok {
		t.Fatalf("expected mixed-port to be set in tun mode")
	}
	switch v := got.(type) {
	case int:
		if v != cfg.InboundPort {
			t.Fatalf("expected mixed-port=%d, got %v", cfg.InboundPort, v)
		}
	case int64:
		if int(v) != cfg.InboundPort {
			t.Fatalf("expected mixed-port=%d, got %v", cfg.InboundPort, v)
		}
	case uint64:
		if int(v) != cfg.InboundPort {
			t.Fatalf("expected mixed-port=%d, got %v", cfg.InboundPort, v)
		}
	default:
		t.Fatalf("expected mixed-port to be a number, got %T %v", got, got)
	}
}

func TestClashAdapter_TUNTunInet4AddressIsList(t *testing.T) {
	frouter := domain.FRouter{
		ID:   "fr1",
		Name: "test",
		ChainProxy: domain.ChainProxySettings{
			Edges: []domain.ProxyEdge{
				{
					ID:       "e1",
					From:     domain.EdgeNodeLocal,
					To:       domain.EdgeNodeDirect,
					Priority: 0,
					Enabled:  true,
				},
			},
		},
	}

	cfg := domain.ProxyConfig{
		InboundMode:     domain.InboundTUN,
		PreferredEngine: domain.EngineClash,
		FRouterID:       frouter.ID,
		TUNSettings: &domain.TUNConfiguration{
			InterfaceName: "tun0",
			MTU:           9000,
			Address:       []string{"172.19.0.1/30", "172.19.0.5/30"},
			AutoRoute:     true,
			StrictRoute:   true,
			Stack:         "mixed",
			DNSHijack:     true,
		},
	}

	plan, err := nodegroup.CompileProxyPlan(domain.EngineClash, cfg, frouter, nil)
	if err != nil {
		t.Fatalf("CompileProxyPlan: %v", err)
	}

	out, err := (&ClashAdapter{}).BuildConfig(plan, GeoFiles{})
	if err != nil {
		t.Fatalf("BuildConfig: %v", err)
	}

	var m map[string]interface{}
	if err := yaml.Unmarshal(out, &m); err != nil {
		t.Fatalf("yaml.Unmarshal: %v", err)
	}

	tun, _ := m["tun"].(map[string]interface{})
	if tun == nil {
		t.Fatalf("expected tun config")
	}
	inet4, ok := tun["inet4-address"].([]interface{})
	if !ok {
		t.Fatalf("expected tun.inet4-address to be a list, got %T %v", tun["inet4-address"], tun["inet4-address"])
	}
	if len(inet4) != 1 {
		t.Fatalf("expected tun.inet4-address length 1, got %d (%v)", len(inet4), inet4)
	}
	if got, _ := inet4[0].(string); got != "198.18.0.1/30" {
		t.Fatalf("expected tun.inet4-address[0]=%q, got %T %v", "198.18.0.1/30", inet4[0], inet4[0])
	}
}

func TestClashAdapter_TUNSetsRoutingMarkOnLinux(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("routing-mark is only meaningful on linux")
	}

	frouter := domain.FRouter{
		ID:   "fr1",
		Name: "test",
		ChainProxy: domain.ChainProxySettings{
			Edges: []domain.ProxyEdge{
				{
					ID:       "e1",
					From:     domain.EdgeNodeLocal,
					To:       domain.EdgeNodeDirect,
					Priority: 0,
					Enabled:  true,
				},
			},
		},
	}

	cfg := domain.ProxyConfig{
		InboundMode:     domain.InboundTUN,
		PreferredEngine: domain.EngineClash,
		FRouterID:       frouter.ID,
		TUNSettings: &domain.TUNConfiguration{
			InterfaceName: "tun0",
			MTU:           9000,
			Address:       []string{"198.18.0.1/30"},
			AutoRoute:     true,
			Stack:         "mixed",
			DNSHijack:     true,
		},
	}

	plan, err := nodegroup.CompileProxyPlan(domain.EngineClash, cfg, frouter, nil)
	if err != nil {
		t.Fatalf("CompileProxyPlan: %v", err)
	}

	out, err := (&ClashAdapter{}).BuildConfig(plan, GeoFiles{})
	if err != nil {
		t.Fatalf("BuildConfig: %v", err)
	}

	var m map[string]interface{}
	if err := yaml.Unmarshal(out, &m); err != nil {
		t.Fatalf("yaml.Unmarshal: %v", err)
	}

	if got, ok := m["routing-mark"]; !ok {
		t.Fatalf("expected routing-mark to be set on linux")
	} else {
		switch v := got.(type) {
		case int:
			if v != 6666 {
				t.Fatalf("expected routing-mark=6666, got %v", v)
			}
		case int64:
			if v != 6666 {
				t.Fatalf("expected routing-mark=6666, got %v", v)
			}
		case uint64:
			if v != 6666 {
				t.Fatalf("expected routing-mark=6666, got %v", v)
			}
		default:
			t.Fatalf("expected routing-mark to be a number, got %T %v", got, got)
		}
	}
}

func TestClashAdapter_TUNDoesNotEnableAutoRedirectByDefaultOnLinux(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("auto-redirect is linux-specific")
	}

	frouter := domain.FRouter{
		ID:   "fr1",
		Name: "test",
		ChainProxy: domain.ChainProxySettings{
			Edges: []domain.ProxyEdge{
				{
					ID:       "e1",
					From:     domain.EdgeNodeLocal,
					To:       domain.EdgeNodeDirect,
					Priority: 0,
					Enabled:  true,
				},
			},
		},
	}

	cfg := domain.ProxyConfig{
		InboundMode:     domain.InboundTUN,
		PreferredEngine: domain.EngineClash,
		FRouterID:       frouter.ID,
		TUNSettings: &domain.TUNConfiguration{
			InterfaceName: "tun0",
			MTU:           9000,
			Address:       []string{"198.18.0.1/30"},
			AutoRoute:     true,
			Stack:         "mixed",
			DNSHijack:     true,
		},
	}

	plan, err := nodegroup.CompileProxyPlan(domain.EngineClash, cfg, frouter, nil)
	if err != nil {
		t.Fatalf("CompileProxyPlan: %v", err)
	}

	out, err := (&ClashAdapter{}).BuildConfig(plan, GeoFiles{})
	if err != nil {
		t.Fatalf("BuildConfig: %v", err)
	}

	var m map[string]interface{}
	if err := yaml.Unmarshal(out, &m); err != nil {
		t.Fatalf("yaml.Unmarshal: %v", err)
	}

	tun, _ := m["tun"].(map[string]interface{})
	if tun == nil {
		t.Fatalf("expected tun config")
	}

	if _, ok := tun["auto-redirect"]; ok {
		t.Fatalf("expected tun.auto-redirect to be omitted unless explicitly enabled")
	}
}

func TestClashAdapter_TUNRespectsAutoRedirectOnLinux(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("auto-redirect is linux-specific")
	}

	frouter := domain.FRouter{
		ID:   "fr1",
		Name: "test",
		ChainProxy: domain.ChainProxySettings{
			Edges: []domain.ProxyEdge{
				{
					ID:       "e1",
					From:     domain.EdgeNodeLocal,
					To:       domain.EdgeNodeDirect,
					Priority: 0,
					Enabled:  true,
				},
			},
		},
	}

	cfg := domain.ProxyConfig{
		InboundMode:     domain.InboundTUN,
		PreferredEngine: domain.EngineClash,
		FRouterID:       frouter.ID,
		TUNSettings: &domain.TUNConfiguration{
			InterfaceName: "tun0",
			MTU:           9000,
			Address:       []string{"198.18.0.1/30"},
			AutoRoute:     true,
			AutoRedirect:  true,
			Stack:         "mixed",
			DNSHijack:     true,
		},
	}

	plan, err := nodegroup.CompileProxyPlan(domain.EngineClash, cfg, frouter, nil)
	if err != nil {
		t.Fatalf("CompileProxyPlan: %v", err)
	}

	out, err := (&ClashAdapter{}).BuildConfig(plan, GeoFiles{})
	if err != nil {
		t.Fatalf("BuildConfig: %v", err)
	}

	var m map[string]interface{}
	if err := yaml.Unmarshal(out, &m); err != nil {
		t.Fatalf("yaml.Unmarshal: %v", err)
	}

	tun, _ := m["tun"].(map[string]interface{})
	if tun == nil {
		t.Fatalf("expected tun config")
	}

	got, ok := tun["auto-redirect"].(bool)
	if !ok {
		t.Fatalf("expected tun.auto-redirect to be a bool, got %T %v", tun["auto-redirect"], tun["auto-redirect"])
	}
	if !got {
		t.Fatalf("expected tun.auto-redirect=true when enabled in settings")
	}
}

func TestClashAdapter_TUNDefaultsMTU1500OnLinux(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("linux-only defaults")
	}

	frouter := domain.FRouter{
		ID:   "fr1",
		Name: "test",
		ChainProxy: domain.ChainProxySettings{
			Edges: []domain.ProxyEdge{
				{
					ID:       "e1",
					From:     domain.EdgeNodeLocal,
					To:       domain.EdgeNodeDirect,
					Priority: 0,
					Enabled:  true,
				},
			},
		},
	}

	cfg := domain.ProxyConfig{
		InboundMode:     domain.InboundTUN,
		PreferredEngine: domain.EngineClash,
		FRouterID:       frouter.ID,
		TUNSettings: &domain.TUNConfiguration{
			InterfaceName: "tun0",
			AutoRoute:     true,
			Stack:         "mixed",
			DNSHijack:     true,
		},
	}

	plan, err := nodegroup.CompileProxyPlan(domain.EngineClash, cfg, frouter, nil)
	if err != nil {
		t.Fatalf("CompileProxyPlan: %v", err)
	}

	out, err := (&ClashAdapter{}).BuildConfig(plan, GeoFiles{})
	if err != nil {
		t.Fatalf("BuildConfig: %v", err)
	}

	var m map[string]interface{}
	if err := yaml.Unmarshal(out, &m); err != nil {
		t.Fatalf("yaml.Unmarshal: %v", err)
	}

	tun, _ := m["tun"].(map[string]interface{})
	if tun == nil {
		t.Fatalf("expected tun config")
	}

	got, ok := tun["mtu"]
	if !ok {
		t.Fatalf("expected tun.mtu to be set on linux by default")
	}
	switch v := got.(type) {
	case int:
		if v != 1500 {
			t.Fatalf("expected tun.mtu=1500 on linux by default, got %d", v)
		}
	case int64:
		if v != 1500 {
			t.Fatalf("expected tun.mtu=1500 on linux by default, got %d", v)
		}
	case uint64:
		if v != 1500 {
			t.Fatalf("expected tun.mtu=1500 on linux by default, got %d", v)
		}
	default:
		t.Fatalf("expected tun.mtu to be a number, got %T %v", got, got)
	}
}

func TestClashAdapter_TUNEnablesSnifferByDefault(t *testing.T) {
	frouter := domain.FRouter{
		ID:   "fr1",
		Name: "test",
		ChainProxy: domain.ChainProxySettings{
			Edges: []domain.ProxyEdge{
				{
					ID:       "e1",
					From:     domain.EdgeNodeLocal,
					To:       domain.EdgeNodeDirect,
					Priority: 0,
					Enabled:  true,
				},
			},
		},
	}

	cfg := domain.ProxyConfig{
		InboundMode:     domain.InboundTUN,
		PreferredEngine: domain.EngineClash,
		FRouterID:       frouter.ID,
		TUNSettings: &domain.TUNConfiguration{
			InterfaceName: "tun0",
			MTU:           1500,
			Address:       []string{"198.18.0.1/30"},
			AutoRoute:     true,
			Stack:         "mixed",
			DNSHijack:     true,
		},
	}

	plan, err := nodegroup.CompileProxyPlan(domain.EngineClash, cfg, frouter, nil)
	if err != nil {
		t.Fatalf("CompileProxyPlan: %v", err)
	}

	out, err := (&ClashAdapter{}).BuildConfig(plan, GeoFiles{})
	if err != nil {
		t.Fatalf("BuildConfig: %v", err)
	}

	var m map[string]interface{}
	if err := yaml.Unmarshal(out, &m); err != nil {
		t.Fatalf("yaml.Unmarshal: %v", err)
	}

	sniffer, _ := m["sniffer"].(map[string]interface{})
	if sniffer == nil {
		t.Fatalf("expected sniffer config in tun mode")
	}
	if got, ok := sniffer["enable"].(bool); !ok || !got {
		t.Fatalf("expected sniffer.enable=true, got %T %v", sniffer["enable"], sniffer["enable"])
	}
}

func TestClashAdapter_TUNBlocksQUICOnUDP443(t *testing.T) {
	frouter := domain.FRouter{
		ID:   "fr1",
		Name: "test",
		ChainProxy: domain.ChainProxySettings{
			Edges: []domain.ProxyEdge{
				{
					ID:       "e1",
					From:     domain.EdgeNodeLocal,
					To:       domain.EdgeNodeDirect,
					Priority: 0,
					Enabled:  true,
				},
			},
		},
	}

	cfg := domain.ProxyConfig{
		InboundMode:     domain.InboundTUN,
		PreferredEngine: domain.EngineClash,
		FRouterID:       frouter.ID,
		TUNSettings: &domain.TUNConfiguration{
			InterfaceName: "tun0",
			MTU:           1500,
			Address:       []string{"198.18.0.1/30"},
			AutoRoute:     true,
			Stack:         "mixed",
			DNSHijack:     true,
		},
	}

	plan, err := nodegroup.CompileProxyPlan(domain.EngineClash, cfg, frouter, nil)
	if err != nil {
		t.Fatalf("CompileProxyPlan: %v", err)
	}

	out, err := (&ClashAdapter{}).BuildConfig(plan, GeoFiles{})
	if err != nil {
		t.Fatalf("BuildConfig: %v", err)
	}

	var m map[string]interface{}
	if err := yaml.Unmarshal(out, &m); err != nil {
		t.Fatalf("yaml.Unmarshal: %v", err)
	}

	rules, _ := m["rules"].([]interface{})
	if len(rules) < 4 {
		t.Fatalf("expected at least 4 rules in tun mode, got %d", len(rules))
	}
	got, ok := rules[3].(string)
	if !ok {
		t.Fatalf("expected rules[3] to be string, got %T %v", rules[3], rules[3])
	}
	if got != "AND,((NETWORK,UDP),(DST-PORT,443)),REJECT" {
		t.Fatalf("expected rules[3] to be QUIC block rule, got %q", got)
	}
}
