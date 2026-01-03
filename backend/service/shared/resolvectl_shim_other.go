//go:build !linux
// +build !linux

package shared

func ResolvectlHelperSocketPath() string { return "" }

func ResolvectlShimBinDir() string { return "" }

func EnsureResolvectlShim() (string, error) { return "", nil }
