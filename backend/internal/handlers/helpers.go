package handlers

import (
	"encoding/json"
	"net/http"

	"ssh-manager-panel/internal/auth"
	"ssh-manager-panel/internal/middleware"
)

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

// claimsFromRequest reads the JWT claims that middleware.RequireAuth
// attached to the request context. Every handler that needs to know "who is
// making this request" (for audit logging and RBAC) goes through this.
func claimsFromRequest(r *http.Request) *auth.Claims {
	c, _ := r.Context().Value(middleware.ClaimsKey).(*auth.Claims)
	return c
}
