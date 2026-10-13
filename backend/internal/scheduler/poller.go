// Package scheduler runs the periodic background jobs: sampling real
// per-account bandwidth counters and reaping accounts whose expiration date
// has passed. Both only ever read real data or call through sshmgr - there
// is no synthetic data generation here.
package scheduler

import (
	"context"
	"log"
	"time"

	"ssh-manager-panel/internal/handlers"
	"ssh-manager-panel/internal/models"
	"ssh-manager-panel/internal/sshmgr"
)

type Poller struct {
	repo         handlers.AccountRepository
	os           *sshmgr.Manager
	accounts     *handlers.AccountService
	insertSample func(ctx context.Context, accountID string, bytesIn, bytesOut uint64) error
	interval     time.Duration
}

func New(
	repo handlers.AccountRepository,
	os *sshmgr.Manager,
	accounts *handlers.AccountService,
	insertSample func(ctx context.Context, accountID string, bytesIn, bytesOut uint64) error,
	interval time.Duration,
) *Poller {
	return &Poller{repo: repo, os: os, accounts: accounts, insertSample: insertSample, interval: interval}
}

func (p *Poller) Run(ctx context.Context) {
	ticker := time.NewTicker(p.interval)
	defer ticker.Stop()

	// Run once immediately on startup instead of waiting a full interval.
	p.tick(ctx)

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			p.tick(ctx)
		}
	}
}

func (p *Poller) tick(ctx context.Context) {
	if n, err := p.accounts.ReapExpired(ctx); err != nil {
		log.Printf("scheduler: reap expired accounts failed: %v", err)
	} else if n > 0 {
		log.Printf("scheduler: reaped %d expired account(s)", n)
	}

	accounts, err := p.repo.List(ctx)
	if err != nil {
		log.Printf("scheduler: list accounts failed: %v", err)
		return
	}
	for _, a := range accounts {
		if a.Status != models.StatusActive || a.LinuxUID == nil {
			continue
		}
		ingress, egress, err := p.os.IPAccounting(ctx, *a.LinuxUID)
		if err != nil {
			log.Printf("scheduler: ip accounting for %s failed: %v", a.Username, err)
			continue
		}
		if err := p.insertSample(ctx, a.ID, ingress, egress); err != nil {
			log.Printf("scheduler: insert bandwidth sample for %s failed: %v", a.Username, err)
		}
	}
}
