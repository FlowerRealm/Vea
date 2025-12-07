//go:build linux
// +build linux

package service

import (
	"bufio"
	"bytes"
	"fmt"
	"log"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"strconv"

	"vea/backend/domain"
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

// SetupTUNPrivileges 设置 TUN 模式所需权限（需要 root 权限）
func (s *Service) SetupTUNPrivileges() error {
	// 检查是否以 root 运行
	if os.Geteuid() != 0 {
		return fmt.Errorf("TUN setup requires root privileges")
	}

	// 1. 创建专用用户和组
	if err := s.ensureTUNUser(); err != nil {
		return fmt.Errorf("failed to create TUN user: %w", err)
	}

	// 2. 设置 sing-box 二进制的 capabilities
	if err := s.setTUNCapabilities(); err != nil {
		return fmt.Errorf("failed to set capabilities: %w", err)
	}

	return nil
}

// ensureTUNUser 创建 TUN 专用用户和组
func (s *Service) ensureTUNUser() error {
	// 检查用户是否已存在
	if _, err := user.Lookup(tunUserName); err == nil {
		// 用户已存在
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
func (s *Service) setTUNCapabilities() error {
	// 获取 sing-box 二进制路径
	binaryPath, err := s.getEngineBinaryPath(domain.EngineSingBox)
	if err != nil {
		return fmt.Errorf("sing-box binary not found: %w", err)
	}

	// 设置所有必要的 capabilities
	cmd := exec.Command("setcap", requiredCapabilities+"+ep", binaryPath)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("setcap failed: %w", err)
	}

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

	return nil
}

// TUNCapabilityStatus 描述 TUN 权限状态
type TUNCapabilityStatus struct {
	UserExists     bool     // vea-tun 用户是否存在
	BinaryFound    bool     // sing-box 二进制是否找到
	BinaryPath     string   // sing-box 二进制路径
	CurrentCaps    []string // 当前已有的 capabilities
	MissingCaps    []string // 缺少的 capabilities
	FullyConfigured bool    // 是否完全配置
}

// CheckTUNCapabilities 检查 TUN 权限是否已配置
func (s *Service) CheckTUNCapabilities() (bool, error) {
	status := s.GetTUNCapabilityStatus()
	return status.FullyConfigured, nil
}

// GetTUNCapabilityStatus 获取详细的 TUN 权限状态
func (s *Service) GetTUNCapabilityStatus() TUNCapabilityStatus {
	status := TUNCapabilityStatus{}

	// 1. 检查 vea-tun 用户是否存在
	if _, err := user.Lookup(tunUserName); err == nil {
		status.UserExists = true
		log.Printf("[TUN-Check] vea-tun 用户存在")
	} else {
		log.Printf("[TUN-Check] vea-tun 用户不存在: %v", err)
	}

	// 2. 检查 sing-box 二进制
	binaryPath, err := s.getEngineBinaryPath(domain.EngineSingBox)
	if err != nil {
		log.Printf("[TUN-Check] 获取 sing-box 路径失败: %v", err)
		return status
	}
	status.BinaryFound = true
	status.BinaryPath = binaryPath
	log.Printf("[TUN-Check] sing-box 路径: %s", binaryPath)

	// 3. 使用 getcap 检查当前 capabilities
	cmd := exec.Command("getcap", binaryPath)
	output, err := cmd.Output()
	if err != nil {
		log.Printf("[TUN-Check] getcap 执行失败: %v", err)
	} else {
		log.Printf("[TUN-Check] getcap 输出: %s", string(output))
	}

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

	if status.FullyConfigured {
		log.Printf("[TUN-Check] TUN 权限已完全配置")
	} else {
		log.Printf("[TUN-Check] TUN 权限未完全配置: userExists=%v, binaryFound=%v, missingCaps=%v",
			status.UserExists, status.BinaryFound, status.MissingCaps)
	}

	return status
}

// EnsureTUNCapabilities 确保 TUN 权限已配置，缺少则自动修复
// 返回 (是否需要重启应用, 错误)
func (s *Service) EnsureTUNCapabilities() (bool, error) {
	status := s.GetTUNCapabilityStatus()

	if status.FullyConfigured {
		return false, nil
	}

	if !status.BinaryFound {
		return false, fmt.Errorf("sing-box 未安装，请先安装 sing-box 组件")
	}

	log.Printf("[TUN-Setup] 检测到 TUN 权限不完整，尝试自动配置...")
	log.Printf("[TUN-Setup] 缺少: user=%v, caps=%v", !status.UserExists, status.MissingCaps)

	// 尝试使用 pkexec 自动提权配置
	// 构建 setcap 命令
	binaryPath := status.BinaryPath

	// 如果用户不存在，先创建用户
	if !status.UserExists {
		log.Printf("[TUN-Setup] 使用 pkexec 创建 vea-tun 用户...")
		cmd := exec.Command("pkexec", "useradd", "-r", "-s", "/usr/sbin/nologin", "-M", tunUserName)
		var stderr bytes.Buffer
		cmd.Stderr = &stderr
		if err := cmd.Run(); err != nil {
			// 检查是否因为用户已存在而失败
			if !bytes.Contains(stderr.Bytes(), []byte("already exists")) &&
				!bytes.Contains(stderr.Bytes(), []byte("已存在")) {
				return false, fmt.Errorf("创建 vea-tun 用户失败: %v, stderr: %s\n请手动运行: sudo useradd -r -s /usr/sbin/nologin -M vea-tun", err, stderr.String())
			}
		}
		log.Printf("[TUN-Setup] vea-tun 用户已创建")
	}

	// 设置 capabilities
	if len(status.MissingCaps) > 0 {
		log.Printf("[TUN-Setup] 使用 pkexec 设置 capabilities...")

		// 先 chown（必须在 setcap 之前，因为 chown 会清除 capabilities）
		cmd := exec.Command("pkexec", "chown", tunUserName+":"+tunUserName, binaryPath)
		var stderr bytes.Buffer
		cmd.Stderr = &stderr
		if err := cmd.Run(); err != nil {
			return false, fmt.Errorf("设置文件所有者失败: %v, stderr: %s\n请手动运行: sudo chown %s:%s %s",
				err, stderr.String(), tunUserName, tunUserName, binaryPath)
		}
		log.Printf("[TUN-Setup] chown 完成")

		// 然后 setcap
		cmd = exec.Command("pkexec", "setcap", requiredCapabilities+"+ep", binaryPath)
		stderr.Reset()
		cmd.Stderr = &stderr
		if err := cmd.Run(); err != nil {
			return false, fmt.Errorf("设置 capabilities 失败: %v, stderr: %s\n请手动运行: sudo setcap '%s+ep' %s",
				err, stderr.String(), requiredCapabilities, binaryPath)
		}
		log.Printf("[TUN-Setup] setcap 完成")
	}

	// 验证配置
	newStatus := s.GetTUNCapabilityStatus()
	if !newStatus.FullyConfigured {
		return false, fmt.Errorf("自动配置后权限仍不完整: missingCaps=%v\n请手动运行: sudo setcap '%s+ep' %s",
			newStatus.MissingCaps, requiredCapabilities, binaryPath)
	}

	log.Printf("[TUN-Setup] TUN 权限配置完成")
	return false, nil
}

// StartTUNProcess 启动 TUN 进程（sing-box 已有 capabilities，直接运行）
func (s *Service) StartTUNProcess(binaryPath, configPath string) (*exec.Cmd, error) {
	log.Printf("[TUN-Process] 启动 TUN 进程: binary=%s, config=%s", binaryPath, configPath)

	// 验证 binary capabilities
	capCmd := exec.Command("getcap", binaryPath)
	capOutput, err := capCmd.Output()
	if err != nil {
		log.Printf("[TUN-Process] 警告：无法验证 capabilities: %v", err)
	} else {
		log.Printf("[TUN-Process] Binary capabilities: %s", string(capOutput))
	}

	// 验证文件权限
	fileInfo, err := os.Stat(binaryPath)
	if err != nil {
		log.Printf("[TUN-Process] 警告：无法获取文件信息: %v", err)
	} else {
		log.Printf("[TUN-Process] Binary 权限: %v", fileInfo.Mode())
	}

	// 创建命令（直接运行，sing-box 已有 capabilities）
	cmd := exec.Command(binaryPath, "run", "-c", configPath)

	// 设置环境变量
	// 1. ENABLE_DEPRECATED_SPECIAL_OUTBOUNDS 用于兼容旧配置
	// 2. 把 plugins 目录添加到 PATH，让 sing-box 能找到 v2ray-plugin 等插件
	pluginsDir := filepath.Join(artifactsRoot, "plugins", "v2ray-plugin")
	currentPath := os.Getenv("PATH")
	newPath := pluginsDir + ":" + currentPath
	cmd.Env = append(os.Environ(),
		"ENABLE_DEPRECATED_SPECIAL_OUTBOUNDS=true",
		"PATH="+newPath,
	)

	// 实时输出 stdout 和 stderr
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to create stdout pipe: %w", err)
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to create stderr pipe: %w", err)
	}

	// 启动进程
	log.Printf("[TUN-Process] 执行: %s %v", binaryPath, cmd.Args[1:])
	if err := cmd.Start(); err != nil {
		log.Printf("[TUN-Process] 启动失败: %v", err)
		return nil, fmt.Errorf("failed to start TUN process: %w", err)
	}

	log.Printf("[TUN-Process] 进程已启动, PID: %d", cmd.Process.Pid)

	// 启动 goroutine 实时读取 stdout
	go func() {
		scanner := bufio.NewScanner(stdout)
		for scanner.Scan() {
			log.Printf("[sing-box] %s", scanner.Text())
		}
	}()

	// 启动 goroutine 实时读取 stderr
	go func() {
		scanner := bufio.NewScanner(stderr)
		for scanner.Scan() {
			log.Printf("[sing-box-err] %s", scanner.Text())
		}
	}()

	return cmd, nil
}

// CleanupTUNUser 清理 TUN 用户（用于卸载）
func (s *Service) CleanupTUNUser() error {
	if os.Geteuid() != 0 {
		return fmt.Errorf("cleanup requires root privileges")
	}

	// 删除用户
	cmd := exec.Command("userdel", tunUserName)
	_ = cmd.Run() // 忽略错误

	// 删除组
	cmd = exec.Command("groupdel", tunGroupName)
	_ = cmd.Run() // 忽略错误

	return nil
}
