// Package router provides HTTP routing for the configuration center.
// It defines all API routes and their handlers.
package router

import (
	"net/http"
	"strings"

	"config-center/internal/handler"
	"config-center/internal/middleware"
	"config-center/internal/service"
	"config-center/internal/store"
)

// Router is the application HTTP router.
type Router struct {
	mux      *http.ServeMux
	store    store.Store
	services *Services
}

// Services holds all service instances used by handlers.
type Services struct {
	AppService       *service.AppService
	ConfigService    *service.ConfigService
	VersionService   *service.VersionService
	ClientService    *service.ClientService
	AuditService     *service.AuditService
	RollbackService  *service.RollbackService
	ValidationService *service.ValidationService
	DiffService      *service.DiffService
}

// NewRouter creates a new Router with the given store and services.
func NewRouter(st store.Store, svcs *Services) *Router {
	return &Router{
		mux:      http.NewServeMux(),
		store:    st,
		services: svcs,
	}
}

// Handler returns the fully configured HTTP handler with all middleware and routes.
func (r *Router) Handler() http.Handler {
	var h http.Handler = r.mux

	// Apply global middleware
	h = middleware.Recovery(h)
	h = middleware.Logging(h)
	h = middleware.CORS(h)

	return h
}

// RegisterRoutes registers all API routes on the underlying mux.
func (r *Router) RegisterRoutes() {
	h := handler.NewHandlers(
		r.services.AppService,
		r.services.ConfigService,
		r.services.VersionService,
		r.services.ClientService,
		r.services.AuditService,
		r.services.RollbackService,
		r.services.ValidationService,
		r.services.DiffService,
	)

	// Health check endpoints
	r.mux.HandleFunc("/health", r.wrapHandler(h.Health))
	r.mux.HandleFunc("/ready", r.wrapHandler(h.Ready))

	// API routes - use /api prefix
	r.mux.HandleFunc("/api/apps", r.wrapHandler(h.ListApps))       // GET=list, POST=create
	r.mux.HandleFunc("/api/apps/", r.wrapHandler(h.AppByID))       // GET=get, PUT=update, DELETE=delete

	r.mux.HandleFunc("/api/configs", r.wrapHandler(h.ListConfigs))  // GET=list, POST=create
	r.mux.HandleFunc("/api/configs/", r.wrapHandler(h.ConfigByKey)) // GET=get, PUT=update, DELETE=delete

	r.mux.HandleFunc("/api/versions", r.wrapHandler(h.ListVersions))    // GET=list, POST=create
	r.mux.HandleFunc("/api/versions/", r.wrapHandler(h.VersionByNumber)) // GET=get by number

	r.mux.HandleFunc("/api/client/pull", r.wrapHandler(h.ClientPull))
	r.mux.HandleFunc("/api/client/batch-pull", r.wrapHandler(h.ClientBatchPull))

	r.mux.HandleFunc("/api/audit-logs", r.wrapHandler(h.ListAuditLogs))

	r.mux.HandleFunc("/api/rollback", r.wrapHandler(h.Rollback))

	r.mux.HandleFunc("/api/validate", r.wrapHandler(h.ValidateConfig))

	r.mux.HandleFunc("/api/diff", r.wrapHandler(h.DiffConfig))

	// Static file serving for frontend
	fs := http.FileServer(http.Dir("web"))
	r.mux.Handle("/", http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		path := req.URL.Path
		if path == "/" || path == "" {
			// Serve the main page
			req.URL.Path = "/index.html"
		}
		// Only serve static files for non-API, non-empty paths
		if !strings.HasPrefix(path, "/api/") && path != "/health" && path != "/ready" {
			fs.ServeHTTP(w, req)
		} else {
			http.NotFound(w, req)
		}
	}))
}

// wrapHandler wraps a handler function to provide consistent behavior:
// - Method routing (GET, POST, PUT, DELETE)
// - Error recovery
func (r *Router) wrapHandler(h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		// Ensure CORS headers are set for all responses
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, If-None-Match")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")

		// Handle OPTIONS preflight
		if req.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		h(w, req)
	}
}
