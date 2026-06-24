package main

import (
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"metapi/aggrsite/config"
	"metapi/aggrsite/db"
	"metapi/aggrsite/handler"
	_ "metapi/aggrsite/platform" // register all platform adapters
	"metapi/aggrsite/service"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

func main() {
	// Setup structured logging
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	slog.SetDefault(logger)

	// Init Config and Database
	config.Init()
	db.Init()

	// Start cron scheduler
	service.StartScheduler()

	// Setup HTTP router
	r := chi.NewRouter()
	r.Use(middleware.RealIP)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	handler.MountRoutes(r)
	MountWeb(r)

	// Start server
	addr := config.C.ListenHost + ":" + config.C.Port
	slog.Info("AggrSite starting", "address", addr)

	// Graceful shutdown
	go func() {
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
		<-sigCh
		slog.Info("Shutting down...")
		service.StopScheduler()
		os.Exit(0)
	}()

	if err := http.ListenAndServe(addr, r); err != nil {
		slog.Error("Server failed", "err", err)
		os.Exit(1)
	}
}
