package service

import (
	"bufio"
	"errors"
	"fmt"
	"os/exec"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"

	"vea/backend/domain"
)

var (
	defaultProxyIgnoreHosts = []string{
		"localhost",
		"127.0.0.1",
		"::1",
		"*.local",
		"10.0.0.0/8",
		"172.16.0.0/12",
		"192.168.0.0/16",
	}

	ErrProxyUnsupported    = errors.New("system proxy configuration not supported on this platform")
	ErrProxyXrayNotRunning = errors.New("system proxy requires Xray to be running")
)

func (s *Service) SystemProxySettings() domain.SystemProxySettings {
	return s.store.GetSystemProxySettings()
}

func (s *Service) UpdateSystemProxySettings(settings domain.SystemProxySettings) (domain.SystemProxySettings, error) {
	normalized := normalizeSystemProxySettings(settings)
	updated, err := s.store.UpdateSystemProxySettings(func(current domain.SystemProxySettings) (domain.SystemProxySettings, error) {
		normalized.UpdatedAt = time.Now()
		return normalized, nil
	})
	if err != nil {
		return domain.SystemProxySettings{}, err
	}
	if err := s.applySystemProxy(updated); err != nil {
		return updated, err
	}
	return updated, nil
}

func normalizeSystemProxySettings(settings domain.SystemProxySettings) domain.SystemProxySettings {
	seen := make(map[string]struct{})
	hosts := make([]string, 0, len(settings.IgnoreHosts)+len(defaultProxyIgnoreHosts))
	for _, host := range settings.IgnoreHosts {
		h := strings.TrimSpace(host)
		if h == "" {
			continue
		}
		h = strings.ToLower(h)
		if _, ok := seen[h]; ok {
			continue
		}
		seen[h] = struct{}{}
		hosts = append(hosts, h)
	}
	for _, def := range defaultProxyIgnoreHosts {
		h := strings.ToLower(def)
		if _, ok := seen[h]; ok {
			continue
		}
		seen[h] = struct{}{}
		hosts = append(hosts, h)
	}
	sort.Strings(hosts)
	settings.IgnoreHosts = hosts
	return settings
}

func (s *Service) applySystemProxy(settings domain.SystemProxySettings) error {
	apply := settings.Enabled && s.IsXrayEnabled()
	if settings.Enabled && !apply {
		return ErrProxyXrayNotRunning
	}
	return configureSystemProxy(apply, xrayDefaultListenAddr, xrayDefaultInboundPort, settings.IgnoreHosts)
}

func configureSystemProxy(enable bool, host string, port int, ignore []string) error {
	switch runtime.GOOS {
	case "linux":
		return configureLinuxProxy(enable, host, port, ignore)
	case "windows":
		return configureWindowsProxy(enable, host, port, ignore)
	case "darwin":
		return configureMacProxy(enable, host, port, ignore)
	default:
		if enable {
			return ErrProxyUnsupported
		}
		return nil
	}
}

func configureLinuxProxy(enable bool, host string, port int, ignore []string) error {
	if _, err := exec.LookPath("gsettings"); err != nil {
		if enable {
			return ErrProxyUnsupported
		}
		return nil
	}
	mode := "none"
	if enable {
		mode = "manual"
	}
	if err := runCommand("gsettings", "set", "org.gnome.system.proxy", "mode", mode); err != nil {
		return err
	}
	if !enable {
		return nil
	}
	portStr := strconv.Itoa(port)
	if err := runCommand("gsettings", "set", "org.gnome.system.proxy.socks", "host", host); err != nil {
		return err
	}
	if err := runCommand("gsettings", "set", "org.gnome.system.proxy.socks", "port", portStr); err != nil {
		return err
	}
	_ = runCommand("gsettings", "set", "org.gnome.system.proxy.http", "host", host)
	_ = runCommand("gsettings", "set", "org.gnome.system.proxy.http", "port", portStr)
	_ = runCommand("gsettings", "set", "org.gnome.system.proxy.https", "host", host)
	_ = runCommand("gsettings", "set", "org.gnome.system.proxy.https", "port", portStr)
	if len(ignore) > 0 {
		quoted := make([]string, 0, len(ignore))
		for _, item := range ignore {
			quoted = append(quoted, fmt.Sprintf("'%s'", item))
		}
		list := fmt.Sprintf("[%s]", strings.Join(quoted, ", "))
		if err := runCommand("gsettings", "set", "org.gnome.system.proxy", "ignore-hosts", list); err != nil {
			return err
		}
	}
	return nil
}

func configureWindowsProxy(enable bool, host string, port int, ignore []string) error {
	if _, err := exec.LookPath("reg"); err != nil {
		if enable {
			return ErrProxyUnsupported
		}
		return nil
	}
	key := `HKCU\\Software\\Microsoft\\Windows\\CurrentVersion\\Internet Settings`
	enableValue := "0"
	if enable {
		enableValue = "1"
	}
	if err := runCommand("reg", "add", key, "/v", "ProxyEnable", "/t", "REG_DWORD", "/d", enableValue, "/f"); err != nil {
		return err
	}
	server := ""
	override := ""
	if enable {
		server = fmt.Sprintf("%s:%d", host, port)
		overrideList := append([]string{}, ignore...)
		overrideList = append(overrideList, "<local>")
		override = strings.Join(overrideList, ";")
	}
	if err := runCommand("reg", "add", key, "/v", "ProxyServer", "/t", "REG_SZ", "/d", server, "/f"); err != nil {
		return err
	}
	if err := runCommand("reg", "add", key, "/v", "ProxyOverride", "/t", "REG_SZ", "/d", override, "/f"); err != nil {
		return err
	}
	if _, err := exec.LookPath("RunDll32.exe"); err == nil {
		_ = runCommand("RunDll32.exe", "user32.dll,UpdatePerUserSystemParameters")
	}
	if enable {
		if _, err := exec.LookPath("netsh"); err == nil {
			netshProxy := fmt.Sprintf("%s:%d", host, port)
			_ = runCommand("netsh", "winhttp", "set", "proxy", netshProxy)
		}
	} else {
		if _, err := exec.LookPath("netsh"); err == nil {
			_ = runCommand("netsh", "winhttp", "reset", "proxy")
		}
	}
	return nil
}

func configureMacProxy(enable bool, host string, port int, ignore []string) error {
	if _, err := exec.LookPath("networksetup"); err != nil {
		if enable {
			return ErrProxyUnsupported
		}
		return nil
	}
	services, err := listMacNetworkServices()
	if err != nil {
		return err
	}
	portStr := strconv.Itoa(port)
	for _, svc := range services {
		if enable {
			_ = runCommand("networksetup", "-setwebproxy", svc, host, portStr)
			_ = runCommand("networksetup", "-setsecurewebproxy", svc, host, portStr)
			_ = runCommand("networksetup", "-setsocksfirewallproxy", svc, host, portStr)
			_ = runCommand("networksetup", "-setwebproxystate", svc, "on")
			_ = runCommand("networksetup", "-setsecurewebproxystate", svc, "on")
			_ = runCommand("networksetup", "-setsocksfirewallproxystate", svc, "on")
			if len(ignore) > 0 {
				args := append([]string{"-setproxybypassdomains", svc}, ignore...)
				_ = runCommand("networksetup", args...)
			}
		} else {
			_ = runCommand("networksetup", "-setwebproxystate", svc, "off")
			_ = runCommand("networksetup", "-setsecurewebproxystate", svc, "off")
			_ = runCommand("networksetup", "-setsocksfirewallproxystate", svc, "off")
		}
	}
	return nil
}

func listMacNetworkServices() ([]string, error) {
	cmd := exec.Command("networksetup", "-listallnetworkservices")
	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}
	scanner := bufio.NewScanner(strings.NewReader(string(out)))
	services := make([]string, 0)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "*") {
			continue
		}
		services = append(services, line)
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return services, nil
}

func runCommand(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	return cmd.Run()
}
