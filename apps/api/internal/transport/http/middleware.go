package http

import (
	"context"
	"log/slog"
	"net"
	nethttp "net/http"
	"strings"
	"time"

	"github.com/danielgtaylor/huma/v2"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/httprate"
	"github.com/google/uuid"

	"heatseeker/api/internal/app/auth"
	"heatseeker/api/internal/domain"
	"heatseeker/api/internal/platform/config"
)

// Principal is the authenticated caller.
type Principal struct {
	UserID   uuid.UUID
	DeviceID uuid.UUID
}

type principalKey struct{}

// principalFrom returns the caller or ErrUnauthorized.
func principalFrom(ctx context.Context) (Principal, error) {
	p, ok := ctx.Value(principalKey{}).(Principal)
	if !ok {
		return Principal{}, domain.ErrUnauthorized
	}
	return p, nil
}

// optionalPrincipal returns the caller if authenticated.
func optionalPrincipal(ctx context.Context) *Principal {
	if p, ok := ctx.Value(principalKey{}).(Principal); ok {
		return &p
	}
	return nil
}

// authMiddleware parses a bearer token when present and enforces it on
// operations that declare the "bearer" security requirement.
func authMiddleware(api huma.API, tokens *auth.Tokens) func(huma.Context, func(huma.Context)) {
	return func(ctx huma.Context, next func(huma.Context)) {
		required := false
		for _, sec := range ctx.Operation().Security {
			if _, ok := sec["bearer"]; ok {
				required = true
				break
			}
		}
		header := ctx.Header("Authorization")
		if header == "" {
			if required {
				_ = huma.WriteErr(api, ctx, nethttp.StatusUnauthorized, "authentication required")
				return
			}
			next(ctx)
			return
		}
		raw, ok := strings.CutPrefix(header, "Bearer ")
		if !ok || tokens == nil {
			_ = huma.WriteErr(api, ctx, nethttp.StatusUnauthorized, "malformed authorization header")
			return
		}
		claims, err := tokens.ParseAccess(strings.TrimSpace(raw))
		if err != nil {
			_ = huma.WriteErr(api, ctx, nethttp.StatusUnauthorized, "invalid or expired token")
			return
		}
		userID, err := uuid.Parse(claims.Subject)
		if err != nil {
			_ = huma.WriteErr(api, ctx, nethttp.StatusUnauthorized, "invalid token subject")
			return
		}
		deviceID, _ := uuid.Parse(claims.DeviceID)
		next(huma.WithValue(ctx, principalKey{}, Principal{UserID: userID, DeviceID: deviceID}))
	}
}

// keyByClientIP buckets rate limits by the client IP resolved by chi's
// ClientIPFrom* middleware (see NewServer), never by spoofable headers.
func keyByClientIP(r *nethttp.Request) (string, error) {
	ip := middleware.GetClientIP(r.Context())
	if ip == "" {
		if host, _, err := net.SplitHostPort(r.RemoteAddr); err == nil {
			ip = host
		} else {
			ip = r.RemoteAddr
		}
	}
	return httprate.CanonicalizeIP(ip), nil
}

// authRateLimit throttles credential endpoints per client IP.
func authRateLimit(cfg config.Auth) func(nethttp.Handler) nethttp.Handler {
	register := httprate.LimitBy(max(cfg.RegisterRatePerMinute, 1), time.Minute, keyByClientIP)
	login := httprate.LimitBy(max(cfg.LoginRatePerMinute, 1), time.Minute, keyByClientIP)
	return func(next nethttp.Handler) nethttp.Handler {
		return nethttp.HandlerFunc(func(w nethttp.ResponseWriter, r *nethttp.Request) {
			switch {
			case r.Method == nethttp.MethodPost && strings.HasSuffix(r.URL.Path, "/auth/register"):
				register(next).ServeHTTP(w, r)
			case r.Method == nethttp.MethodPost && (strings.HasSuffix(r.URL.Path, "/auth/login") || strings.HasSuffix(r.URL.Path, "/auth/google")):
				login(next).ServeHTTP(w, r)
			default:
				next.ServeHTTP(w, r)
			}
		})
	}
}

// requestLogger writes one structured line per request.
func requestLogger(log *slog.Logger) func(nethttp.Handler) nethttp.Handler {
	if log == nil {
		log = slog.Default()
	}
	return func(next nethttp.Handler) nethttp.Handler {
		return nethttp.HandlerFunc(func(w nethttp.ResponseWriter, r *nethttp.Request) {
			start := time.Now()
			ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)
			next.ServeHTTP(ww, r)
			if r.URL.Path == "/healthz" || r.URL.Path == "/readyz" {
				return
			}
			log.Info("http",
				"method", r.Method,
				"path", r.URL.Path,
				"status", ww.Status(),
				"bytes", ww.BytesWritten(),
				"duration_ms", time.Since(start).Milliseconds(),
				"request_id", middleware.GetReqID(r.Context()),
				"ip", middleware.GetClientIP(r.Context()),
			)
		})
	}
}
