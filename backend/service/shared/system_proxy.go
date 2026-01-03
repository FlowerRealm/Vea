package shared

// SystemProxyConfig describes the desired system proxy state.
// It is intentionally OS-agnostic; platform-specific implementations decide
// how to apply it.
type SystemProxyConfig struct {
	Enabled bool

	HTTPHost  string
	HTTPPort  int
	HTTPSHost string
	HTTPSPort int

	SOCKSHost string
	SOCKSPort int

	IgnoreHosts []string
}

// ApplySystemProxy applies the system proxy settings for the current platform.
// It returns a non-empty message when the operation is a best-effort / partial
// apply (e.g. unsupported platform/desktop).
func ApplySystemProxy(cfg SystemProxyConfig) (string, error) {
	return applySystemProxy(cfg)
}
