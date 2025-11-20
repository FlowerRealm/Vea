package service

import (
	"archive/tar"
	"archive/zip"
	"bufio"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"crypto/tls"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"vea/backend/domain"
	"vea/backend/store"
)

const (
	defaultConfigSyncInterval      = time.Hour
	defaultComponentUpdateInterval = 12 * time.Hour
	maxDownloadSize                = 50 << 20 // 50 MiB guard rail
	downloadTimeout                = 5 * time.Minute // 增加到 5 分钟以支持慢速网络
	measurementPort                = 31346

	artifactsRoot = "artifacts"
	geoDir        = "geo"
	componentFile = "artifact.bin"
)

var (
	speedTestTimeout = 30 * time.Second
)

var httpClient = &http.Client{
	Timeout: downloadTimeout,
}

var (
	// httpClientDirect attempts HTTP/2 downloads without honoring proxy settings.
	httpClientDirect = newHTTPClient(true, false)
	// httpClientHTTP11 forces HTTP/1.1 while still allowing environment proxies.
	httpClientHTTP11 = newHTTPClient(false, true)
	// httpClientDirectHTTP11 downgrades to HTTP/1.1 without using proxies.
	httpClientDirectHTTP11 = newHTTPClient(true, true)
)

func newHTTPClient(bypassProxy, forceHTTP11 bool) *http.Client {
	tr := &http.Transport{
		Proxy:               http.ProxyFromEnvironment,
		DialContext:         (&net.Dialer{Timeout: 60 * time.Second, KeepAlive: 60 * time.Second}).DialContext,
		ForceAttemptHTTP2:   !forceHTTP11,
		DisableKeepAlives:   true,
		TLSHandshakeTimeout: 30 * time.Second, // 增加 TLS 握手超时
	}
	if bypassProxy {
		tr.Proxy = nil
	}
	if forceHTTP11 {
		tr.TLSClientConfig = &tls.Config{
			MinVersion: tls.VersionTLS12,
			NextProtos: []string{"http/1.1"},
		}
	}
	return &http.Client{
		Timeout:   downloadTimeout,
		Transport: tr,
	}
}

// ErrXrayNotInstalled is returned when xray-related operations are invoked without installing the component.
var ErrXrayNotInstalled = errXrayComponentNotInstalled

type componentDefault struct {
	name            string
	repo            string
	assetCandidates []string
	archiveType     string
}

type componentInstallInfo struct {
	dir             string
	lastInstalledAt time.Time
}

func (d *componentDefault) fallbackAsset() string {
	if d == nil || len(d.assetCandidates) == 0 {
		return ""
	}
	return d.assetCandidates[0]
}

func (d *componentDefault) fallbackURL() string {
	if d == nil || d.repo == "" {
		return ""
	}
	asset := d.fallbackAsset()
	if asset == "" {
		return ""
	}
	return latestDownloadURL(d.repo, asset)
}

func componentDefaultFor(component domain.CoreComponent) (*componentDefault, error) {
	switch component.Kind {
	case domain.ComponentXray:
		assets, err := xrayAssetCandidates()
		if err != nil {
			return nil, err
		}
		return &componentDefault{
			name:            "xray-core",
			repo:            "XTLS/Xray-core",
			assetCandidates: assets,
			archiveType:     inferArchiveTypeFromName(assets[0]),
		}, nil
	case domain.ComponentGeo:
		slug := strings.ToLower(component.Name)
		asset := "geoip.dat"
		display := component.Name
		if strings.Contains(slug, "site") {
			asset = "geosite.dat"
		}
		if display == "" && asset == "geoip.dat" {
			display = "GeoIP"
		} else if display == "" {
			display = "GeoSite"
		}
		return &componentDefault{
			name:            display,
			repo:            "Loyalsoldier/v2ray-rules-dat",
			assetCandidates: []string{asset},
			archiveType:     "raw",
		}, nil
	default:
		return nil, nil
	}
}

func resolveComponentAssetFallback(kind domain.CoreComponentKind, repo string, candidates []string) (releaseAssetInfo, error) {
	if repo == "" || len(candidates) == 0 {
		return releaseAssetInfo{}, errReleaseAssetNotFound
	}
	tag, version, err := fetchLatestReleaseTag(repo)
	if err != nil {
		return releaseAssetInfo{}, err
	}
	version = strings.TrimSpace(version)
	for _, candidate := range candidates {
		candidate = strings.TrimSpace(candidate)
		if candidate == "" {
			continue
		}
		assetOptions := []string{candidate}
		for _, asset := range assetOptions {
			ok, size, probeErr := probeReleaseAsset(repo, tag, asset)
			if probeErr != nil {
				continue
			}
			if ok {
				url := fmt.Sprintf("https://github.com/%s/releases/download/%s/%s", repo, tag, asset)
				return releaseAssetInfo{Name: asset, DownloadURL: url, Version: tag, Size: size}, nil
			}
		}
	}
	return releaseAssetInfo{}, errReleaseAssetNotFound
}

func defaultArchiveForKind(kind domain.CoreComponentKind) string {
	switch kind {
	case domain.ComponentXray:
		return "zip"
	case domain.ComponentGeo:
		return "raw"
	default:
		return "zip"
	}
}

func xrayAssetCandidates() ([]string, error) {
	key := runtime.GOOS + "/" + runtime.GOARCH
	switch key {
	case "linux/amd64":
		return []string{"Xray-linux-64.zip"}, nil
	case "linux/386":
		return []string{"Xray-linux-32.zip"}, nil
	case "linux/arm64":
		return []string{"Xray-linux-arm64-v8a.zip", "Xray-linux-arm64.zip"}, nil
	case "linux/arm":
		return []string{"Xray-linux-arm32-v7a.zip"}, nil
	case "windows/amd64":
		return []string{"Xray-windows-64.zip"}, nil
	case "windows/386":
		return []string{"Xray-windows-32.zip"}, nil
	case "darwin/amd64":
		return []string{"Xray-macos-64.zip"}, nil
	case "darwin/arm64":
		return []string{"Xray-macos-arm64.zip"}, nil
	default:
		return nil, fmt.Errorf("unsupported platform %s for xray release asset", key)
	}
}

type Service struct {
	store *store.MemoryStore
	tasks []Task

	xrayMu      sync.Mutex
	xrayCmd     *exec.Cmd
	xrayRuntime XrayRuntime
	xrayEnabled bool

	measureMu sync.Mutex

	jobMu        sync.Mutex
	speedQueue   chan string
	latencyQueue chan string
	speedJobs    map[string]struct{}
	latencyJobs  map[string]struct{}
}

type XrayStatus struct {
	Enabled      bool   `json:"enabled"`
	Running      bool   `json:"running"`
	PID          int    `json:"pid"`
	Binary       string `json:"binary"`
	Config       string `json:"config"`
	GeoIP        string `json:"geoip"`
	GeoSite      string `json:"geosite"`
	InboundPort  int    `json:"inboundPort"`
	ActiveNodeID string `json:"activeNodeId"`
}

type Task interface {
	Start(ctx context.Context)
}

func NewService(store *store.MemoryStore, tasks ...Task) *Service {
	svc := &Service{
		store:        store,
		tasks:        tasks,
		speedQueue:   make(chan string, 32),
		latencyQueue: make(chan string, 32),
		speedJobs:    make(map[string]struct{}),
		latencyJobs:  make(map[string]struct{}),
	}
	if removed := svc.store.CleanupOrphanNodes(); removed > 0 {
		log.Printf("purged %d orphan nodes referencing deleted configs", removed)
	}
	svc.restoreXrayState()
	svc.resetNodeProbeStats()
	go svc.speedWorker()
	go svc.latencyWorker()
	return svc
}

func (s *Service) AttachTasks(tasks ...Task) {
	s.tasks = append(s.tasks, tasks...)
}

func (s *Service) Start(ctx context.Context) {
	for _, task := range s.tasks {
		go task.Start(ctx)
	}
}

func (s *Service) restoreXrayState() {
	comp, err := s.getComponentByKind(domain.ComponentXray)
	if err != nil {
		return
	}
	enabled := comp.Meta != nil && strings.EqualFold(comp.Meta["enabled"], "true")
	if !enabled {
		s.xrayEnabled = false
		return
	}
	s.xrayEnabled = true
	active := ""
	if comp.Meta != nil {
		active = comp.Meta["activeNodeId"]
	}
	go func(nodeID string) {
		if err := s.RestartXray(nodeID); err != nil {
			s.xrayMu.Lock()
			s.xrayEnabled = false
			s.xrayMu.Unlock()
			if updErr := s.updateXrayMeta(func(component domain.CoreComponent) (domain.CoreComponent, error) {
				if component.Meta == nil {
					component.Meta = map[string]string{}
				}
				component.Meta["enabled"] = "false"
				return component, nil
			}); updErr != nil && !errors.Is(updErr, errXrayComponentNotInstalled) {
				log.Printf("auto start xray meta update failed: %v", updErr)
			}
			log.Printf("auto start xray failed: %v", err)
		}
	}(active)
}

func (s *Service) resetNodeProbeStats() {
	for _, node := range s.store.ListNodes() {
		_, _ = s.store.UpdateNode(node.ID, func(n domain.Node) (domain.Node, error) {
			n.LastLatencyMS = 0
			n.LastLatencyAt = time.Time{}
			n.LastSpeedMbps = 0
			n.LastSpeedAt = time.Time{}
			n.LastSpeedError = ""
			return n, nil
		})
	}
}

func (s *Service) speedWorker() {
	for id := range s.speedQueue {
		if id == "" {
			s.clearSpeedJob(id)
			continue
		}
		if _, err := s.ProbeSpeed(id); err != nil {
			log.Printf("speedtest for %s failed: %v", id, err)
			_, updateErr := s.store.UpdateNode(id, func(n domain.Node) (domain.Node, error) {
				n.LastSpeedError = err.Error()
				n.LastSpeedAt = time.Now()
				return n, nil
			})
			if updateErr != nil {
				log.Printf("record speedtest error for %s failed: %v", id, updateErr)
			}
		}
		s.clearSpeedJob(id)
	}
}

func (s *Service) latencyWorker() {
	for id := range s.latencyQueue {
		if id == "" {
			s.clearLatencyJob(id)
			continue
		}
		if _, err := s.ProbeLatency(id); err != nil {
			log.Printf("ping node %s failed: %v", id, err)
			_, _ = s.store.UpdateNode(id, func(n domain.Node) (domain.Node, error) {
				n.LastLatencyMS = 0
				n.LastLatencyAt = time.Now()
				return n, nil
			})
		}
		s.clearLatencyJob(id)
	}
}

func (s *Service) clearSpeedJob(id string) {
	s.jobMu.Lock()
	delete(s.speedJobs, id)
	s.jobMu.Unlock()
}

func (s *Service) clearLatencyJob(id string) {
	s.jobMu.Lock()
	delete(s.latencyJobs, id)
	s.jobMu.Unlock()
}

func (s *Service) ListNodes() []domain.Node {
	return s.store.ListNodes()
}

func (s *Service) CreateNode(node domain.Node) domain.Node {
	return s.store.CreateNode(node)
}

func (s *Service) CreateNodeFromShare(link string) (domain.Node, error) {
	node, err := parseNodeShareLink(link)
	if err != nil {
		return domain.Node{}, err
	}
	return s.CreateNode(node), nil
}

func (s *Service) ListNodesByConfig(configID string) []domain.Node {
	return s.store.ListNodesByConfig(configID)
}

func (s *Service) UpdateNode(id string, mutate func(domain.Node) (domain.Node, error)) (domain.Node, error) {
	return s.store.UpdateNode(id, mutate)
}

func (s *Service) DeleteNode(id string) error {
	return s.store.DeleteNode(id)
}

func (s *Service) ResetNodeTraffic(id string) (domain.Node, error) {
	return s.store.ResetNodeTraffic(id)
}

func (s *Service) ResetNodeSpeeds(ids []string) {
	target := ids
	if len(target) == 0 {
		nodes := s.store.ListNodes()
		target = make([]string, 0, len(nodes))
		for _, n := range nodes {
			target = append(target, n.ID)
		}
	}
	for _, id := range target {
		if id == "" {
			continue
		}
		_, _ = s.store.UpdateNode(id, func(n domain.Node) (domain.Node, error) {
			n.LastSpeedMbps = 0
			n.LastSpeedAt = time.Time{}
			n.LastSpeedError = ""
			return n, nil
		})
		s.jobMu.Lock()
		delete(s.speedJobs, id)
		s.jobMu.Unlock()
	}
	if s.speedQueue != nil {
		for {
			select {
			case <-s.speedQueue:
			default:
				return
			}
		}
	}
}

func (s *Service) SpeedtestAsync(id string) {
	if id == "" {
		return
	}
	s.jobMu.Lock()
	if _, ok := s.speedJobs[id]; ok {
		s.jobMu.Unlock()
		return
	}
	s.speedJobs[id] = struct{}{}
	s.jobMu.Unlock()
	_, _ = s.store.UpdateNode(id, func(n domain.Node) (domain.Node, error) {
		n.LastSpeedMbps = 0
		n.LastSpeedAt = time.Time{}
		n.LastSpeedError = ""
		return n, nil
	})
	select {
	case s.speedQueue <- id:
	default:
		go func() { s.speedQueue <- id }()
	}
}

func (s *Service) PingAsync(id string) {
	if id == "" {
		return
	}
	s.jobMu.Lock()
	if _, ok := s.latencyJobs[id]; ok {
		s.jobMu.Unlock()
		return
	}
	s.latencyJobs[id] = struct{}{}
	s.jobMu.Unlock()
	_, _ = s.store.UpdateNode(id, func(n domain.Node) (domain.Node, error) {
		n.LastLatencyMS = 0
		n.LastLatencyAt = time.Time{}
		return n, nil
	})
	select {
	case s.latencyQueue <- id:
	default:
		go func() { s.latencyQueue <- id }()
	}
}

func (s *Service) IncrementNodeTraffic(id string, up, down int64) (domain.Node, error) {
	return s.store.IncrementNodeTraffic(id, up, down)
}

func (s *Service) ProbeLatency(id string) (domain.Node, error) {
	node, err := s.store.GetNode(id)
	if err != nil {
		return domain.Node{}, err
	}
	_, _ = s.store.UpdateNode(id, func(n domain.Node) (domain.Node, error) {
		n.LastLatencyMS = 0
		n.LastLatencyAt = time.Time{}
		return n, nil
	})

	addr := net.JoinHostPort(node.Address, strconv.Itoa(node.Port))
	start := time.Now()
	dialer := net.Dialer{Timeout: 3 * time.Second}
	conn, err := dialer.Dial("tcp", addr)
	if err != nil {
		log.Printf("tcp ping failed for %s (%s): %v", node.ID, addr, err)
		return node, err
	}
	_ = conn.Close()
	ms := time.Since(start).Milliseconds()
	if ms <= 0 {
		ms = 1
	}
	updated, err := s.store.UpdateNode(id, func(n domain.Node) (domain.Node, error) {
		n.LastLatencyMS = ms
		n.LastLatencyAt = time.Now()
		return n, nil
	})
	if err != nil {
		return node, err
	}
	return updated, nil
}

func (s *Service) ProbeSpeed(id string) (domain.Node, error) {
	node, err := s.store.GetNode(id)
	if err != nil {
		return domain.Node{}, err
	}
	lastReported := 0.0
	_, _ = s.store.UpdateNode(id, func(n domain.Node) (domain.Node, error) {
		n.LastSpeedMbps = 0
		n.LastSpeedAt = time.Time{}
		n.LastSpeedError = ""
		return n, nil
	})
	// Launch measurement xray bound to reserved port.
	stop, err := s.startMeasurementXray(node)
	if err != nil {
		return domain.Node{}, err
	}
	defer stop()

	// Try multiple endpoints to measure download throughput via SOCKS5.
	ctx, cancel := context.WithTimeout(context.Background(), speedTestTimeout)
	defer cancel()
	progress := func(speedMBps float64) {
		if speedMBps <= 0 {
			return
		}
		smoothed := speedMBps
		if lastReported > 0 {
			if speedMBps > lastReported {
				smoothed = lastReported + (speedMBps-lastReported)*0.4
			} else {
				smoothed = lastReported*0.6 + speedMBps*0.4
			}
		}
		lastReported = smoothed
		_, updateErr := s.store.UpdateNode(id, func(n domain.Node) (domain.Node, error) {
			n.LastSpeedMbps = smoothed
			n.LastSpeedAt = time.Now()
			n.LastSpeedError = ""
			return n, nil
		})
		if updateErr != nil {
			log.Printf("record speed progress for %s failed: %v", id, updateErr)
		}
	}
	mbps, err := measureDownloadThroughSocks5(ctx, "127.0.0.1", measurementPort, progress)
	if err != nil {
		return domain.Node{}, err
	}
	finalSpeed := mbps
	if lastReported > 0 && finalSpeed < lastReported {
		finalSpeed = lastReported
	}
	return s.store.UpdateNode(id, func(n domain.Node) (domain.Node, error) {
		n.LastSpeedMbps = finalSpeed
		n.LastSpeedAt = time.Now()
		n.LastSpeedError = ""
		return n, nil
	})
}

func (s *Service) ListConfigs() []domain.Config {
	return s.store.ListConfigs()
}

func (s *Service) CreateConfig(cfg domain.Config) (domain.Config, error) {
	cfg = normalizeConfigInput(cfg)
	if cfg.SourceURL == "" && strings.TrimSpace(cfg.Payload) == "" {
		return domain.Config{}, errors.New("payload or sourceUrl must be provided")
	}
	var rawData []byte

	if cfg.SourceURL != "" && strings.TrimSpace(cfg.Payload) == "" {
		data, checksum, err := downloadResource(cfg.SourceURL)
		if err != nil {
			return domain.Config{}, fmt.Errorf("fetch config source: %w", err)
		}
		cfg.Payload = string(data)
		cfg.Checksum = checksum
		cfg.LastSyncedAt = time.Now()
		cfg.LastSyncError = ""
		rawData = data
	} else {
		cfg.Checksum = checksumBytes([]byte(cfg.Payload))
		cfg.LastSyncError = ""
		cfg.LastSyncedAt = time.Now()
		rawData = []byte(cfg.Payload)
	}

	created := s.store.CreateConfig(cfg)
	if len(rawData) > 0 {
		s.syncSubscriptionNodes(created, rawData)
	}
	return created, nil
}

func (s *Service) UpdateConfig(id string, mutate func(domain.Config) (domain.Config, error)) (domain.Config, error) {
	current, err := s.store.GetConfig(id)
	if err != nil {
		return domain.Config{}, err
	}

	updated, err := mutate(current)
	if err != nil {
		return domain.Config{}, err
	}
	updated = normalizeConfigInput(updated)
	updated.SourceURL = strings.TrimSpace(updated.SourceURL)

	var (
		payload   string
		checksum  string
		lastSync  time.Time
		lastError string
		rawData   []byte
	)

	switch {
	case updated.SourceURL == "":
		if strings.TrimSpace(updated.Payload) == "" {
			return domain.Config{}, errors.New("payload required when sourceUrl is empty")
		}
		payload = updated.Payload
		checksum = checksumBytes([]byte(payload))
		lastSync = time.Now()
		lastError = ""
	case updated.SourceURL != current.SourceURL || strings.TrimSpace(updated.Payload) == "":
		data, sum, fetchErr := downloadResource(updated.SourceURL)
		if fetchErr != nil {
			// preserve existing payload; surface error
			payload = current.Payload
			checksum = current.Checksum
			lastSync = current.LastSyncedAt
			lastError = fetchErr.Error()
		} else {
			payload = string(data)
			checksum = sum
			lastSync = time.Now()
			lastError = ""
			rawData = data
		}
	default:
		// keep current payload/checksum
		payload = current.Payload
		checksum = current.Checksum
		lastSync = current.LastSyncedAt
		lastError = current.LastSyncError
	}

	saved, err := s.store.UpdateConfig(id, func(cfg domain.Config) (domain.Config, error) {
		cfg.Name = updated.Name
		cfg.Format = updated.Format
		cfg.Payload = payload
		cfg.SourceURL = updated.SourceURL
		cfg.AutoUpdateInterval = updated.AutoUpdateInterval
		cfg.ExpireAt = updated.ExpireAt
		cfg.Checksum = checksum
		cfg.LastSyncError = lastError
		if !lastSync.IsZero() {
			cfg.LastSyncedAt = lastSync
		}
		return cfg, nil
	})
	if err != nil {
		return domain.Config{}, err
	}
	if lastError == "" {
		if len(rawData) == 0 {
			rawData = []byte(payload)
		}
		if len(rawData) > 0 {
			s.syncSubscriptionNodes(saved, rawData)
		}
	}
	return saved, nil
}

func (s *Service) DeleteConfig(id string) error {
	return s.store.DeleteConfig(id)
}

func (s *Service) IncrementConfigTraffic(id string, up, down int64) (domain.Config, error) {
	return s.store.IncrementConfigTraffic(id, up, down)
}

func (s *Service) RefreshConfig(id string) (domain.Config, error) {
	cfg, err := s.store.GetConfig(id)
	if err != nil {
		return domain.Config{}, err
	}
	return s.refreshConfigFromSource(cfg)
}

func (s *Service) SyncConfigNodes(id string) ([]domain.Node, error) {
	if _, err := s.RefreshConfig(id); err != nil {
		return nil, err
	}
	return s.ListNodesByConfig(id), nil
}

func (s *Service) AutoUpdateConfigs() {
	for _, cfg := range s.ListConfigs() {
		interval := cfg.AutoUpdateInterval
		if interval <= 0 {
			interval = defaultConfigSyncInterval
		}
		if time.Since(cfg.LastSyncedAt) >= interval {
			if _, err := s.refreshConfigFromSource(cfg); err != nil {
				log.Printf("config auto-update failed for %s: %v", cfg.ID, err)
			}
		}
	}
}

func (s *Service) ListGeo() []domain.GeoResource {
	return s.store.ListGeo()
}

func (s *Service) UpsertGeo(res domain.GeoResource) domain.GeoResource {
	return s.store.UpsertGeo(res)
}

func (s *Service) DeleteGeo(id string) error {
	return s.store.DeleteGeo(id)
}

func (s *Service) SyncGeoResources() {
	for _, res := range s.ListGeo() {
		if _, err := s.refreshGeoResource(res); err != nil {
			log.Printf("geo auto-update failed for %s: %v", res.ID, err)
		}
	}
}

func (s *Service) RefreshGeo(id string) (domain.GeoResource, error) {
	res, err := s.store.GetGeo(id)
	if err != nil {
		return domain.GeoResource{}, err
	}
	return s.refreshGeoResource(res)
}

func (s *Service) ListComponents() []domain.CoreComponent {
	s.ensureCoreComponents()
	components := s.store.ListComponents()
	snapshot := make([]domain.CoreComponent, 0, len(components))
	for _, comp := range components {
		sanitized := comp
		detected := false
		detectedInfo := componentInstallInfo{}
		if comp.InstallDir == "" {
			if info, ok := detectExistingComponentInstall(comp.Kind); ok {
				detected = true
				detectedInfo = info
				sanitized.InstallDir = info.dir
				sanitized.LastInstalledAt = info.lastInstalledAt
				sanitized.LastSyncError = ""
				sanitized.ArchiveType = defaultArchiveForKind(comp.Kind)
			}
		}
		if !detected && comp.InstallDir != "" {
			invalid := false
			if info, err := os.Stat(comp.InstallDir); err != nil || !info.IsDir() {
				invalid = true
			} else if comp.Kind == domain.ComponentXray {
				if _, err := findXrayBinary(comp.InstallDir); err != nil {
					invalid = true
				}
			}
			if invalid {
				sanitized.InstallDir = ""
				sanitized.LastSyncError = ""
				sanitized.LastVersion = ""
				sanitized.LastInstalledAt = time.Time{}
			}
		} else {
			sanitized.LastSyncError = ""
			sanitized.LastVersion = ""
			sanitized.LastInstalledAt = time.Time{}
		}
		if detected {
			_, _ = s.store.UpdateComponent(comp.ID, func(component domain.CoreComponent) (domain.CoreComponent, error) {
				component.InstallDir = detectedInfo.dir
				component.LastInstalledAt = detectedInfo.lastInstalledAt
				component.ArchiveType = defaultArchiveForKind(component.Kind)
				component.LastSyncError = ""
				if component.AutoUpdateInterval <= 0 {
					component.AutoUpdateInterval = defaultComponentUpdateInterval
				}
				return component, nil
			})
		}
		if sanitized.Meta != nil {
			metaCopy := make(map[string]string, len(sanitized.Meta))
			for k, v := range sanitized.Meta {
				if k == "_clearLastSyncError" {
					continue
				}
				metaCopy[k] = v
			}
			if len(metaCopy) > 0 {
				sanitized.Meta = metaCopy
			} else {
				sanitized.Meta = nil
			}
		}
		snapshot = append(snapshot, sanitized)
	}
	return snapshot
}

func (s *Service) ensureCoreComponents() {
	components := s.store.ListComponents()
	hasXray := false
	for _, comp := range components {
		switch comp.Kind {
		case domain.ComponentXray:
			hasXray = true
		}
		if hasXray {
			return
		}
	}
	if !hasXray {
		s.ensureCoreComponentRecord(domain.ComponentXray)
	}
}

func (s *Service) ensureCoreComponentRecord(kind domain.CoreComponentKind) {
	component := domain.CoreComponent{
		Kind:               kind,
		AutoUpdateInterval: defaultComponentUpdateInterval,
	}
	if info, ok := detectExistingComponentInstall(kind); ok {
		component.InstallDir = info.dir
		component.LastInstalledAt = info.lastInstalledAt
	}
	if _, err := s.CreateComponent(component); err != nil {
		log.Printf("ensure component %s failed: %v", kind, err)
	}
}

func detectExistingComponentInstall(kind domain.CoreComponentKind) (componentInstallInfo, bool) {
	dir := resolveInstallDir(domain.CoreComponent{Kind: kind})
	info, err := os.Stat(dir)
	if err != nil || !info.IsDir() {
		return componentInstallInfo{}, false
	}
	switch kind {
	case domain.ComponentXray:
		if _, err := findXrayBinary(dir); err != nil {
			return componentInstallInfo{}, false
		}
	}
	return componentInstallInfo{dir: dir, lastInstalledAt: info.ModTime()}, true
}

func normalizeComponentInput(component domain.CoreComponent) domain.CoreComponent {
	component.SourceURL = strings.TrimSpace(component.SourceURL)
	component.Name = strings.TrimSpace(component.Name)
	component.ArchiveType = normalizeArchiveType(component.ArchiveType)
	if component.Kind == "" {
		component.Kind = domain.ComponentGeneric
	}
	return component
}

func (s *Service) ensureComponentDefaults(component *domain.CoreComponent) error {
	if component.Kind == "" {
		component.Kind = domain.ComponentGeneric
	}
	def, defErr := componentDefaultFor(*component)
	if component.Name == "" {
		if def != nil && def.name != "" {
			component.Name = def.name
		} else {
			component.Name = string(component.Kind)
		}
	}
	if component.SourceURL == "" {
		if def != nil {
			component.SourceURL = def.fallbackURL()
		}
		if component.SourceURL == "" {
			if defErr != nil {
				return defErr
			}
			return fmt.Errorf("sourceUrl is required for component kind %s", component.Kind)
		}
	}
	if component.ArchiveType == "" {
		if def != nil && def.archiveType != "" {
			component.ArchiveType = def.archiveType
		} else {
			component.ArchiveType = inferArchiveTypeFromName(component.SourceURL)
		}
	}
	component.ArchiveType = normalizeArchiveType(component.ArchiveType)
	if err := validateArchiveType(component.ArchiveType); err != nil {
		return err
	}
	if component.AutoUpdateInterval <= 0 {
		component.AutoUpdateInterval = defaultComponentUpdateInterval
	}
	return nil
}

func (s *Service) CreateComponent(component domain.CoreComponent) (domain.CoreComponent, error) {
	component = normalizeComponentInput(component)
	if err := s.ensureComponentDefaults(&component); err != nil {
		return domain.CoreComponent{}, err
	}
	component.LastSyncError = ""
	if component.InstallDir == "" {
		component.LastInstalledAt = time.Time{}
	} else if component.LastInstalledAt.IsZero() {
		component.LastInstalledAt = time.Now()
	}
	if component.InstallDir == "" {
		component.LastVersion = ""
		component.Checksum = ""
	}
	created := s.store.CreateComponent(component)
	return created, nil
}

func (s *Service) UpdateComponent(id string, mutate func(domain.CoreComponent) (domain.CoreComponent, error)) (domain.CoreComponent, error) {
	current, err := s.store.GetComponent(id)
	if err != nil {
		return domain.CoreComponent{}, err
	}
	updated, err := mutate(current)
	if err != nil {
		return domain.CoreComponent{}, err
	}
	updated = normalizeComponentInput(updated)
	if updated.Name == "" {
		updated.Name = current.Name
	}
	if updated.Kind == "" {
		updated.Kind = current.Kind
	}
	if err := s.ensureComponentDefaults(&updated); err != nil {
		return domain.CoreComponent{}, err
	}
	return s.store.UpdateComponent(id, func(component domain.CoreComponent) (domain.CoreComponent, error) {
		component.Name = updated.Name
		component.Kind = updated.Kind
		component.SourceURL = updated.SourceURL
		component.ArchiveType = updated.ArchiveType
		component.AutoUpdateInterval = updated.AutoUpdateInterval
		return component, nil
	})
}

func (s *Service) DeleteComponent(id string) error {
	return s.store.DeleteComponent(id)
}

func resolveInstallDir(component domain.CoreComponent) string {
	base := filepath.Join(artifactsRoot, "core")
	switch component.Kind {
	case domain.ComponentXray:
		return filepath.Join(base, "xray")
	case domain.ComponentGeo:
		return filepath.Join(base, "geo")
	default:
		name := sanitizeID(component.Name)
		if name == "" {
			name = sanitizeID(component.ID)
		}
		if name == "" {
			name = "component"
		}
		return filepath.Join(base, name)
	}
}

func (s *Service) InstallComponent(id string) (domain.CoreComponent, error) {
	component, err := s.store.GetComponent(id)
	if err != nil {
		return domain.CoreComponent{}, err
	}

	component = normalizeComponentInput(component)
	def, _ := componentDefaultFor(component)

	var (
		downloadURL     = strings.TrimSpace(component.SourceURL)
		archiveType     = component.ArchiveType
		releaseInfo     releaseAssetInfo
		repo            string
		assetCandidates []string
	)

	if def != nil {
		repo = def.repo
		assetCandidates = def.assetCandidates
		if archiveType == "" {
			archiveType = def.archiveType
		}
		if downloadURL == "" {
			downloadURL = def.fallbackURL()
		}
	}

	if repo != "" && len(assetCandidates) > 0 {
		info, releaseErr := fetchLatestReleaseAsset(repo, assetCandidates)
		if releaseErr == nil && info.DownloadURL != "" {
			releaseInfo = info
			downloadURL = info.DownloadURL
			archiveType = inferArchiveTypeFromName(info.Name)
		} else {
			fallbackInfo, fallbackErr := resolveComponentAssetFallback(component.Kind, repo, assetCandidates)
			if fallbackErr == nil && fallbackInfo.DownloadURL != "" {
				releaseInfo = fallbackInfo
				downloadURL = fallbackInfo.DownloadURL
				archiveType = inferArchiveTypeFromName(fallbackInfo.Name)
			} else {
				if releaseErr != nil && !errors.Is(releaseErr, errReleaseAssetNotFound) {
					log.Printf("github release lookup failed for %s: %v", repo, releaseErr)
				}
				if fallbackErr != nil && !errors.Is(fallbackErr, errReleaseAssetNotFound) {
					log.Printf("github release fallback failed for %s: %v", repo, fallbackErr)
				}
			}
		}
	}

	if downloadURL == "" {
		return domain.CoreComponent{}, errors.New("component sourceUrl is empty")
	}

	if archiveType == "" {
		archiveType = inferArchiveTypeFromName(downloadURL)
	}
	archiveType = normalizeArchiveType(archiveType)
	if err := validateArchiveType(archiveType); err != nil {
		return domain.CoreComponent{}, err
	}

	data, checksum, err := downloadResource(downloadURL)
	if err != nil {
		updated, updateErr := s.store.UpdateComponent(id, func(comp domain.CoreComponent) (domain.CoreComponent, error) {
			comp.LastSyncError = err.Error()
			comp.LastInstalledAt = time.Now()
			if comp.Meta == nil {
				comp.Meta = map[string]string{}
			}
			comp.Meta["downloadUrl"] = downloadURL
			return comp, nil
		})
		if updateErr != nil {
			return domain.CoreComponent{}, updateErr
		}
		return updated, err
	}

	targetDir := resolveInstallDir(component)
	legacyDir := filepath.Join(artifactsRoot, "components", sanitizeID(component.ID))
	_ = os.RemoveAll(legacyDir)

	if component.Kind == domain.ComponentXray {
		s.terminateXrayProcess(true)
	}

	installDir, installErr := installComponentArchive(targetDir, archiveType, data)
	if installErr != nil {
		updated, updateErr := s.store.UpdateComponent(id, func(comp domain.CoreComponent) (domain.CoreComponent, error) {
			comp.LastSyncError = installErr.Error()
			comp.LastInstalledAt = time.Now()
			return comp, nil
		})
		if updateErr != nil {
			return domain.CoreComponent{}, updateErr
		}
		return updated, installErr
	}

	if component.Kind == domain.ComponentXray {
		if err := cleanupXrayInstall(installDir); err != nil {
			log.Printf("cleanup xray install failed: %v", err)
		}
	}

	version := releaseInfo.Version
	if version == "" {
		version = deriveComponentVersion(downloadURL, checksum)
	}
	installed, err := s.store.UpdateComponent(id, func(comp domain.CoreComponent) (domain.CoreComponent, error) {
		comp.InstallDir = installDir
		comp.LastInstalledAt = time.Now()
		comp.LastVersion = version
		comp.Checksum = checksum
		comp.LastSyncError = ""
		comp.ArchiveType = archiveType
		if comp.SourceURL == "" {
			if def != nil && def.fallbackURL() != "" {
				comp.SourceURL = def.fallbackURL()
			} else {
				comp.SourceURL = downloadURL
			}
		}
		if comp.Meta == nil {
			comp.Meta = map[string]string{}
		}
		comp.Meta["_clearLastSyncError"] = "1"
		comp.Meta["downloadUrl"] = downloadURL
		if releaseInfo.Name != "" {
			comp.Meta["assetName"] = releaseInfo.Name
		}
		if releaseInfo.Version != "" {
			comp.Meta["releaseTag"] = releaseInfo.Version
		}
		if releaseInfo.Size > 0 {
			comp.Meta["assetSizeBytes"] = strconv.FormatInt(releaseInfo.Size, 10)
		}
		if def != nil && def.repo != "" {
			comp.Meta["repo"] = def.repo
		}
		return comp, nil
	})
	if err != nil {
		return domain.CoreComponent{}, err
	}

	if installed.Kind == domain.ComponentXray {
		if err := s.regenerateXrayConfig(s.ActiveXrayNodeID()); err != nil && !errors.Is(err, errXrayComponentNotInstalled) {
			log.Printf("regenerate xray config failed: %v", err)
		}
		if s.IsXrayEnabled() {
			if err := s.RestartXray(" "); err != nil {
				log.Printf("restart xray after install failed: %v", err)
			}
		}
	}
	return installed, nil
}

func (s *Service) AutoUpdateComponents() {
	for _, component := range s.ListComponents() {
		interval := component.AutoUpdateInterval
		if interval <= 0 {
			continue
		}
		if component.LastInstalledAt.IsZero() || time.Since(component.LastInstalledAt) >= interval {
			if _, err := s.InstallComponent(component.ID); err != nil {
				log.Printf("component auto-update failed for %s: %v", component.ID, err)
			}
		}
	}
}

func (s *Service) ListTrafficRules() []domain.TrafficRule {
	return s.store.ListTrafficRules()
}

func (s *Service) CreateTrafficRule(rule domain.TrafficRule) domain.TrafficRule {
	return s.store.CreateTrafficRule(rule)
}

func (s *Service) UpdateTrafficRule(id string, mutate func(domain.TrafficRule) (domain.TrafficRule, error)) (domain.TrafficRule, error) {
	return s.store.UpdateTrafficRule(id, mutate)
}

func (s *Service) DeleteTrafficRule(id string) error {
	return s.store.DeleteTrafficRule(id)
}

func (s *Service) UpdateTrafficProfile(mutator func(domain.TrafficProfile) (domain.TrafficProfile, error)) (domain.TrafficProfile, error) {
	return s.store.UpdateTrafficProfile(mutator)
}

func (s *Service) GetTrafficProfile() domain.TrafficProfile {
	return s.store.GetTrafficProfile()
}

func (s *Service) Snapshot() domain.ServiceState {
	return s.store.Snapshot()
}

func (s *Service) Errors() (error, error, error, error, error) {
	return s.store.Errors()
}

func (s *Service) RestartXray(activeNodeID string) error {
	s.xrayMu.Lock()
	defer s.xrayMu.Unlock()

	runtime, chosenNodeID, err := s.prepareXrayRuntime(activeNodeID)
	if err != nil {
		return err
	}

	if s.xrayCmd != nil {
		_ = s.xrayCmd.Process.Kill()
		go s.xrayCmd.Wait()
		s.xrayCmd = nil
	}

	cmd := exec.Command(runtime.Binary, "-c", runtime.Config)
	coreDir := filepath.Dir(runtime.Binary)
	cmd.Dir = coreDir
	cmd.Env = prependPath(os.Environ(), coreDir)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		return err
	}
	s.xrayCmd = cmd
	s.xrayRuntime = runtime
	s.xrayRuntime.ActiveNodeID = chosenNodeID

	if chosenNodeID != "" {
		s.recordNodeSelection(chosenNodeID)
		log.Printf("xray selector active node: %s", chosenNodeID)
	}

	go func(c *exec.Cmd) {
		_ = c.Wait()
		s.xrayMu.Lock()
		if s.xrayCmd == c {
			s.xrayCmd = nil
		}
		s.xrayMu.Unlock()
	}(cmd)

	log.Printf("xray started on %s", runtime.Config)
	return nil
}

func (s *Service) regenerateXrayConfig(activeNodeID string) error {
	runtime, chosenNodeID, err := s.prepareXrayRuntime(activeNodeID)
	if err != nil {
		return err
	}

	s.xrayMu.Lock()
	s.xrayRuntime = runtime
	s.xrayRuntime.ActiveNodeID = chosenNodeID
	s.xrayMu.Unlock()
	return nil
}

func (s *Service) ActiveXrayNodeID() string {
	s.xrayMu.Lock()
	active := s.xrayRuntime.ActiveNodeID
	s.xrayMu.Unlock()
	if active != "" {
		return active
	}
	comp, err := s.getComponentByKind(domain.ComponentXray)
	if err != nil {
		return ""
	}
	if comp.Meta != nil {
		return comp.Meta["activeNodeId"]
	}
	return ""
}

func (s *Service) LastSelectedNodeID() string {
	var latest time.Time
	var latestID string
	for _, node := range s.ListNodes() {
		if node.ID == "" {
			continue
		}
		if node.LastSelectedAt.After(latest) {
			latest = node.LastSelectedAt
			latestID = node.ID
		}
	}
	return latestID
}

func (s *Service) XrayStatus() XrayStatus {
	s.xrayMu.Lock()
	defer s.xrayMu.Unlock()
	st := XrayStatus{
		Enabled:      s.xrayEnabled,
		Running:      false,
		PID:          0,
		Binary:       s.xrayRuntime.Binary,
		Config:       s.xrayRuntime.Config,
		GeoIP:        s.xrayRuntime.GeoIP,
		GeoSite:      s.xrayRuntime.GeoSite,
		InboundPort:  s.xrayRuntime.InboundPort,
		ActiveNodeID: s.xrayRuntime.ActiveNodeID,
	}
	if s.xrayCmd != nil && s.xrayCmd.Process != nil {
		if s.xrayCmd.ProcessState == nil || !s.xrayCmd.ProcessState.Exited() {
			st.Running = true
			st.PID = s.xrayCmd.Process.Pid
		}
	}
	if strings.TrimSpace(st.Binary) == "" {
		if comp, err := s.getComponentByKind(domain.ComponentXray); err == nil {
			if comp.InstallDir != "" {
				if bin, err := findXrayBinary(comp.InstallDir); err == nil {
					st.Binary = bin
				}
			}
		}
	}
	return st
}

// SwitchXrayNodeAsync triggers a non-blocking xray restart toward the
// desired active node. It updates in-memory status optimistically and
// performs the heavy work in a background goroutine to avoid request
// latency on the /nodes/:id/select API.
func (s *Service) SwitchXrayNodeAsync(activeNodeID string) {
	if activeNodeID != "" {
		s.recordNodeSelection(activeNodeID)
	}
	// Optimistic update so前端能立刻看到目标节点
	s.xrayMu.Lock()
	s.xrayRuntime.ActiveNodeID = activeNodeID
	enabled := s.xrayEnabled
	s.xrayMu.Unlock()

	if err := s.persistXrayActiveNode(activeNodeID); err != nil && !errors.Is(err, errXrayComponentNotInstalled) {
		log.Printf("persist active node failed: %v", err)
	}

	if !enabled {
		return
	}

	go func(id string) {
		if err := s.RestartXray(id); err != nil {
			log.Printf("async restart xray failed: %v", err)
		}
	}(activeNodeID)
}

func (s *Service) recordNodeSelection(nodeID string) {
	if nodeID == "" {
		return
	}
	if _, err := s.store.UpdateNode(nodeID, func(n domain.Node) (domain.Node, error) {
		n.LastSelectedAt = time.Now()
		return n, nil
	}); err != nil {
		log.Printf("record node selection failed for %s: %v", nodeID, err)
	}
}

func (s *Service) terminateXrayProcess(keepEnabled bool) {
	s.xrayMu.Lock()
	if s.xrayCmd != nil {
		cmd := s.xrayCmd
		_ = cmd.Process.Kill()
		s.xrayCmd = nil
		go cmd.Wait()
	}
	if !keepEnabled {
		s.xrayEnabled = false
	}
	s.xrayRuntime.InboundPort = 0
	s.xrayMu.Unlock()
}

func (s *Service) IsXrayEnabled() bool {
	s.xrayMu.Lock()
	defer s.xrayMu.Unlock()
	return s.xrayEnabled
}

func (s *Service) EnableXray(activeNodeID string) error {
	s.xrayMu.Lock()
	s.xrayEnabled = true
	s.xrayMu.Unlock()

	if err := s.updateXrayMeta(func(component domain.CoreComponent) (domain.CoreComponent, error) {
		if component.Meta == nil {
			component.Meta = map[string]string{}
		}
		component.Meta["enabled"] = "true"
		return component, nil
	}); err != nil {
		s.xrayMu.Lock()
		s.xrayEnabled = false
		s.xrayMu.Unlock()
		return err
	}

	if err := s.RestartXray(activeNodeID); err != nil {
		s.xrayMu.Lock()
		s.xrayEnabled = false
		s.xrayMu.Unlock()
		if updErr := s.updateXrayMeta(func(component domain.CoreComponent) (domain.CoreComponent, error) {
			if component.Meta == nil {
				component.Meta = map[string]string{}
			}
			component.Meta["enabled"] = "false"
			return component, nil
		}); updErr != nil && !errors.Is(updErr, errXrayComponentNotInstalled) {
			log.Printf("xray enable rollback meta failed: %v", updErr)
		}
		return err
	}
	if err := s.applySystemProxy(s.SystemProxySettings()); err != nil && !errors.Is(err, ErrProxyUnsupported) {
		log.Printf("apply system proxy failed: %v", err)
	}
	return nil
}

func (s *Service) DisableXray() error {
	s.terminateXrayProcess(false)

	if err := s.updateXrayMeta(func(component domain.CoreComponent) (domain.CoreComponent, error) {
		if component.Meta == nil {
			component.Meta = map[string]string{}
		}
		component.Meta["enabled"] = "false"
		return component, nil
	}); err != nil && !errors.Is(err, errXrayComponentNotInstalled) {
		return err
	}
	settings := s.SystemProxySettings()
	settings.Enabled = false
	if err := s.applySystemProxy(settings); err != nil && !errors.Is(err, ErrProxyUnsupported) {
		log.Printf("disable system proxy failed: %v", err)
	}
	return nil
}

func (s *Service) persistXrayActiveNode(activeNodeID string) error {
	return s.updateXrayMeta(func(component domain.CoreComponent) (domain.CoreComponent, error) {
		if component.Meta == nil {
			component.Meta = map[string]string{}
		}
		component.Meta["activeNodeId"] = activeNodeID
		return component, nil
	})
}

func (s *Service) updateXrayMeta(mutate func(domain.CoreComponent) (domain.CoreComponent, error)) error {
	comp, err := s.getComponentByKind(domain.ComponentXray)
	if err != nil {
		return err
	}
	_, err = s.store.UpdateComponent(comp.ID, mutate)
	return err
}

func normalizeConfigInput(cfg domain.Config) domain.Config {
	cfg.SourceURL = strings.TrimSpace(cfg.SourceURL)
	cfg.Payload = strings.TrimSpace(cfg.Payload)
	if cfg.SourceURL == "" && isLikelyURL(cfg.Payload) {
		cfg.SourceURL = cfg.Payload
		cfg.Payload = ""
	}
	if cfg.Format == "" {
		if format, ok := inferFormatFromPayload(cfg.Payload); ok {
			cfg.Format = format
		}
	}
	return cfg
}

func (s *Service) refreshConfigFromSource(cfg domain.Config) (domain.Config, error) {
	if cfg.SourceURL == "" {
		updated, err := s.store.UpdateConfig(cfg.ID, func(c domain.Config) (domain.Config, error) {
			c.LastSyncedAt = time.Now()
			c.LastSyncError = ""
			return c, nil
		})
		if err != nil {
			return domain.Config{}, err
		}
		return updated, nil
	}

	data, checksum, err := downloadResource(cfg.SourceURL)
	if err != nil {
		updated, updateErr := s.store.UpdateConfig(cfg.ID, func(c domain.Config) (domain.Config, error) {
			c.LastSyncedAt = time.Now()
			c.LastSyncError = err.Error()
			return c, nil
		})
		if updateErr != nil {
			return domain.Config{}, updateErr
		}
		return updated, err
	}

	updated, err := s.store.UpdateConfig(cfg.ID, func(c domain.Config) (domain.Config, error) {
		c.Payload = string(data)
		c.Checksum = checksum
		c.LastSyncedAt = time.Now()
		c.LastSyncError = ""
		return c, nil
	})
	if err != nil {
		return domain.Config{}, err
	}
	s.syncSubscriptionNodes(updated, data)
	return updated, nil
}

func (s *Service) syncSubscriptionNodes(cfg domain.Config, raw []byte) {
	if cfg.ID == "" {
		return
	}
	nodes, err := parseSubscriptionNodes(raw)
	if err != nil {
		log.Printf("subscription parse failed for config %s: %v", cfg.ID, err)
		s.store.ReplaceNodesForConfig(cfg.ID, nil)
		return
	}
	if len(nodes) == 0 {
		s.store.ReplaceNodesForConfig(cfg.ID, nil)
		return
	}
	for i := range nodes {
		nodes[i].SourceConfigID = cfg.ID
		if cfg.Name != "" {
			nodes[i].Tags = uniqueStrings(append(nodes[i].Tags, cfg.Name))
		}
		if strings.TrimSpace(nodes[i].Name) == "" {
			base := cfg.Name
			if base == "" {
				base = "subscription"
			}
			nodes[i].Name = fmt.Sprintf("%s-%d", base, i+1)
		}
	}
	created := s.store.ReplaceNodesForConfig(cfg.ID, nodes)
	log.Printf("synced %d nodes from subscription config %s", len(created), cfg.ID)
}

func (s *Service) refreshGeoResource(res domain.GeoResource) (domain.GeoResource, error) {
	if res.SourceURL == "" {
		res.LastSynced = time.Now()
		res.LastSyncError = ""
		return s.store.UpsertGeo(res), nil
	}

	data, checksum, err := downloadResource(res.SourceURL)
	if err != nil {
		res.LastSynced = time.Now()
		res.LastSyncError = err.Error()
		return s.store.UpsertGeo(res), err
	}

	changed := res.Checksum != checksum || res.FileSizeBytes != int64(len(data))
	path := res.ArtifactPath
	if changed {
		path, err = writeGeoArtifact(res.ID, data)
		if err != nil {
			res.LastSynced = time.Now()
			res.LastSyncError = fmt.Sprintf("write artifact: %v", err)
			return s.store.UpsertGeo(res), err
		}
	}

	res.Checksum = checksum
	res.FileSizeBytes = int64(len(data))
	res.LastSynced = time.Now()
	res.LastSyncError = ""
	res.ArtifactPath = path
	if changed && (res.Version == "" || strings.HasPrefix(res.Version, "auto-")) {
		res.Version = "auto-" + checksum[:12]
	}
	return s.store.UpsertGeo(res), nil
}

func shouldRetryDownload(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) {
		return true
	}
	msg := err.Error()
	if strings.Contains(msg, "EOF") || strings.Contains(msg, "http2:") || strings.Contains(msg, "protocol error") {
		return true
	}
	var urlErr *url.Error
	if errors.As(err, &urlErr) && urlErr != nil {
		return shouldRetryDownload(urlErr.Err)
	}
	var opErr *net.OpError
	if errors.As(err, &opErr) && opErr != nil {
		return shouldRetryDownload(opErr.Err)
	}
	return false
}

func shouldRetryHTTP11(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) {
		return true
	}
	var recordErr *tls.RecordHeaderError
	if errors.As(err, &recordErr) && recordErr != nil {
		return true
	}
	var urlErr *url.Error
	if errors.As(err, &urlErr) && urlErr != nil {
		return shouldRetryHTTP11(urlErr.Err)
	}
	var opErr *net.OpError
	if errors.As(err, &opErr) && opErr != nil {
		return shouldRetryHTTP11(opErr.Err)
	}
	msg := strings.ToLower(err.Error())
	if strings.Contains(msg, "http2:") || strings.Contains(msg, "protocol error") || strings.Contains(msg, "eof") {
		return true
	}
	return false
}

func downloadResource(source string) ([]byte, string, error) {
	if source == "" {
		return nil, "", errors.New("empty source url")
	}

	doRequest := func(client *http.Client) (*http.Response, error) {
		req, reqErr := http.NewRequest(http.MethodGet, source, nil)
		if reqErr != nil {
			return nil, reqErr
		}
		req.Header.Set("User-Agent", githubUserAgent())
		return client.Do(req)
	}

	resp, err := doRequest(httpClientDirect)
	if err != nil && shouldRetryHTTP11(err) {
		resp, err = doRequest(httpClientDirectHTTP11)
	}
	if err != nil {
		resp, err = doRequest(httpClient)
		if err != nil && shouldRetryHTTP11(err) {
			resp, err = doRequest(httpClientHTTP11)
		}
	}
	if err != nil {
		return nil, "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, "", fmt.Errorf("unexpected status %s", resp.Status)
	}

	limited := io.LimitReader(resp.Body, maxDownloadSize+1)
	data, err := io.ReadAll(limited)
	if err != nil {
		return nil, "", err
	}
	if int64(len(data)) > maxDownloadSize {
		return nil, "", fmt.Errorf("resource exceeds max size of %d bytes", maxDownloadSize)
	}

	return data, checksumBytes(data), nil
}

func checksumBytes(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func installComponentArchive(targetDir, archiveType string, data []byte) (string, error) {
	archiveType = normalizeArchiveType(archiveType)
	if err := validateArchiveType(archiveType); err != nil {
		return "", err
	}

	if targetDir == "" {
		return "", errors.New("install dir is empty")
	}
	if err := os.MkdirAll(filepath.Dir(targetDir), 0o755); err != nil {
		return "", err
	}
	tmpDir := targetDir + ".tmp"

	if err := os.RemoveAll(tmpDir); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return "", err
	}
	if err := os.MkdirAll(tmpDir, 0o755); err != nil {
		return "", err
	}

	var installErr error
	switch archiveType {
	case "raw":
		target := filepath.Join(tmpDir, componentFile)
		if err := os.WriteFile(target, data, 0o755); err != nil {
			installErr = err
		}
	case "zip":
		reader, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
		if err != nil {
			installErr = err
			break
		}
		for _, file := range reader.File {
			if err := extractZipEntry(file, tmpDir); err != nil {
				installErr = err
				break
			}
		}
	case "tar.gz", "tgz":
		if err := extractTarGz(data, tmpDir); err != nil {
			installErr = err
		}
	default:
		installErr = fmt.Errorf("unsupported archive type %s", archiveType)
	}

	if installErr != nil {
		_ = os.RemoveAll(tmpDir)
		return "", installErr
	}

	if err := os.RemoveAll(targetDir); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return "", err
	}
	if err := os.Rename(tmpDir, targetDir); err != nil {
		return "", err
	}
	return targetDir, nil
}

func extractZipEntry(file *zip.File, baseDir string) error {
	cleanName := filepath.Clean(file.Name)
	if cleanName == "." {
		return nil
	}
	targetPath, err := safeJoin(baseDir, cleanName)
	if err != nil {
		return err
	}
	if file.FileInfo().IsDir() {
		return os.MkdirAll(targetPath, 0o755)
	}

	if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
		return err
	}

	rc, err := file.Open()
	if err != nil {
		return err
	}
	defer rc.Close()

	mode := file.Mode()
	if mode == 0 {
		mode = 0o755
	}

	out, err := os.OpenFile(targetPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, mode)
	if err != nil {
		return err
	}
	defer out.Close()

	if _, err := io.Copy(out, rc); err != nil {
		return err
	}
	return nil
}

func extractTarGz(data []byte, baseDir string) error {
	gr, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return err
	}
	defer gr.Close()
	return extractTar(gr, baseDir)
}

func extractTar(r io.Reader, baseDir string) error {
	tr := tar.NewReader(r)
	for {
		header, err := tr.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return err
		}

		cleanName := filepath.Clean(header.Name)
		if cleanName == "." {
			continue
		}
		targetPath, err := safeJoin(baseDir, cleanName)
		if err != nil {
			return err
		}

		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(targetPath, 0o755); err != nil {
				return err
			}
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
				return err
			}
			out, err := os.OpenFile(targetPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, fs.FileMode(header.Mode))
			if err != nil {
				return err
			}
			if _, err := io.Copy(out, tr); err != nil {
				out.Close()
				return err
			}
			out.Close()
		default:
			return fmt.Errorf("unsupported tar entry type %d", header.Typeflag)
		}
	}
	return nil
}

func safeJoin(baseDir, rel string) (string, error) {
	target := filepath.Join(baseDir, rel)
	baseDirClean := filepath.Clean(baseDir)
	targetClean := filepath.Clean(target)
	if !strings.HasPrefix(targetClean, baseDirClean+string(os.PathSeparator)) && targetClean != baseDirClean {
		return "", fmt.Errorf("invalid path traversal detected: %s", rel)
	}
	return targetClean, nil
}

func normalizeArchiveType(input string) string {
	v := strings.TrimSpace(strings.ToLower(input))
	switch v {
	case "", "auto":
		return "zip"
	case "tgz":
		return "tar.gz"
	case "gz":
		return "tar.gz"
	default:
		return v
	}
}

func inferArchiveTypeFromName(name string) string {
	trimmed := strings.ToLower(strings.TrimSpace(name))
	switch {
	case strings.HasSuffix(trimmed, ".tar.gz"):
		return "tar.gz"
	case strings.HasSuffix(trimmed, ".tgz"):
		return "tgz"
	case strings.HasSuffix(trimmed, ".zip"):
		return "zip"
	default:
		return "raw"
	}
}

func validateArchiveType(t string) error {
	switch t {
	case "zip", "tar.gz", "tgz", "raw":
		return nil
	default:
		return fmt.Errorf("unsupported archive type: %s", t)
	}
}

func deriveComponentVersion(sourceURL, checksum string) string {
	trimmed := strings.TrimSpace(sourceURL)
	if trimmed == "" {
		return "auto-" + checksum[:12]
	}
	trimmed = strings.TrimSuffix(trimmed, "/")
	base := filepath.Base(trimmed)
	base = strings.TrimSuffix(base, ".zip")
	base = strings.TrimSuffix(base, ".tar.gz")
	base = strings.TrimSuffix(base, ".tgz")
	base = strings.TrimSuffix(base, ".gz")
	if base == "" || base == "." || base == "/" {
		return "auto-" + checksum[:12]
	}
	return base
}

func writeGeoArtifact(id string, data []byte) (string, error) {
	if id == "" {
		return "", errors.New("empty geo id")
	}
	dir := filepath.Join(artifactsRoot, geoDir)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	target := filepath.Join(dir, sanitizeID(id)+".bin")
	tmp := target + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return "", err
	}
	if err := os.Rename(tmp, target); err != nil {
		return "", err
	}
	return target, nil
}

func sanitizeID(id string) string {
	replacer := strings.NewReplacer("/", "_", "\\", "_", ":", "_", "..", "_")
	return replacer.Replace(id)
}

func cleanupXrayInstall(dir string) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}
	allowed := map[string]struct{}{
		"xray":                {},
		"xray.exe":            {},
		"geoip.dat":           {},
		"geosite.dat":         {},
		"config.json":         {},
		"config-measure.json": {},
		"license":             {},
		"license.txt":         {},
	}
	for _, entry := range entries {
		name := entry.Name()
		lower := strings.ToLower(name)
		if _, ok := allowed[lower]; ok {
			continue
		}
		path := filepath.Join(dir, name)
		if err := os.RemoveAll(path); err != nil {
			return err
		}
	}
	return nil
}

type socksTarget struct {
	host  string
	port  int
	path  string
	tls   bool
	bytes int64
}

// startMeasurementXray starts the measurement xray process bound to the reserved port.
// Returns a stop function to terminate the process and clean up.
func (s *Service) startMeasurementXray(node domain.Node) (func(), error) {
	s.measureMu.Lock()
	unlocked := false
	release := func() {
		if !unlocked {
			s.measureMu.Unlock()
			unlocked = true
		}
	}

	comp, err := s.getComponentByKind(domain.ComponentXray)
	if err != nil {
		release()
		return nil, err
	}
	if comp.InstallDir == "" {
		release()
		return nil, fmt.Errorf("xray component not installed")
	}
	coreDir := filepath.Join(artifactsRoot, "core", xrayCoreDirName)
	coreDirAbs, err := filepath.Abs(coreDir)
	if err != nil {
		release()
		return nil, err
	}
	if err := os.MkdirAll(coreDirAbs, 0o755); err != nil {
		release()
		return nil, err
	}
	binary, err := findXrayBinary(comp.InstallDir)
	if err != nil {
		release()
		return nil, err
	}
	binaryDest := filepath.Join(coreDirAbs, filepath.Base(binary))
	if err := copyFileIfChanged(binary, binaryDest); err != nil {
		release()
		return nil, err
	}
	if err := os.Chmod(binaryDest, 0o755); err != nil {
		release()
		return nil, err
	}

	preparedNode, err := s.prepareNodeForXray(node)
	if err != nil {
		release()
		return nil, err
	}
	// 插件路径已废弃：不再构建或确保任何插件二进制。
	cfgBytes, _, err := buildMeasurementXrayConfig(preparedNode, measurementPort)
	if err != nil {
		release()
		return nil, err
	}
	cfgPath := filepath.Join(coreDirAbs, "config-measure.json")
	// remove historical speedtest configs to avoid clutter
	_ = filepath.WalkDir(coreDirAbs, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d == nil {
			return nil
		}
		if !strings.HasPrefix(filepath.Base(path), "config-speedtest-") {
			return nil
		}
		if strings.HasSuffix(path, ".json") {
			_ = os.Remove(path)
		}
		return nil
	})
	if err := writeAtomic(cfgPath, cfgBytes, 0o644); err != nil {
		release()
		return nil, err
	}

	binaryDestAbs, err := filepath.Abs(binaryDest)
	if err != nil {
		release()
		return nil, err
	}
	cfgPathAbs, err := filepath.Abs(cfgPath)
	if err != nil {
		release()
		return nil, err
	}

	cmd := exec.Command(binaryDestAbs, "-c", cfgPathAbs)
	cmd.Dir = coreDirAbs
	cmd.Env = prependPath(os.Environ(), coreDirAbs)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		release()
		return nil, fmt.Errorf("start measurement xray: %w", err)
	}

	// Wait for the SOCKS5 port to become ready with retries.
	readyCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	var lastErr error
	for {
		select {
		case <-readyCtx.Done():
			_ = cmd.Process.Kill()
			go cmd.Wait()
			release()
			return nil, fmt.Errorf("xray measurement not ready: %w (last error: %v)", readyCtx.Err(), lastErr)
		default:
			c, err := net.DialTimeout("tcp", net.JoinHostPort("127.0.0.1", strconv.Itoa(measurementPort)), 200*time.Millisecond)
			if err == nil {
				_ = c.Close()
				stop := func() {
					defer release()
					if cmd.ProcessState == nil || !cmd.ProcessState.Exited() {
						_ = cmd.Process.Signal(os.Interrupt)
						done := make(chan struct{})
						go func() {
							_ = cmd.Wait()
							close(done)
						}()
						select {
						case <-done:
						case <-time.After(2 * time.Second):
							_ = cmd.Process.Kill()
							<-done
						}
					}
					time.Sleep(500 * time.Millisecond)
				}
				return stop, nil
			}
			lastErr = err
			time.Sleep(100 * time.Millisecond)
		}
	}
}

func (s *Service) prepareNodesForXray(nodes []domain.Node) ([]domain.Node, error) {
	prepared := make([]domain.Node, 0, len(nodes))
	for _, node := range nodes {
		p, err := s.prepareNodeForXray(node)
		if err != nil {
			return nil, err
		}
		prepared = append(prepared, p)
	}
	return prepared, nil
}

func (s *Service) prepareNodeForXray(node domain.Node) (domain.Node, error) {
	prepared := node
	if node.Security != nil {
		copySec := cloneNodeSecurity(node.Security)
		prepared.Security = copySec
	}
	ensureShadowsocksHTTPHeaders(&prepared)
	return prepared, nil
}

func cloneNodeSecurity(sec *domain.NodeSecurity) *domain.NodeSecurity {
	if sec == nil {
		return nil
	}
	copySec := *sec
	if sec.ALPN != nil {
		copySec.ALPN = append([]string(nil), sec.ALPN...)
	}
	return &copySec
}

const (
	shadowsocksHTTPMethodKey   = "__method"
	shadowsocksHTTPVersionKey  = "__version"
	defaultHTTPObfsUserAgent   = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/118.0.0.0 Safari/537.36"
	defaultHTTPObfsAcceptEnc   = "gzip, deflate"
	defaultHTTPObfsConnection  = "keep-alive"
	defaultHTTPObfsHTTPVersion = "1.1"
)

func ensureShadowsocksHTTPHeaders(node *domain.Node) {
	if node.Protocol != domain.ProtocolShadowsocks {
		return
	}

	if node.Security == nil {
		node.Security = &domain.NodeSecurity{}
	}

	var hostFromOpts, pathFromOpts string
	if node.Security != nil {
		_, options := parseShadowsocksPluginOptions(node.Security.Plugin, node.Security.PluginOpts)
		if len(options) > 0 {
			hostFromOpts = firstNonEmpty(options["obfs-host"], options["host"])
			pathFromOpts = firstNonEmpty(options["obfs-uri"], options["path"])
		}
		node.Security.Plugin = ""
		node.Security.PluginOpts = ""
	}

	if node.Transport == nil {
		node.Transport = &domain.NodeTransport{}
	}

	host := strings.TrimSpace(hostFromOpts)
	if host == "" {
		host = strings.TrimSpace(node.Transport.Host)
	}
	if host == "" {
		host = strings.TrimSpace(node.Address)
	}
	if host == "" {
		return
	}

	path := strings.TrimSpace(pathFromOpts)
	if path == "" {
		path = strings.TrimSpace(node.Transport.Path)
	}
	if path == "" {
		path = "/"
	}

	node.Transport.Type = "tcp"
	node.Transport.Host = host
	node.Transport.Path = path
	// 设置默认 HTTP 请求头，兼容常见 obfs 配置
	if node.Transport.Headers == nil {
		node.Transport.Headers = map[string]string{}
	}
	// Method/Version
	if strings.TrimSpace(node.Transport.Headers[shadowsocksHTTPMethodKey]) == "" {
		node.Transport.Headers[shadowsocksHTTPMethodKey] = "GET"
	}
	if strings.TrimSpace(node.Transport.Headers[shadowsocksHTTPVersionKey]) == "" {
		node.Transport.Headers[shadowsocksHTTPVersionKey] = defaultHTTPObfsHTTPVersion
	}
	// Common headers: Host is injected via transport.Host; add UA/Accept-Encoding/Connection/Pragma
	if _, ok := node.Transport.Headers["User-Agent"]; !ok {
		node.Transport.Headers["User-Agent"] = defaultHTTPObfsUserAgent
	}
	if _, ok := node.Transport.Headers["Accept-Encoding"]; !ok {
		node.Transport.Headers["Accept-Encoding"] = defaultHTTPObfsAcceptEnc
	}
	if _, ok := node.Transport.Headers["Connection"]; !ok {
		node.Transport.Headers["Connection"] = defaultHTTPObfsConnection
	}
	if _, ok := node.Transport.Headers["Pragma"]; !ok {
		node.Transport.Headers["Pragma"] = "no-cache"
	}
}

func parseShadowsocksPluginOptions(plugin, extra string) (string, map[string]string) {
	plugin = strings.TrimSpace(plugin)
	extra = strings.TrimSpace(extra)
	if plugin == "" && extra == "" {
		return "", nil
	}
	options := make(map[string]string)
	var name string
	if plugin != "" {
		parts := strings.Split(plugin, ";")
		name = strings.ToLower(strings.TrimSpace(parts[0]))
		if len(parts) > 1 {
			for _, part := range parts[1:] {
				addPluginOption(options, part)
			}
		}
	}
	if extra != "" {
		for _, token := range strings.Split(extra, ";") {
			addPluginOption(options, token)
		}
	}
	return name, options
}

func addPluginOption(opts map[string]string, token string) {
	token = strings.TrimSpace(token)
	if token == "" {
		return
	}
	key := token
	val := ""
	if idx := strings.Index(token, "="); idx >= 0 {
		key = token[:idx]
		val = token[idx+1:]
	}
	key = strings.ToLower(strings.TrimSpace(key))
	val = strings.TrimSpace(val)
	if key != "" {
		opts[key] = val
	}
}

func prependPath(env []string, dir string) []string {
	dir = strings.TrimSpace(dir)
	if dir == "" {
		return env
	}
	pathPrefix := "PATH="
	sep := string(os.PathListSeparator)
	for i, kv := range env {
		if strings.HasPrefix(kv, pathPrefix) {
			envCopy := append([]string{}, env...)
			current := strings.TrimPrefix(kv, pathPrefix)
			envCopy[i] = pathPrefix + dir + sep + current
			return envCopy
		}
	}
	return append(append([]string{}, env...), pathPrefix+dir)
}

func measureLatencyThroughSocks5(ctx context.Context, proxyHost string, proxyPort int) (int64, error) {
	candidates := []socksTarget{
		{"www.gstatic.com", 80, "/generate_204", false, 0},
		{"example.com", 80, "/", false, 0},
		{"speed.cloudflare.com", 443, "/__up", true, 0},
	}

	var lastErr error
	for _, t := range candidates {
		lat, err := latencyViaSocksOnce(ctx, proxyHost, proxyPort, t)
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

func latencyViaSocksOnce(ctx context.Context, proxyHost string, proxyPort int, t socksTarget) (int64, error) {
	deadline := time.Now().Add(5 * time.Second)
	d := net.Dialer{}
	conn, err := d.DialContext(ctx, "tcp", net.JoinHostPort(proxyHost, strconv.Itoa(proxyPort)))
	if err != nil {
		return 0, err
	}
	defer conn.Close()
	_ = conn.SetDeadline(deadline)
	brw := bufio.NewReadWriter(bufio.NewReader(conn), bufio.NewWriter(conn))
	// SOCKS5 greet
	if _, err := brw.Write([]byte{0x05, 0x01, 0x00}); err != nil {
		return 0, err
	}
	if err := brw.Flush(); err != nil {
		return 0, err
	}
	resp := make([]byte, 2)
	if _, err := io.ReadFull(brw, resp); err != nil {
		return 0, err
	}
	if resp[0] != 0x05 || resp[1] != 0x00 {
		return 0, fmt.Errorf("socks5 noauth rejected")
	}
	// CONNECT request
	hostBytes := []byte(t.host)
	req := []byte{0x05, 0x01, 0x00, 0x03, byte(len(hostBytes))}
	req = append(req, hostBytes...)
	req = append(req, byte(t.port>>8), byte(t.port&0xff))
	if _, err := brw.Write(req); err != nil {
		return 0, err
	}
	if err := brw.Flush(); err != nil {
		return 0, err
	}
	hdr := make([]byte, 4)
	if _, err := io.ReadFull(brw, hdr); err != nil {
		return 0, err
	}
	if hdr[1] != 0x00 {
		return 0, fmt.Errorf("socks5 connect failed: 0x%02x", hdr[1])
	}
	atyp := hdr[3]
	switch atyp {
	case 0x01:
		tmp := make([]byte, 6)
		if _, err := io.ReadFull(brw, tmp); err != nil {
			return 0, err
		}
	case 0x03:
		ln, _ := brw.ReadByte()
		tmp := make([]byte, int(ln)+2)
		if _, err := io.ReadFull(brw, tmp); err != nil {
			return 0, err
		}
	case 0x04:
		tmp := make([]byte, 18)
		if _, err := io.ReadFull(brw, tmp); err != nil {
			return 0, err
		}
	default:
		return 0, fmt.Errorf("unknown atyp %d", atyp)
	}

	var downstream net.Conn = conn
	if t.tls {
		tlsConn := tls.Client(conn, &tls.Config{ServerName: t.host, InsecureSkipVerify: true})
		if err := tlsConn.Handshake(); err != nil {
			tlsConn.Close()
			return 0, err
		}
		downstream = tlsConn
		brw = bufio.NewReadWriter(bufio.NewReader(downstream), bufio.NewWriter(downstream))
	}

	request := fmt.Sprintf("HEAD %s HTTP/1.1\r\nHost: %s\r\nConnection: close\r\nUser-Agent: VeaLatency\r\n\r\n", t.path, t.host)
	start := time.Now()
	if _, err := brw.WriteString(request); err != nil {
		downstream.Close()
		return 0, err
	}
	if err := brw.Flush(); err != nil {
		downstream.Close()
		return 0, err
	}

	buf := make([]byte, 1024)
	for {
		n, rerr := brw.Read(buf)
		if n > 0 {
			if idx := indexOfHeaderEnd(buf[:n]); idx >= 0 {
				break
			}
		}
		if rerr != nil {
			if rerr == io.EOF {
				break
			}
			downstream.Close()
			return 0, rerr
		}
	}
	latency := time.Since(start).Milliseconds()
	downstream.Close()
	if latency <= 0 {
		latency = 1
	}
	return latency, nil
}

// measureDownloadThroughSocks5 downloads bytes from several public test endpoints
// through a SOCKS5 proxy on host:port and returns measured Mbps.
func measureDownloadThroughSocks5(ctx context.Context, proxyHost string, proxyPort int, progress func(float64)) (float64, error) {
	// 多目标回落，避免单一域名被策略或对端封禁导致误判
	// 优化：使用更小的文件进行测速，10MB足够且比原来的50MB/100MB快很多
	sizes := []int64{10 * 1024 * 1024}
	candidates := func(size int64) []socksTarget {
		return []socksTarget{
			{"cachefly.cachefly.net", 80, "/10mb.test", false, 10 * 1024 * 1024},
			{"speedtest.tele2.net", 80, "/10MB.zip", false, 10 * 1024 * 1024},
			{"ipv4.download.thinkbroadband.com", 80, "/10MB.zip", false, 10 * 1024 * 1024},
		}
	}
	var (
		totalBytes   int64
		totalSeconds float64
		lastErr      error
	)
	for _, size := range sizes {
		for _, t := range candidates(size) {
			subCtx, cancel := context.WithTimeout(ctx, speedTestTimeout)
			bytesRead, seconds, err := downloadViaSocks5Once(subCtx, proxyHost, proxyPort, t, func(bytes int64, elapsed float64) {
				if progress == nil {
					return
				}
				if elapsed < 0.5 {
					return
				}
				aggBytes := totalBytes + bytes
				aggSeconds := totalSeconds + elapsed
				if aggSeconds < 0.5 {
					return
				}
				progress((float64(aggBytes) / aggSeconds) / (1024 * 1024))
			})
			cancel()
			if err != nil {
				lastErr = fmt.Errorf("%s:%d %s: %w", t.host, t.port, t.path, err)
				continue
			}
			if seconds <= 0 {
				continue
			}
			totalBytes += bytesRead
			totalSeconds += seconds
			if progress != nil && totalSeconds >= 0.5 {
				progress((float64(totalBytes) / totalSeconds) / (1024 * 1024))
			}
			// 成功一个就足够
			if totalBytes >= 5*1024*1024 && totalSeconds >= 0.5 {
				mbps := (float64(totalBytes) / totalSeconds) / (1024 * 1024)
				if progress != nil {
					progress(mbps)
				}
				return mbps, nil
			}
		}
	}
	if lastErr != nil {
		return 0, lastErr
	}
	return 0, errors.New("no throughput data collected")
}

func downloadViaSocks5Once(ctx context.Context, proxyHost string, proxyPort int, t socksTarget, progress func(int64, float64)) (int64, float64, error) {
	deadline := time.Now().Add(speedTestTimeout)
	d := net.Dialer{}
	conn, err := d.DialContext(ctx, "tcp", net.JoinHostPort(proxyHost, strconv.Itoa(proxyPort)))
	if err != nil {
		return 0, 0, err
	}
	defer conn.Close()
	_ = conn.SetDeadline(deadline)
	brw := bufio.NewReadWriter(bufio.NewReader(conn), bufio.NewWriter(conn))
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
	hostBytes := []byte(t.host)
	req := []byte{0x05, 0x01, 0x00, 0x03, byte(len(hostBytes))}
	req = append(req, hostBytes...)
	req = append(req, byte(t.port>>8), byte(t.port&0xff))
	if _, err := brw.Write(req); err != nil {
		return 0, 0, err
	}
	if err := brw.Flush(); err != nil {
		return 0, 0, err
	}
	hdr := make([]byte, 4)
	if _, err := io.ReadFull(brw, hdr); err != nil {
		return 0, 0, err
	}
	if hdr[1] != 0x00 {
		return 0, 0, fmt.Errorf("socks5 connect failed: 0x%02x", hdr[1])
	}
	atyp := hdr[3]
	switch atyp {
	case 0x01:
		tmp := make([]byte, 6)
		if _, err := io.ReadFull(brw, tmp); err != nil {
			return 0, 0, err
		}
	case 0x03:
		ln, _ := brw.ReadByte()
		tmp := make([]byte, int(ln)+2)
		if _, err := io.ReadFull(brw, tmp); err != nil {
			return 0, 0, err
		}
	case 0x04:
		tmp := make([]byte, 18)
		if _, err := io.ReadFull(brw, tmp); err != nil {
			return 0, 0, err
		}
	default:
		return 0, 0, fmt.Errorf("unknown atyp %d", atyp)
	}

	var downstream net.Conn = conn
	if t.tls {
		tlsConn := tls.Client(conn, &tls.Config{ServerName: t.host, InsecureSkipVerify: true})
		if err := tlsConn.Handshake(); err != nil {
			tlsConn.Close()
			return 0, 0, err
		}
		downstream = tlsConn
		brw = bufio.NewReadWriter(bufio.NewReader(downstream), bufio.NewWriter(downstream))
	}
	reqLine := fmt.Sprintf("GET %s HTTP/1.1\r\nHost: %s\r\nConnection: close\r\nUser-Agent: VeaSpeedTest\r\n\r\n", t.path, t.host)
	if _, err := brw.WriteString(reqLine); err != nil {
		downstream.Close()
		return 0, 0, err
	}
	if err := brw.Flush(); err != nil {
		downstream.Close()
		return 0, 0, err
	}

	start := time.Now()
	var headerEnded bool
	var buf [64 * 1024]byte
	var total int64
	limit := t.bytes
	if limit == 0 {
		limit = 2 * 1024 * 1024
	}
	deadline2 := time.Now().Add(speedTestTimeout)
	_ = downstream.SetDeadline(deadline2)
	lastProgress := time.Now()
	reportProgress := func() {
		if progress == nil || !headerEnded {
			return
		}
		elapsed := time.Since(start).Seconds()
		if elapsed < 0.5 {
			return
		}
		progress(total, elapsed)
	}
	for {
		n, rerr := brw.Read(buf[:])
		if n > 0 {
			if !headerEnded {
				idx := indexOfHeaderEnd(buf[:n])
				if idx >= 0 {
					headerEnded = true
					total += int64(n - idx)
				}
			} else {
				total += int64(n)
			}
			if total >= limit {
				break
			}
			if time.Since(lastProgress) >= 500*time.Millisecond {
				reportProgress()
				lastProgress = time.Now()
			}
		}
		if rerr != nil {
			if rerr == io.EOF {
				break
			}
			downstream.Close()
			return 0, 0, rerr
		}
		if time.Since(start) > speedTestTimeout {
			break
		}
	}
	elapsed := time.Since(start).Seconds()
	if elapsed < 0.5 {
		elapsed = 0.5
	}
	downstream.Close()
	reportProgress()
	return total, elapsed, nil
}

func indexOfHeaderEnd(b []byte) int {
	for i := 3; i < len(b); i++ {
		if b[i-3] == '\r' && b[i-2] == '\n' && b[i-1] == '\r' && b[i] == '\n' {
			return i + 1
		}
	}
	return -1
}

func parseNodeShareLink(link string) (domain.Node, error) {
	trimmed := strings.TrimSpace(link)
	if trimmed == "" {
		return domain.Node{}, errors.New("empty share link")
	}
	switch {
	case strings.HasPrefix(trimmed, "vmess://"):
		return parseVmessLink(trimmed)
	case strings.HasPrefix(trimmed, "vless://"):
		return parseGenericURLLink(trimmed, domain.ProtocolVLESS)
	case strings.HasPrefix(trimmed, "trojan://"):
		return parseGenericURLLink(trimmed, domain.ProtocolTrojan)
	case strings.HasPrefix(trimmed, "ss://"):
		return parseShadowsocksLink(trimmed)
	default:
		return domain.Node{}, fmt.Errorf("unsupported share link: %s", trimmed)
	}
}

type vmessShare struct {
	Ps          string `json:"ps"`
	Add         string `json:"add"`
	Port        string `json:"port"`
	ID          string `json:"id"`
	Aid         string `json:"aid"`
	Net         string `json:"net"`
	Type        string `json:"type"`
	Host        string `json:"host"`
	Path        string `json:"path"`
	Tls         string `json:"tls"`
	Sni         string `json:"sni"`
	Alpn        string `json:"alpn"`
	Scy         string `json:"scy"`
	Flow        string `json:"flow"`
	Fingerprint string `json:"fp"`
}

func parseVmessLink(link string) (domain.Node, error) {
	encoded := strings.TrimPrefix(link, "vmess://")
	data, err := decodeBase64Padding(encoded)
	if err != nil {
		return domain.Node{}, fmt.Errorf("decode vmess share: %w", err)
	}
	var share vmessShare
	if err := json.Unmarshal(data, &share); err != nil {
		return domain.Node{}, fmt.Errorf("parse vmess json: %w", err)
	}
	port, err := strconv.Atoi(share.Port)
	if err != nil {
		return domain.Node{}, fmt.Errorf("invalid vmess port: %w", err)
	}
	name := strings.TrimSpace(share.Ps)
	if name == "" {
		name = share.Add
	}
	alterID, _ := strconv.Atoi(strings.TrimSpace(share.Aid))
	var alpn []string
	if share.Alpn != "" {
		alpn = parseList(share.Alpn)
	}
	tlsEnabled := strings.EqualFold(share.Tls, "tls") || strings.EqualFold(share.Tls, "xtls") || strings.EqualFold(share.Tls, "reality")
	transportType := strings.TrimSpace(share.Net)
	transport := &domain.NodeTransport{Type: transportType}
	switch transportType {
	case "ws":
		transport.Path = share.Path
		if share.Host != "" {
			transport.Host = share.Host
			transport.Headers = map[string]string{"Host": share.Host}
		}
	case "grpc":
		transport.ServiceName = share.Path
	case "tcp":
		// share.Type sometimes includes http header domain
		if share.Type == "http" && share.Host != "" {
			transport.Headers = map[string]string{"Host": share.Host}
		}
	case "http":
		transport.Path = share.Path
		transport.Host = share.Host
	}

	return domain.Node{
		Name:     name,
		Address:  share.Add,
		Port:     port,
		Protocol: domain.ProtocolVMess,
		Security: &domain.NodeSecurity{
			UUID:       share.ID,
			AlterID:    alterID,
			Encryption: share.Scy,
			Flow:       share.Flow,
		},
		Transport: transport,
		TLS: &domain.NodeTLS{
			Enabled:     tlsEnabled,
			Type:        share.Tls,
			ServerName:  firstNonEmpty(share.Sni, share.Host),
			ALPN:        alpn,
			Fingerprint: share.Fingerprint,
			Insecure:    false,
		},
	}, nil
}

func parseGenericURLLink(link string, protocol domain.NodeProtocol) (domain.Node, error) {
	u, err := url.Parse(link)
	if err != nil {
		return domain.Node{}, fmt.Errorf("invalid %s link: %w", protocol, err)
	}
	portStr := u.Port()
	if portStr == "" {
		return domain.Node{}, errors.New("missing port in share link")
	}
	port, err := strconv.Atoi(portStr)
	if err != nil {
		return domain.Node{}, fmt.Errorf("invalid port: %w", err)
	}
	q := u.Query()
	name := strings.TrimSpace(u.Fragment)
	if name == "" {
		name = strings.TrimSpace(q.Get("remarks"))
		if name == "" {
			name = strings.TrimSpace(q.Get("name"))
		}
	}
	if name == "" {
		name = u.Hostname()
	}
	tags := uniqueStrings(append(parseTagList(q.Get("tag")), parseTagList(q.Get("tags"))...))
	security := &domain.NodeSecurity{}
	transport := &domain.NodeTransport{}
	tls := &domain.NodeTLS{}

	switch protocol {
	case domain.ProtocolVLESS:
		uuid := u.User.Username()
		if uuid == "" {
			uuid = q.Get("id")
		}
		security.UUID = uuid
		security.Flow = q.Get("flow")
		sec := q.Get("encryption")
		if sec == "" {
			sec = "none"
		}
		security.Encryption = sec
	case domain.ProtocolTrojan:
		pass, _ := u.User.Password()
		security.Password = pass
	case domain.ProtocolShadowsocks:
		password, _ := u.User.Password()
		method := u.User.Username()
		if method == "" {
			decoded, err := decodeBase64Padding(method)
			if err == nil {
				parts := strings.SplitN(string(decoded), ":", 2)
				if len(parts) == 2 {
					method = parts[0]
					password = parts[1]
				}
			}
		}
		security.Method = method
		security.Password = password
		tls.Enabled = false
	}

	transportType := q.Get("type")
	if transportType == "" {
		transportType = q.Get("network")
	}
	transport.Type = transportType
	transport.Host = q.Get("host")
	transport.Path = q.Get("path")
	transport.ServiceName = q.Get("serviceName")
	if hdr := q.Get("headerType"); hdr == "http" && transport.Host != "" {
		transport.Headers = map[string]string{"Host": transport.Host}
	}

	tlsType := q.Get("security")
	if tlsType == "" {
		tlsType = q.Get("tls")
	}
	tls.Enabled = tlsType != "" && tlsType != "none"
	tls.Type = tlsType
	tls.ServerName = firstNonEmpty(q.Get("sni"), q.Get("serverName"), q.Get("host"))
	tls.Insecure = q.Get("allowInsecure") == "1" || strings.EqualFold(q.Get("insecure"), "true")
	if fp := q.Get("fp"); fp != "" {
		tls.Fingerprint = fp
	}
	if pk := q.Get("pbk"); pk != "" {
		tls.RealityPublicKey = pk
	}
	if sid := q.Get("sid"); sid != "" {
		tls.RealityShortID = sid
	}
	if alpn := q.Get("alpn"); alpn != "" {
		tls.ALPN = parseList(alpn)
	}

	node := domain.Node{
		Name:      name,
		Address:   u.Hostname(),
		Port:      port,
		Protocol:  protocol,
		Tags:      tags,
		Security:  security,
		Transport: transport,
		TLS:       tls,
	}
	ensureShadowsocksHTTPHeaders(&node)
	return node, nil
}

func parseShadowsocksLink(link string) (domain.Node, error) {
	u, err := url.Parse(link)
	if err != nil {
		return domain.Node{}, fmt.Errorf("invalid shadowsocks link: %w", err)
	}
	if strings.HasPrefix(u.Host, "ssr://") {
		return domain.Node{}, errors.New("ssr links are not supported")
	}
	host, portStr, err := net.SplitHostPort(u.Host)
	if err != nil {
		return domain.Node{}, fmt.Errorf("invalid host:port in shadowsocks link: %w", err)
	}
	port, err := strconv.Atoi(portStr)
	if err != nil {
		return domain.Node{}, fmt.Errorf("invalid port in shadowsocks link: %w", err)
	}
	name := strings.TrimSpace(u.Fragment)
	if decodedName, decodeErr := url.QueryUnescape(name); decodeErr == nil {
		name = decodedName
	}
	if name == "" {
		name = host
	}
	security := &domain.NodeSecurity{}
	user := u.User.Username()
	pass, _ := u.User.Password()
	if user != "" {
		decoded, err := decodeBase64Padding(user)
		if err == nil {
			user = string(decoded)
		}
	}
	if strings.Contains(user, ":") {
		parts := strings.SplitN(user, ":", 2)
		security.Method = parts[0]
		security.Password = parts[1]
	} else {
		security.Method = user
		security.Password = pass
	}
	if plugin := u.Query().Get("plugin"); plugin != "" {
		security.Plugin = plugin
	}
	if pluginOpts := u.Query().Get("plugin-opts"); pluginOpts != "" {
		security.PluginOpts = pluginOpts
	}
	node := domain.Node{
		Name:     name,
		Address:  host,
		Port:     port,
		Protocol: domain.ProtocolShadowsocks,
		Security: security,
	}
	ensureShadowsocksHTTPHeaders(&node)
	return node, nil
}

func decodeBase64Padding(value string) ([]byte, error) {
	switch len(value) % 4 {
	case 2:
		value += "=="
	case 3:
		value += "="
	}
	if data, err := base64.StdEncoding.DecodeString(value); err == nil {
		return data, nil
	}
	return base64.URLEncoding.DecodeString(value)
}

func parseTagList(raw string) []string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return nil
	}
	items := strings.Split(trimmed, ",")
	result := make([]string, 0, len(items))
	for _, item := range items {
		val := strings.TrimSpace(item)
		if val != "" {
			result = append(result, val)
		}
	}
	return result
}

func uniqueStrings(items []string) []string {
	if len(items) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(items))
	result := make([]string, 0, len(items))
	for _, item := range items {
		if _, ok := seen[item]; ok {
			continue
		}
		seen[item] = struct{}{}
		result = append(result, item)
	}
	return result
}

func parseList(raw string) []string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return nil
	}
	parts := strings.FieldsFunc(trimmed, func(r rune) bool {
		switch r {
		case ',', '|', ';':
			return true
		default:
			return false
		}
	})
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		if val := strings.TrimSpace(part); val != "" {
			result = append(result, val)
		}
	}
	return result
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

func parseSubscriptionNodes(raw []byte) ([]domain.Node, error) {
	text := strings.TrimSpace(string(raw))
	if text == "" {
		return nil, nil
	}
	cleaned := strings.ReplaceAll(strings.ReplaceAll(text, "\r", ""), "\n", "")
	candidates := make([]string, 0, 2)
	if decoded, err := decodeBase64Padding(cleaned); err == nil {
		decodedText := strings.TrimSpace(string(decoded))
		if decodedText != "" {
			candidates = append(candidates, decodedText)
		}
	}
	candidates = append(candidates, text)

	for _, candidate := range candidates {
		lines := strings.Split(candidate, "\n")
		nodes := make([]domain.Node, 0, len(lines))
		for _, line := range lines {
			share := strings.TrimSpace(line)
			if share == "" {
				continue
			}
			if !isShareLink(share) {
				continue
			}
			node, err := parseNodeShareLink(share)
			if err != nil {
				log.Printf("skip share entry: %v", err)
				continue
			}
			nodes = append(nodes, node)
		}
		if len(nodes) > 0 {
			return nodes, nil
		}
	}
	return nil, nil
}

func isShareLink(line string) bool {
	return strings.HasPrefix(line, "vmess://") ||
		strings.HasPrefix(line, "vless://") ||
		strings.HasPrefix(line, "trojan://") ||
		strings.HasPrefix(line, "ss://")
}

func isLikelyURL(val string) bool {
	if val == "" {
		return false
	}
	u, err := url.Parse(strings.TrimSpace(val))
	if err != nil {
		return false
	}
	if u.Scheme == "" || u.Host == "" {
		return false
	}
	return strings.HasPrefix(u.Scheme, "http")
}

func inferFormatFromPayload(payload string) (domain.ConfigFormat, bool) {
	switch {
	case strings.HasPrefix(payload, "vmess://"),
		strings.HasPrefix(payload, "vless://"),
		strings.HasPrefix(payload, "trojan://"),
		strings.HasPrefix(payload, "ss://"):
		return domain.ConfigFormatXray, true
	default:
		return "", false
	}
}
