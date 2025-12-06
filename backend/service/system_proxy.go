package service

import (
	"bufio"
	"errors"
	"fmt"
	"log"
	"os"
	"os/exec"
	"os/user"
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
	ErrProxyXrayNotRunning = errors.New("system proxy requires an active proxy (Xray or sing-box)")
)

func (s *Service) SystemProxySettings() domain.SystemProxySettings {
	return s.store.GetSystemProxySettings()
}

func (s *Service) UpdateSystemProxySettings(settings domain.SystemProxySettings) (domain.SystemProxySettings, error) {
	normalized := normalizeSystemProxySettings(settings)
	log.Printf("[SystemProxy] UpdateSystemProxySettings: enabled=%v, xrayEnabled=%v", normalized.Enabled, s.IsXrayEnabled())

	// 先保存设置
	updated, err := s.store.UpdateSystemProxySettings(func(current domain.SystemProxySettings) (domain.SystemProxySettings, error) {
		normalized.UpdatedAt = time.Now()
		return normalized, nil
	})
	if err != nil {
		return domain.SystemProxySettings{}, err
	}

	// 尝试应用系统代理配置
	if err := s.applySystemProxy(updated); err != nil {
		log.Printf("[SystemProxy] applySystemProxy failed: %v", err)
		return updated, err
	}

	log.Printf("[SystemProxy] applySystemProxy success")
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
	host, port, proxyRunning, xrayRunning := s.resolveProxyEndpoint()

	apply := settings.Enabled
	if apply && port <= 0 {
		return ErrProxyXrayNotRunning
	}

	log.Printf("[SystemProxy] apply=%v host=%s port=%d proxyRunning=%v xrayRunning=%v", apply, host, port, proxyRunning, xrayRunning)
	return configureSystemProxy(apply, host, port, settings.IgnoreHosts)
}

// resolveProxyEndpoint 统一选择系统代理应指向的监听端口
func (s *Service) resolveProxyEndpoint() (host string, port int, proxyRunning bool, xrayRunning bool) {
	host = xrayDefaultListenAddr

	// 1) 优先使用新代理流程（ProxyProfile）
	s.proxyMu.Lock()
	proxyRunning = s.proxyCmd != nil && s.proxyCmd.Process != nil
	activeProfileID := s.activeProfile
	s.proxyMu.Unlock()
	if proxyRunning && activeProfileID != "" {
		if profile, err := s.store.GetProxyProfile(activeProfileID); err == nil && profile.InboundPort > 0 {
			return host, profile.InboundPort, proxyRunning, s.IsXrayEnabled()
		}
	}

	// 2) 退回旧的 Xray 运行时（兼容旧 UI）
	s.xrayMu.Lock()
	xrayRunning = s.xrayEnabled
	if s.xrayRuntime.InboundPort > 0 {
		port = s.xrayRuntime.InboundPort
	}
	s.xrayMu.Unlock()
	if xrayRunning && port > 0 {
		return host, port, proxyRunning, xrayRunning
	}

	// 3) 前端配置的端口
	if settings := s.GetFrontendSettings(); len(settings) > 0 {
		if proxyPort, ok := settings["proxy.port"].(float64); ok && proxyPort > 0 {
			return host, int(proxyPort), proxyRunning, xrayRunning
		}
		if proxyPort, ok := settings["proxy.port"].(int); ok && proxyPort > 0 {
			return host, proxyPort, proxyRunning, xrayRunning
		}
	}

	// 4) 默认端口
	return host, xrayDefaultInboundPort, proxyRunning, xrayRunning
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
	targetUser := resolveTargetUser()
	env := buildUserEnv(targetUser)
	ensureDBUSSession(env)

	log.Printf("[SystemProxy] Applying via user=%s env[DBUS]=%s", targetUser, env["DBUS_SESSION_BUS_ADDRESS"])

	gsettingsPath, err := findGSettings()
	if err != nil {
		log.Printf("[SystemProxy] gsettings not found: %v", err)
		if enable {
			return ErrProxyUnsupported
		}
		return nil
	}

	mode := "none"
	if enable {
		mode = "manual"
	}
	log.Printf("[SystemProxy] Setting proxy mode to: %s", mode)
	if err := runCommandAsUser(targetUser, env, gsettingsPath, "set", "org.gnome.system.proxy", "mode", mode); err != nil {
		log.Printf("[SystemProxy] Failed to set proxy mode: %v", err)
		return err
	}
	if !enable {
		return nil
	}
	portStr := strconv.Itoa(port)
	log.Printf("[SystemProxy] Configuring SOCKS proxy: %s:%s", host, portStr)
	if err := runCommandAsUser(targetUser, env, gsettingsPath, "set", "org.gnome.system.proxy.socks", "host", host); err != nil {
		log.Printf("[SystemProxy] Failed to set SOCKS host: %v", err)
		return err
	}
	if err := runCommandAsUser(targetUser, env, gsettingsPath, "set", "org.gnome.system.proxy.socks", "port", portStr); err != nil {
		log.Printf("[SystemProxy] Failed to set SOCKS port: %v", err)
		return err
	}
	if err := runCommandAsUser(targetUser, env, gsettingsPath, "set", "org.gnome.system.proxy.http", "host", host); err != nil {
		log.Printf("[SystemProxy] Failed to set HTTP host: %v", err)
		return err
	}
	if err := runCommandAsUser(targetUser, env, gsettingsPath, "set", "org.gnome.system.proxy.http", "port", portStr); err != nil {
		log.Printf("[SystemProxy] Failed to set HTTP port: %v", err)
		return err
	}
	if err := runCommandAsUser(targetUser, env, gsettingsPath, "set", "org.gnome.system.proxy.https", "host", host); err != nil {
		log.Printf("[SystemProxy] Failed to set HTTPS host: %v", err)
		return err
	}
	if err := runCommandAsUser(targetUser, env, gsettingsPath, "set", "org.gnome.system.proxy.https", "port", portStr); err != nil {
		log.Printf("[SystemProxy] Failed to set HTTPS port: %v", err)
		return err
	}
	if len(ignore) > 0 {
		quoted := make([]string, 0, len(ignore))
		for _, item := range ignore {
			quoted = append(quoted, fmt.Sprintf("'%s'", item))
		}
		list := fmt.Sprintf("[%s]", strings.Join(quoted, ", "))
		if err := runCommandAsUser(targetUser, env, gsettingsPath, "set", "org.gnome.system.proxy", "ignore-hosts", list); err != nil {
			log.Printf("[SystemProxy] Failed to set ignore-hosts: %v", err)
			return err
		}
	}

	// 读取回写结果，若未生效则报错提醒
	if err := verifyGSettings(targetUser, env, gsettingsPath, "org.gnome.system.proxy", "mode", mode); err != nil {
		if err := fallbackDconfWrite(env, "/org/gnome/system/proxy/mode", fmt.Sprintf("'%s'", mode)); err != nil {
			return err
		}
	}
	if err := verifyGSettings(targetUser, env, gsettingsPath, "org.gnome.system.proxy.socks", "host", host); err != nil {
		if err := fallbackDconfWrite(env, "/org/gnome/system/proxy/socks/host", fmt.Sprintf("'%s'", host)); err != nil {
			return err
		}
	}
	if err := verifyGSettings(targetUser, env, gsettingsPath, "org.gnome.system.proxy.socks", "port", portStr); err != nil {
		if err := fallbackDconfWrite(env, "/org/gnome/system/proxy/socks/port", fmt.Sprintf("uint32 %s", portStr)); err != nil {
			return err
		}
	}

	log.Printf("[SystemProxy] Linux proxy configured successfully")
	return nil
}

func findGSettings() (string, error) {
	if path, err := exec.LookPath("gsettings"); err == nil {
		return path, nil
	}
	// Fallback to common paths
	commonPaths := []string{"/usr/bin/gsettings", "/bin/gsettings", "/usr/local/bin/gsettings"}
	for _, p := range commonPaths {
		if _, err := os.Stat(p); err == nil {
			return p, nil
		}
	}
	return "", errors.New("gsettings not found in PATH or common locations")
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
	return runCommandWithEnv(nil, name, args...)
}

// runCommandAsUser 在 Linux 上支持以目标桌面用户执行（当前为 root 时），避免 gsettings 写到错误用户
func runCommandAsUser(username string, env map[string]string, name string, args ...string) error {
	_, err := runCommandAsUserWithOutput(username, env, name, args...)
	return err
}

func runCommandAsUserWithOutput(username string, env map[string]string, name string, args ...string) (string, error) {
	if runtime.GOOS == "linux" && username != "" && os.Geteuid() == 0 {
		if current, err := user.Current(); err == nil && current.Username != username {
			// sudo -u <user> env KEY=VAL ... <command> <args>
			// 必须显式传递环境变量，因为 sudo 默认会清除环境
			sudoArgs := []string{"-u", username, "env"}
			for k, v := range env {
				sudoArgs = append(sudoArgs, fmt.Sprintf("%s=%s", k, v))
			}
			sudoArgs = append(sudoArgs, name)
			sudoArgs = append(sudoArgs, args...)

			// 注意：这里不再传入 env 给 runCommandWithEnvOutput，因为我们已经把 env 放到命令行参数里了
			// 这样 sudo 就会执行 `env KEY=VAL ... command`
			return runCommandWithEnvOutput(nil, "sudo", sudoArgs...)
		}
	}
	return runCommandWithEnvOutput(env, name, args...)
}

func runCommandWithEnv(extraEnv map[string]string, name string, args ...string) error {
	_, err := runCommandWithEnvOutput(extraEnv, name, args...)
	return err
}

func runCommandWithEnvOutput(extraEnv map[string]string, name string, args ...string) (string, error) {
	cmd := exec.Command(name, args...)
	cmd.Env = mergeEnv(os.Environ(), extraEnv)
	output, err := cmd.CombinedOutput()
	if err != nil {
		log.Printf("[SystemProxy] Command failed: %s %v, error: %v, output: %s", name, args, err, string(output))
		return string(output), err
	}
	log.Printf("[SystemProxy] Command success: %s %v", name, args)
	return string(output), nil
}

func mergeEnv(base []string, extra map[string]string) []string {
	if len(extra) == 0 {
		return base
	}
	envMap := make(map[string]string, len(base)+len(extra))
	for _, kv := range base {
		parts := strings.SplitN(kv, "=", 2)
		if len(parts) == 2 {
			envMap[parts[0]] = parts[1]
		}
	}
	for k, v := range extra {
		envMap[k] = v
	}
	result := make([]string, 0, len(envMap))
	for k, v := range envMap {
		result = append(result, k+"="+v)
	}
	return result
}

// resolveTargetUser 解析应写入系统代理设置的桌面用户
func resolveTargetUser() string {
	// 1. 优先尝试 SUDO_USER (sudo 提权)
	if u := strings.TrimSpace(os.Getenv("SUDO_USER")); u != "" {
		return u
	}
	// 2. 尝试 PKEXEC_UID (pkexec 提权)
	if uid := strings.TrimSpace(os.Getenv("PKEXEC_UID")); uid != "" {
		if userByUID, err := user.LookupId(uid); err == nil {
			return userByUID.Username
		}
	}
	// 3. 尝试从 /proc/self/loginuid 获取 (systemd 登录会话)
	if data, err := os.ReadFile("/proc/self/loginuid"); err == nil {
		uid := strings.TrimSpace(string(data))
		if uid != "" && uid != "4294967295" { // -1 (unsigned)
			if userByUID, err := user.LookupId(uid); err == nil {
				return userByUID.Username
			}
		}
	}
	// 4. 回退到当前用户
	if current, err := user.Current(); err == nil {
		return current.Username
	}
	return ""
}

// buildUserEnv 为目标用户构造必要的环境变量（DBUS、XDG、HOME）
func buildUserEnv(username string) map[string]string {
	env := make(map[string]string)
	if username == "" {
		return env
	}
	u, err := user.Lookup(username)
	if err != nil {
		return env
	}
	env["HOME"] = u.HomeDir
	env["USER"] = username
	env["LOGNAME"] = username

	uid, err := strconv.Atoi(u.Uid)
	if err != nil {
		return env
	}

	// 尝试探测 DBUS 地址
	// 1. 检查 /run/user/<uid>/bus
	runtimeDir := fmt.Sprintf("/run/user/%d", uid)
	busPath := runtimeDir + "/bus"
	if _, err := os.Stat(busPath); err == nil {
		env["XDG_RUNTIME_DIR"] = runtimeDir
		env["DBUS_SESSION_BUS_ADDRESS"] = "unix:path=" + busPath
		return env
	}

	// 2. 尝试从该用户的进程中窃取 DBUS 地址 (需要 /proc 访问权限)
	// 查找该用户的 gnome-session 或 systemd --user 进程
	// 这是一个尽力而为的操作
	return env
}

// ensureDBUSSession 如果未能从目标用户构造出 DBUS 地址，则尝试当前用户兜底
func ensureDBUSSession(env map[string]string) {
	if env == nil {
		return
	}
	if env["DBUS_SESSION_BUS_ADDRESS"] != "" {
		return
	}
	// 尝试使用当前用户的 DBUS（如果也是普通用户运行）
	if dbus := os.Getenv("DBUS_SESSION_BUS_ADDRESS"); dbus != "" {
		env["DBUS_SESSION_BUS_ADDRESS"] = dbus
	}
	// 再次检查 /run/user/<current_uid>/bus
	path := fmt.Sprintf("/run/user/%d/bus", os.Getuid())
	if _, err := os.Stat(path); err == nil {
		env["DBUS_SESSION_BUS_ADDRESS"] = "unix:path=" + path
		if env["XDG_RUNTIME_DIR"] == "" {
			env["XDG_RUNTIME_DIR"] = fmt.Sprintf("/run/user/%d", os.Getuid())
		}
	}
	log.Printf("[SystemProxy] DBUS_SESSION_BUS_ADDRESS resolved to: %s", env["DBUS_SESSION_BUS_ADDRESS"])
}

// verifyGSettings 读取 gsettings 写回的值并校验
func verifyGSettings(username string, env map[string]string, gsettingsPath, schema, key, expected string) error {
	raw, err := runCommandAsUserWithOutput(username, env, gsettingsPath, "get", schema, key)
	if err != nil {
		return fmt.Errorf("verify %s.%s failed: %w", schema, key, err)
	}
	got := normalizeGSettingsValue(raw)
	if normalizeGSettingsValue(expected) != got {
		return fmt.Errorf("gsettings %s.%s not applied, got=%s expected=%s", schema, key, got, expected)
	}
	return nil
}

func normalizeGSettingsValue(val string) string {
	v := strings.TrimSpace(val)
	v = strings.TrimPrefix(v, "uint32 ")
	v = strings.Trim(v, "'\"")
	return v
}

// fallbackDconfWrite 在 gsettings 验证失败时直接写 dconf，确保值落盘
func fallbackDconfWrite(env map[string]string, key, value string) error {
	dconfPath, err := exec.LookPath("dconf")
	if err != nil {
		// try absolute paths
		for _, p := range []string{"/usr/bin/dconf", "/bin/dconf"} {
			if _, e := os.Stat(p); e == nil {
				dconfPath = p
				break
			}
		}
	}
	if dconfPath == "" {
		return fmt.Errorf("gsettings not applied and dconf missing")
	}

	log.Printf("[SystemProxy] fallback dconf write: %s = %s", key, value)
	if _, err := runCommandAsUserWithOutput(resolveTargetUser(), env, dconfPath, "write", key, value); err != nil {
		return fmt.Errorf("dconf write failed for %s: %w", key, err)
	}
	return nil
}
