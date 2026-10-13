package handlers

import (
	"net/http"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"ssh-manager-panel/internal/models"
	"ssh-manager-panel/internal/stats"
)

type DashboardHandlers struct {
	accounts AccountRepository
	pool     *pgxpool.Pool
}

func NewDashboardHandlers(accounts AccountRepository, pool *pgxpool.Pool) *DashboardHandlers {
	return &DashboardHandlers{accounts: accounts, pool: pool}
}

// Status returns real host metrics plus real, DB-derived account counts.
// Every number here comes from a live source: /proc for host metrics, `ps`
// for session counts, Postgres for account totals.
func (h *DashboardHandlers) Status(w http.ResponseWriter, r *http.Request) {
	st, err := stats.Collect()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to collect server status")
		return
	}

	accounts, err := h.accounts.List(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load accounts")
		return
	}
	st.TotalAccounts = len(accounts)

	sessionCounts, _ := stats.ActiveSSHSessionCounts()
	activeAccounts := 0
	totalSessions := 0
	for _, a := range accounts {
		if a.Status == models.StatusActive {
			activeAccounts++
		}
		totalSessions += sessionCounts[a.Username]
	}
	st.ActiveAccounts = activeAccounts
	st.ActiveSessions = totalSessions

	writeJSON(w, http.StatusOK, st)
}

// Usage returns each account's cumulative real bandwidth counters, computed
// as the delta between the most recent two bandwidth_samples rows (which
// the stats poller inserts from systemd IPAccounting data). An account with
// fewer than two samples yet reports null, not zero - there genuinely is no
// measurement yet, and the UI must not present that as "no traffic".
type usageRow struct {
	AccountID     string  `json:"account_id"`
	Username      string  `json:"username"`
	BytesInDelta  *int64  `json:"bytes_in_delta"`
	BytesOutDelta *int64  `json:"bytes_out_delta"`
	SampledAt     *string `json:"sampled_at,omitempty"`
}

func (h *DashboardHandlers) Usage(w http.ResponseWriter, r *http.Request) {
	rows, err := h.pool.Query(r.Context(), `
		WITH ranked AS (
			SELECT account_id, bytes_in_total, bytes_out_total, sampled_at,
			       ROW_NUMBER() OVER (PARTITION BY account_id ORDER BY sampled_at DESC) AS rn
			FROM bandwidth_samples
		)
		SELECT a.id, a.username,
		       newest.bytes_in_total, newest.bytes_out_total, newest.sampled_at,
		       older.bytes_in_total, older.bytes_out_total
		FROM ssh_accounts a
		LEFT JOIN ranked newest ON newest.account_id = a.id AND newest.rn = 1
		LEFT JOIN ranked older  ON older.account_id  = a.id AND older.rn = 2
		WHERE a.status != 'deleted'`)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load usage data")
		return
	}
	defer rows.Close()

	var out []usageRow
	for rows.Next() {
		var (
			id, username                 string
			newIn, newOut, oldIn, oldOut *int64
			sampledAt                    *time.Time
		)
		if err := rows.Scan(&id, &username, &newIn, &newOut, &sampledAt, &oldIn, &oldOut); err != nil {
			continue
		}
		row := usageRow{AccountID: id, Username: username}
		if newIn != nil && oldIn != nil && newOut != nil && oldOut != nil {
			dIn := *newIn - *oldIn
			dOut := *newOut - *oldOut
			if dIn >= 0 {
				row.BytesInDelta = &dIn
			}
			if dOut >= 0 {
				row.BytesOutDelta = &dOut
			}
		}
		if sampledAt != nil {
			s := sampledAt.Format(time.RFC3339)
			row.SampledAt = &s
		}
		out = append(out, row)
	}
	writeJSON(w, http.StatusOK, out)
}
