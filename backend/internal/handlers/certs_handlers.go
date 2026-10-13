package handlers

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"ssh-manager-panel/internal/certs"
)

type CertHandlers struct {
	mgr  *certs.Manager
	pool *pgxpool.Pool
}

func NewCertHandlers(mgr *certs.Manager, pool *pgxpool.Pool) *CertHandlers {
	return &CertHandlers{mgr: mgr, pool: pool}
}

type issueCertRequest struct {
	Domain string `json:"domain"`
	Email  string `json:"email,omitempty"`
}

// Issue requests a new certificate for a subdomain the admin has already
// pointed at this server via DNS. The panel records the attempt and its
// outcome; it never claims a certificate is active without certbot actually
// having succeeded.
func (h *CertHandlers) Issue(w http.ResponseWriter, r *http.Request) {
	var req issueCertRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if err := certs.ValidateDomain(req.Domain); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	_, err := h.pool.Exec(r.Context(), `
		INSERT INTO certificates (domain, cert_path, key_path, status)
		VALUES ($1, $2, $3, 'pending')
		ON CONFLICT (domain) DO UPDATE SET status = 'pending', last_error = NULL, updated_at = now()`,
		req.Domain, h.mgr.CertPath(req.Domain), h.mgr.KeyPath(req.Domain))
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to record certificate request")
		return
	}

	if err := h.mgr.Issue(r.Context(), req.Domain, req.Email); err != nil {
		_, _ = h.pool.Exec(r.Context(), `UPDATE certificates SET status = 'failed', last_error = $2, updated_at = now() WHERE domain = $1`, req.Domain, err.Error())
		writeError(w, http.StatusBadGateway, "certificate issuance failed: "+err.Error())
		return
	}

	_, _ = h.pool.Exec(r.Context(), `
		UPDATE certificates SET status = 'active', issued_at = now(), expires_at = now() + interval '90 days', last_error = NULL, updated_at = now()
		WHERE domain = $1`, req.Domain)

	writeJSON(w, http.StatusOK, map[string]string{"status": "active", "domain": req.Domain})
}

func (h *CertHandlers) List(w http.ResponseWriter, r *http.Request) {
	rows, err := h.pool.Query(r.Context(), `
		SELECT id, domain, provider, status, issued_at, expires_at, COALESCE(last_error, '')
		FROM certificates ORDER BY created_at DESC`)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list certificates")
		return
	}
	defer rows.Close()

	type certRow struct {
		ID        string  `json:"id"`
		Domain    string  `json:"domain"`
		Provider  string  `json:"provider"`
		Status    string  `json:"status"`
		IssuedAt  *string `json:"issued_at,omitempty"`
		ExpiresAt *string `json:"expires_at,omitempty"`
		LastError string  `json:"last_error,omitempty"`
	}
	var out []certRow
	for rows.Next() {
		var c certRow
		var issuedAt, expiresAt *time.Time
		if err := rows.Scan(&c.ID, &c.Domain, &c.Provider, &c.Status, &issuedAt, &expiresAt, &c.LastError); err != nil {
			continue
		}
		if issuedAt != nil {
			s := issuedAt.Format(time.RFC3339)
			c.IssuedAt = &s
		}
		if expiresAt != nil {
			s := expiresAt.Format(time.RFC3339)
			c.ExpiresAt = &s
		}
		out = append(out, c)
	}
	writeJSON(w, http.StatusOK, out)
}
