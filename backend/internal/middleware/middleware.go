// Package middleware holds HTTP middleware: JWT authentication, role
// enforcement, and per-IP rate limiting on sensitive endpoints.
package middleware

import (
	"context"
	"net"
	"net/http"
	"sync"

	"golang.org/x/time/rate"

	"ssh-manager-panel/internal/auth"
)

type ctxKey string

const ClaimsKey ctxKey = "claims"

// RequireAuth validates the Bearer access token and attaches its claims to
// the request context. Every account-management and stats route sits behind
// this - there is no unauthenticated access to anything that touches the
// server.
func RequireAuth(tm *auth.TokenManager) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			h := r.Header.Get("Authorization")
			const prefix = "Bearer "
			if len(h) <= len(prefix) || h[:len(prefix)] != prefix {
				http.Error(w, "missing bearer token", http.StatusUnauthorized)
				return
			}
			claims, err := tm.Parse(h[len(prefix):])
			if err != nil || claims.TokenUse != "access" {
				http.Error(w, "invalid or expired token", http.StatusUnauthorized)
				return
			}
			ctx := context.WithValue(r.Context(), ClaimsKey, claims)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// RequireRole restricts a route to one or more admin roles. A "viewer" can
// read the dashboard but cannot be allowed to create/delete SSH accounts,
// and only "admin" can manage certificates or other admins.
func RequireRole(roles ...string) func(http.Handler) http.Handler {
	allowed := make(map[string]bool, len(roles))
	for _, r := range roles {
		allowed[r] = true
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			claims, ok := r.Context().Value(ClaimsKey).(*auth.Claims)
			if !ok {
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}
			if !allowed[claims.Role] {
				http.Error(w, "forbidden: insufficient role", http.StatusForbidden)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// IPRateLimiter gives each client IP its own token bucket. Used on
// /api/auth/login so credential stuffing can't run at wire speed.
type IPRateLimiter struct {
	mu       sync.Mutex
	limiters map[string]*rate.Limiter
	r        rate.Limit
	b        int
}

func NewIPRateLimiter(ratePerSecond float64, burst int) *IPRateLimiter {
	return &IPRateLimiter{
		limiters: make(map[string]*rate.Limiter),
		r:        rate.Limit(ratePerSecond),
		b:        burst,
	}
}

func (l *IPRateLimiter) get(ip string) *rate.Limiter {
	l.mu.Lock()
	defer l.mu.Unlock()
	lim, ok := l.limiters[ip]
	if !ok {
		lim = rate.NewLimiter(l.r, l.b)
		l.limiters[ip] = lim
	}
	return lim
}

func clientIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		return xff
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

func (l *IPRateLimiter) Limit(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ip := clientIP(r)
		if !l.get(ip).Allow() {
			http.Error(w, "too many requests, slow down", http.StatusTooManyRequests)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// MaxBodyBytes rejects any request body larger than limit before it reaches
// a handler's json.Decode - without this, an unauthenticated client could
// send a multi-gigabyte body to /api/auth/login and exhaust memory before
// any auth check runs.
func MaxBodyBytes(limit int64) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			r.Body = http.MaxBytesReader(w, r.Body, limit)
			next.ServeHTTP(w, r)
		})
	}
}

// SecurityHeaders sets the small set of response headers that cost nothing
// and close off classes of attack against the admin panel: clickjacking
// (framing the login page), MIME-sniffing a JSON error body as HTML/JS, and
// leaking the panel's URLs (which can contain account usernames) via the
// Referer header on outbound links. HSTS is intentionally left to the
// reverse proxy (see deploy/nginx) since only it knows whether TLS is
// actually terminated in front of this process.
func SecurityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("X-Frame-Options", "DENY")
		h.Set("Referrer-Policy", "no-referrer")
		h.Set("Cache-Control", "no-store")
		next.ServeHTTP(w, r)
	})
}
