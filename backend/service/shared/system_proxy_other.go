//go:build !linux && !windows && !darwin
// +build !linux,!windows,!darwin

package shared

func applySystemProxy(cfg SystemProxyConfig) (string, error) {
	// Not implemented yet.
	return "当前平台暂不支持自动切换系统代理：已保存设置，但未应用到系统", nil
}
