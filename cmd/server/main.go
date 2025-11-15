package main

import (
	"context"
	"embed"
	"flag"
	"io"
	"io/fs"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"vea/internal/api"
	"vea/internal/persist"
	"vea/internal/service"
	"vea/internal/store"
	"vea/internal/tasks"

	"github.com/gin-gonic/gin"
)

//go:embed web
var webFS embed.FS

func main() {
	addr := flag.String("addr", ":8080", "HTTP listen address")
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

	webRoot, err := fs.Sub(webFS, "web")
	if err != nil {
		log.Fatalf("failed to create web sub filesystem: %v", err)
	}
	router := api.NewRouter(serviceInstance, webRoot)

	srv := &http.Server{
		Addr:    *addr,
		Handler: router,
	}

	go func() {
		<-ctx.Done()
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
	keywords := []string{"error", "failed", "fatal", "panic", "Error", "Failed", "Fatal", "Panic"}
	for _, keyword := range keywords {
		if contains(s, keyword) {
			return w.output.Write(p)
		}
	}
	// 其他日志不输出
	return len(p), nil
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) && indexOf(s, substr) >= 0)
}

func indexOf(s, substr string) int {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}
