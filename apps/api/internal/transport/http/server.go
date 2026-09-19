// Package http exposes the application services over REST with an OpenAPI 3.1
// description generated from the handler types (huma on chi).
package http

import (
	"context"
	"fmt"
	"log/slog"
	nethttp "net/http"
	"strings"
	"time"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humachi"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"

	"heatseeker/api/internal/app/auth"
	"heatseeker/api/internal/app/drive"
	"heatseeker/api/internal/app/groups"
	"heatseeker/api/internal/app/materials"
	"heatseeker/api/internal/app/subjects"
	appsync "heatseeker/api/internal/app/sync"
	"heatseeker/api/internal/app/tags"
	"heatseeker/api/internal/platform/config"
)

// Version is stamped at build time via -ldflags.
var Version = "dev"

// Deps are the collaborators the HTTP layer needs. Services may be nil when
// the router is built only to emit the OpenAPI document.
type Deps struct {
	Cfg       *config.Config
	Log       *slog.Logger
	Tokens    *auth.Tokens
	Auth      *auth.Service
	Groups    *groups.Service
	Subjects  *subjects.Service
	Tags      *tags.Service
	Sync      *appsync.Service
	Materials *materials.Service
	Drive     *drive.Service
	Health    func(ctx context.Context) error
}

// Server bundles the router and the huma API (for spec export).
type Server struct {
	Router chi.Router
	API    huma.API
}

// NewServer builds the router with all routes registered.
func NewServer(d Deps) *Server {
	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	// Resolve the client IP once, according to the declared trust model, so
	// that rate limiting and logs key off the real client and not the proxy.
	if d.Cfg != nil && d.Cfg.App.TrustedProxyCount > 0 {
		r.Use(middleware.ClientIPFromXFFTrustedProxies(d.Cfg.App.TrustedProxyCount))
	} else {
		r.Use(middleware.ClientIPFromRemoteAddr)
	}
	r.Use(requestLogger(d.Log))
	r.Use(middleware.Recoverer)
	r.Use(timeoutExceptMedia(60 * time.Second))
	if d.Cfg != nil {
		r.Use(cors.Handler(cors.Options{
			AllowOriginFunc:  allowOrigin(d.Cfg.App.CORSOrigins, d.Cfg.IsDev()),
			AllowedMethods:   []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
			AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-Request-Id", "Idempotency-Key", "Range"},
			ExposedHeaders:   []string{"X-Request-Id", "Content-Range", "Content-Length", "Accept-Ranges", "Content-Disposition"},
			AllowCredentials: false,
			MaxAge:           300,
		}))
	}
	r.Get("/healthz", healthHandler(d))
	r.Get("/readyz", healthHandler(d))

	cfg := huma.DefaultConfig("Heatseeker API", Version)
	cfg.Info.Description = "API ядра Heatseeker: группы, участники, предметы, теги, материалы, Google Диск, синхронизация."
	cfg.Servers = []*huma.Server{{URL: "/api/v1"}}
	cfg.Components.SecuritySchemes = map[string]*huma.SecurityScheme{
		"bearer": {Type: "http", Scheme: "bearer", BearerFormat: "JWT"},
	}

	var api huma.API
	r.Route("/api/v1", func(r chi.Router) {
		if d.Cfg != nil {
			r.Use(authRateLimit(d.Cfg.Auth))
		}
		api = humachi.New(r, cfg)
		api.UseMiddleware(authMiddleware(api, d.Tokens))
		registerAuth(api, d)
		registerMe(api, d)
		registerGroups(api, d)
		registerSubjects(api, d)
		registerTags(api, d)
		registerSync(api, d)
		registerMaterials(api, d)
		registerDrive(api, d)
		registerMedia(api, d)
	})
	return &Server{Router: r, API: api}
}

// ListenAndServe runs the HTTP server until ctx is cancelled.
func (s *Server) ListenAndServe(ctx context.Context, port int, log *slog.Logger) error {
	srv := &nethttp.Server{
		Addr:              fmt.Sprintf(":%d", port),
		Handler:           s.Router,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      120 * time.Second,
		IdleTimeout:       120 * time.Second,
	}
	errCh := make(chan error, 1)
	go func() {
		log.Info("http server listening", "addr", srv.Addr, "docs", fmt.Sprintf("http://localhost:%d/api/v1/docs", port))
		if err := srv.ListenAndServe(); err != nil && err != nethttp.ErrServerClosed {
			errCh <- err
		}
		close(errCh)
	}()
	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		return srv.Shutdown(shutdownCtx)
	}
}

// timeoutExceptMedia bounds request time, except for file transfers that
// legitimately take longer (streams and uploads through the API).
func timeoutExceptMedia(limit time.Duration) func(nethttp.Handler) nethttp.Handler {
	return func(next nethttp.Handler) nethttp.Handler {
		bounded := middleware.Timeout(limit)(next)
		return nethttp.HandlerFunc(func(w nethttp.ResponseWriter, r *nethttp.Request) {
			if isMediaPath(r.URL.Path) {
				next.ServeHTTP(w, r)
				return
			}
			bounded.ServeHTTP(w, r)
		})
	}
}

func isMediaPath(path string) bool {
	return strings.HasPrefix(path, "/api/v1/media/") ||
		(strings.HasPrefix(path, "/api/v1/materials/") && strings.HasSuffix(path, "/stream"))
}

func healthHandler(d Deps) nethttp.HandlerFunc {
	return func(w nethttp.ResponseWriter, r *nethttp.Request) {
		if d.Health != nil {
			if err := d.Health(r.Context()); err != nil {
				nethttp.Error(w, "unhealthy: "+err.Error(), nethttp.StatusServiceUnavailable)
				return
			}
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprintf(w, `{"status":"ok","version":%q}`, Version)
	}
}
