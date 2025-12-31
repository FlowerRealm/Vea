package adapters

import (
	"io"
	"os/exec"
	"strings"
	"time"

	"vea/backend/domain"
	"vea/backend/service/nodegroup"
)

// ========== 进程管理结构体 ==========

// ProcessHandle 进程句柄，用于管理内核进程的生命周期
type ProcessHandle struct {
	Cmd        *exec.Cmd // 进程命令
	ConfigPath string    // 配置文件路径
	BinaryPath string    // 二进制文件路径
	StartedAt  time.Time // 启动时间
	Port       int       // 监听端口（用于等待就绪）

	// Done 可选：当外部已经启动了 wait goroutine 并会回收子进程时，用于避免重复 Wait。
	// 约定：该通道在 Cmd.Wait() 返回后 close。
	Done chan struct{}

	// LogCloser 可选：用于关闭日志文件等资源（应在 Cmd.Wait() 返回后调用）。
	LogCloser io.Closer
}

// ProcessConfig 进程启动配置
type ProcessConfig struct {
	BinaryPath  string   // 二进制文件绝对路径
	ConfigDir   string   // 配置文件所在目录（作为工作目录）
	Environment []string // 额外的环境变量

	UsePkexec bool // 可选：通过 pkexec 提权启动（用于需要与系统服务交互的场景）

	Stdout io.Writer // 可选：内核 stdout 的输出目标（默认 os.Stdout）
	Stderr io.Writer // 可选：内核 stderr 的输出目标（默认 os.Stderr）
}

// CoreAdapter 内核适配器接口
// 每个代理内核（Xray、sing-box）都需要实现这个接口
type CoreAdapter interface {
	// Kind 返回内核类型
	Kind() domain.CoreEngineKind

	// BinaryNames 返回二进制文件的可能名称
	BinaryNames() []string

	// SupportedProtocols 返回支持的节点协议列表
	SupportedProtocols() []domain.NodeProtocol

	// SupportsProtocol 检查是否支持特定协议（便捷方法）
	SupportsProtocol(protocol domain.NodeProtocol) bool

	// SupportsInbound 检查是否支持特定入站模式
	SupportsInbound(mode domain.InboundMode) bool

	// BuildConfig 根据运行计划生成内核配置文件（plan 是唯一真相）
	BuildConfig(plan nodegroup.RuntimePlan, geo GeoFiles) ([]byte, error)

	// RequiresPrivileges 检查是否需要特权（主要用于 TUN 模式）
	RequiresPrivileges(config domain.ProxyConfig) bool

	// GetCommandArgs 返回启动内核的命令行参数
	GetCommandArgs(configPath string) []string

	// Start 启动内核进程
	Start(cfg ProcessConfig, configPath string) (*ProcessHandle, error)

	// Stop 停止内核进程
	Stop(handle *ProcessHandle) error

	// WaitForReady 等待内核就绪（通常是检测端口监听）
	WaitForReady(handle *ProcessHandle, timeout time.Duration) error
}

// GeoFiles Geo 资源文件路径
type GeoFiles struct {
	GeoIP        string
	GeoSite      string
	ArtifactsDir string // artifacts 目录的绝对路径，用于构建插件路径等
}

// shortenID 截取 ID 前 8 位（用于生成配置标签）
func shortenID(id string) string {
	if len(id) > 8 {
		return id[:8]
	}
	return id
}

func mergeEnv(base []string, extra []string) []string {
	if len(extra) == 0 {
		return base
	}

	out := make([]string, len(base))
	copy(out, base)

	for _, kv := range extra {
		if kv == "" {
			continue
		}
		key, _, ok := strings.Cut(kv, "=")
		if !ok || key == "" {
			continue
		}
		prefix := key + "="
		replaced := false
		for i := range out {
			if strings.HasPrefix(out[i], prefix) {
				out[i] = kv
				replaced = true
				break
			}
		}
		if !replaced {
			out = append(out, kv)
		}
	}

	return out
}
