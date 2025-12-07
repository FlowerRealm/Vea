package service

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"vea/backend/domain"
)

var (
	errXrayComponentNotInstalled = errors.New("xray component not installed")
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

	// 从前端设置读取端口，如果没有则使用默认值
	inboundPort := xrayDefaultInboundPort
	frontendSettings := s.GetFrontendSettings()
	if proxyPort, ok := frontendSettings["proxy.port"].(float64); ok && proxyPort > 0 {
		inboundPort = int(proxyPort)
	}

	configBytes, chosenNodeID, err := buildXrayConfig(preparedNodes, geo, inboundPort, activeNodeID)
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
		InboundPort:  inboundPort,
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
