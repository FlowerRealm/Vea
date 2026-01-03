//go:build windows
// +build windows

package shared

import (
	"bytes"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
)

const (
	winInetOptionSettingsChanged = 39
	winInetOptionRefresh         = 37
)

var (
	wininetDLL            = syscall.NewLazyDLL("wininet.dll")
	procInternetSetOption = wininetDLL.NewProc("InternetSetOptionW")
)

func applySystemProxy(cfg SystemProxyConfig) (string, error) {
	key := `HKCU\Software\Microsoft\Windows\CurrentVersion\Internet Settings`

	if !cfg.Enabled {
		if err := regAdd(key, "ProxyEnable", "REG_DWORD", "0"); err != nil {
			return "", err
		}
		// 清空，避免 UI 显示遗留值
		_ = regAdd(key, "ProxyServer", "REG_SZ", "")
		_ = regAdd(key, "ProxyOverride", "REG_SZ", "")
		_ = notifyWinINet()
		return "", nil
	}

	server := buildWinProxyServer(cfg)
	if strings.TrimSpace(server) == "" {
		return "", fmt.Errorf("invalid system proxy config: empty proxy server")
	}

	if err := regAdd(key, "ProxyEnable", "REG_DWORD", "1"); err != nil {
		return "", err
	}
	if err := regAdd(key, "ProxyServer", "REG_SZ", server); err != nil {
		return "", err
	}
	if len(cfg.IgnoreHosts) > 0 {
		override := strings.Join(cfg.IgnoreHosts, ";")
		if err := regAdd(key, "ProxyOverride", "REG_SZ", override); err != nil {
			return "", err
		}
	}

	_ = notifyWinINet()
	return "", nil
}

func buildWinProxyServer(cfg SystemProxyConfig) string {
	parts := make([]string, 0, 3)
	if cfg.HTTPHost != "" && cfg.HTTPPort > 0 {
		parts = append(parts, "http="+cfg.HTTPHost+":"+strconv.Itoa(cfg.HTTPPort))
	}
	if cfg.HTTPSHost != "" && cfg.HTTPSPort > 0 {
		parts = append(parts, "https="+cfg.HTTPSHost+":"+strconv.Itoa(cfg.HTTPSPort))
	}
	if cfg.SOCKSHost != "" && cfg.SOCKSPort > 0 {
		parts = append(parts, "socks="+cfg.SOCKSHost+":"+strconv.Itoa(cfg.SOCKSPort))
	}
	return strings.Join(parts, ";")
}

func regAdd(key, name, typ, data string) error {
	if _, err := exec.LookPath("reg"); err != nil {
		return fmt.Errorf("reg.exe not found: %w", err)
	}
	args := []string{"add", key, "/v", name, "/t", typ, "/d", data, "/f"}
	cmd := exec.Command("reg", args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = "unknown error"
		}
		return fmt.Errorf("reg %s failed: %v (%s)", strings.Join(args, " "), err, msg)
	}
	return nil
}

func notifyWinINet() error {
	// 触发 WinINet 立刻刷新（让系统/应用尽快生效）。
	if err := internetSetOption(winInetOptionSettingsChanged); err != nil {
		return err
	}
	if err := internetSetOption(winInetOptionRefresh); err != nil {
		return err
	}
	return nil
}

func internetSetOption(option uintptr) error {
	ret, _, callErr := procInternetSetOption.Call(0, option, 0, 0)
	if ret == 0 {
		if callErr != syscall.Errno(0) {
			return callErr
		}
		return fmt.Errorf("InternetSetOptionW failed (option=%d)", option)
	}
	return nil
}
