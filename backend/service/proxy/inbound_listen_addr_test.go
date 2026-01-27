package proxy

import (
	"testing"

	"vea/backend/domain"
)

func TestInboundListenAddrForEngine_AllowLAN(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		cfg  domain.ProxyConfig
		want string
	}{
		{
			name: "no_inbound_config_defaults_loopback",
			cfg:  domain.ProxyConfig{},
			want: "127.0.0.1",
		},
		{
			name: "allowlan_with_empty_listen",
			cfg:  domain.ProxyConfig{InboundConfig: &domain.InboundConfiguration{AllowLAN: true}},
			want: "0.0.0.0",
		},
		{
			name: "allowlan_overrides_loopback_listen",
			cfg:  domain.ProxyConfig{InboundConfig: &domain.InboundConfiguration{Listen: "127.0.0.1", AllowLAN: true}},
			want: "0.0.0.0",
		},
		{
			name: "allowlan_overrides_localhost_listen",
			cfg:  domain.ProxyConfig{InboundConfig: &domain.InboundConfiguration{Listen: "localhost", AllowLAN: true}},
			want: "0.0.0.0",
		},
		{
			name: "allowlan_overrides_ipv6_loopback_listen",
			cfg:  domain.ProxyConfig{InboundConfig: &domain.InboundConfiguration{Listen: "::1", AllowLAN: true}},
			want: "::",
		},
		{
			name: "allowlan_keeps_custom_listen",
			cfg:  domain.ProxyConfig{InboundConfig: &domain.InboundConfiguration{Listen: "192.168.1.10", AllowLAN: true}},
			want: "192.168.1.10",
		},
		{
			name: "explicit_listen_kept_when_allowlan_false",
			cfg:  domain.ProxyConfig{InboundConfig: &domain.InboundConfiguration{Listen: "0.0.0.0", AllowLAN: false}},
			want: "0.0.0.0",
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := inboundListenAddrForEngine(domain.EngineSingBox, tc.cfg)
			if got != tc.want {
				t.Fatalf("expected %q, got %q", tc.want, got)
			}
		})
	}
}
