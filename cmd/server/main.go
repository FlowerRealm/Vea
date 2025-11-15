package main

import (
	"context"
	"embed"
	"flag"
	"io/fs"
	"log"
	"net/http"
	"os/signal"
	"syscall"
	"time"

	"vea/internal/api"
	"vea/internal/persist"
	"vea/internal/service"
	"vea/internal/store"
	"vea/internal/tasks"
)

//go:embed web
var webFS embed.FS

func main() {
	addr := flag.String("addr", ":8080", "HTTP listen address")
	statePath := flag.String("state", "data/state.json", "path to state snapshot")
	flag.Parse()

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
