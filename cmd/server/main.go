package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"config-center/config"
	"config-center/model"
	"config-center/service"
	"config-center/store"
)

var (
	urlStore   *store.URLStore
	logStore   *store.AccessLogStore
	urlSvc     *service.URLService
	redirSvc   *service.RedirectService
	cfg        *config.Config
)

func main() {
	addr := os.Getenv("ADDR")
	if addr == "" {
		addr = ":8080"
	}

	var err error
	cfg = config.Default()
	cfg.Storage.URLFilePath("/app/data/urls.json")
	cfg.Storage.LogFilePath("/app/data/access.log")

	urlStore, err = store.NewURLStore(cfg)
	if err != nil {
		log.Fatalf("Failed to create URLStore: %v", err)
	}

	ctx := context.Background()
	if err := urlStore.Load(ctx); err != nil {
		log.Fatalf("Failed to load URLStore: %v", err)
	}

	logStore, err = store.NewAccessLogStore(cfg)
	if err != nil {
		log.Fatalf("Failed to create AccessLogStore: %v", err)
	}

	if err := logStore.Open(ctx); err != nil {
		log.Fatalf("Failed to open AccessLogStore: %v", err)
	}

	urlSvc, err = service.NewURLService(cfg, urlStore)
	if err != nil {
		log.Fatalf("Failed to create URLService: %v", err)
	}

	redirSvc, err = service.NewRedirectService(urlStore, logStore)
	if err != nil {
		log.Fatalf("Failed to create RedirectService: %v", err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/health", healthHandler)
	mux.HandleFunc("/api/v1/urls", createURLHandler)
	mux.HandleFunc("/api/v1/urls/", getURLHandler)
	mux.HandleFunc("/r/", redirectHandler)
	mux.HandleFunc("/debug/snapshot", snapshotHandler)

	srv := &http.Server{
		Addr:         addr,
		Handler:      mux,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
	}

	go func() {
		log.Printf("URL Shortener server listening on %s", addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server error: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	sig := <-quit
	log.Printf("Received signal %v, shutting down...", sig)

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_ = srv.Shutdown(shutdownCtx)
	_ = urlStore.Close()
	_ = logStore.Close()
	log.Println("Server stopped")
}

func healthHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status": "ok",
		"time":   time.Now().Format(time.RFC3339),
	})
}

func createURLHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req model.CreateReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, fmt.Sprintf("Invalid request: %v", err), http.StatusBadRequest)
		return
	}

	ctx := r.Context()
	result, err := urlSvc.Create(ctx, &req)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to create URL: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

func getURLHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	code := strings.TrimPrefix(r.URL.Path, "/api/v1/urls/")
	if code == "" {
		http.Error(w, "Code is required", http.StatusBadRequest)
		return
	}

	entry, err := urlStore.Get(code)
	if err != nil {
		http.Error(w, fmt.Sprintf("URL not found: %v", err), http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(entry)
}

func redirectHandler(w http.ResponseWriter, r *http.Request) {
	code := strings.TrimPrefix(r.URL.Path, "/r/")
	if code == "" {
		http.Error(w, "Code is required", http.StatusBadRequest)
		return
	}

	ctx := r.Context()
	result, err := redirSvc.HandleRedirect(ctx, &service.RedirectRequest{
		Code:      code,
		Timestamp: time.Now(),
	})
	if err != nil {
		http.Error(w, fmt.Sprintf("Redirect failed: %v", err), http.StatusNotFound)
		return
	}

	if result.Status == 410 {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusGone)
		json.NewEncoder(w).Encode(map[string]string{"error": "URL disabled"})
		return
	}

	http.Redirect(w, r, result.RawURL, http.StatusFound)
}

func snapshotHandler(w http.ResponseWriter, r *http.Request) {
	snapshot := urlStore.RawSnapshot()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"count": len(snapshot),
	})
}