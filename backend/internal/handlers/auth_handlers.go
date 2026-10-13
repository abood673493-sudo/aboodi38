package handlers

import (
	"encoding/json"
	"errors"
	"net"
	"net/http"

	"ssh-manager-panel/internal/auth"
)

type AuthHandlers struct {
	svc *auth.Service
}

func NewAuthHandlers(svc *auth.Service) *AuthHandlers {
	return &AuthHandlers{svc: svc}
}

type loginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type loginResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	Username     string `json:"username"`
	Role         string `json:"role"`
}

func clientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

func (h *AuthHandlers) Login(w http.ResponseWriter, r *http.Request) {
	var req loginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Username == "" || req.Password == "" {
		writeError(w, http.StatusBadRequest, "username and password are required")
		return
	}

	result, err := h.svc.Login(r.Context(), req.Username, req.Password, clientIP(r))
	if err != nil {
		switch {
		case errors.Is(err, auth.ErrAccountLocked):
			writeError(w, http.StatusTooManyRequests, "account temporarily locked, try again later")
		case errors.Is(err, auth.ErrAccountInactive):
			writeError(w, http.StatusForbidden, "account disabled")
		default:
			writeError(w, http.StatusUnauthorized, "invalid username or password")
		}
		return
	}

	writeJSON(w, http.StatusOK, loginResponse{
		AccessToken:  result.AccessToken,
		RefreshToken: result.RefreshToken,
		Username:     result.Username,
		Role:         result.Role,
	})
}

type refreshRequest struct {
	RefreshToken string `json:"refresh_token"`
}

func (h *AuthHandlers) Refresh(w http.ResponseWriter, r *http.Request) {
	var req refreshRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.RefreshToken == "" {
		writeError(w, http.StatusBadRequest, "refresh_token is required")
		return
	}
	access, err := h.svc.Refresh(req.RefreshToken)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "invalid or expired refresh token")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"access_token": access})
}
