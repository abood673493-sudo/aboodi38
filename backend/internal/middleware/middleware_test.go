package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"ssh-manager-panel/internal/auth"
)

func testTokenManager() *auth.TokenManager {
	return auth.NewTokenManager("test-secret-at-least-32-characters-long", 15*time.Minute, 7*24*time.Hour)
}

func okHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
}

func TestRequireAuth_RejectsMissingToken(t *testing.T) {
	tm := testTokenManager()
	h := RequireAuth(tm)(okHandler())

	req := httptest.NewRequest(http.MethodGet, "/api/accounts", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 with no Authorization header, got %d", rec.Code)
	}
}

func TestRequireAuth_RejectsGarbageToken(t *testing.T) {
	tm := testTokenManager()
	h := RequireAuth(tm)(okHandler())

	req := httptest.NewRequest(http.MethodGet, "/api/accounts", nil)
	req.Header.Set("Authorization", "Bearer not-a-real-token")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 with garbage token, got %d", rec.Code)
	}
}

func TestRequireAuth_RejectsRefreshTokenOnAccessRoute(t *testing.T) {
	tm := testTokenManager()
	refresh, err := tm.IssueRefreshToken("1", "root", "admin")
	if err != nil {
		t.Fatal(err)
	}
	h := RequireAuth(tm)(okHandler())

	req := httptest.NewRequest(http.MethodGet, "/api/accounts", nil)
	req.Header.Set("Authorization", "Bearer "+refresh)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected a refresh token to be rejected on an access-only route, got %d", rec.Code)
	}
}

func TestRequireAuth_AcceptsValidAccessToken(t *testing.T) {
	tm := testTokenManager()
	access, err := tm.IssueAccessToken("1", "root", "admin")
	if err != nil {
		t.Fatal(err)
	}
	h := RequireAuth(tm)(okHandler())

	req := httptest.NewRequest(http.MethodGet, "/api/accounts", nil)
	req.Header.Set("Authorization", "Bearer "+access)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 with a valid access token, got %d", rec.Code)
	}
}

func TestRequireRole_BlocksInsufficientRole(t *testing.T) {
	h := RequireRole("admin")(okHandler())

	req := httptest.NewRequest(http.MethodDelete, "/api/accounts/1", nil)
	ctx := context.WithValue(req.Context(), ClaimsKey, &auth.Claims{Role: "viewer"})
	req = req.WithContext(ctx)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected a viewer to be forbidden from an admin-only route, got %d", rec.Code)
	}
}

func TestRequireRole_AllowsListedRole(t *testing.T) {
	h := RequireRole("admin", "operator")(okHandler())

	req := httptest.NewRequest(http.MethodPost, "/api/accounts", nil)
	ctx := context.WithValue(req.Context(), ClaimsKey, &auth.Claims{Role: "operator"})
	req = req.WithContext(ctx)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected an operator to be allowed, got %d", rec.Code)
	}
}

func TestRequireRole_NoClaimsIsUnauthorized(t *testing.T) {
	h := RequireRole("admin")(okHandler())

	req := httptest.NewRequest(http.MethodDelete, "/api/accounts/1", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 when no claims are attached (RequireAuth skipped), got %d", rec.Code)
	}
}

func TestIPRateLimiter_BlocksBurstExceeded(t *testing.T) {
	limiter := NewIPRateLimiter(0.001, 2) // effectively no refill during the test
	h := limiter.Limit(okHandler())

	req := httptest.NewRequest(http.MethodPost, "/api/auth/login", nil)
	req.RemoteAddr = "203.0.113.5:12345"

	codes := make([]int, 0, 3)
	for i := 0; i < 3; i++ {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		codes = append(codes, rec.Code)
	}

	if codes[0] != http.StatusOK || codes[1] != http.StatusOK {
		t.Fatalf("expected the first 2 requests (burst) to succeed, got %v", codes)
	}
	if codes[2] != http.StatusTooManyRequests {
		t.Fatalf("expected the 3rd request to be rate limited, got %v", codes)
	}
}

func TestIPRateLimiter_TracksClientsIndependently(t *testing.T) {
	limiter := NewIPRateLimiter(0.001, 1)
	h := limiter.Limit(okHandler())

	req1 := httptest.NewRequest(http.MethodPost, "/api/auth/login", nil)
	req1.RemoteAddr = "203.0.113.5:12345"
	req2 := httptest.NewRequest(http.MethodPost, "/api/auth/login", nil)
	req2.RemoteAddr = "203.0.113.9:54321"

	rec1 := httptest.NewRecorder()
	h.ServeHTTP(rec1, req1)
	rec2 := httptest.NewRecorder()
	h.ServeHTTP(rec2, req2)

	if rec1.Code != http.StatusOK || rec2.Code != http.StatusOK {
		t.Fatalf("expected independent IPs to each get their own bucket, got %d and %d", rec1.Code, rec2.Code)
	}
}

func TestSecurityHeaders_SetsExpectedHeaders(t *testing.T) {
	h := SecurityHeaders(okHandler())
	req := httptest.NewRequest(http.MethodGet, "/api/dashboard/status", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	want := map[string]string{
		"X-Content-Type-Options": "nosniff",
		"X-Frame-Options":        "DENY",
		"Referrer-Policy":        "no-referrer",
		"Cache-Control":          "no-store",
	}
	for k, v := range want {
		if got := rec.Header().Get(k); got != v {
			t.Errorf("header %s = %q, want %q", k, got, v)
		}
	}
}
