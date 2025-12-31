package shared

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// FindBinaryInDir 在目录中查找二进制文件（支持子目录 1 层）
func FindBinaryInDir(dir string, candidates []string) (string, error) {
	if dir == "" {
		return "", errors.New("install dir is empty")
	}
	if len(candidates) == 0 {
		return "", errors.New("binary candidates are empty")
	}

	// 先在根目录查找
	for _, name := range candidates {
		path := filepath.Join(dir, name)
		if _, err := os.Stat(path); err == nil {
			return path, nil
		}
	}

	// 在子目录中查找（深度 1 层）
	entries, err := os.ReadDir(dir)
	if err != nil {
		return "", fmt.Errorf("binary not found in %s (candidates: %v): %w", dir, candidates, err)
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		subdir := filepath.Join(dir, entry.Name())
		for _, name := range candidates {
			path := filepath.Join(subdir, name)
			if _, err := os.Stat(path); err == nil {
				return path, nil
			}
		}
	}

	return "", fmt.Errorf("binary not found in %s (candidates: %v)", dir, candidates)
}

// FindSingBoxBinary 在 artifacts/core/sing-box 下查找 sing-box 二进制（支持子目录 1 层）
func FindSingBoxBinary() (string, error) {
	var lastErr error
	for _, root := range ArtifactsSearchRoots() {
		dir := filepath.Join(root, "core", "sing-box")
		path, err := FindBinaryInDir(dir, []string{"sing-box", "sing-box.exe"})
		if err == nil {
			return path, nil
		}
		lastErr = err
	}
	if lastErr != nil {
		return "", lastErr
	}
	return "", errors.New("artifacts root is empty")
}
