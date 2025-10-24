package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"vea/internal/api"
	"vea/internal/service"
	"vea/internal/store"
	"vea/internal/tasks"
)

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	memory := store.NewMemoryStore()
	serviceInstance := service.NewService(memory)

	taskRunner := []service.Task{
		&tasks.ConfigSync{Service: serviceInstance, Interval: time.Minute},
		&tasks.GeoSync{Service: serviceInstance, Interval: 12 * time.Hour},
		&tasks.NodeProbe{Service: serviceInstance, Interval: 45 * time.Second},
	}
	serviceInstance.AttachTasks(taskRunner...)
	serviceInstance.Start(ctx)

	router := api.NewRouter(serviceInstance)

	srv := &http.Server{
		Addr:    getAddr(),
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

func getAddr() string {
	addr := os.Getenv("VEA_HTTP_ADDR")
	if addr == "" {
		addr = ":8080"
	}
	return addr
}
