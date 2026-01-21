//go:build windows
// +build windows

package shared

import (
	"fmt"
)

// CheckTUNCapabilities 检查 Windows TUN 条件（无一次性配置步骤）
func CheckTUNCapabilities() (bool, error) {
	// Windows 下没有类似 Linux setcap 的“一次性配置”步骤。
	// TUN 是否能成功工作更多取决于运行时环境（Wintun 是否可用/系统策略），
	// 不应在能力检查阶段硬绑定为“当前进程是否管理员”来避免误报。
	return true, nil
}

func CheckTUNCapabilitiesForBinary(binaryPath string) (bool, error) {
	return CheckTUNCapabilities()
}

// SetupTUN Windows 不需要特殊设置
func SetupTUN() error {
	return fmt.Errorf("TUN setup is automatic on Windows (requires running as Administrator)")
}

func SetupTUNForBinary(binaryPath string) error {
	return SetupTUN()
}

// SetupTUNForSingBoxBinary Windows stub（仅为跨平台编译提供一致接口）
func SetupTUNForSingBoxBinary(binaryPath string) error {
	return SetupTUNForBinary(binaryPath)
}

// TUNCapabilityStatus Windows stub
type TUNCapabilityStatus struct {
	UserExists      bool
	BinaryFound     bool
	BinaryPath      string
	CurrentCaps     []string
	MissingCaps     []string
	FullyConfigured bool
}

// GetTUNCapabilityStatus Windows 版本
func GetTUNCapabilityStatus() TUNCapabilityStatus {
	return TUNCapabilityStatus{
		FullyConfigured: true,
	}
}

// EnsureTUNCapabilities Windows 下无一次性配置动作
func EnsureTUNCapabilities() (bool, error) {
	// Windows 下没有“一次性配置”动作；这里保持 no-op 以避免前端/状态误判。
	return false, nil
}

func EnsureTUNCapabilitiesForBinary(binaryPath string) (bool, error) {
	return EnsureTUNCapabilities()
}

// CleanConflictingIPTablesRules Windows 不需要清理 iptables 规则
func CleanConflictingIPTablesRules() {
	// Windows 不使用 iptables，无需清理
}
