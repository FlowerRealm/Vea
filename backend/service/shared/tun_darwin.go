//go:build darwin
// +build darwin

package shared

import (
	"fmt"
	"os"
)

// CheckTUNCapabilities 检查 macOS TUN 权限（root 权限）
func CheckTUNCapabilities() (bool, error) {
	return os.Geteuid() == 0, nil
}

func CheckTUNCapabilitiesForBinary(binaryPath string) (bool, error) {
	return CheckTUNCapabilities()
}

// SetupTUN macOS TUN 设置说明
func SetupTUN() error {
	return fmt.Errorf("on macOS, TUN mode requires running Vea with sudo")
}

func SetupTUNForBinary(binaryPath string) error {
	return SetupTUN()
}

// SetupTUNForSingBoxBinary macOS stub（仅为跨平台编译提供一致接口）
func SetupTUNForSingBoxBinary(binaryPath string) error {
	return SetupTUNForBinary(binaryPath)
}

// TUNCapabilityStatus macOS stub
type TUNCapabilityStatus struct {
	UserExists      bool
	BinaryFound     bool
	BinaryPath      string
	CurrentCaps     []string
	MissingCaps     []string
	FullyConfigured bool
}

// GetTUNCapabilityStatus macOS 版本
func GetTUNCapabilityStatus() TUNCapabilityStatus {
	return TUNCapabilityStatus{
		FullyConfigured: os.Geteuid() == 0,
	}
}

// EnsureTUNCapabilities macOS 下检查是否有 root 权限
func EnsureTUNCapabilities() (bool, error) {
	if os.Geteuid() != 0 {
		return false, fmt.Errorf("TUN mode requires root privileges on macOS. Please run: sudo vea")
	}
	return false, nil
}

func EnsureTUNCapabilitiesForBinary(binaryPath string) (bool, error) {
	return EnsureTUNCapabilities()
}
