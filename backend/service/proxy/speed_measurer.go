package proxy

import (
	"bufio"
	"bytes"
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"vea/backend/domain"
	"vea/backend/repository"
	"vea/backend/service/adapters"
	"vea/backend/service/nodegroup"
	"vea/backend/service/shared"
)

const (
	speedTestTimeout   = 30 * time.Second
	latencyTestTimeout = 8 * time.Second
	speedTestDuration  = 3 * time.Second
	speedTestWorkers   = 1
	measureConcurrency = 4
	speedProgressTick  = 100 * time.Millisecond
)

// socksTarget 测速目标
type socksTarget struct {
	host  string
	port  int
	path  string
	tls   bool
	bytes int64
}

type downloadFunc func(ctx context.Context, proxyHost string, proxyPort int, target socksTarget, minSeconds float64, progress func(int64, float64)) (int64, float64, error)

// SpeedMeasurer 速度测量器实现
type SpeedMeasurer struct {
	components repository.ComponentRepository
	settings   repository.SettingsRepository
	geoRepo    repository.GeoRepository
	adapters   map[domain.CoreEngineKind]adapters.CoreAdapter

	measureSem chan struct{}
}

// NewSpeedMeasurer 创建速度测量器
func NewSpeedMeasurer(
	components repository.ComponentRepository,
	geoRepo repository.GeoRepository,
	settings repository.SettingsRepository,
) *SpeedMeasurer {
	return &SpeedMeasurer{
		components: components,
		settings:   settings,
		geoRepo:    geoRepo,
		adapters: map[domain.CoreEngineKind]adapters.CoreAdapter{
			domain.EngineXray:    &adapters.XrayAdapter{},
			domain.EngineSingBox: &adapters.SingBoxAdapter{},
			domain.EngineClash:   &adapters.ClashAdapter{},
		},
		measureSem: make(chan struct{}, measureConcurrency),
	}
}

// MeasureSpeed 测量 FRouter 速度，返回 Mbps
func (m *SpeedMeasurer) MeasureSpeed(frouter domain.FRouter, nodes []domain.Node, onProgress func(speedMbps float64)) (float64, error) {
	compiled, err := nodegroup.CompileFRouter(frouter, nodes)
	if err != nil {
		return 0, err
	}
	activeNodes := nodegroup.FilterNodesByID(nodes, nodegroup.ActiveNodeIDs(compiled))

	// 直连（图中不引用任何代理节点）不应该依赖引擎进程：直接在本机网络做测量即可。
	if len(activeNodes) == 0 {
		ctx, cancel := context.WithTimeout(context.Background(), speedTestTimeout)
		defer cancel()
		mbps, err := measureDownloadDirect(ctx, onProgress)
		if err != nil {
			return 0, fmt.Errorf("measure download direct: %w", err)
		}
		return mbps, nil
	}

	// 启动测速代理
	stop, port, err := m.startMeasurement(frouter, nodes)
	if err != nil {
		return 0, fmt.Errorf("start measurement: %w", err)
	}
	defer stop()

	// 执行测速
	ctx, cancel := context.WithTimeout(context.Background(), speedTestTimeout)
	defer cancel()

	mbps, err := measureDownloadThroughSocks5(ctx, "127.0.0.1", port, onProgress)
	if err != nil {
		return 0, fmt.Errorf("measure download: %w", err)
	}

	return mbps, nil
}

// MeasureLatency 测量 FRouter 延迟，返回毫秒。
//
// 语义（按实际需求，不搞花活）：
// - 延迟 = 本机直连到“默认路径节点”的连接延迟（TCP connect；若启用 TLS 且非 Reality/QUIC，则包含 TLS handshake）。
// - 不启动代理内核，不走任何链路/分流：这就是“本地到节点”的延迟。
func (m *SpeedMeasurer) MeasureLatency(frouter domain.FRouter, nodes []domain.Node) (int64, error) {
	compiled, err := nodegroup.CompileFRouter(frouter, nodes)
	if err != nil {
		return 0, err
	}

	switch compiled.Default.Kind {
	case nodegroup.ActionNode:
		var target *domain.Node
		for i := range nodes {
			if nodes[i].ID == compiled.Default.NodeID {
				target = &nodes[i]
				break
			}
		}
		if target == nil {
			return 0, fmt.Errorf("default node not found: %s", compiled.Default.NodeID)
		}

		ctx, cancel := context.WithTimeout(context.Background(), latencyTestTimeout)
		defer cancel()

		latency, err := measureNodeLatencyDirect(ctx, *target)
		if err != nil {
			return 0, fmt.Errorf("measure node latency direct: %w", err)
		}
		if latency <= 0 {
			latency = 1
		}
		return latency, nil

	case nodegroup.ActionDirect:
		// 没有节点可测：保留原有“直连互联网”延迟作为参考值。
		ctx, cancel := context.WithTimeout(context.Background(), latencyTestTimeout)
		defer cancel()

		latency, err := measureLatencyDirect(ctx)
		if err != nil {
			return 0, fmt.Errorf("measure latency direct: %w", err)
		}
		if latency <= 0 {
			latency = 1
		}
		return latency, nil

	case nodegroup.ActionBlock:
		// 阻断本身没有“延迟”意义。
		return 0, nil
	default:
		return 0, nil
	}
}

func (m *SpeedMeasurer) acquireMeasureSlot() func() {
	sem := m.measureSem
	if sem == nil {
		sem = make(chan struct{}, 1)
		m.measureSem = sem
	}
	sem <- struct{}{}
	released := false
	return func() {
		if released {
			return
		}
		<-sem
		released = true
	}
}

// startMeasurement 启动测速临时代理进程（支持并发：每次测量独立端口 + 独立配置目录）。
func (m *SpeedMeasurer) startMeasurement(frouter domain.FRouter, nodes []domain.Node) (func(), int, error) {
	release := m.acquireMeasureSlot()

	ctx := context.Background()
	if m.components == nil {
		release()
		return nil, 0, fmt.Errorf("speed measurer missing component repository")
	}

	preferred := domain.EngineAuto
	if m.settings != nil {
		if cfg, err := m.settings.GetProxyConfig(ctx); err == nil {
			if cfg.PreferredEngine != "" {
				preferred = cfg.PreferredEngine
			}
		}
	}

	engine, engineComponent, err := selectEngineForFRouter(ctx, domain.InboundSOCKS, frouter, nodes, preferred, m.components, m.settings, m.adapters)
	if err != nil && preferred != "" && preferred != domain.EngineAuto {
		engine, engineComponent, err = selectEngineForFRouter(ctx, domain.InboundSOCKS, frouter, nodes, domain.EngineAuto, m.components, m.settings, m.adapters)
	}
	if err != nil {
		release()
		return nil, 0, err
	}
	if (engineComponent.InstallDir == "" || engineComponent.LastInstalledAt.IsZero()) && preferred != "" && preferred != domain.EngineAuto {
		engine, engineComponent, err = selectEngineForFRouter(ctx, domain.InboundSOCKS, frouter, nodes, domain.EngineAuto, m.components, m.settings, m.adapters)
		if err != nil {
			release()
			return nil, 0, err
		}
	}

	adapter := m.adapters[engine]

	// 获取引擎二进制路径
	binaryPath, err := m.getEngineBinaryPath(engineComponent, adapter)
	if err != nil {
		release()
		return nil, 0, err
	}
	if _, err := os.Stat(binaryPath); err != nil {
		release()
		return nil, 0, fmt.Errorf("engine binary not found: %s", binaryPath)
	}

	// 独立端口：允许多个节点同时测速。
	port, err := pickFreeLocalPort()
	if err != nil {
		release()
		return nil, 0, fmt.Errorf("pick free port: %w", err)
	}

	// 独立配置目录：避免 config-measure.json 被并发覆盖。
	configDir, err := createMeasurementConfigDir(engine)
	if err != nil {
		release()
		return nil, 0, fmt.Errorf("create measurement config dir: %w", err)
	}

	// 准备 Geo 文件
	geoDir := filepath.Join(shared.ArtifactsRoot, shared.GeoDir)
	geo := adapters.GeoFiles{
		GeoIP:        filepath.Join(geoDir, "geoip.dat"),
		GeoSite:      filepath.Join(geoDir, "geosite.dat"),
		ArtifactsDir: shared.ArtifactsRoot,
	}

	plan, err := nodegroup.CompileMeasurementPlan(engine, port, frouter, nodes)
	if err != nil {
		_ = os.RemoveAll(configDir)
		release()
		return nil, 0, err
	}

	// 构建测速配置
	configBytes, err := adapter.BuildConfig(plan, geo)
	if err != nil {
		_ = os.RemoveAll(configDir)
		release()
		return nil, 0, fmt.Errorf("build measurement config: %w", err)
	}

	// sing-box 的 route.rule_set 依赖本地 .srs 文件；缺失时会在运行期直接 FATAL。
	if engine == domain.EngineSingBox {
		tags, err := shared.ExtractSingBoxRuleSetTagsFromConfig(configBytes)
		if err != nil {
			_ = os.RemoveAll(configDir)
			release()
			return nil, 0, fmt.Errorf("extract sing-box rule-set: %w", err)
		}
		if err := shared.EnsureSingBoxRuleSets(tags); err != nil {
			_ = os.RemoveAll(configDir)
			release()
			return nil, 0, fmt.Errorf("ensure sing-box rule-set: %w", err)
		}
	}

	// 写入配置文件
	configPath := filepath.Join(configDir, "config-measure.json")
	if err := os.WriteFile(configPath, configBytes, 0600); err != nil {
		_ = os.RemoveAll(configDir)
		release()
		return nil, 0, err
	}

	// mihomo/clash 需要 GeoSite.dat/GeoIP.dat（大小写敏感）。
	if engine == domain.EngineClash {
		if err := ensureClashGeoData(configDir); err != nil {
			_ = os.RemoveAll(configDir)
			release()
			return nil, 0, err
		}
	}

	// 启动进程
	processCfg := adapters.ProcessConfig{
		BinaryPath: binaryPath,
		ConfigDir:  configDir,
	}

	handle, err := adapter.Start(processCfg, configPath)
	if err != nil {
		_ = os.RemoveAll(configDir)
		release()
		return nil, 0, fmt.Errorf("start measurement process: %w", err)
	}
	handle.Port = port

	// 等待就绪
	if err := adapter.WaitForReady(handle, 5*time.Second); err != nil {
		_ = adapter.Stop(handle)
		_ = os.RemoveAll(configDir)
		release()
		return nil, 0, fmt.Errorf("measurement process not ready: %w", err)
	}

	// 返回停止函数
	stop := func() {
		defer release()
		_ = adapter.Stop(handle)
		time.Sleep(200 * time.Millisecond)
		_ = os.RemoveAll(configDir)
	}
	return stop, port, nil
}

func pickFreeLocalPort() (int, error) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, err
	}
	defer ln.Close()

	_, portStr, err := net.SplitHostPort(ln.Addr().String())
	if err != nil {
		return 0, err
	}
	port, err := strconv.Atoi(portStr)
	if err != nil {
		return 0, err
	}
	if port <= 0 {
		return 0, fmt.Errorf("invalid port: %d", port)
	}
	return port, nil
}

func createMeasurementConfigDir(engine domain.CoreEngineKind) (string, error) {
	base := filepath.Join(shared.ArtifactsRoot, "runtime", "measure", engineArtifactsDirName(engine))
	if err := os.MkdirAll(base, 0o755); err != nil {
		return "", err
	}
	return os.MkdirTemp(base, "run-")
}

// getEngineBinaryPath 获取引擎二进制路径
func (m *SpeedMeasurer) getEngineBinaryPath(component domain.CoreComponent, adapter adapters.CoreAdapter) (string, error) {
	if adapter == nil {
		return "", errors.New("core adapter not found")
	}

	// 1) 优先使用组件记录的 binary 元信息（如果存在）
	if component.Meta != nil {
		if binaryPath := strings.TrimSpace(component.Meta["binary"]); binaryPath != "" {
			if _, err := os.Stat(binaryPath); err == nil {
				return binaryPath, nil
			}
		}
	}

	// 2) 在组件 InstallDir（以及一层子目录）中查找
	if component.InstallDir != "" {
		if binaryPath, err := shared.FindBinaryInDir(component.InstallDir, adapter.BinaryNames()); err == nil {
			return binaryPath, nil
		}
	}

	return "", fmt.Errorf("engine binary not found for %s", adapter.Kind())
}

func engineFromComponent(component domain.CoreComponent) (domain.CoreEngineKind, bool) {
	switch component.Kind {
	case domain.ComponentXray:
		return domain.EngineXray, true
	case domain.ComponentSingBox:
		return domain.EngineSingBox, true
	case domain.ComponentClash:
		return domain.EngineClash, true
	}
	return "", false
}

func installedEnginesFromComponents(components []domain.CoreComponent) map[domain.CoreEngineKind]domain.CoreComponent {
	installed := make(map[domain.CoreEngineKind]domain.CoreComponent)
	for _, c := range components {
		if c.InstallDir == "" || c.LastInstalledAt.IsZero() {
			continue
		}
		if e, ok := engineFromComponent(c); ok {
			installed[e] = c
		}
	}
	return installed
}

// measureDownloadThroughSocks5 通过 SOCKS5 代理测量下载速度
func measureDownloadThroughSocks5(ctx context.Context, proxyHost string, proxyPort int, progress func(float64)) (float64, error) {
	// 单个测速目标在某些网络/节点下会经常 EOF（例如被中间盒子/运营商重置）。
	// 这里不搞配置化和花活：给 2~3 个“够稳定的大文件”做回退即可。
	targets := []socksTarget{
		// Google downloads：全局可用性高，文件大，且路径多年稳定。
		{"dl.google.com", 443, "/chrome/install/GoogleChromeStandaloneEnterprise64.msi", true, 0},
		// Tele2 speedtest（HTTP）：路径稳定，便于绕开某些 TLS/HTTP2 兼容性问题。
		{"speedtest.tele2.net", 80, "/100MB.zip", false, 0},
		// Cloudflare：保留一个备选（部分网络反而更快）。
		{"speed.cloudflare.com", 443, "/__down?bytes=50000000", true, 0},
	}

	var lastErr error
	for _, t := range targets {
		mbps, err := measureDownloadFixedDurationWith(ctx, proxyHost, proxyPort, []socksTarget{t}, speedTestDuration, speedTestWorkers, downloadViaSocks5Once, progress)
		if err == nil {
			return mbps, nil
		}
		lastErr = err
	}
	if lastErr == nil {
		lastErr = errors.New("speed test targets are empty")
	}
	return 0, lastErr
}

func measureDownloadDirect(ctx context.Context, progress func(float64)) (float64, error) {
	targets := []socksTarget{
		{"dl.google.com", 443, "/chrome/install/GoogleChromeStandaloneEnterprise64.msi", true, 0},
		{"speedtest.tele2.net", 80, "/100MB.zip", false, 0},
		{"speed.cloudflare.com", 443, "/__down?bytes=50000000", true, 0},
	}

	var lastErr error
	for _, t := range targets {
		mbps, err := measureDownloadFixedDurationWith(ctx, "", 0, []socksTarget{t}, speedTestDuration, speedTestWorkers, downloadDirectOnce, progress)
		if err == nil {
			return mbps, nil
		}
		lastErr = err
	}
	if lastErr == nil {
		lastErr = errors.New("speed test targets are empty")
	}
	return 0, lastErr
}

func measureDownloadFixedDurationWith(
	ctx context.Context,
	proxyHost string,
	proxyPort int,
	targets []socksTarget,
	duration time.Duration,
	workers int,
	download downloadFunc,
	progress func(float64),
) (float64, error) {
	if duration <= 0 {
		return 0, fmt.Errorf("invalid speed test duration: %s", duration)
	}
	if workers <= 0 {
		workers = 1
	}
	if len(targets) == 0 {
		return 0, errors.New("speed test targets are empty")
	}
	if download == nil {
		return 0, errors.New("download func is nil")
	}

	testCtx, cancel := context.WithTimeout(ctx, duration)
	defer cancel()

	var total atomic.Int64
	var wg sync.WaitGroup

	var lastErrMu sync.Mutex
	var lastErr error

	for i := 0; i < workers; i++ {
		i := i
		wg.Add(1)
		go func() {
			defer wg.Done()

			t := targets[i%len(targets)]
			if strings.Contains(t.path, "?") {
				t.path = fmt.Sprintf("%s&vea=%d", t.path, i)
			} else {
				t.path = fmt.Sprintf("%s?vea=%d", t.path, i)
			}

			for {
				if testCtx.Err() != nil {
					return
				}

				var lastReported int64
				bytesRead, _, err := download(testCtx, proxyHost, proxyPort, t, 0, func(bytes int64, _ float64) {
					delta := bytes - lastReported
					if delta <= 0 {
						return
					}
					total.Add(delta)
					lastReported = bytes
				})

				// 补齐最后一次 progress 之后的剩余字节。
				if bytesRead > lastReported {
					total.Add(bytesRead - lastReported)
				}

				// 固定时长窗口到期：不要把 i/o timeout / EOF 等错误当成失败。
				if testCtx.Err() != nil {
					return
				}

				// 在 3s 窗口内，网络错误（EOF/timeout/reset）都视为一次尝试失败，继续下一轮。
				if err != nil {
					lastErrMu.Lock()
					lastErr = err
					lastErrMu.Unlock()
					if bytesRead == 0 {
						sleep := clampSleepToRemaining(testCtx, 20*time.Millisecond)
						if sleep > 0 {
							time.Sleep(sleep)
						}
					}
					continue
				}
			}
		}()
	}

	done := make(chan struct{})
	if progress != nil {
		start := time.Now()
		go func() {
			defer close(done)

			ticker := time.NewTicker(speedProgressTick)
			defer ticker.Stop()

			for {
				select {
				case <-testCtx.Done():
					return
				case <-ticker.C:
					elapsed := time.Since(start).Seconds()
					if elapsed <= 0 {
						continue
					}
					bytes := total.Load()
					progress((float64(bytes) / elapsed) / (1024 * 1024))
				}
			}
		}()
	}

	wg.Wait()
	cancel()
	if progress != nil {
		<-done
	}

	bytes := total.Load()
	mbps := (float64(bytes) / duration.Seconds()) / (1024 * 1024)
	if progress != nil {
		progress(mbps)
	}
	if bytes <= 0 {
		if lastErr != nil {
			return 0, fmt.Errorf("no throughput data collected: %w", lastErr)
		}
		return 0, errors.New("no throughput data collected")
	}
	return mbps, nil
}

func clampSleepToRemaining(ctx context.Context, sleep time.Duration) time.Duration {
	if sleep <= 0 {
		return 0
	}
	dl, ok := ctx.Deadline()
	if !ok {
		return sleep
	}
	remaining := time.Until(dl)
	if remaining <= 0 {
		return 0
	}
	// 窗口太短时不要 sleep：Windows 上 time.Sleep 可能有较粗的粒度，
	// 小窗口 + 固定 backoff 会直接“睡过头”，导致完全没有重试机会。
	if remaining < 2*sleep {
		return 0
	}
	return sleep
}

// downloadViaSocks5Once 单次 SOCKS5 下载
func downloadViaSocks5Once(ctx context.Context, proxyHost string, proxyPort int, t socksTarget, minSeconds float64, progress func(int64, float64)) (int64, float64, error) {
	deadline := time.Now().Add(speedTestTimeout)
	if dl, ok := ctx.Deadline(); ok && dl.Before(deadline) {
		deadline = dl
	}
	d := net.Dialer{}
	conn, err := d.DialContext(ctx, "tcp", net.JoinHostPort(proxyHost, strconv.Itoa(proxyPort)))
	if err != nil {
		return 0, 0, err
	}
	defer conn.Close()
	_ = conn.SetDeadline(deadline)

	brw := bufio.NewReadWriter(bufio.NewReader(conn), bufio.NewWriter(conn))

	// SOCKS5 握手
	if _, err := brw.Write([]byte{0x05, 0x01, 0x00}); err != nil {
		return 0, 0, err
	}
	if err := brw.Flush(); err != nil {
		return 0, 0, err
	}

	resp := make([]byte, 2)
	if _, err := io.ReadFull(brw, resp); err != nil {
		return 0, 0, err
	}
	if resp[0] != 0x05 || resp[1] != 0x00 {
		return 0, 0, fmt.Errorf("socks5 noauth rejected")
	}

	// 连接请求
	req, err := buildSocks5ConnectRequest(t.host, t.port)
	if err != nil {
		return 0, 0, err
	}
	if _, err := brw.Write(req); err != nil {
		return 0, 0, err
	}
	if err := brw.Flush(); err != nil {
		return 0, 0, err
	}

	// 读取连接响应
	respHdr := make([]byte, 4)
	if _, err := io.ReadFull(brw, respHdr); err != nil {
		return 0, 0, err
	}
	if respHdr[1] != 0x00 {
		return 0, 0, fmt.Errorf("socks5 connect error: %d", respHdr[1])
	}

	// 跳过绑定地址
	switch respHdr[3] {
	case 0x01: // IPv4
		skip := make([]byte, 4+2)
		if _, err := io.ReadFull(brw, skip); err != nil {
			return 0, 0, err
		}
	case 0x03: // Domain
		lenBuf := make([]byte, 1)
		if _, err := io.ReadFull(brw, lenBuf); err != nil {
			return 0, 0, err
		}
		skip := make([]byte, int(lenBuf[0])+2)
		if _, err := io.ReadFull(brw, skip); err != nil {
			return 0, 0, err
		}
	case 0x04: // IPv6
		skip := make([]byte, 16+2)
		if _, err := io.ReadFull(brw, skip); err != nil {
			return 0, 0, err
		}
	}

	var downstream net.Conn = conn
	if t.tls {
		tlsConn := tls.Client(conn, &tls.Config{ServerName: t.host, InsecureSkipVerify: true})
		if err := tlsConn.Handshake(); err != nil {
			_ = tlsConn.Close()
			return 0, 0, err
		}
		_ = tlsConn.SetDeadline(deadline)
		downstream = tlsConn
		brw = bufio.NewReadWriter(bufio.NewReader(downstream), bufio.NewWriter(downstream))
	}

	// 发送 HTTP 请求
	httpReq := fmt.Sprintf("GET %s HTTP/1.1\r\nHost: %s\r\nConnection: close\r\n\r\n", t.path, t.host)
	if _, err := brw.WriteString(httpReq); err != nil {
		return 0, 0, err
	}
	if err := brw.Flush(); err != nil {
		return 0, 0, err
	}

	// 读取响应并计时
	start := time.Now()
	var totalRead int64
	buf := make([]byte, 32*1024)
	header := make([]byte, 0, 4096)
	headerEnded := false
	lastProgress := time.Now()

	for {
		select {
		case <-ctx.Done():
			return totalRead, time.Since(start).Seconds(), ctx.Err()
		default:
		}

		n, rerr := brw.Read(buf)
		if n > 0 {
			if !headerEnded {
				header = append(header, buf[:n]...)
				if len(header) > 64*1024 {
					return totalRead, time.Since(start).Seconds(), fmt.Errorf("http header too large")
				}
				if idx := indexOfHeaderEnd(header); idx >= 0 {
					status, ok := parseHTTPStatusCode(header[:idx])
					if ok && status >= 400 {
						return totalRead, time.Since(start).Seconds(), fmt.Errorf("http status %d", status)
					}
					headerEnded = true
					body := header[idx:]
					totalRead += int64(len(body))
					header = nil
				}
			} else {
				totalRead += int64(n)
			}

			// 定期报告进度
			if progress != nil && time.Since(lastProgress) >= speedProgressTick {
				elapsed := time.Since(start).Seconds()
				if elapsed >= minSeconds {
					progress(totalRead, elapsed)
				}
				lastProgress = time.Now()
			}
		}
		if rerr != nil {
			if rerr == io.EOF {
				break
			}
			return totalRead, time.Since(start).Seconds(), rerr
		}
		if time.Since(start) > speedTestTimeout {
			break
		}
	}

	elapsed := time.Since(start).Seconds()
	if elapsed < minSeconds {
		elapsed = minSeconds
	}
	return totalRead, elapsed, nil
}

// downloadDirectOnce 单次直连下载（不走 SOCKS5）
func downloadDirectOnce(ctx context.Context, _ string, _ int, t socksTarget, minSeconds float64, progress func(int64, float64)) (int64, float64, error) {
	deadline := time.Now().Add(speedTestTimeout)
	if dl, ok := ctx.Deadline(); ok && dl.Before(deadline) {
		deadline = dl
	}
	d := net.Dialer{}
	conn, err := d.DialContext(ctx, "tcp", net.JoinHostPort(t.host, strconv.Itoa(t.port)))
	if err != nil {
		return 0, 0, err
	}
	defer conn.Close()
	_ = conn.SetDeadline(deadline)

	var downstream net.Conn = conn
	if t.tls {
		tlsConn := tls.Client(conn, &tls.Config{ServerName: t.host, InsecureSkipVerify: true})
		if err := tlsConn.Handshake(); err != nil {
			_ = tlsConn.Close()
			return 0, 0, err
		}
		_ = tlsConn.SetDeadline(deadline)
		downstream = tlsConn
	}

	brw := bufio.NewReadWriter(bufio.NewReader(downstream), bufio.NewWriter(downstream))

	httpReq := fmt.Sprintf("GET %s HTTP/1.1\r\nHost: %s\r\nConnection: close\r\n\r\n", t.path, t.host)
	if _, err := brw.WriteString(httpReq); err != nil {
		return 0, 0, err
	}
	if err := brw.Flush(); err != nil {
		return 0, 0, err
	}

	start := time.Now()
	var totalRead int64
	buf := make([]byte, 32*1024)
	header := make([]byte, 0, 4096)
	headerEnded := false
	lastProgress := time.Now()

	for {
		select {
		case <-ctx.Done():
			return totalRead, time.Since(start).Seconds(), ctx.Err()
		default:
		}

		n, rerr := brw.Read(buf)
		if n > 0 {
			if !headerEnded {
				header = append(header, buf[:n]...)
				if len(header) > 64*1024 {
					return totalRead, time.Since(start).Seconds(), fmt.Errorf("http header too large")
				}
				if idx := indexOfHeaderEnd(header); idx >= 0 {
					status, ok := parseHTTPStatusCode(header[:idx])
					if ok && status >= 400 {
						return totalRead, time.Since(start).Seconds(), fmt.Errorf("http status %d", status)
					}
					headerEnded = true
					body := header[idx:]
					totalRead += int64(len(body))
					header = nil
				}
			} else {
				totalRead += int64(n)
			}

			if progress != nil && time.Since(lastProgress) >= speedProgressTick {
				elapsed := time.Since(start).Seconds()
				if elapsed >= minSeconds {
					progress(totalRead, elapsed)
				}
				lastProgress = time.Now()
			}
		}
		if rerr != nil {
			if rerr == io.EOF {
				break
			}
			return totalRead, time.Since(start).Seconds(), rerr
		}
		if time.Since(start) > speedTestTimeout {
			break
		}
	}

	elapsed := time.Since(start).Seconds()
	if elapsed < minSeconds {
		elapsed = minSeconds
	}
	return totalRead, elapsed, nil
}

func measureNodeLatencyDirect(ctx context.Context, node domain.Node) (int64, error) {
	proto := domain.NodeProtocol(strings.ToLower(strings.TrimSpace(string(node.Protocol))))
	switch proto {
	case domain.ProtocolHysteria2, domain.ProtocolTUIC:
		return 0, fmt.Errorf("latency probe not supported for %s (udp/quic)", node.Protocol)
	}

	const maxAttempts = 3
	var best int64
	var lastErr error
	for i := 0; i < maxAttempts; i++ {
		attemptCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
		latency, err := nodeLatencyOnce(attemptCtx, node)
		cancel()
		if err != nil {
			lastErr = err
			continue
		}
		if latency <= 0 {
			latency = 1
		}
		if best == 0 || latency < best {
			best = latency
		}
	}
	if best > 0 {
		return best, nil
	}
	if lastErr != nil {
		return 0, lastErr
	}
	return 0, errors.New("no latency candidate successful")
}

func nodeLatencyOnce(ctx context.Context, node domain.Node) (int64, error) {
	host := strings.TrimSpace(node.Address)
	if strings.HasPrefix(host, "[") && strings.HasSuffix(host, "]") && len(host) > 2 {
		host = host[1 : len(host)-1]
	}
	if host == "" || node.Port <= 0 {
		return 0, fmt.Errorf("invalid node address/port: %q:%d", node.Address, node.Port)
	}

	deadline := time.Now().Add(5 * time.Second)
	if dl, ok := ctx.Deadline(); ok {
		deadline = dl
	}

	start := time.Now()
	d := net.Dialer{}
	raw, err := d.DialContext(ctx, "tcp", net.JoinHostPort(host, strconv.Itoa(node.Port)))
	if err != nil {
		return 0, err
	}
	_ = raw.SetDeadline(deadline)

	closeFn := raw.Close
	if shouldNodeTLSHandshake(node) {
		cfg := &tls.Config{
			ServerName:         tlsServerName(node, host),
			InsecureSkipVerify: node.TLS.Insecure,
		}
		if len(node.TLS.ALPN) > 0 {
			cfg.NextProtos = node.TLS.ALPN
		}

		tlsConn := tls.Client(raw, cfg)
		if err := tlsConn.HandshakeContext(ctx); err != nil {
			_ = raw.Close()
			return 0, err
		}
		closeFn = tlsConn.Close
	}

	latency := time.Since(start).Milliseconds()
	_ = closeFn()
	if latency <= 0 {
		latency = 1
	}
	return latency, nil
}

func shouldNodeTLSHandshake(node domain.Node) bool {
	if node.TLS == nil || !node.TLS.Enabled {
		return false
	}
	if strings.EqualFold(node.TLS.Type, "reality") || strings.TrimSpace(node.TLS.RealityPublicKey) != "" {
		return false
	}
	proto := domain.NodeProtocol(strings.ToLower(strings.TrimSpace(string(node.Protocol))))
	switch proto {
	case domain.ProtocolHysteria2, domain.ProtocolTUIC:
		return false
	default:
		return true
	}
}

func tlsServerName(node domain.Node, fallbackHost string) string {
	if node.TLS != nil && strings.TrimSpace(node.TLS.ServerName) != "" {
		return strings.TrimSpace(node.TLS.ServerName)
	}
	return fallbackHost
}

func measureLatencyDirect(ctx context.Context) (int64, error) {
	candidates := []socksTarget{
		{"www.gstatic.com", 80, "/generate_204", false, 0},
		{"example.com", 80, "/", false, 0},
		{"speed.cloudflare.com", 443, "/__up", true, 0},
	}

	var lastErr error
	for _, t := range candidates {
		lat, err := latencyDirectOnce(ctx, t)
		if err == nil {
			if lat <= 0 {
				lat = 1
			}
			return lat, nil
		}
		lastErr = err
	}
	if lastErr == nil {
		lastErr = errors.New("no latency candidate successful")
	}
	return 0, lastErr
}

func latencyDirectOnce(ctx context.Context, t socksTarget) (int64, error) {
	deadline := time.Now().Add(5 * time.Second)
	d := net.Dialer{}
	conn, err := d.DialContext(ctx, "tcp", net.JoinHostPort(t.host, strconv.Itoa(t.port)))
	if err != nil {
		return 0, err
	}
	defer conn.Close()
	_ = conn.SetDeadline(deadline)

	var downstream net.Conn = conn
	if t.tls {
		tlsConn := tls.Client(conn, &tls.Config{ServerName: t.host, InsecureSkipVerify: true})
		if err := tlsConn.Handshake(); err != nil {
			tlsConn.Close()
			return 0, err
		}
		_ = tlsConn.SetDeadline(deadline)
		downstream = tlsConn
	}

	brw := bufio.NewReadWriter(bufio.NewReader(downstream), bufio.NewWriter(downstream))

	request := fmt.Sprintf("HEAD %s HTTP/1.1\r\nHost: %s\r\nConnection: close\r\nUser-Agent: VeaLatency\r\n\r\n", t.path, t.host)
	start := time.Now()
	if _, err := brw.WriteString(request); err != nil {
		_ = downstream.Close()
		return 0, err
	}
	if err := brw.Flush(); err != nil {
		_ = downstream.Close()
		return 0, err
	}

	buf := make([]byte, 1024)
	header := make([]byte, 0, 2048)
	for {
		n, rerr := brw.Read(buf)
		if n > 0 {
			header = append(header, buf[:n]...)
			if len(header) > 64*1024 {
				_ = downstream.Close()
				return 0, fmt.Errorf("http header too large")
			}
			if idx := indexOfHeaderEnd(header); idx >= 0 {
				status, ok := parseHTTPStatusCode(header[:idx])
				if ok && status >= 400 {
					_ = downstream.Close()
					return 0, fmt.Errorf("http status %d", status)
				}
				break
			}
		}
		if rerr != nil {
			if rerr == io.EOF {
				break
			}
			_ = downstream.Close()
			return 0, rerr
		}
	}

	latency := time.Since(start).Milliseconds()
	_ = downstream.Close()
	if latency <= 0 {
		latency = 1
	}
	return latency, nil
}

func buildSocks5ConnectRequest(host string, port int) ([]byte, error) {
	host = strings.TrimSpace(host)
	if strings.HasPrefix(host, "[") && strings.HasSuffix(host, "]") && len(host) > 2 {
		host = host[1 : len(host)-1]
	}
	if ip := net.ParseIP(host); ip != nil {
		if ip4 := ip.To4(); ip4 != nil {
			req := []byte{0x05, 0x01, 0x00, 0x01}
			req = append(req, ip4...)
			req = append(req, byte(port>>8), byte(port&0xff))
			return req, nil
		}
		ip16 := ip.To16()
		if ip16 == nil {
			return nil, fmt.Errorf("invalid ip: %s", host)
		}
		req := []byte{0x05, 0x01, 0x00, 0x04}
		req = append(req, ip16...)
		req = append(req, byte(port>>8), byte(port&0xff))
		return req, nil
	}
	if len(host) > 255 {
		return nil, fmt.Errorf("host too long: %d", len(host))
	}
	hostBytes := []byte(host)
	req := []byte{0x05, 0x01, 0x00, 0x03, byte(len(hostBytes))}
	req = append(req, hostBytes...)
	req = append(req, byte(port>>8), byte(port&0xff))
	return req, nil
}

func parseHTTPStatusCode(header []byte) (int, bool) {
	end := bytes.Index(header, []byte("\r\n"))
	if end < 0 {
		end = bytes.IndexByte(header, '\n')
	}
	if end < 0 {
		return 0, false
	}
	line := strings.TrimSpace(string(header[:end]))
	if !strings.HasPrefix(line, "HTTP/") {
		return 0, false
	}
	fields := strings.Fields(line)
	if len(fields) < 2 {
		return 0, false
	}
	code, err := strconv.Atoi(fields[1])
	if err != nil {
		return 0, false
	}
	return code, true
}

func indexOfHeaderEnd(b []byte) int {
	for i := 3; i < len(b); i++ {
		if b[i-3] == '\r' && b[i-2] == '\n' && b[i-1] == '\r' && b[i] == '\n' {
			return i + 1
		}
	}
	return -1
}
