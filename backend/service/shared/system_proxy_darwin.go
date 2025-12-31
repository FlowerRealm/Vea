//go:build darwin
// +build darwin

package shared

import (
	"bytes"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
)

func applySystemProxy(cfg SystemProxyConfig) (string, error) {
	if _, err := exec.LookPath("networksetup"); err != nil {
		return "networksetup 未找到：已保存设置，但无法自动切换系统代理", nil
	}

	services, err := listNetworkServices()
	if err != nil {
		return "", err
	}

	// Best-effort across services; stop at the first hard error to avoid partial state.
	for _, svc := range services {
		if !cfg.Enabled {
			if err := runNetworksetup("-setwebproxystate", svc, "off"); err != nil {
				return "", err
			}
			if err := runNetworksetup("-setsecurewebproxystate", svc, "off"); err != nil {
				return "", err
			}
			if err := runNetworksetup("-setsocksfirewallproxystate", svc, "off"); err != nil {
				return "", err
			}
			continue
		}

		// HTTP
		if cfg.HTTPHost != "" && cfg.HTTPPort > 0 {
			if err := runNetworksetup("-setwebproxy", svc, cfg.HTTPHost, strconv.Itoa(cfg.HTTPPort)); err != nil {
				return "", err
			}
			if err := runNetworksetup("-setwebproxystate", svc, "on"); err != nil {
				return "", err
			}
		} else {
			if err := runNetworksetup("-setwebproxystate", svc, "off"); err != nil {
				return "", err
			}
		}

		// HTTPS
		if cfg.HTTPSHost != "" && cfg.HTTPSPort > 0 {
			if err := runNetworksetup("-setsecurewebproxy", svc, cfg.HTTPSHost, strconv.Itoa(cfg.HTTPSPort)); err != nil {
				return "", err
			}
			if err := runNetworksetup("-setsecurewebproxystate", svc, "on"); err != nil {
				return "", err
			}
		} else {
			if err := runNetworksetup("-setsecurewebproxystate", svc, "off"); err != nil {
				return "", err
			}
		}

		// SOCKS
		if cfg.SOCKSHost != "" && cfg.SOCKSPort > 0 {
			if err := runNetworksetup("-setsocksfirewallproxy", svc, cfg.SOCKSHost, strconv.Itoa(cfg.SOCKSPort)); err != nil {
				return "", err
			}
			if err := runNetworksetup("-setsocksfirewallproxystate", svc, "on"); err != nil {
				return "", err
			}
		} else {
			if err := runNetworksetup("-setsocksfirewallproxystate", svc, "off"); err != nil {
				return "", err
			}
		}

		// Bypass domains (optional)
		if len(cfg.IgnoreHosts) > 0 {
			args := append([]string{"-setproxybypassdomains", svc}, cfg.IgnoreHosts...)
			if err := runNetworksetup(args...); err != nil {
				return "", err
			}
		}
	}

	return "", nil
}

func listNetworkServices() ([]string, error) {
	cmd := exec.Command("networksetup", "-listallnetworkservices")
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = "unknown error"
		}
		return nil, fmt.Errorf("networksetup -listallnetworkservices failed: %v (%s)", err, msg)
	}

	lines := strings.Split(string(out), "\n")
	services := make([]string, 0, len(lines))
	for _, line := range lines {
		s := strings.TrimSpace(line)
		if s == "" {
			continue
		}
		if strings.HasPrefix(s, "An asterisk") {
			continue
		}
		// Disabled services are prefixed with "*".
		s = strings.TrimSpace(strings.TrimPrefix(s, "*"))
		if s == "" {
			continue
		}
		services = append(services, s)
	}
	if len(services) == 0 {
		return nil, fmt.Errorf("no network services found")
	}
	return services, nil
}

func runNetworksetup(args ...string) error {
	cmd := exec.Command("networksetup", args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = "unknown error"
		}
		return fmt.Errorf("networksetup %s failed: %v (%s)", strings.Join(args, " "), err, msg)
	}
	return nil
}
