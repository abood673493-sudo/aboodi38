// Command server is the SSH Manager Panel API + WebSocket/TLS bridge. It is
// meant to run as an UNPRIVILEGED service account (see deploy/systemd); any
// operation that needs root (creating/deleting Linux users, issuing
// certificates) goes through the narrowly-scoped sshpanel-helper via sudo,
// never by this process running as root itself.
package main

import (
	"context"
	"crypto/tls"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"

	"ssh-manager-panel/internal/auth"
	"ssh-manager-panel/internal/bridge"
	"ssh-manager-panel/internal/certs"
	"ssh-manager-panel/internal/config"
	"ssh-manager-panel/internal/db"
	"ssh-manager-panel/internal/handlers"
	"ssh-manager-panel/internal/middleware"
	"ssh-manager-panel/internal/scheduler"
	"ssh-manager-panel/internal/sshmgr"
)

func main() {
	cfg := config.Load()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	pool, err := db.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("failed to connect to database: %v", err)
	}
	defer pool.Close()

	// --- wiring ---
	tokenMgr := auth.NewTokenManager(cfg.JWTSecret, cfg.AccessTokenTTL, cfg.RefreshTokenTTL)
	authRepo := auth.NewPGRepository(pool)
	authSvc := auth.NewService(authRepo, tokenMgr, cfg.MaxFailedLogins, cfg.LockoutDuration)

	osMgr := sshmgr.NewManager(cfg.SudoHelperPath, cfg.SSHGroup, cfg.ServiceUser)
	accountRepo := handlers.NewPGAccountRepository(pool)
	accountSvc := handlers.NewAccountService(accountRepo, osMgr)

	certMgr := certs.NewManager(osMgr, cfg.CertStorageDir)

	authH := handlers.NewAuthHandlers(authSvc)
	accountH := handlers.NewAccountHandlers(accountSvc)
	dashboardH := handlers.NewDashboardHandlers(accountRepo, pool)
	certH := handlers.NewCertHandlers(certMgr, pool)

	// --- background jobs: real bandwidth sampling + expiration reaper ---
	insertSample := func(ctx context.Context, accountID string, bytesIn, bytesOut uint64) error {
		_, err := pool.Exec(ctx, `
			INSERT INTO bandwidth_samples (account_id, bytes_in_total, bytes_out_total)
			VALUES ($1, $2, $3)`, accountID, int64(bytesIn), int64(bytesOut))
		return err
	}
	poller := scheduler.New(accountRepo, osMgr, accountSvc, insertSample, cfg.StatsPollInterval)
	go poller.Run(ctx)

	// --- router ---
	r := chi.NewRouter()
	r.Use(chimw.RequestID)
	r.Use(chimw.RealIP)
	r.Use(chimw.Logger)
	r.Use(chimw.Recoverer)
	r.Use(chimw.Timeout(30 * time.Second))
	corsOrigins := []string{"*"}
	if cfg.FrontendOrigin != "" {
		// Once a real frontend origin is configured, stop accepting "*" -
		// the panel API should only ever answer browser requests from its
		// own UI.
		corsOrigins = []string{cfg.FrontendOrigin}
	}
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   corsOrigins,
		AllowedMethods:   []string{"GET", "POST", "PUT", "PATCH", "DELETE"},
		AllowedHeaders:   []string{"Authorization", "Content-Type"},
		AllowCredentials: false,
		MaxAge:           300,
	}))
	r.Use(middleware.SecurityHeaders)
	r.Use(middleware.MaxBodyBytes(1 << 20)) // 1 MiB - every request this API accepts is small JSON

	loginLimiter := middleware.NewIPRateLimiter(0.5, 5) // ~1 attempt/2s, burst 5, per IP

	r.Route("/api", func(r chi.Router) {
		r.With(loginLimiter.Limit).Post("/auth/login", authH.Login)
		r.Post("/auth/refresh", authH.Refresh)

		r.Group(func(r chi.Router) {
			r.Use(middleware.RequireAuth(tokenMgr))

			r.Get("/dashboard/status", dashboardH.Status)
			r.Get("/dashboard/usage", dashboardH.Usage)

			r.Get("/accounts", accountH.List)
			r.With(middleware.RequireRole("admin", "operator")).Post("/accounts", accountH.Create)
			r.With(middleware.RequireRole("admin", "operator")).Patch("/accounts/{id}", accountH.Update)
			r.With(middleware.RequireRole("admin", "operator")).Post("/accounts/{id}/disable", accountH.Disable)
			r.With(middleware.RequireRole("admin", "operator")).Post("/accounts/{id}/enable", accountH.Enable)
			r.With(middleware.RequireRole("admin")).Delete("/accounts/{id}", accountH.Delete)

			r.Get("/certificates", certH.List)
			r.With(middleware.RequireRole("admin")).Post("/certificates", certH.Issue)
		})
	})

	srv := &http.Server{Addr: cfg.ListenAddr, Handler: r}

	go func() {
		log.Printf("API listening on %s", cfg.ListenAddr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("API server failed: %v", err)
		}
	}()

	// --- WebSocket/TLS bridge (separate listener, separate port) ---
	br := bridge.New(cfg.SSHDPort, cfg.BridgeDomain)
	bridgeSrv := &http.Server{Addr: cfg.BridgeListenAddr, Handler: http.HandlerFunc(br.ServeHTTP)}

	go func() {
		if cfg.BridgeTLSCert == "" || cfg.BridgeTLSKey == "" {
			log.Printf("bridge: BRIDGE_TLS_CERT/KEY not set - bridge disabled (set them once a certificate has been issued)")
			return
		}
		cert, err := tls.LoadX509KeyPair(cfg.BridgeTLSCert, cfg.BridgeTLSKey)
		if err != nil {
			log.Printf("bridge: failed to load TLS certificate, bridge disabled: %v", err)
			return
		}
		bridgeSrv.TLSConfig = &tls.Config{Certificates: []tls.Certificate{cert}, MinVersion: tls.VersionTLS12}
		log.Printf("WS/TLS SSH bridge listening on %s", cfg.BridgeListenAddr)
		if err := bridgeSrv.ListenAndServeTLS("", ""); err != nil && err != http.ErrServerClosed {
			log.Printf("bridge server stopped: %v", err)
		}
	}()

	<-ctx.Done()
	log.Println("shutting down...")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_ = srv.Shutdown(shutdownCtx)
	_ = bridgeSrv.Shutdown(shutdownCtx)
}
