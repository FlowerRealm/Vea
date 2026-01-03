package shared

import (
	"log"
	"os"
	"path/filepath"
	"strings"
)

// LegacyArtifactsRoot is the historical artifacts directory next to the executable.
//
// Note: it may be read-only (packaged apps) or root-owned (previous sudo runs).
// We keep it for compatibility as a read fallback, but runtime writes should use ArtifactsRoot.
var LegacyArtifactsRoot string

func init() {
	LegacyArtifactsRoot = absPath(filepath.Join(executableDir(), "artifacts"))

	if configured := strings.TrimSpace(os.Getenv("VEA_ARTIFACTS_ROOT")); configured != "" {
		candidate := absPath(configured)
		if isWritableDir(candidate) {
			ArtifactsRoot = candidate
			log.Printf("[Init] artifacts root (VEA_ARTIFACTS_ROOT): %s", ArtifactsRoot)
			if LegacyArtifactsRoot != "" && ArtifactsRoot != LegacyArtifactsRoot {
				log.Printf("[Init] legacy artifacts root: %s", LegacyArtifactsRoot)
			}
			return
		}

		// 现实情况：用户以前用 sudo/pkexec 跑过，目录可能变成 root-owned。
		// 这时直接信任 VEA_ARTIFACTS_ROOT 只会让后续所有写入都 permission denied。
		log.Printf("[Init] artifacts root (VEA_ARTIFACTS_ROOT) is not writable: %s; falling back", candidate)
	}

	// Prefer legacy layout only when writable; otherwise fall back to per-user config dir.
	if LegacyArtifactsRoot != "" && isWritableDir(LegacyArtifactsRoot) {
		ArtifactsRoot = LegacyArtifactsRoot
	} else if userRoot := defaultUserArtifactsRoot(); userRoot != "" {
		ArtifactsRoot = absPath(userRoot)
	} else {
		cwd, _ := os.Getwd()
		ArtifactsRoot = absPath(filepath.Join(cwd, "artifacts"))
	}

	log.Printf("[Init] artifacts root: %s", ArtifactsRoot)
	if LegacyArtifactsRoot != "" && ArtifactsRoot != LegacyArtifactsRoot {
		log.Printf("[Init] legacy artifacts root: %s", LegacyArtifactsRoot)
	}
}

func executableDir() string {
	exePath, err := os.Executable()
	if err == nil {
		if realPath, err := filepath.EvalSymlinks(exePath); err == nil {
			exePath = realPath
		}
		return filepath.Dir(exePath)
	}
	cwd, _ := os.Getwd()
	return cwd
}

func absPath(path string) string {
	if strings.TrimSpace(path) == "" {
		return ""
	}
	if abs, err := filepath.Abs(path); err == nil {
		return abs
	}
	return path
}

func defaultUserArtifactsRoot() string {
	// Align with Electron's app.getPath('userData') default:
	// - Linux: ~/.config/Vea
	// - macOS: ~/Library/Application Support/Vea
	// - Windows: %APPDATA%\\Vea
	base, err := os.UserConfigDir()
	if err == nil && strings.TrimSpace(base) != "" {
		return filepath.Join(base, "Vea", "artifacts")
	}

	home, err := os.UserHomeDir()
	if err == nil && strings.TrimSpace(home) != "" {
		return filepath.Join(home, ".vea", "artifacts")
	}

	return ""
}

func isWritableDir(dir string) bool {
	if strings.TrimSpace(dir) == "" {
		return false
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return false
	}

	// Probe actual write permission.
	probe := filepath.Join(dir, ".vea_write_probe")
	f, err := os.OpenFile(probe, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		return false
	}
	_ = f.Close()
	_ = os.Remove(probe)
	return true
}

// ArtifactsSearchRoots returns a stable search order for artifacts.
//
// It always prefers ArtifactsRoot (writable, per-user) and falls back to LegacyArtifactsRoot
// (next to executable) for compatibility.
func ArtifactsSearchRoots() []string {
	seen := make(map[string]struct{}, 2)
	out := make([]string, 0, 2)
	add := func(p string) {
		p = absPath(p)
		if p == "" {
			return
		}
		if _, ok := seen[p]; ok {
			return
		}
		seen[p] = struct{}{}
		out = append(out, p)
	}
	add(ArtifactsRoot)
	add(LegacyArtifactsRoot)
	return out
}
