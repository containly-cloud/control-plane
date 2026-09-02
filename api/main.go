package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"api/internal/app"
	"api/internal/config"
	"api/internal/identity"
	"api/internal/storage"
	"api/internal/system"
	"api/internal/web"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo}))
	ctx, cancelMonitoring := context.WithCancel(context.Background())
	defer cancelMonitoring()

	paths, err := config.ResolvePaths()
	if err != nil {
		logger.Error("unable to resolve Containly data directory", "error", err)
		os.Exit(1)
	}

	store, err := storage.Open(ctx, paths, logger)
	if err != nil {
		logger.Error("unable to open local database", "error", err)
		os.Exit(1)
	}
	defer store.Close()

	if err := store.Migrate(ctx); err != nil {
		logger.Error("unable to migrate local database", "error", err)
		os.Exit(1)
	}
	monitoringSettings, err := store.GetSystemMonitoringSettings(ctx)
	if err != nil {
		logger.Error("unable to load system monitoring settings", "error", err)
		os.Exit(1)
	}
	monitor := system.NewMonitor(system.NewCollector(paths), store, logger)
	monitor.Start(ctx, monitoringSettings)

	handler, err := app.New(app.Dependencies{
		Logger:               logger,
		RootCreator:          identity.NewRootCreator(store),
		SetupStatus:          store,
		Authenticator:        identity.NewAuthenticator(store),
		SystemOverview:       monitor,
		SessionInventory:     store,
		BackupManager:        store,
		ControlUsers:         store,
		MonitoringSettings:   store,
		MonitoringController: monitor,
		WebHandler:           web.HandleWeb(),
	})
	if err != nil {
		logger.Error("unable to configure application", "error", err)
		os.Exit(1)
	}

	server := &http.Server{
		Addr:              ":8888",
		Handler:           handler,
		ReadHeaderTimeout: 10 * time.Second,
	}

	go func() {
		logger.Info("Control Plane started", "address", server.Addr, "data_directory", paths.Root)
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error("Control Plane server failed", "error", err)
			os.Exit(1)
		}
	}()

	shutdown := make(chan os.Signal, 1)
	signal.Notify(shutdown, syscall.SIGINT, syscall.SIGTERM)
	<-shutdown

	shutdownContext, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := server.Shutdown(shutdownContext); err != nil {
		logger.Error("graceful shutdown failed", "error", err)
	}
}
