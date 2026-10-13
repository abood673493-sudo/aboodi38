package handlers

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"

	"ssh-manager-panel/internal/auth"
	"ssh-manager-panel/internal/models"
	"ssh-manager-panel/internal/stats"
)

type AccountHandlers struct {
	svc *AccountService
}

func NewAccountHandlers(svc *AccountService) *AccountHandlers {
	return &AccountHandlers{svc: svc}
}

func handleServiceError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, ErrUsernameTaken):
		writeError(w, http.StatusConflict, err.Error())
	case errors.Is(err, ErrAccountNotFound):
		writeError(w, http.StatusNotFound, err.Error())
	case errors.Is(err, auth.ErrWeakPassword):
		writeError(w, http.StatusBadRequest, err.Error())
	default:
		writeError(w, http.StatusInternalServerError, err.Error())
	}
}

func (h *AccountHandlers) List(w http.ResponseWriter, r *http.Request) {
	accounts, err := h.svc.List(r.Context())
	if err != nil {
		handleServiceError(w, err)
		return
	}

	// Enrich with real, live active-connection counts. If `ps` can't be
	// read for some reason, counts are simply omitted (zero) rather than
	// guessed.
	sessionCounts, _ := stats.ActiveSSHSessionCounts()
	for _, a := range accounts {
		a.ActiveConnections = sessionCounts[a.Username]
	}

	writeJSON(w, http.StatusOK, accounts)
}

func (h *AccountHandlers) Create(w http.ResponseWriter, r *http.Request) {
	var req models.CreateAccountRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	claims := claimsFromRequest(r)
	account, err := h.svc.CreateAccount(r.Context(), claims.AdminID, req)
	if err != nil {
		handleServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, account)
}

func (h *AccountHandlers) Update(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var req models.UpdateAccountRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	claims := claimsFromRequest(r)
	account, err := h.svc.Update(r.Context(), claims.AdminID, id, req)
	if err != nil {
		handleServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, account)
}

func (h *AccountHandlers) Disable(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	claims := claimsFromRequest(r)
	if err := h.svc.Disable(r.Context(), claims.AdminID, id); err != nil {
		handleServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "disabled"})
}

func (h *AccountHandlers) Enable(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	claims := claimsFromRequest(r)
	if err := h.svc.Enable(r.Context(), claims.AdminID, id); err != nil {
		handleServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "active"})
}

func (h *AccountHandlers) Delete(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	claims := claimsFromRequest(r)
	if err := h.svc.Delete(r.Context(), claims.AdminID, id); err != nil {
		handleServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}
