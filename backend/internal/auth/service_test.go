package auth

import (
	"context"
	"errors"
	"testing"
	"time"

	"ssh-manager-panel/internal/models"
)

// fakeRepo is an in-memory stand-in for the Postgres-backed Repository,
// letting the lockout/failed-attempt logic in Service be tested without a
// real database.
type fakeRepo struct {
	admins      map[string]*models.Admin // by username
	auditEvents []string
}

func newFakeRepo() *fakeRepo {
	return &fakeRepo{admins: map[string]*models.Admin{}}
}

func (f *fakeRepo) GetAdminByUsername(_ context.Context, username string) (*models.Admin, error) {
	a, ok := f.admins[username]
	if !ok {
		return nil, nil
	}
	return a, nil
}

func (f *fakeRepo) RecordFailedLogin(_ context.Context, adminID string, lockedUntil *time.Time) error {
	for _, a := range f.admins {
		if a.ID == adminID {
			a.FailedLogins++
			a.LockedUntil = lockedUntil
		}
	}
	return nil
}

func (f *fakeRepo) ResetFailedLogins(_ context.Context, adminID string) error {
	for _, a := range f.admins {
		if a.ID == adminID {
			a.FailedLogins = 0
			a.LockedUntil = nil
		}
	}
	return nil
}

func (f *fakeRepo) InsertAuditLog(_ context.Context, _ *string, action, target string, _ map[string]any, _ string) error {
	f.auditEvents = append(f.auditEvents, action+":"+target)
	return nil
}

func newTestService(repo *fakeRepo) *Service {
	tm := NewTokenManager("test-secret-at-least-32-characters-long", 15*time.Minute, 7*24*time.Hour)
	return NewService(repo, tm, 3, time.Minute)
}

func mustHash(t *testing.T, pw string) string {
	t.Helper()
	h, err := HashPassword(pw)
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}
	return h
}

func TestLogin_Success(t *testing.T) {
	repo := newFakeRepo()
	hash := mustHash(t, "CorrectHorseBattery9!")
	repo.admins["root"] = &models.Admin{ID: "1", Username: "root", PasswordHash: hash, Role: models.RoleAdmin, IsActive: true}

	svc := newTestService(repo)
	res, err := svc.Login(context.Background(), "root", "CorrectHorseBattery9!", "127.0.0.1")
	if err != nil {
		t.Fatalf("expected success, got error: %v", err)
	}
	if res.AccessToken == "" || res.RefreshToken == "" {
		t.Fatal("expected non-empty tokens")
	}
	if res.Role != "admin" {
		t.Fatalf("expected role admin, got %s", res.Role)
	}
}

func TestLogin_WrongPassword(t *testing.T) {
	repo := newFakeRepo()
	hash := mustHash(t, "CorrectHorseBattery9!")
	repo.admins["root"] = &models.Admin{ID: "1", Username: "root", PasswordHash: hash, Role: models.RoleAdmin, IsActive: true}

	svc := newTestService(repo)
	_, err := svc.Login(context.Background(), "root", "wrong-password", "127.0.0.1")
	if !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("expected ErrInvalidCredentials, got %v", err)
	}
}

func TestLogin_UnknownUserDoesNotLeak(t *testing.T) {
	repo := newFakeRepo()
	svc := newTestService(repo)
	_, err := svc.Login(context.Background(), "nosuchuser", "whatever123!A", "127.0.0.1")
	if !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("expected ErrInvalidCredentials for unknown user, got %v", err)
	}
}

func TestLogin_LocksAfterMaxFailedAttempts(t *testing.T) {
	repo := newFakeRepo()
	hash := mustHash(t, "CorrectHorseBattery9!")
	repo.admins["root"] = &models.Admin{ID: "1", Username: "root", PasswordHash: hash, Role: models.RoleAdmin, IsActive: true}

	svc := newTestService(repo) // maxFailedLogins = 3

	for i := 0; i < 2; i++ {
		if _, err := svc.Login(context.Background(), "root", "wrong", "127.0.0.1"); !errors.Is(err, ErrInvalidCredentials) {
			t.Fatalf("attempt %d: expected ErrInvalidCredentials, got %v", i, err)
		}
	}
	// third failure should trip the lock
	_, err := svc.Login(context.Background(), "root", "wrong", "127.0.0.1")
	if !errors.Is(err, ErrAccountLocked) {
		t.Fatalf("expected ErrAccountLocked on 3rd failure, got %v", err)
	}

	// even the correct password must now be rejected while locked
	_, err = svc.Login(context.Background(), "root", "CorrectHorseBattery9!", "127.0.0.1")
	if !errors.Is(err, ErrAccountLocked) {
		t.Fatalf("expected ErrAccountLocked while locked, got %v", err)
	}
}

func TestLogin_DisabledAdminRejected(t *testing.T) {
	repo := newFakeRepo()
	hash := mustHash(t, "CorrectHorseBattery9!")
	repo.admins["root"] = &models.Admin{ID: "1", Username: "root", PasswordHash: hash, Role: models.RoleAdmin, IsActive: false}

	svc := newTestService(repo)
	_, err := svc.Login(context.Background(), "root", "CorrectHorseBattery9!", "127.0.0.1")
	if !errors.Is(err, ErrAccountInactive) {
		t.Fatalf("expected ErrAccountInactive, got %v", err)
	}
}

func TestTokenManager_RejectsTamperedToken(t *testing.T) {
	tm := NewTokenManager("test-secret-at-least-32-characters-long", 15*time.Minute, 7*24*time.Hour)
	tok, err := tm.IssueAccessToken("1", "root", "admin")
	if err != nil {
		t.Fatal(err)
	}
	tampered := tok[:len(tok)-2] + "xx"
	if _, err := tm.Parse(tampered); err == nil {
		t.Fatal("expected tampered token to be rejected")
	}
}

func TestPasswordStrength(t *testing.T) {
	cases := map[string]bool{
		"short1!A":              false,
		"alllowercase12345":     false,
		"NoSymbolsOrDigitsHere": false,
		"Valid-Password123!":    true,
	}
	for pw, wantOK := range cases {
		err := ValidatePasswordStrength(pw)
		gotOK := err == nil
		if gotOK != wantOK {
			t.Errorf("ValidatePasswordStrength(%q) ok=%v, want %v (err=%v)", pw, gotOK, wantOK, err)
		}
	}
}
