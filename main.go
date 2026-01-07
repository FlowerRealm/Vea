package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"time"

	"vea/backend/api"
	"vea/backend/persist"
	"vea/backend/repository"
	"vea/backend/repository/events"
	"vea/backend/repository/memory"
	"vea/backend/service"
	"vea/backend/service/component"
	configsvc "vea/backend/service/config"
	"vea/backend/service/frouter"
	"vea/backend/service/geo"
	"vea/backend/service/nodes"
	"vea/backend/service/proxy"
	"vea/backend/service/shared"
	"vea/backend/tasks"

	"github.com/gin-gonic/gin"
)

func main() {
	os.Exit(run())
}

func run() int {
	// 检查是否是子命令
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "setup-tun":
			if err := setupTUNMode(os.Args[2:]); err != nil {
				log.Print(err)
				return 1
			}
			return 0
		case "resolvectl-helper":
			runResolvectlHelper()
			return 0
		case "resolvectl-shim":
			runResolvectlShim()
			return 0
		}
	}

	addr := flag.String("addr", ":19080", "HTTP listen address")
	statePath := flag.String("state", "data/state.json", "path to state snapshot")
	dev := flag.Bool("dev", false, "enable development mode with verbose logging")
	flag.Parse()

	// 配置日志级别
	if *dev {
		gin.SetMode(gin.DebugMode)
		log.SetFlags(log.LstdFlags | log.Lshortfile)
		log.Println("运行在开发模式 - 显示所有日志")
	} else {
		gin.SetMode(gin.ReleaseMode)
		log.SetFlags(log.LstdFlags)
	}

	appLogPath, appLogStartedAt, closeAppLog := setupAppLogging()
	if closeAppLog != nil {
		defer closeAppLog()
	}

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	// ========== 新架构初始化 ==========

	// 1. 创建事件总线
	eventBus := events.NewBus()

	// 2. 创建内存存储
	memStore := memory.NewStore(eventBus)

	// 3. 加载状态（严格版本校验）
	hasStateFile := true
	if _, err := os.Stat(*statePath); err != nil {
		if os.IsNotExist(err) {
			hasStateFile = false
		}
	}

	state, err := persist.LoadV2(*statePath)
	if err != nil {
		log.Printf("load snapshot failed: %v", err)
		log.Printf("拒绝启动以避免覆盖 state 文件: %s", *statePath)
		log.Printf("请移动/删除该文件或修正 schemaVersion 后重试")
		return 1
	}

	memStore.LoadState(state)
	if hasStateFile {
		log.Printf("state loaded from %s", *statePath)
	} else {
		log.Printf("未找到状态文件 %s，将以空状态启动", *statePath)
	}

	// 4. 创建仓储层
	nodeRepo := memory.NewNodeRepo(memStore)
	frouterRepo := memory.NewFRouterRepo(memStore)
	configRepo := memory.NewConfigRepo(memStore)
	geoRepo := memory.NewGeoRepo(memStore)
	componentRepo := memory.NewComponentRepo(memStore)
	settingsRepo := memory.NewSettingsRepo(memStore)

	repos := repository.NewRepositories(memStore, nodeRepo, frouterRepo, configRepo, geoRepo, componentRepo, settingsRepo)

	// 5. 创建服务层
	nodeSvc := nodes.NewService(nodeRepo)
	frouterSvc := frouter.NewService(frouterRepo, nodeRepo)

	// 创建速度测量器并注入到测量相关服务
	speedMeasurer := proxy.NewSpeedMeasurer(componentRepo, geoRepo, settingsRepo)
	nodeSvc.SetMeasurer(speedMeasurer)
	frouterSvc.SetMeasurer(speedMeasurer)

	configSvc := configsvc.NewService(configRepo, nodeSvc, frouterSvc)
	proxySvc := proxy.NewService(frouterRepo, nodeRepo, componentRepo, settingsRepo)
	componentSvc := component.NewService(componentRepo)
	geoSvc := geo.NewService(geoRepo)

	// 6. 创建 Facade（门面服务）
	facade := service.NewFacade(nodeSvc, frouterSvc, configSvc, proxySvc, componentSvc, geoSvc, repos)
	facade.SetAppLog(appLogPath, appLogStartedAt)

	// 7. 设置持久化（事件驱动）
	snapshotter := persist.NewSnapshotterV2(*statePath, memStore)
	snapshotter.SubscribeEvents(eventBus)

	// 7.1 确保核心组件存在（清空数据后也应显示 xray/sing-box/clash）
	if err := componentSvc.EnsureDefaultComponents(context.Background()); err != nil {
		log.Printf("ensure default components failed: %v", err)
	}

	// 7.2 确保默认 Geo 资源存在
	if err := geoSvc.EnsureDefaultResources(context.Background()); err != nil {
		log.Printf("ensure default geo resources failed: %v", err)
	}

	// 7.25 确保默认 FRouter 存在（空状态启动也应可用）
	if err := facade.EnsureDefaultFRouter(context.Background()); err != nil {
		log.Printf("ensure default frouter failed: %v", err)
	}

	// 7.3 启动后台任务（订阅/Geo）
	tasks.NewScheduler(configSvc, geoSvc).Start(ctx)

	// 8. 创建路由
	router := api.NewRouter(facade)

	srv := &http.Server{
		Addr:    *addr,
		Handler: router,
	}

	cleanupDone := make(chan struct{})
	go func() {
		<-ctx.Done()
		log.Println("收到退出信号，正在清理...")

		// 停止代理进程
		if err := facade.StopProxy(); err != nil {
			log.Printf("停止代理失败: %v", err)
		} else {
			log.Println("代理进程已停止")
		}

		// 保存最终状态
		if err := snapshotter.SaveNow(); err != nil {
			log.Printf("保存状态失败: %v", err)
		}

		// 然后关闭 HTTP 服务器
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := srv.Shutdown(shutdownCtx); err != nil {
			log.Printf("graceful shutdown failed: %v", err)
		}
		close(cleanupDone)
	}()

	log.Printf("server listening on %s", srv.Addr)
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Printf("listen: %v", err)
		cancel()
		<-cleanupDone
		return 1
	}
	<-cleanupDone
	return 0
}

func setupAppLogging() (path string, startedAt time.Time, closeFn func()) {
	startedAt = time.Now()
	path = filepath.Join(shared.ArtifactsRoot, "runtime", "app.log")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		log.Printf("[AppLog] create log dir failed: %v", err)
		return "", time.Time{}, nil
	}

	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		log.Printf("[AppLog] open log file failed (%s): %v", path, err)
		return "", time.Time{}, nil
	}

	_, _ = fmt.Fprintf(f, "----- app start %s pid=%d -----\n", startedAt.Format(time.RFC3339Nano), os.Getpid())
	log.SetOutput(io.MultiWriter(os.Stderr, f))
	log.Printf("[AppLog] writing to %s", path)
	return path, startedAt, func() { _ = f.Close() }
}

// setupTUNMode 设置 TUN 模式权限
func setupTUNMode(args []string) error {
	log.Println("Setting up TUN mode privileges...")

	fs := flag.NewFlagSet("setup-tun", flag.ContinueOnError)
	binary := fs.String("binary", "", "path to core binary (optional)")
	singBoxBinary := fs.String("singbox-binary", "", "path to sing-box binary (optional)")
	if err := fs.Parse(args); err != nil {
		return err
	}

	switch runtime.GOOS {
	case "windows":
		return errors.New("TUN setup is not needed on Windows. Just run Vea as Administrator.")
	case "darwin":
		return errors.New("TUN setup is not needed on macOS. Just run Vea with sudo.")
	case "linux":
		// Linux 平台需要 root 权限（该子命令一般通过 sudo 调用）
		if os.Geteuid() != 0 {
			return errors.New("TUN setup requires root privileges. Run: sudo vea setup-tun")
		}
		var err error
		binaryPath := strings.TrimSpace(*binary)
		if binaryPath == "" {
			binaryPath = strings.TrimSpace(*singBoxBinary) // legacy flag
		}
		if binaryPath != "" {
			err = shared.SetupTUNForBinary(binaryPath)
		} else {
			err = shared.SetupTUN()
		}
		if err != nil {
			return fmt.Errorf("TUN setup failed: %w", err)
		}
		log.Println("TUN setup complete.")
		return nil
	default:
		return fmt.Errorf("TUN setup is not supported on %s", runtime.GOOS)
	}
}
