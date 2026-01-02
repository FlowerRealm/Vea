package main

import (
	"context"
	"flag"
	"log"
	"net/http"
	"os"
	"os/signal"
	"runtime"
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
	// 检查是否是子命令
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "setup-tun":
			setupTUNMode()
			return
		case "resolvectl-helper":
			runResolvectlHelper()
			return
		case "resolvectl-shim":
			runResolvectlShim()
			return
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
		os.Exit(1)
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

	configSvc := configsvc.NewService(configRepo, nodeSvc)
	proxySvc := proxy.NewService(frouterRepo, nodeRepo, componentRepo, settingsRepo)
	componentSvc := component.NewService(componentRepo)
	geoSvc := geo.NewService(geoRepo)

	// 6. 创建 Facade（门面服务）
	facade := service.NewFacade(nodeSvc, frouterSvc, configSvc, proxySvc, componentSvc, geoSvc, repos)

	// 7. 设置持久化（事件驱动）
	snapshotter := persist.NewSnapshotterV2(*statePath, memStore)
	snapshotter.SubscribeEvents(eventBus)

	// 7.1 确保核心组件存在（清空数据后也应显示 xray/sing-box）
	if err := componentSvc.EnsureDefaultComponents(context.Background()); err != nil {
		log.Printf("ensure default components failed: %v", err)
	}

	// 7.2 确保默认 Geo 资源存在
	if err := geoSvc.EnsureDefaultResources(context.Background()); err != nil {
		log.Printf("ensure default geo resources failed: %v", err)
	}

	// 7.3 启动后台任务（订阅/Geo）
	tasks.NewScheduler(configSvc, geoSvc).Start(ctx)

	// 8. 创建路由
	router := api.NewRouter(facade)

	srv := &http.Server{
		Addr:    *addr,
		Handler: router,
	}

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
	}()

	log.Printf("server listening on %s", srv.Addr)
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("listen: %v", err)
	}
}

// setupTUNMode 设置 TUN 模式权限
func setupTUNMode() {
	log.Println("Setting up TUN mode privileges...")

	fs := flag.NewFlagSet("setup-tun", flag.ContinueOnError)
	singBoxBinary := fs.String("singbox-binary", "", "path to sing-box binary (optional)")
	if err := fs.Parse(os.Args[2:]); err != nil {
		log.Fatal(err)
	}

	switch runtime.GOOS {
	case "windows":
		log.Fatal("TUN setup is not needed on Windows. Just run Vea as Administrator.")
	case "darwin":
		log.Fatal("TUN setup is not needed on macOS. Just run Vea with sudo.")
	case "linux":
		// Linux 平台需要 root 权限（该子命令一般通过 sudo 调用）
		if os.Geteuid() != 0 {
			log.Fatal("TUN setup requires root privileges. Run: sudo vea setup-tun")
		}
		var err error
		if *singBoxBinary != "" {
			err = shared.SetupTUNForSingBoxBinary(*singBoxBinary)
		} else {
			err = shared.SetupTUN()
		}
		if err != nil {
			log.Fatalf("TUN setup failed: %v", err)
		}
		log.Println("TUN setup complete.")
	default:
		log.Fatalf("TUN setup is not supported on %s", runtime.GOOS)
	}
}
