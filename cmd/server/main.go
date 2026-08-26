// Package main is the entry point for the Configuration Center server.
// It initializes all components, starts the HTTP server, and handles graceful shutdown.
package main

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"config-center/internal/config"
	"config-center/internal/handler"
	"config-center/internal/router"
	"config-center/internal/service"
	"config-center/internal/store"
	"config-center/pkg/logger"
)

func main() {
	// Parse command line flags
	configFile := flag.String("config", "", "Path to configuration file (JSON)")
	flag.Parse()

	// Load configuration
	cfg, err := config.Load(*configFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to load configuration: %v\n", err)
		os.Exit(1)
	}

	// Set up logger
	logLevel := cfg.GetLoggerLevel()
	logger.Default().SetLevel(logLevel)
	logger.Infof("Starting Config Center server...")
	logger.Infof("Configuration: %s", cfg.String())

	// Initialize store
	var st store.Store
	switch cfg.Storage.Type {
	case "memory":
		st = store.NewMemoryStore()
	case "file":
		if cfg.Storage.FilePath == "" {
			logger.Warnf("No file path specified for file storage, using memory store")
			st = store.NewMemoryStore()
		} else {
			// File store would be initialized here in a full implementation
			logger.Warnf("File store not implemented, falling back to memory store")
			st = store.NewMemoryStore()
		}
	default:
		logger.Infof("Using in-memory store (type: %s)", cfg.Storage.Type)
		st = store.NewMemoryStore()
	}

	defer st.Close()

	// Initialize services
	appSvc := service.NewAppService(st)
	configSvc := service.NewConfigService(st, appSvc)
	versionSvc := service.NewVersionService(st, appSvc, configSvc)
	clientSvc := service.NewClientService(st, appSvc, cfg.Cache.Enabled)
	auditSvc := service.NewAuditService(st)
	rollbackSvc := service.NewRollbackService(st, appSvc, configSvc, versionSvc, auditSvc)
	validationSvc := service.NewValidationService(st, appSvc)
	diffSvc := service.NewDiffService(st, appSvc)

	// Create default app if none exist
	ctx := context.Background()
	apps, _, err := appSvc.ListApps(ctx, 1, 1)
	if err == nil && len(apps) == 0 {
		logger.Infof("Initializing default application...")
		for _, defaultApp := range appSvc.DefaultApps() {
			if _, err := appSvc.CreateApp(ctx, defaultApp.ID, defaultApp.Name, defaultApp.Description, defaultApp.Owner); err != nil {
				logger.Warnf("Failed to create default app %s: %v", defaultApp.ID, err)
			}
		}
	}

	// Set up services for notification
	// Hook up config change notifications to client service
	go func() {
		// This would listen for config change events in a full implementation
	}()

	// Initialize router
	svcs := &router.Services{
		AppService:       appSvc,
		ConfigService:    configSvc,
		VersionService:   versionSvc,
		ClientService:    clientSvc,
		AuditService:     auditSvc,
		RollbackService:  rollbackSvc,
		ValidationService: validationSvc,
		DiffService:      diffSvc,
	}

	r := router.NewRouter(st, svcs)
	r.RegisterRoutes()

	// Initialize defect handler for defect verification endpoints
	defectHandler, err := handler.NewDefectHandler(cfg)
	if err != nil {
		logger.Warnf("Failed to initialize defect handler: %v", err)
	} else {
		r.SetDefectHandler(defectHandler)
	}

	handler := r.Handler()

	// Create HTTP server
	addr := fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port)
	srv := &http.Server{
		Addr:         addr,
		Handler:      handler,
		ReadTimeout:  cfg.Server.ReadTimeout,
		WriteTimeout: cfg.Server.WriteTimeout,
		IdleTimeout:  cfg.Server.IdleTimeout,
	}

	// Start server in a goroutine
	go func() {
		logger.Infof("Config Center server listening on %s", addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Fatalf("Failed to start server: %v", err)
		}
	}()

	// Wait for interrupt signal for graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	sig := <-quit
	logger.Infof("Received signal %v, shutting down gracefully...", sig)

	// Create shutdown context with timeout
	shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.Server.ShutdownTimeout)
	defer cancel()

	// Shutdown the HTTP server
	if err := srv.Shutdown(shutdownCtx); err != nil {
		logger.Errorf("Server shutdown error: %v", err)
	}

	// Clean up resources
	clientSvc.Close()
	if err := st.Close(); err != nil {
		logger.Errorf("Store close error: %v", err)
	}

	logger.Infof("Server stopped")
	os.Exit(0)
}
