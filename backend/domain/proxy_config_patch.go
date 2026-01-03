package domain

// ApplyPatch applies a partial update to ProxyConfig.
// Zero values in patch (""/0/nil) are treated as "not set".
func (c ProxyConfig) ApplyPatch(patch ProxyConfig) ProxyConfig {
	if patch.InboundMode != "" {
		c.InboundMode = patch.InboundMode
	}
	if patch.InboundPort != 0 {
		c.InboundPort = patch.InboundPort
	}
	if patch.InboundConfig != nil {
		c.InboundConfig = patch.InboundConfig
	}
	if patch.TUNSettings != nil {
		c.TUNSettings = patch.TUNSettings
	}
	if patch.ResolvedService != nil {
		c.ResolvedService = patch.ResolvedService
	}
	if patch.DNSConfig != nil {
		c.DNSConfig = patch.DNSConfig
	}
	if patch.LogConfig != nil {
		c.LogConfig = patch.LogConfig
	}
	if patch.PerformanceConfig != nil {
		c.PerformanceConfig = patch.PerformanceConfig
	}
	if patch.XrayConfig != nil {
		c.XrayConfig = patch.XrayConfig
	}
	if patch.PreferredEngine != "" {
		c.PreferredEngine = patch.PreferredEngine
	}
	if patch.FRouterID != "" {
		c.FRouterID = patch.FRouterID
	}
	return c
}
