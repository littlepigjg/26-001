package handler

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"config-center/internal/config"
	"config-center/internal/model"
	"config-center/internal/service"
	"config-center/internal/store"
	"config-center/pkg/logger"
	"config-center/pkg/response"
)

// DefectHandler handles defect verification endpoints.
type DefectHandler struct {
	cfg      *config.Config
	urlStore *store.URLStore
}

// NewDefectHandler creates a new DefectHandler.
func NewDefectHandler(cfg *config.Config) (*DefectHandler, error) {
	urlStore, err := store.NewURLStore(cfg)
	if err != nil {
		logger.Errorf("Failed to create URLStore: %v", err)
		return nil, err
	}
	logger.Infof("DefectHandler initialized successfully")
	return &DefectHandler{cfg: cfg, urlStore: urlStore}, nil
}

// VerifyContextCancel tests context cancellation in health checks.
func (h *DefectHandler) VerifyContextCancel(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	start := time.Now()
	err := h.urlStore.Load(ctx)
	elapsed := time.Since(start)

	result := map[string]interface{}{
		"operation":     "url_store_load_with_cancelled_context",
		"elapsed_ms":    elapsed.Milliseconds(),
		"error":         fmt.Sprintf("%v", err),
		"defect_exists": err == nil,
	}

	if err == nil {
		response.Success(w, result)
	} else {
		response.JSON(w, http.StatusServiceUnavailable, result)
	}
}

// VerifyContextTimeout tests context timeout in health checks.
func (h *DefectHandler) VerifyContextTimeout(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	start := time.Now()
	err := h.urlStore.Load(ctx)
	elapsed := time.Since(start)

	defectExists := err == nil && elapsed > 150*time.Millisecond

	result := map[string]interface{}{
		"operation":     "url_store_load_with_short_timeout",
		"timeout_ms":    50,
		"elapsed_ms":    elapsed.Milliseconds(),
		"error":         fmt.Sprintf("%v", err),
		"defect_exists": defectExists,
	}

	if defectExists {
		response.Success(w, result)
	} else {
		response.JSON(w, http.StatusServiceUnavailable, result)
	}
}

// VerifyCreateWithTimeout tests URL creation with context timeout.
func (h *DefectHandler) VerifyCreateWithTimeout(w http.ResponseWriter, r *http.Request) {
	urlSvc, err := service.NewURLService(h.cfg, h.urlStore)
	if err != nil {
		response.InternalError(w, err.Error())
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
	defer cancel()

	start := time.Now()
	_, err2 := urlSvc.Create(ctx, &model.CreateReq{
		RawURL:     "https://example.com/test/defect/verify",
		CustomCode: "verify1",
		MaxVisits:  10,
	})
	elapsed := time.Since(start)

	defectExists := err2 == nil && elapsed > 150*time.Millisecond

	result := map[string]interface{}{
		"operation":     "url_create_with_short_timeout",
		"timeout_ms":    30,
		"elapsed_ms":    elapsed.Milliseconds(),
		"error":         fmt.Sprintf("%v", err2),
		"defect_exists": defectExists,
	}

	if defectExists {
		response.Success(w, result)
	} else {
		response.JSON(w, http.StatusServiceUnavailable, result)
	}
}

// VerifyBasicFunctionality ensures basic operations work.
func (h *DefectHandler) VerifyBasicFunctionality(w http.ResponseWriter, r *http.Request) {
	ctx := context.Background()

	if err := h.urlStore.Load(ctx); err != nil {
		response.InternalError(w, fmt.Sprintf("Load failed: %v", err))
		return
	}

	shortURL := &model.ShortURL{
		Code:      "basic1",
		RawURL:    "https://example.com/basic",
		CreatedAt: time.Now(),
		Visits:    0,
		Custom:    false,
		Disabled:  false,
	}

	if err := h.urlStore.Save(shortURL, false); err != nil {
		response.InternalError(w, fmt.Sprintf("Save failed: %v", err))
		return
	}

	got, err := h.urlStore.Get("basic1")
	if err != nil {
		response.InternalError(w, fmt.Sprintf("Get failed: %v", err))
		return
	}

	snapshot := h.urlStore.RawSnapshot()

	response.Success(w, map[string]interface{}{
		"load_ok":      true,
		"save_ok":      true,
		"get_raw_url":  got.RawURL,
		"snapshot_len": len(snapshot),
	})
}
