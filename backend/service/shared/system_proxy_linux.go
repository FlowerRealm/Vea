//go:build linux
// +build linux

package shared

import (
	"bytes"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
)

func applySystemProxy(cfg SystemProxyConfig) (string, error) {
	if !commandExists("gsettings") {
		return "gsettings 未找到：已保存设置，但无法在当前桌面环境自动切换系统代理", nil
	}

	if !cfg.Enabled {
		// GNOME: disable proxy
		if err := gsettingsSet("org.gnome.system.proxy", "mode", "'none'"); err != nil {
			return "", err
		}
		return "", nil
	}

	// Enable manual proxy
	if err := gsettingsSet("org.gnome.system.proxy", "mode", "'manual'"); err != nil {
		return "", err
	}

	// Ignore hosts
	if len(cfg.IgnoreHosts) > 0 {
		if err := gsettingsSet("org.gnome.system.proxy", "ignore-hosts", formatGVariantStringList(cfg.IgnoreHosts)); err != nil {
			return "", err
		}
	}

	// HTTP/HTTPS/SOCKS
	if err := applyGnomeProxySection("http", cfg.HTTPHost, cfg.HTTPPort); err != nil {
		return "", err
	}
	if err := applyGnomeProxySection("https", cfg.HTTPSHost, cfg.HTTPSPort); err != nil {
		return "", err
	}
	if err := applyGnomeProxySection("socks", cfg.SOCKSHost, cfg.SOCKSPort); err != nil {
		return "", err
	}

	return "", nil
}

func commandExists(name string) bool {
	_, err := exec.LookPath(name)
	return err == nil
}

func gsettingsSet(schema, key, value string) error {
	cmd := exec.Command("gsettings", "set", schema, key, value)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = "unknown error"
		}
		return fmt.Errorf("gsettings set %s %s failed: %v (%s)", schema, key, err, msg)
	}
	return nil
}

func applyGnomeProxySection(section, host string, port int) error {
	schema := "org.gnome.system.proxy." + section
	if host == "" || port <= 0 {
		// Clear
		if err := gsettingsSet(schema, "host", "''"); err != nil {
			return err
		}
		if err := gsettingsSet(schema, "port", "0"); err != nil {
			return err
		}
		return nil
	}
	if err := gsettingsSet(schema, "host", "'"+escapeGVariantString(host)+"'"); err != nil {
		return err
	}
	if err := gsettingsSet(schema, "port", strconv.Itoa(port)); err != nil {
		return err
	}
	return nil
}

func formatGVariantStringList(items []string) string {
	if len(items) == 0 {
		return "[]"
	}
	b := strings.Builder{}
	b.WriteString("[")
	for i, it := range items {
		if i > 0 {
			b.WriteString(", ")
		}
		b.WriteString("'")
		b.WriteString(escapeGVariantString(it))
		b.WriteString("'")
	}
	b.WriteString("]")
	return b.String()
}

func escapeGVariantString(s string) string {
	// gsettings uses GVariant parser; keep escaping minimal and predictable.
	// Single quotes are the only delimiter we use here.
	return strings.ReplaceAll(s, "'", "\\'")
}
