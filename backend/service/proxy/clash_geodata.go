package proxy

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"vea/backend/service/shared"
)

const (
	// 与 Geo 默认资源保持一致（避免 Clash 启动时自行在线拉取 GeoSite.dat/GeoIP.dat）。
	defaultGeoIPURL   = "https://github.com/Loyalsoldier/v2ray-rules-dat/releases/latest/download/geoip.dat"
	defaultGeoSiteURL = "https://github.com/Loyalsoldier/v2ray-rules-dat/releases/latest/download/geosite.dat"

	// 部分网络环境下 GitHub 直连不稳定；提供 jsDelivr 的兜底地址（与 mihomo 的默认生态一致）。
	fallbackGeoIPURL   = "https://fastly.jsdelivr.net/gh/MetaCubeX/meta-rules-dat@release/geoip.dat"
	fallbackGeoSiteURL = "https://fastly.jsdelivr.net/gh/MetaCubeX/meta-rules-dat@release/geosite.dat"
	// mihomo 对 GEOIP 规则默认使用 metadb（geoip.metadb）；缺失时会在启动时在线下载，导致启动期依赖网络。
	fallbackGeoIPMetadbURL = "https://fastly.jsdelivr.net/gh/MetaCubeX/meta-rules-dat@release/geoip.metadb"
)

func ensureClashGeoData(configDir string) error {
	configDir = strings.TrimSpace(configDir)
	if configDir == "" {
		return fmt.Errorf("config dir is empty")
	}

	srcDir := filepath.Join(shared.ArtifactsRoot, shared.GeoDir)
	pairs := []struct {
		src string
		dst string
		url []string
	}{
		{src: filepath.Join(srcDir, "geoip.dat"), dst: filepath.Join(configDir, "GeoIP.dat"), url: []string{defaultGeoIPURL, fallbackGeoIPURL}},
		{src: filepath.Join(srcDir, "geosite.dat"), dst: filepath.Join(configDir, "GeoSite.dat"), url: []string{defaultGeoSiteURL, fallbackGeoSiteURL}},
		{src: filepath.Join(srcDir, "geoip.metadb"), dst: filepath.Join(configDir, "geoip.metadb"), url: []string{fallbackGeoIPMetadbURL}},
	}

	var firstErr error
	for _, pair := range pairs {
		if fileExists(pair.dst) {
			continue
		}
		if err := ensureGeoSourceFile(pair.src, pair.url...); err != nil {
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		if err := copyFileAtomic(pair.src, pair.dst, 0o644); err != nil {
			return err
		}
	}
	return firstErr
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return info.Mode().IsRegular()
}

func ensureGeoSourceFile(dstPath string, urls ...string) error {
	dstPath = strings.TrimSpace(dstPath)
	if dstPath == "" {
		return errors.New("geo dst path is empty")
	}
	if fileExists(dstPath) {
		return nil
	}

	lastErr := error(nil)
	foundURL := false
	for _, url := range urls {
		url = strings.TrimSpace(url)
		if url == "" {
			continue
		}
		foundURL = true

		data, _, err := shared.DownloadWithProgress(url, nil)
		if err != nil {
			lastErr = err
			continue
		}
		if len(data) == 0 {
			lastErr = fmt.Errorf("download %s: empty content", filepath.Base(dstPath))
			continue
		}

		if err := os.MkdirAll(filepath.Dir(dstPath), 0o755); err != nil {
			return err
		}

		tmp, err := os.CreateTemp(filepath.Dir(dstPath), "."+filepath.Base(dstPath)+".tmp-*")
		if err != nil {
			return err
		}
		tmpPath := tmp.Name()
		cleanup := func() {
			_ = tmp.Close()
			_ = os.Remove(tmpPath)
		}

		if _, err := tmp.Write(data); err != nil {
			cleanup()
			return err
		}
		if err := tmp.Chmod(0o644); err != nil {
			cleanup()
			return err
		}
		if err := tmp.Close(); err != nil {
			_ = os.Remove(tmpPath)
			return err
		}

		if err := os.Rename(tmpPath, dstPath); err != nil {
			_ = os.Remove(tmpPath)
			if fileExists(dstPath) {
				return nil
			}
			return err
		}
		return nil
	}

	if !foundURL {
		return fmt.Errorf("geo source url is empty for %s", dstPath)
	}
	if lastErr != nil {
		return fmt.Errorf("download %s: %w", filepath.Base(dstPath), lastErr)
	}
	return fmt.Errorf("download %s failed: unknown error", filepath.Base(dstPath))

}

func copyFileAtomic(src, dst string, perm os.FileMode) error {
	src = strings.TrimSpace(src)
	dst = strings.TrimSpace(dst)
	if src == "" || dst == "" {
		return fmt.Errorf("copy: src/dst is empty")
	}

	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}

	tmp, err := os.CreateTemp(filepath.Dir(dst), "."+filepath.Base(dst)+".tmp-*")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()

	cleanup := func() {
		_ = tmp.Close()
		_ = os.Remove(tmpPath)
	}

	if _, err := io.Copy(tmp, in); err != nil {
		cleanup()
		return err
	}
	if err := tmp.Chmod(perm); err != nil {
		cleanup()
		return err
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmpPath)
		return err
	}

	if err := os.Rename(tmpPath, dst); err != nil {
		_ = os.Remove(tmpPath)
		if fileExists(dst) {
			return nil
		}
		return err
	}
	return nil
}
