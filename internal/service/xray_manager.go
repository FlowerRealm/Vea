package service

import (
	"archive/zip"
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"

	"vea/internal/domain"
)

var (
	errXrayComponentNotInstalled = errors.New("xray component not installed")
	moduleRootOnce               sync.Once
	moduleRootDir                string
	moduleRootErr                error
)

type XrayRuntime struct {
	Binary       string
	Config       string
	GeoIP        string
	GeoSite      string
	InboundPort  int
	ActiveNodeID string
}

const (
	xrayCoreDirName   = "xray"
	defaultGeoIPURL   = "https://github.com/Loyalsoldier/v2ray-rules-dat/releases/latest/download/geoip.dat"
	defaultGeoSiteURL = "https://github.com/Loyalsoldier/v2ray-rules-dat/releases/latest/download/geosite.dat"
)

func (s *Service) prepareXrayRuntime(desiredNodeID string) (XrayRuntime, string, error) {
	nodes := s.ListNodes()
	if len(nodes) == 0 {
		return XrayRuntime{}, "", fmt.Errorf("no nodes configured")
	}

	component, err := s.getComponentByKind(domain.ComponentXray)
	if err != nil {
		return XrayRuntime{}, "", err
	}
	if component.InstallDir == "" {
		return XrayRuntime{}, "", fmt.Errorf("xray component has no install directory")
	}

	coreDir := filepath.Join(artifactsRoot, "core", xrayCoreDirName)
	coreDirAbs, err := filepath.Abs(coreDir)
	if err != nil {
		return XrayRuntime{}, "", err
	}
	if err := os.MkdirAll(coreDirAbs, 0o755); err != nil {
		return XrayRuntime{}, "", err
	}

	binarySource, err := findXrayBinary(component.InstallDir)
	if err != nil {
		return XrayRuntime{}, "", err
	}
	binaryDest := filepath.Join(coreDirAbs, filepath.Base(binarySource))
	if err := copyFileIfChanged(binarySource, binaryDest); err != nil {
		return XrayRuntime{}, "", err
	}
	if err := os.Chmod(binaryDest, 0o755); err != nil {
		return XrayRuntime{}, "", err
	}
	binaryDestAbs, err := filepath.Abs(binaryDest)
	if err != nil {
		return XrayRuntime{}, "", err
	}

	geo, err := s.ensureXrayGeoFiles(coreDirAbs, component.InstallDir)
	if err != nil {
		return XrayRuntime{}, "", err
	}
	geoIPAbs, err := filepath.Abs(geo.GeoIP)
	if err != nil {
		return XrayRuntime{}, "", err
	}
	geoSiteAbs, err := filepath.Abs(geo.GeoSite)
	if err != nil {
		return XrayRuntime{}, "", err
	}
	geo.GeoIP = geoIPAbs
	geo.GeoSite = geoSiteAbs

	activeNodeID := desiredNodeID
	if activeNodeID == "" && component.Meta != nil {
		activeNodeID = component.Meta["activeNodeId"]
	}

    preparedNodes, err := s.prepareNodesForXray(nodes)
    if err != nil {
        return XrayRuntime{}, "", err
    }
    // 插件路径已废弃：不再构建或确保任何插件二进制。

	configBytes, chosenNodeID, err := buildXrayConfig(preparedNodes, geo, xrayDefaultInboundPort, activeNodeID)
	if err != nil {
		return XrayRuntime{}, "", err
	}
	configPath := filepath.Join(coreDirAbs, "config.json")
	if err := writeAtomic(configPath, configBytes, 0o644); err != nil {
		return XrayRuntime{}, "", err
	}
	configPathAbs, err := filepath.Abs(configPath)
	if err != nil {
		return XrayRuntime{}, "", err
	}

	_, err = s.store.UpdateComponent(component.ID, func(comp domain.CoreComponent) (domain.CoreComponent, error) {
		if comp.Meta == nil {
			comp.Meta = map[string]string{}
		}
		comp.Meta["binary"] = binaryDestAbs
		comp.Meta["config"] = configPathAbs
		comp.Meta["geoip"] = geoIPAbs
		comp.Meta["geosite"] = geoSiteAbs
		comp.Meta["activeNodeId"] = chosenNodeID
		comp.Meta["enabled"] = strconv.FormatBool(s.xrayEnabled)
		comp.InstallDir = component.InstallDir
		return comp, nil
	})
	if err != nil {
		return XrayRuntime{}, "", err
	}

	runtime := XrayRuntime{
		Binary:       binaryDestAbs,
		Config:       configPathAbs,
		GeoIP:        geoIPAbs,
		GeoSite:      geoSiteAbs,
		InboundPort:  xrayDefaultInboundPort,
		ActiveNodeID: chosenNodeID,
	}
	return runtime, chosenNodeID, nil
}

func (s *Service) ensureXrayGeoFiles(coreDir, installDir string) (GeoFiles, error) {
	geo := GeoFiles{}

	geoDest := filepath.Join(coreDir, "geoip.dat")
	siteDest := filepath.Join(coreDir, "geosite.dat")

	if installDir != "" {
		if err := copyIfExists(filepath.Join(installDir, "geoip.dat"), geoDest); err != nil {
			return GeoFiles{}, err
		}
		if err := copyIfExists(filepath.Join(installDir, "geosite.dat"), siteDest); err != nil {
			return GeoFiles{}, err
		}
	}

	if err := s.ensureGeoFile(domain.GeoIP, geoDest, defaultGeoIPURL); err != nil {
		return GeoFiles{}, err
	}
	if err := s.ensureGeoFile(domain.GeoSite, siteDest, defaultGeoSiteURL); err != nil {
		return GeoFiles{}, err
	}

	geo.GeoIP = geoDest
	geo.GeoSite = siteDest
	return geo, nil
}

func (s *Service) ensureGeoFile(geoType domain.GeoResourceType, destPath, fallbackURL string) error {
	// Fast path: if dest already exists and is non-empty, reuse it.
	if fi, err := os.Stat(destPath); err == nil && fi.Size() > 0 {
		return nil
	}
	for _, res := range s.ListGeo() {
		if res.Type != geoType {
			continue
		}
		if res.ArtifactPath == "" {
			continue
		}
		if err := copyFileIfChanged(res.ArtifactPath, destPath); err != nil {
			return err
		}
		return nil
	}

	data, _, err := downloadResource(fallbackURL)
	if err != nil {
		return err
	}
	return writeAtomic(destPath, data, 0o644)
}

func findXrayBinary(dir string) (string, error) {
	candidates := []string{"xray", "xray.exe"}
	for _, name := range candidates {
		candidate := filepath.Join(dir, name)
		if _, err := os.Stat(candidate); err == nil {
			return candidate, nil
		}
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return "", err
	}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		if strings.HasPrefix(entry.Name(), "xray") {
			return filepath.Join(dir, entry.Name()), nil
		}
	}
	return "", fmt.Errorf("xray binary not found in %s", dir)
}

func nodesRequireXrayPlugin(nodes []domain.Node) bool {
	for _, node := range nodes {
		if nodeRequiresXrayPlugin(node) {
			return true
		}
	}
	return false
}

func nodeRequiresXrayPlugin(node domain.Node) bool {
	if node.Protocol != domain.ProtocolShadowsocks {
		return false
	}
	if node.Security == nil {
		return false
	}
	plugin := strings.ToLower(strings.TrimSpace(node.Security.Plugin))
	if plugin == "" {
		return false
	}
	if !strings.Contains(plugin, "xray-plugin") {
		return false
	}
	return strings.Contains(strings.ToLower(node.Security.PluginOpts), "obfs=http")
}

func ensureXrayPluginBinary(coreDir string) (string, error) {
	binaryName := xrayPluginBinaryName()
	pluginPath := filepath.Join(coreDir, binaryName)
	if fi, err := os.Stat(pluginPath); err == nil && fi.Mode()&0o111 != 0 {
		return pluginPath, nil
	}

	stageDir := filepath.Join(coreDir, "xray-plugin.stage")
	if err := os.RemoveAll(stageDir); err != nil && !errors.Is(err, os.ErrNotExist) {
		return "", err
	}
	if err := os.MkdirAll(stageDir, 0o755); err != nil {
		return "", err
	}

	const repo = "teddysun/xray-plugin"
	tag, _, err := fetchLatestReleaseTag(repo)
	if err != nil {
		return "", err
	}

	candidates := xrayPluginVersionedAssets(tag)
	var assetName string
	for _, name := range candidates {
		ok, _, probeErr := probeReleaseAsset(repo, tag, name)
		if probeErr == nil && ok {
			assetName = name
			break
		}
	}
	if assetName == "" {
		assetName = candidates[0]
	}
	downloadURL := fmt.Sprintf("https://github.com/%s/releases/download/%s/%s", repo, tag, assetName)

	data, _, err := downloadResource(downloadURL)
	if err != nil {
		return "", err
	}

	lowerName := strings.ToLower(assetName)
	switch {
	case strings.HasSuffix(lowerName, ".zip"):
		reader, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
		if err != nil {
			return "", err
		}
		for _, file := range reader.File {
			if err := extractZipEntry(file, stageDir); err != nil {
				return "", err
			}
		}
	case strings.HasSuffix(lowerName, ".tar.gz"), strings.HasSuffix(lowerName, ".tgz"):
		if err := extractTarGz(data, stageDir); err != nil {
			return "", err
		}
	default:
		return "", fmt.Errorf("unsupported xray-plugin asset: %s", assetName)
	}

	var extracted string
	err = filepath.WalkDir(stageDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		name := strings.ToLower(d.Name())
		if strings.HasPrefix(name, "xray-plugin") {
			extracted = path
			return io.EOF
		}
		return nil
	})
	if err != nil && !errors.Is(err, io.EOF) {
		return "", err
	}
	if extracted == "" {
		return "", fmt.Errorf("xray-plugin binary not found in asset %s", assetName)
	}

	if err := copyFile(extracted, pluginPath); err != nil {
		return "", err
	}
	if err := os.Chmod(pluginPath, 0o755); err != nil {
		return "", err
	}
	_ = os.RemoveAll(stageDir)
	return pluginPath, nil
}

func xrayPluginBinaryName() string {
	if runtime.GOOS == "windows" {
		return "xray-plugin.exe"
	}
	return "xray-plugin"
}

func xrayPluginAssetBase() string {
	osName := runtime.GOOS
	arch := runtime.GOARCH
	archName := arch
	switch arch {
	case "amd64":
		archName = "amd64"
	case "386":
		archName = "386"
	case "arm64":
		archName = "arm64"
	case "arm":
		archName = "arm"
	case "mips", "mipsle", "mips64", "mips64le", "ppc64le", "riscv64", "s390x":
		archName = arch
	}

	osNameNormalized := osName
	switch osName {
	case "darwin", "linux", "windows", "freebsd":
		// already normalized
	default:
		osNameNormalized = osName
	}

	base := fmt.Sprintf("xray-plugin-%s-%s", osNameNormalized, archName)
	return base
}

func xrayPluginVersionedAssets(tag string) []string {
	base := xrayPluginAssetBase()
	tag = strings.TrimSpace(tag)
	if tag == "" {
		tag = "latest"
	}
	return []string{
		fmt.Sprintf("%s-%s.tar.gz", base, tag),
		fmt.Sprintf("%s-%s.zip", base, tag),
	}
}

func xrayPluginAssetCandidates() []string {
	base := xrayPluginAssetBase()
	return []string{
		base + ".tar.gz",
		base + ".zip",
	}
}

func nodesRequireSimpleObfs(nodes []domain.Node) bool {
	for _, node := range nodes {
		if nodeRequiresSimpleObfs(node) {
			return true
		}
	}
	return false
}

func nodeRequiresSimpleObfs(node domain.Node) bool {
	if node.Protocol != domain.ProtocolShadowsocks {
		return false
	}
	if node.Security == nil {
		return false
	}
	plugin := strings.ToLower(strings.TrimSpace(node.Security.Plugin))
	return plugin == "obfs-local" || plugin == "simple-obfs"
}

func ensureSimpleObfsBinary(coreDir string) (string, error) {
	binaryName := simpleObfsBinaryName()
	pluginPath := filepath.Join(coreDir, binaryName)
	if fi, err := os.Stat(pluginPath); err == nil && fi.Mode()&0o111 != 0 {
		return pluginPath, nil
	}

	root, err := moduleRoot()
	if err != nil {
		return "", err
	}

	cmd := exec.Command("go", "build", "-o", pluginPath, "./cmd/obfs-local")
	cmd.Dir = root
	cmd.Env = os.Environ()
	if output, err := cmd.CombinedOutput(); err != nil {
		return "", fmt.Errorf("build obfs-local plugin: %v: %s", err, strings.TrimSpace(string(output)))
	}
	if err := os.Chmod(pluginPath, 0o755); err != nil {
		return "", err
	}
	return pluginPath, nil
}

func simpleObfsBinaryName() string {
	if runtime.GOOS == "windows" {
		return "obfs-local.exe"
	}
	return "obfs-local"
}

func moduleRoot() (string, error) {
	moduleRootOnce.Do(func() {
		cwd, err := os.Getwd()
		if err != nil {
			moduleRootErr = err
			return
		}
		dir := cwd
		for {
			if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
				moduleRootDir = dir
				return
			}
			parent := filepath.Dir(dir)
			if parent == dir {
				moduleRootErr = fmt.Errorf("go.mod not found from %s", cwd)
				return
			}
			dir = parent
		}
	})
	return moduleRootDir, moduleRootErr
}

func (s *Service) getComponentByKind(kind domain.CoreComponentKind) (domain.CoreComponent, error) {
	for _, comp := range s.ListComponents() {
		if comp.Kind == kind {
			return comp, nil
		}
	}
	return domain.CoreComponent{}, errXrayComponentNotInstalled
}

func copyFile(src, dst string) error {
	input, err := os.Open(src)
	if err != nil {
		return err
	}
	defer input.Close()

	tmp := dst + ".tmp"
	output, err := os.OpenFile(tmp, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		return err
	}
	if _, err := io.Copy(output, input); err != nil {
		output.Close()
		return err
	}
	if err := output.Close(); err != nil {
		return err
	}
	return os.Rename(tmp, dst)
}

// copyFileIfChanged copies src to dst only when dst does not exist or size differs.
func copyFileIfChanged(src, dst string) error {
	srcInfo, err := os.Stat(src)
	if err != nil {
		return err
	}
	if dstInfo, err := os.Stat(dst); err == nil {
		if dstInfo.Size() == srcInfo.Size() {
			return nil
		}
	}
	return copyFile(src, dst)
}

func copyIfExists(src, dst string) error {
	if strings.TrimSpace(src) == "" {
		return nil
	}
	if _, err := os.Stat(src); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}
	return copyFileIfChanged(src, dst)
}

func writeAtomic(path string, data []byte, perm os.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, perm); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}
