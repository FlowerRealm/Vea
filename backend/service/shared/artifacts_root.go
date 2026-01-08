package shared

import (
	"log"
	"os"
	"path/filepath"
	"strings"
)

func init() {
	ArtifactsRoot = absPath(filepath.Join(UserDataRoot(), "artifacts"))
	log.Printf("[Init] artifacts root: %s", ArtifactsRoot)
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

// ArtifactsSearchRoots returns a stable search order for artifacts.
//
// It always prefers ArtifactsRoot (per-user) only.
func ArtifactsSearchRoots() []string {
	root := strings.TrimSpace(ArtifactsRoot)
	if root == "" {
		return nil
	}
	return []string{absPath(root)}
}
