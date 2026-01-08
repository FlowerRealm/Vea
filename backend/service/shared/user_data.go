package shared

import (
	"os"
	"path/filepath"
	"strings"
)

const (
	// EnvUserDataDir aligns with Electron app.getPath("userData") and forces the backend
	// to use the exact same directory.
	EnvUserDataDir = "VEA_USER_DATA_DIR"
)

// UserDataRoot returns the per-user data root directory.
//
// Default (no EnvUserDataDir):
// - Linux: ~/.config/Vea
// - macOS: ~/Library/Application Support/Vea
// - Windows: %APPDATA%\Vea
func UserDataRoot() string {
	if configured := strings.TrimSpace(os.Getenv(EnvUserDataDir)); configured != "" {
		return absPath(configured)
	}

	base, err := os.UserConfigDir()
	if err == nil && strings.TrimSpace(base) != "" {
		return absPath(filepath.Join(base, "Vea"))
	}

	home, err := os.UserHomeDir()
	if err == nil && strings.TrimSpace(home) != "" {
		return absPath(filepath.Join(home, ".vea"))
	}

	if tmp := strings.TrimSpace(os.TempDir()); tmp != "" {
		return absPath(filepath.Join(tmp, "Vea"))
	}

	return ""
}

func DefaultStatePath() string {
	root := UserDataRoot()
	if strings.TrimSpace(root) == "" {
		// Extremely unlikely; keep old behavior as the last fallback.
		return "data/state.json"
	}
	return filepath.Join(root, "data", "state.json")
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
