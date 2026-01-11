//go:build linux
// +build linux

package shared

import (
	"bytes"
	"fmt"
	"log"
	"os"
	"os/exec"
	"os/user"
	"strconv"
	"strings"
	"sync"
)

const (
	tunUserName  = "vea-tun"
	tunGroupName = "vea-tun"
	// TUN 模式所需的所有 capabilities
	// cap_net_admin: 创建 TUN 设备、设置路由、nftables 规则
	// cap_net_bind_service: 绑定低端口（<1024）
	// cap_net_raw: 使用 SO_BINDTODEVICE 绑定物理接口（bind_interface 需要）
	requiredCapabilities = "cap_net_admin,cap_net_bind_service,cap_net_raw"
)

var ensureTUNOnceMu sync.Mutex
var ensureTUNOnceDone chan struct{}
var ensureTUNOnceNeedRestart bool
var ensureTUNOnceErr error

var cleanConflictingIPTablesOnce sync.Once

// CheckTUNCapabilities 检查 Linux TUN 权限是否已配置
func CheckTUNCapabilities() (bool, error) {
	if binaryPath, err := FindSingBoxBinary(); err == nil {
		return CheckTUNCapabilitiesForBinary(binaryPath)
	}
	if binaryPath, err := FindClashBinary(); err == nil {
		return CheckTUNCapabilitiesForBinary(binaryPath)
	}
	return false, nil
}

// CheckTUNCapabilitiesForBinary 检查 Linux TUN 权限是否已配置（指定内核二进制路径）
func CheckTUNCapabilitiesForBinary(binaryPath string) (bool, error) {
	// root 本身就具备创建 TUN/设置路由等权限；不强制要求 capabilities 已配置。
	if os.Geteuid() == 0 {
		return true, nil
	}

	// 检查 vea-tun 用户是否存在
	if _, err := user.Lookup(tunUserName); err != nil {
		log.Printf("[TUN-Check] vea-tun 用户不存在: %v", err)
		return false, nil
	}
	log.Printf("[TUN-Check] vea-tun 用户存在")

	// 检查二进制路径
	binaryPath = strings.TrimSpace(binaryPath)
	if binaryPath == "" {
		log.Printf("[TUN-Check] binary path is empty")
		return false, nil
	}
	if _, err := os.Stat(binaryPath); err != nil {
		log.Printf("[TUN-Check] binary not found: %s (%v)", binaryPath, err)
		return false, nil
	}
	log.Printf("[TUN-Check] binary path: %s", binaryPath)

	// 使用 getcap 检查当前 capabilities
	cmd := exec.Command("getcap", binaryPath)
	output, err := cmd.Output()
	if err != nil {
		log.Printf("[TUN-Check] getcap 执行失败: %v", err)
		return false, nil
	}
	log.Printf("[TUN-Check] getcap 输出: %s", string(output))

	// 检查必要的 capabilities
	requiredCaps := []string{"cap_net_admin", "cap_net_bind_service", "cap_net_raw"}
	for _, cap := range requiredCaps {
		if !bytes.Contains(output, []byte(cap)) {
			log.Printf("[TUN-Check] 缺少 capability: %s", cap)
			return false, nil
		}
	}

	log.Printf("[TUN-Check] TUN 权限已完全配置")
	return true, nil
}

// SetupTUN 配置 Linux TUN 权限
func SetupTUN() error {
	if binaryPath, err := FindSingBoxBinary(); err == nil {
		return SetupTUNForBinary(binaryPath)
	}
	if binaryPath, err := FindClashBinary(); err == nil {
		return SetupTUNForBinary(binaryPath)
	}
	return fmt.Errorf("core binary not found: please install sing-box or clash component first")
}

// SetupTUNForBinary 配置 Linux TUN 权限（指定内核二进制路径）
func SetupTUNForBinary(binaryPath string) error {
	// 检查是否以 root 运行
	if os.Geteuid() != 0 {
		return fmt.Errorf("TUN setup requires root privileges")
	}

	// 1. 创建专用用户和组
	if err := ensureTUNUser(); err != nil {
		return fmt.Errorf("failed to create TUN user: %w", err)
	}

	// 2. 设置二进制的 capabilities
	if err := setTUNCapabilitiesForBinary(binaryPath); err != nil {
		return fmt.Errorf("failed to set capabilities: %w", err)
	}

	return nil
}

// SetupTUNForSingBoxBinary 配置 Linux TUN 权限（指定 sing-box 二进制路径）
func SetupTUNForSingBoxBinary(binaryPath string) error {
	return SetupTUNForBinary(binaryPath)
}

// ensureTUNUser 创建 TUN 专用用户和组
func ensureTUNUser() error {
	// 检查用户是否已存在
	if _, err := user.Lookup(tunUserName); err == nil {
		return nil
	}

	// 创建组
	cmd := exec.Command("groupadd", "-r", tunGroupName)
	if err := cmd.Run(); err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 9 {
			// 组已存在（错误码 9），忽略
		} else {
			return fmt.Errorf("groupadd failed: %w", err)
		}
	}

	// 创建系统用户（无登录权限）
	cmd = exec.Command("useradd",
		"-r",                      // 系统用户
		"-s", "/usr/sbin/nologin", // 禁止登录
		"-g", tunGroupName, // 主组
		"-M",                      // 不创建家目录
		"-c", "Vea TUN Mode User", // 注释
		tunUserName,
	)
	if err := cmd.Run(); err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 9 {
			// 用户已存在，忽略
			return nil
		}
		return fmt.Errorf("useradd failed: %w", err)
	}

	return nil
}

// setTUNCapabilities 设置 sing-box 二进制的 capabilities
func setTUNCapabilities() error {
	// 获取 sing-box 二进制路径
	binaryPath, err := FindSingBoxBinary()
	if err != nil {
		return fmt.Errorf("sing-box binary not found: %w", err)
	}

	return setTUNCapabilitiesForBinary(binaryPath)
}

func setTUNCapabilitiesForBinary(binaryPath string) error {
	// 设置文件所有者为 vea-tun 用户
	u, err := user.Lookup(tunUserName)
	if err != nil {
		return fmt.Errorf("user lookup failed: %w", err)
	}

	uid, _ := strconv.Atoi(u.Uid)
	gid, _ := strconv.Atoi(u.Gid)

	if err := os.Chown(binaryPath, uid, gid); err != nil {
		return fmt.Errorf("chown failed: %w", err)
	}

	// chown 会清除文件 capabilities，所以必须在 chown 之后再 setcap。
	cmd := exec.Command("setcap", requiredCapabilities+"+ep", binaryPath)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("setcap failed: %w", err)
	}

	return nil
}

// TUNCapabilityStatus 描述 TUN 权限状态
type TUNCapabilityStatus struct {
	UserExists      bool     // vea-tun 用户是否存在
	BinaryFound     bool     // 内核二进制是否找到
	BinaryPath      string   // 内核二进制路径
	CurrentCaps     []string // 当前已有的 capabilities
	MissingCaps     []string // 缺少的 capabilities
	FullyConfigured bool     // 是否完全配置
}

// GetTUNCapabilityStatus 获取详细的 TUN 权限状态（仅 Linux）
func GetTUNCapabilityStatus() TUNCapabilityStatus {
	if binaryPath, err := FindSingBoxBinary(); err == nil {
		return GetTUNCapabilityStatusForBinary(binaryPath)
	}
	if binaryPath, err := FindClashBinary(); err == nil {
		return GetTUNCapabilityStatusForBinary(binaryPath)
	}
	return TUNCapabilityStatus{}
}

// GetTUNCapabilityStatusForBinary 获取详细的 TUN 权限状态（指定内核二进制路径；仅 Linux）
func GetTUNCapabilityStatusForBinary(binaryPath string) TUNCapabilityStatus {
	status := TUNCapabilityStatus{}

	// 1. 检查 vea-tun 用户是否存在
	if _, err := user.Lookup(tunUserName); err == nil {
		status.UserExists = true
	}

	// 2. 检查二进制
	binaryPath = strings.TrimSpace(binaryPath)
	if binaryPath == "" {
		return status
	}
	if _, err := os.Stat(binaryPath); err != nil {
		return status
	}
	status.BinaryFound = true
	status.BinaryPath = binaryPath

	// 3. 使用 getcap 检查当前 capabilities
	cmd := exec.Command("getcap", binaryPath)
	output, _ := cmd.Output()

	// 解析当前 capabilities
	requiredCaps := []string{"cap_net_admin", "cap_net_bind_service", "cap_net_raw"}
	for _, cap := range requiredCaps {
		if bytes.Contains(output, []byte(cap)) {
			status.CurrentCaps = append(status.CurrentCaps, cap)
		} else {
			status.MissingCaps = append(status.MissingCaps, cap)
		}
	}

	// 4. 判断是否完全配置
	status.FullyConfigured = status.UserExists && status.BinaryFound && len(status.MissingCaps) == 0

	return status
}

// EnsureTUNCapabilities 确保 TUN 权限已配置，缺少则自动修复（仅 Linux）
// 返回 (是否需要重启应用, 错误)
func EnsureTUNCapabilities() (bool, error) {
	if binaryPath, err := FindSingBoxBinary(); err == nil {
		return EnsureTUNCapabilitiesForBinary(binaryPath)
	}
	if binaryPath, err := FindClashBinary(); err == nil {
		return EnsureTUNCapabilitiesForBinary(binaryPath)
	}
	return false, fmt.Errorf("内核未安装，请先安装 sing-box 或 clash 组件")
}

// EnsureTUNCapabilitiesForBinary 确保 TUN 权限已配置（指定内核二进制路径；仅 Linux）
// 返回 (是否需要重启应用, 错误)
func EnsureTUNCapabilitiesForBinary(binaryPath string) (bool, error) {
	// 幂等：避免前端/后台并发触发多次 pkexec，导致多次弹密码框。
	ensureTUNOnceMu.Lock()
	if ensureTUNOnceDone != nil {
		ch := ensureTUNOnceDone
		ensureTUNOnceMu.Unlock()
		<-ch
		ensureTUNOnceMu.Lock()
		needRestart := ensureTUNOnceNeedRestart
		err := ensureTUNOnceErr
		ensureTUNOnceMu.Unlock()
		return needRestart, err
	}
	ensureTUNOnceDone = make(chan struct{})
	ensureTUNOnceMu.Unlock()

	needRestart, err := ensureTUNCapabilitiesOnce(binaryPath)

	ensureTUNOnceMu.Lock()
	ensureTUNOnceNeedRestart = needRestart
	ensureTUNOnceErr = err
	close(ensureTUNOnceDone)
	ensureTUNOnceDone = nil
	ensureTUNOnceMu.Unlock()
	return needRestart, err
}

// CleanConflictingIPTablesRules 清理可能与 TUN 冲突的 iptables 规则
// 主要清理 XRAY/XRAY_SELF 等历史遗留的 TPROXY 规则，这些规则会拦截所有流量导致 TUN 死循环
func CleanConflictingIPTablesRules() {
	cleanConflictingIPTablesOnce.Do(cleanConflictingIPTablesRulesOnce)
}

func cleanConflictingIPTablesRulesOnce() {
	// 缺少必要命令时直接跳过，避免无意义的提权弹窗/报错。
	if _, err := exec.LookPath("iptables"); err != nil {
		return
	}
	if _, err := exec.LookPath("ip"); err != nil {
		return
	}

	// 构建清理脚本：把所有命令合并成一次执行，避免多次弹授权框。
	//
	// NOTE: 这里只做“按已知默认值”的清理（best-effort）。
	// - XRAY / XRAY_SELF 链名与 fwmark/table 组合来自一些常见的 TPROXY 示例配置（Vea 旧版本/外部工具可能会写入）。
	// - 如果用户使用自定义链名或 mark/table，本清理可能覆盖不到；也不会尝试模糊匹配，以避免误删用户规则。
	//
	// 规则不存在是预期场景（不会输出错误）；其他错误会以 [TUN-Cleanup][WARN] 记录，避免完全静默掩盖问题。
	script := `
run_cmd() {
  desc="$1"
  shift

  out=$("$@" 2>&1)
  rc=$?
  if [ $rc -eq 0 ]; then
    echo "$desc"
    return 0
  fi

  case "$out" in
    *"Bad rule"*|*"does a matching rule exist"*|*"No chain/target/match"*|*"does not exist"*|*"RTNETLINK answers: No such file or directory"*)
      # 规则/链不存在：忽略（预期）
      return 0
      ;;
    *)
      echo "[TUN-Cleanup][WARN] ${desc}: ${out}"
      return 0
      ;;
  esac
}

run_cmd "[TUN-Cleanup] 已删除跳转规则: PREROUTING -> XRAY" iptables -t mangle -D PREROUTING -j XRAY
run_cmd "[TUN-Cleanup] 已清空链: XRAY" iptables -t mangle -F XRAY
run_cmd "[TUN-Cleanup] 已删除链: XRAY" iptables -t mangle -X XRAY
run_cmd "[TUN-Cleanup] 已删除跳转规则: OUTPUT -> XRAY_SELF" iptables -t mangle -D OUTPUT -j XRAY_SELF
run_cmd "[TUN-Cleanup] 已清空链: XRAY_SELF" iptables -t mangle -F XRAY_SELF
run_cmd "[TUN-Cleanup] 已删除链: XRAY_SELF" iptables -t mangle -X XRAY_SELF
	run_cmd "[TUN-Cleanup] 已删除 ip rule: fwmark 0x1 table 100" ip rule del fwmark 0x1 table 100
exit 0
`

	if os.Geteuid() != 0 {
		if _, err := exec.LookPath("pkexec"); err != nil {
			return
		}

		socketPath := ResolvectlHelperSocketPath()
		if err := EnsureRootHelper(socketPath, RootHelperEnsureOptions{ParentPID: os.Getpid()}); err != nil {
			log.Printf("[TUN-Cleanup] 启动 root helper 失败: %v", err)
			return
		}
		resp, err := CallRootHelper(socketPath, RootHelperRequest{Op: "tun-cleanup"})
		if err != nil {
			log.Printf("[TUN-Cleanup] 调用 root helper 失败: %v", err)
			return
		}
		if resp.ExitCode != 0 {
			if strings.TrimSpace(resp.Error) != "" {
				log.Printf("[TUN-Cleanup] root helper 执行失败: %s", strings.TrimSpace(resp.Error))
			} else {
				log.Printf("[TUN-Cleanup] root helper 执行失败: exitCode=%d", resp.ExitCode)
			}
		}
		return
	}

	cmd := exec.Command("sh", "-c", script)
	output, err := cmd.CombinedOutput()
	if len(output) > 0 {
		for _, line := range strings.Split(strings.TrimSpace(string(output)), "\n") {
			if line != "" {
				log.Println(line)
			}
		}
	}
	if err != nil {
		log.Printf("[TUN-Cleanup] 清理脚本执行失败: %v", err)
	}
}

func ensureTUNCapabilitiesOnce(binaryPath string) (bool, error) {
	status := GetTUNCapabilityStatusForBinary(binaryPath)

	if status.FullyConfigured {
		return false, nil
	}

	if !status.BinaryFound {
		return false, fmt.Errorf("内核未安装或路径无效，请先安装组件并重试")
	}

	log.Printf("[TUN-Setup] 检测到 TUN 权限不完整，尝试自动配置...")
	log.Printf("[TUN-Setup] 缺少: user=%v, caps=%v", !status.UserExists, status.MissingCaps)

	resolvedBinaryPath := status.BinaryPath

	socketPath := ResolvectlHelperSocketPath()
	if err := EnsureRootHelper(socketPath, RootHelperEnsureOptions{ParentPID: os.Getpid()}); err != nil {
		return false, fmt.Errorf("启动 root helper 失败: %w", err)
	}

	log.Printf("[TUN-Setup] 使用 root helper 配置: setup-tun --binary %s", resolvedBinaryPath)
	resp, err := CallRootHelper(socketPath, RootHelperRequest{Op: "tun-setup", BinaryPath: resolvedBinaryPath})
	if err != nil {
		return false, fmt.Errorf("调用 root helper 失败: %w", err)
	}
	if resp.ExitCode != 0 {
		if strings.TrimSpace(resp.Error) != "" {
			return false, fmt.Errorf("自动配置 TUN 权限失败: %s", strings.TrimSpace(resp.Error))
		}
		return false, fmt.Errorf("自动配置 TUN 权限失败: exitCode=%d", resp.ExitCode)
	}

	// 验证配置
	newStatus := GetTUNCapabilityStatusForBinary(resolvedBinaryPath)
	if !newStatus.FullyConfigured {
		return false, fmt.Errorf("自动配置后权限仍不完整: missingCaps=%v\n请手动运行: sudo setcap '%s+ep' %s",
			newStatus.MissingCaps, requiredCapabilities, resolvedBinaryPath)
	}

	log.Printf("[TUN-Setup] TUN 权限配置完成")
	return false, nil
}
