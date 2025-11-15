package service

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"vea/internal/domain"
	"vea/internal/store"

	"golang.org/x/net/proxy"
)

// TestE2E_ProxyToCloudflare 测试通过代理访问 Cloudflare
// 验证完整的代理链路：本地 xray 服务端 → Vea 管理的 xray 客户端 → Cloudflare
func TestE2E_ProxyToCloudflare(t *testing.T) {
	if testing.Short() {
		t.Skip("跳过集成测试")
	}

	// 自动查找 xray 二进制
	xrayBin := findXrayBinaryForTest(t)
	if xrayBin == "" {
		t.Skip("未找到 xray 二进制文件，跳过集成测试")
	}
	t.Logf("使用 xray: %s", xrayBin)

	// 创建临时目录
	tmpDir := t.TempDir()
	serverConfigPath := filepath.Join(tmpDir, "server-config.json")
	componentInstallDir := filepath.Join(tmpDir, "xray-install")

	if err := os.MkdirAll(componentInstallDir, 0o755); err != nil {
		t.Fatalf("创建组件目录失败: %v", err)
	}

	// 复制 xray 二进制到组件目录（保留 Windows .exe 后缀）
	xrayFilename := "xray"
	if runtime.GOOS == "windows" {
		xrayFilename = "xray.exe"
	}
	xrayDest := filepath.Join(componentInstallDir, xrayFilename)
	if err := copyFileForTest(xrayBin, xrayDest); err != nil {
		t.Fatalf("复制 xray 二进制失败: %v", err)
	}
	if runtime.GOOS != "windows" {
		if err := os.Chmod(xrayDest, 0o755); err != nil {
			t.Fatalf("设置 xray 权限失败: %v", err)
		}
	}

	// 下载真实的 Geo 文件
	geoIPPath := filepath.Join(componentInstallDir, "geoip.dat")
	geoSitePath := filepath.Join(componentInstallDir, "geosite.dat")

	t.Log("下载 Geo 文件...")
	if err := downloadFileForTest("https://github.com/Loyalsoldier/v2ray-rules-dat/releases/latest/download/geoip.dat", geoIPPath); err != nil {
		t.Fatalf("下载 GeoIP 文件失败: %v", err)
	}
	if err := downloadFileForTest("https://github.com/Loyalsoldier/v2ray-rules-dat/releases/latest/download/geosite.dat", geoSitePath); err != nil {
		t.Fatalf("下载 GeoSite 文件失败: %v", err)
	}
	t.Log("Geo 文件下载完成")

	// 使用高端口避免权限问题
	serverPort := 20086
	testUUID := "b831381d-6324-4d53-ad4f-8cda48b30811"

	// 创建 xray 服务端配置（VLESS）
	serverConfig := map[string]any{
		"log": map[string]any{
			"loglevel": "warning",
		},
		"inbounds": []map[string]any{
			{
				"port":     serverPort,
				"protocol": "vless",
				"settings": map[string]any{
					"clients": []map[string]any{
						{
							"id": testUUID,
						},
					},
					"decryption": "none",
				},
			},
		},
		"outbounds": []map[string]any{
			{
				"protocol": "freedom",
			},
		},
	}

	// 写入服务端配置
	configBytes, err := json.MarshalIndent(serverConfig, "", "  ")
	if err != nil {
		t.Fatalf("生成服务端配置失败: %v", err)
	}
	if err := os.WriteFile(serverConfigPath, configBytes, 0o644); err != nil {
		t.Fatalf("写入服务端配置失败: %v", err)
	}

	// 启动 xray 服务端
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	serverCmd := exec.CommandContext(ctx, xrayBin, "run", "-c", serverConfigPath)
	serverCmd.Stdout = io.Discard
	serverCmd.Stderr = io.Discard

	if err := serverCmd.Start(); err != nil {
		t.Fatalf("启动 xray 服务端失败: %v", err)
	}
	defer func() {
		cancel()
		_ = serverCmd.Wait()
	}()

	// 等待服务端启动
	time.Sleep(500 * time.Millisecond)

	// 创建测试用的 Service
	st := store.NewMemoryStore()
	svc := NewService(st)

	// 更新 xray 组件的 InstallDir
	components := svc.ListComponents()
	var xrayCompID string
	for _, comp := range components {
		if comp.Kind == domain.ComponentXray {
			xrayCompID = comp.ID
			break
		}
	}
	if xrayCompID == "" {
		t.Fatal("未找到 xray 组件")
	}

	_, err = st.UpdateComponent(xrayCompID, func(comp domain.CoreComponent) (domain.CoreComponent, error) {
		comp.InstallDir = componentInstallDir
		comp.LastInstalledAt = time.Now()
		return comp, nil
	})
	if err != nil {
		t.Fatalf("更新组件失败: %v", err)
	}

	// 添加指向本地服务端的节点
	node := domain.Node{
		Name:     "本地测试节点",
		Address:  "127.0.0.1",
		Port:     serverPort,
		Protocol: domain.ProtocolVLESS,
		Security: &domain.NodeSecurity{
			UUID:       testUUID,
			Encryption: "none",
		},
	}

	createdNode := svc.CreateNode(node)
	t.Logf("创建节点: ID=%s, Address=%s:%d", createdNode.ID, createdNode.Address, createdNode.Port)

	// 启动 Vea 管理的 xray 客户端
	if err := svc.EnableXray(createdNode.ID); err != nil {
		t.Fatalf("启动 xray 客户端失败: %v", err)
	}
	defer func() {
		svc.xrayMu.Lock()
		if svc.xrayCmd != nil {
			_ = svc.xrayCmd.Process.Kill()
			svc.xrayCmd = nil
		}
		svc.xrayMu.Unlock()
	}()

	// 等待客户端启动
	time.Sleep(2 * time.Second)

	// 获取 xray 客户端的 SOCKS5 端口
	status := svc.XrayStatus()
	if !status.Running {
		t.Fatal("xray 客户端未运行")
	}
	socksPort := status.InboundPort
	t.Logf("xray 客户端 SOCKS5 端口: %d", socksPort)

	// 测试 1: 通过 SOCKS5 代理测试延迟（Cloudflare）
	t.Run("Latency", func(t *testing.T) {
		latency, err := measureLatencyViaSocks5(ctx, "127.0.0.1", socksPort)
		if err != nil {
			t.Fatalf("延迟测试失败: %v", err)
		}
		if latency <= 0 {
			t.Errorf("期望延迟 > 0，实际得到 %d ms", latency)
		}
		t.Logf("✓ 延迟测试通过: %d ms", latency)
	})

	// 测试 2: 通过 SOCKS5 代理测试速度（Cloudflare）
	t.Run("Speed", func(t *testing.T) {
		// 下载 100KB 测试速度
		bytes, err := downloadViaSocks5(ctx, "127.0.0.1", socksPort, "https://speed.cloudflare.com/__down?bytes=102400")
		if err != nil {
			t.Logf("速度测试失败（可能是网络问题）: %v", err)
			t.Skip("跳过速度测试")
		}
		if bytes <= 0 {
			t.Errorf("期望下载 > 0 bytes，实际得到 %d", bytes)
		}
		speedMbps := float64(bytes) * 8 / 1024 / 1024
		t.Logf("✓ 速度测试通过: 下载 %d bytes (%.2f Mbps)", bytes, speedMbps)
	})
}

// measureLatencyViaSocks5 通过 SOCKS5 代理测量延迟（连接 Cloudflare）
func measureLatencyViaSocks5(ctx context.Context, proxyHost string, proxyPort int) (int64, error) {
	dialer, err := proxy.SOCKS5("tcp", net.JoinHostPort(proxyHost, strconv.Itoa(proxyPort)), nil, proxy.Direct)
	if err != nil {
		return 0, fmt.Errorf("创建 SOCKS5 dialer 失败: %w", err)
	}

	target := "speed.cloudflare.com:443"
	start := time.Now()
	conn, err := dialer.Dial("tcp", target)
	if err != nil {
		return 0, fmt.Errorf("连接失败: %w", err)
	}
	defer conn.Close()

	latency := time.Since(start).Milliseconds()
	if latency <= 0 {
		latency = 1
	}
	return latency, nil
}

// downloadViaSocks5 通过 SOCKS5 代理下载数据（从 Cloudflare）
func downloadViaSocks5(ctx context.Context, proxyHost string, proxyPort int, url string) (int64, error) {
	dialer, err := proxy.SOCKS5("tcp", net.JoinHostPort(proxyHost, strconv.Itoa(proxyPort)), nil, proxy.Direct)
	if err != nil {
		return 0, fmt.Errorf("创建 SOCKS5 dialer 失败: %w", err)
	}

	httpClient := &http.Client{
		Transport: &http.Transport{
			DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
				return dialer.Dial(network, addr)
			},
		},
		Timeout: 30 * time.Second,
	}

	resp, err := httpClient.Get(url)
	if err != nil {
		return 0, fmt.Errorf("下载失败: %w", err)
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, fmt.Errorf("读取失败: %w", err)
	}

	return int64(len(data)), nil
}

// findXrayBinaryForTest 自动查找或安装 xray 二进制文件
func findXrayBinaryForTest(t *testing.T) string {
	t.Helper()

	xrayName := "xray"
	if runtime.GOOS == "windows" {
		xrayName = "xray.exe"
	}

	candidates := []string{
		// 1. 项目中的 xray（最优先）
		filepath.Join("artifacts/core/xray", xrayName),
		filepath.Join("../../../artifacts/core/xray", xrayName),
		// 2. 环境变量
		os.Getenv("XRAY_BINARY"),
		// 3. 系统 PATH
		func() string { path, _ := exec.LookPath("xray"); return path }(),
	}

	// 4. Unix 常见位置
	if runtime.GOOS != "windows" {
		candidates = append(candidates,
			"/usr/local/bin/xray",
			"/usr/bin/xray",
		)
	}

	for _, path := range candidates {
		if path == "" {
			continue
		}
		if absPath, err := filepath.Abs(path); err == nil {
			if _, err := os.Stat(absPath); err == nil {
				return absPath
			}
		}
		if _, err := os.Stat(path); err == nil {
			return path
		}
	}

	// 如果找不到，自动安装
	t.Log("xray 未找到，使用 Vea 自动安装...")
	if err := installXrayForTest(t); err != nil {
		t.Fatalf("安装 xray 失败: %v", err)
	}

	// 再次查找
	installPath := filepath.Join("artifacts/core/xray", xrayName)
	if absPath, err := filepath.Abs(installPath); err == nil {
		if _, err := os.Stat(absPath); err == nil {
			return absPath
		}
	}

	return ""
}

// installXrayForTest 使用 Vea 的 InstallComponent 安装 xray
func installXrayForTest(t *testing.T) error {
	t.Helper()

	st := store.NewMemoryStore()
	svc := NewService(st)

	components := svc.ListComponents()
	var xrayID string
	for _, comp := range components {
		if comp.Kind == domain.ComponentXray {
			xrayID = comp.ID
			break
		}
	}

	if xrayID == "" {
		return fmt.Errorf("xray component not found")
	}

	installDir := "artifacts/core/xray"
	absInstallDir, err := filepath.Abs(installDir)
	if err != nil {
		return fmt.Errorf("failed to get absolute path: %w", err)
	}

	if err := os.MkdirAll(absInstallDir, 0o755); err != nil {
		return fmt.Errorf("failed to create install dir: %w", err)
	}

	_, err = st.UpdateComponent(xrayID, func(comp domain.CoreComponent) (domain.CoreComponent, error) {
		comp.InstallDir = absInstallDir
		return comp, nil
	})
	if err != nil {
		return fmt.Errorf("failed to update component install dir: %w", err)
	}

	t.Log("开始下载 xray...")
	installed, err := svc.InstallComponent(xrayID)
	if err != nil {
		return fmt.Errorf("failed to install xray: %w", err)
	}

	t.Logf("✓ xray 安装成功: %s", installed.LastVersion)
	return nil
}

// copyFileForTest 复制文件（测试辅助函数）
func copyFileForTest(src, dst string) error {
	input, err := os.Open(src)
	if err != nil {
		return err
	}
	defer input.Close()

	output, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer output.Close()

	_, err = io.Copy(output, input)
	return err
}

// downloadFileForTest 下载文件（测试辅助函数，带重试）
func downloadFileForTest(url, dst string) error {
	maxRetries := 3
	var lastErr error

	for i := 0; i < maxRetries; i++ {
		if i > 0 {
			time.Sleep(time.Duration(i) * time.Second)
		}

		client := &http.Client{Timeout: 30 * time.Second}
		resp, err := client.Get(url)
		if err != nil {
			lastErr = fmt.Errorf("HTTP GET 失败: %w", err)
			continue
		}

		if resp.StatusCode != http.StatusOK {
			resp.Body.Close()
			lastErr = fmt.Errorf("HTTP 状态码: %d", resp.StatusCode)
			continue
		}

		output, err := os.Create(dst)
		if err != nil {
			resp.Body.Close()
			return err
		}

		_, err = io.Copy(output, resp.Body)
		resp.Body.Close()
		output.Close()

		if err == nil {
			return nil
		}
		lastErr = err
	}

	return fmt.Errorf("重试 %d 次后失败: %w", maxRetries, lastErr)
}
