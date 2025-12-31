//go:build linux
// +build linux

package shared

import (
	"os"
	"path/filepath"
)

func ResolvectlHelperSocketPath() string {
	return filepath.Join(ArtifactsRoot, "runtime", "resolvectl-helper.sock")
}

func ResolvectlShimBinDir() string {
	return filepath.Join(ArtifactsRoot, "runtime", "bin")
}

func EnsureResolvectlShim() (string, error) {
	dir := ResolvectlShimBinDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}

	shimPath := filepath.Join(dir, "resolvectl")
	script := []byte(`#!/bin/sh
set -eu

if [ -n "${VEA_EXECUTABLE:-}" ]; then
  exec "${VEA_EXECUTABLE}" resolvectl-shim "$@"
fi

exec /usr/bin/resolvectl "$@"
`)

	// Best-effort: keep writes minimal.
	if existing, err := os.ReadFile(shimPath); err == nil {
		if string(existing) == string(script) {
			_ = os.Chmod(shimPath, 0o755)
			return dir, nil
		}
	}

	if err := os.WriteFile(shimPath, script, 0o755); err != nil {
		return "", err
	}
	_ = os.Chmod(shimPath, 0o755)
	return dir, nil
}
