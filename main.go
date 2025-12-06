package main

import (
	"context"
	"flag"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"vea/backend/api"
	"vea/backend/persist"
	"vea/backend/service"
	"vea/backend/store"
	"vea/backend/tasks"

	"github.com/gin-gonic/gin"
)

func main() {
	// 检查是否是子命令
	if len(os.Args) > 1 && os.Args[1] == "setup-tun" {
		setupTUNMode()
		return
	}

	addr := flag.String("addr", ":18080", "HTTP listen address")
	statePath := flag.String("state", "data/state.json", "path to state snapshot")
	dev := flag.Bool("dev", false, "enable development mode with verbose logging")
	flag.Parse()

	// 配置日志级别
	if *dev {
		gin.SetMode(gin.DebugMode)
		log.SetFlags(log.LstdFlags | log.Lshortfile)

		// 开发模式：同时输出到终端和日志文件
		// 使用固定路径，避免 Electron 工作目录问题
		logPath := "/tmp/vea-debug.log"
		logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
		if err != nil {
			log.Printf("无法创建日志文件: %v", err)
		} else {
			// 使用 MultiWriter 同时输出到 stdout 和文件
			multiWriter := io.MultiWriter(os.Stdout, logFile)
			log.SetOutput(multiWriter)
			log.Printf("日志同时输出到终端和 %s", logPath)
		}
		log.Println("运行在开发模式 - 显示所有日志")
	} else {
		gin.SetMode(gin.ReleaseMode)
		log.SetFlags(log.LstdFlags)
		// 创建一个只输出错误和致命日志的 writer
		log.SetOutput(&errorOnlyWriter{output: os.Stderr})
	}

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	memory := store.NewMemoryStore()

	if state, err := persist.Load(*statePath); err != nil {
		log.Printf("load snapshot failed: %v", err)
	} else if len(state.Nodes) > 0 || len(state.Configs) > 0 || len(state.GeoResources) > 0 || len(state.Components) > 0 {
		memory.LoadState(state)
		log.Printf("state loaded from %s", *statePath)
	}

	serviceInstance := service.NewService(memory)

	snapshotter := persist.NewSnapshotter(*statePath, serviceInstance)
	memory.SetAfterWrite(snapshotter.Schedule)
	snapshotter.Schedule()

taskRunner := []service.Task{
	&tasks.ConfigSync{Service: serviceInstance, Interval: time.Minute},
	&tasks.GeoSync{Service: serviceInstance, Interval: 12 * time.Hour},
	&tasks.ComponentUpdate{Service: serviceInstance, Interval: 6 * time.Hour},
}
	serviceInstance.AttachTasks(taskRunner...)
	serviceInstance.Start(ctx)

	router := api.NewRouter(serviceInstance)

	srv := &http.Server{
		Addr:    *addr,
		Handler: router,
	}

	go func() {
		<-ctx.Done()
		log.Println("收到退出信号，正在清理...")

		// 先停止代理进程
		if err := serviceInstance.StopProxy(); err != nil {
			log.Printf("停止代理失败: %v", err)
		} else {
			log.Println("代理进程已停止")
		}

		// 然后关闭 HTTP 服务器
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := srv.Shutdown(shutdownCtx); err != nil {
			log.Printf("graceful shutdown failed: %v", err)
		}
	}()

	log.Printf("server listening on %s", srv.Addr)
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("listen: %v", err)
	}
}

// errorOnlyWriter 只输出包含错误关键字的日志
type errorOnlyWriter struct {
	output io.Writer
}

func (w *errorOnlyWriter) Write(p []byte) (n int, err error) {
	s := string(p)
	// 只输出包含这些关键字的日志
	keywords := []string{"error", "failed", "fatal", "panic"}
	for _, keyword := range keywords {
		if strings.Contains(strings.ToLower(s), keyword) {
			return w.output.Write(p)
		}
	}
	// 其他日志不输出
	return len(p), nil
}

// setupTUNMode 设置 TUN 模式权限
func setupTUNMode() {
	log.Println("Setting up TUN mode privileges...")

	// 检查平台
	if os.Getenv("GOOS") == "windows" {
		log.Fatal("TUN setup is not needed on Windows. Just run Vea as Administrator.")
	}
	if os.Getenv("GOOS") == "darwin" {
		log.Fatal("TUN setup is not needed on macOS. Just run Vea with sudo.")
	}

	// Linux 平台检查 root 权限
	if os.Geteuid() != 0 {
		log.Fatal("TUN setup requires root privileges. Run: sudo vea setup-tun")
	}

	// 创建临时 service 实例
	memory := store.NewMemoryStore()

	// 加载状态以获取已安装的组件信息
	if state, err := persist.Load("data/state.json"); err == nil {
		memory.LoadState(state)
	}

	serviceInstance := service.NewService(memory)

	// 执行 TUN 设置
	log.Println("1. Creating TUN user and group...")
	log.Println("2. Setting capabilities on sing-box binary...")

	if err := serviceInstance.SetupTUNPrivileges(); err != nil {
		log.Fatalf("Setup failed: %v", err)
	}

	log.Println("✓ TUN mode configured successfully!")
	log.Println("")
	log.Println("You can now use TUN mode without root privileges.")
	log.Println("The sing-box binary will run as the 'vea-tun' user with CAP_NET_ADMIN capability.")
}
