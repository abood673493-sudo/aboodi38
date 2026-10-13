package handlers

import (
	"context"
	"errors"
	"testing"
	"time"

	"ssh-manager-panel/internal/models"
)

// --- fakes -------------------------------------------------------------

type fakeAccountRepo struct {
	byID       map[string]*models.SSHAccount
	byUsername map[string]*models.SSHAccount
	nextID     int
}

func newFakeAccountRepo() *fakeAccountRepo {
	return &fakeAccountRepo{byID: map[string]*models.SSHAccount{}, byUsername: map[string]*models.SSHAccount{}}
}

func (f *fakeAccountRepo) Insert(_ context.Context, a *models.SSHAccount) error {
	f.nextID++
	a.ID = "acct-" + time.Now().Format("150405.000000000") + "-" + itoa(f.nextID)
	a.CreatedAt = time.Now()
	a.UpdatedAt = a.CreatedAt
	f.byID[a.ID] = a
	f.byUsername[a.Username] = a
	return nil
}

func itoa(n int) string {
	digits := "0123456789"
	if n == 0 {
		return "0"
	}
	var b []byte
	for n > 0 {
		b = append([]byte{digits[n%10]}, b...)
		n /= 10
	}
	return string(b)
}

func (f *fakeAccountRepo) GetByUsername(_ context.Context, username string) (*models.SSHAccount, error) {
	a, ok := f.byUsername[username]
	if !ok {
		return nil, nil
	}
	return a, nil
}

func (f *fakeAccountRepo) GetByID(_ context.Context, id string) (*models.SSHAccount, error) {
	a, ok := f.byID[id]
	if !ok {
		return nil, nil
	}
	return a, nil
}

func (f *fakeAccountRepo) List(_ context.Context) ([]*models.SSHAccount, error) {
	var out []*models.SSHAccount
	for _, a := range f.byID {
		out = append(out, a)
	}
	return out, nil
}

func (f *fakeAccountRepo) UpdateUsername(_ context.Context, id, newUsername string) error {
	a := f.byID[id]
	delete(f.byUsername, a.Username)
	a.Username = newUsername
	f.byUsername[newUsername] = a
	return nil
}

func (f *fakeAccountRepo) UpdateExpiryAndLimits(_ context.Context, id string, expiresAt *time.Time, maxConn *int, note *string) error {
	a := f.byID[id]
	if expiresAt != nil {
		a.ExpiresAt = expiresAt
	}
	if maxConn != nil {
		a.MaxConnections = *maxConn
	}
	if note != nil {
		a.Note = *note
	}
	return nil
}

func (f *fakeAccountRepo) UpdateStatus(_ context.Context, id string, status models.AccountStatus) error {
	f.byID[id].Status = status
	return nil
}

func (f *fakeAccountRepo) SetLinuxUID(_ context.Context, id string, uid int) error {
	f.byID[id].LinuxUID = &uid
	return nil
}

func (f *fakeAccountRepo) SoftDelete(_ context.Context, id string) error {
	f.byID[id].Status = models.StatusDeleted
	return nil
}

func (f *fakeAccountRepo) ListExpiredButActive(_ context.Context, asOf time.Time) ([]*models.SSHAccount, error) {
	var out []*models.SSHAccount
	for _, a := range f.byID {
		if a.Status == models.StatusActive && a.ExpiresAt != nil && !a.ExpiresAt.After(asOf) {
			out = append(out, a)
		}
	}
	return out, nil
}

func (f *fakeAccountRepo) InsertAuditLog(_ context.Context, _ *string, _, _ string, _ map[string]any, _ string) error {
	return nil
}

// fakeSSHManager records what would have been done on the real OS, letting
// tests assert the service drives the OS layer correctly without ever
// shelling out.
type fakeSSHManager struct {
	created        map[string]string // username -> password
	deleted        map[string]bool
	disabled       map[string]bool
	enabled        map[string]bool
	renamed        map[string]string
	expirations    map[string]*time.Time
	failCreateUser string // if set, CreateAccount fails for this username
}

func newFakeSSHManager() *fakeSSHManager {
	return &fakeSSHManager{
		created: map[string]string{}, deleted: map[string]bool{},
		disabled: map[string]bool{}, enabled: map[string]bool{},
		renamed: map[string]string{}, expirations: map[string]*time.Time{},
	}
}

func (f *fakeSSHManager) CreateAccount(_ context.Context, username, password string) error {
	if username == f.failCreateUser {
		return errors.New("simulated OS failure")
	}
	f.created[username] = password
	return nil
}
func (f *fakeSSHManager) SetPassword(_ context.Context, username, newPassword string) error {
	f.created[username] = newPassword
	return nil
}
func (f *fakeSSHManager) RenameAccount(_ context.Context, oldU, newU string) error {
	f.renamed[oldU] = newU
	return nil
}
func (f *fakeSSHManager) DisableAccount(_ context.Context, username string) error {
	f.disabled[username] = true
	return nil
}
func (f *fakeSSHManager) EnableAccount(_ context.Context, username string) error {
	f.enabled[username] = true
	return nil
}
func (f *fakeSSHManager) DeleteAccount(_ context.Context, username string) error {
	f.deleted[username] = true
	return nil
}
func (f *fakeSSHManager) SetExpiration(_ context.Context, username string, expiresAt *time.Time) error {
	f.expirations[username] = expiresAt
	return nil
}
func (f *fakeSSHManager) LookupUID(_ context.Context, username string) (int, error) {
	return 5000, nil
}

// --- tests ---------------------------------------------------------------

const strongPassword = "Str0ng!PassVal"

func TestCreateAccount_Success(t *testing.T) {
	repo := newFakeAccountRepo()
	os := newFakeSSHManager()
	svc := NewAccountService(repo, os)

	acc, err := svc.CreateAccount(context.Background(), "admin-1", models.CreateAccountRequest{
		Username: "newuser01",
		Password: strongPassword,
	})
	if err != nil {
		t.Fatalf("expected success, got %v", err)
	}
	if os.created["newuser01"] != strongPassword {
		t.Fatal("expected OS CreateAccount to be called with the given password")
	}
	if acc.LinuxUID == nil || *acc.LinuxUID != 5000 {
		t.Fatal("expected linux UID to be populated from LookupUID")
	}
	if acc.Status != models.StatusActive {
		t.Fatalf("expected active status, got %s", acc.Status)
	}
}

func TestCreateAccount_DuplicateUsernameRejected(t *testing.T) {
	repo := newFakeAccountRepo()
	os := newFakeSSHManager()
	svc := NewAccountService(repo, os)

	_, err := svc.CreateAccount(context.Background(), "admin-1", models.CreateAccountRequest{Username: "dupeuser", Password: strongPassword})
	if err != nil {
		t.Fatal(err)
	}
	_, err = svc.CreateAccount(context.Background(), "admin-1", models.CreateAccountRequest{Username: "dupeuser", Password: strongPassword})
	if !errors.Is(err, ErrUsernameTaken) {
		t.Fatalf("expected ErrUsernameTaken, got %v", err)
	}
}

func TestCreateAccount_WeakPasswordRejected(t *testing.T) {
	repo := newFakeAccountRepo()
	os := newFakeSSHManager()
	svc := NewAccountService(repo, os)

	_, err := svc.CreateAccount(context.Background(), "admin-1", models.CreateAccountRequest{Username: "weakpwuser", Password: "weak"})
	if err == nil {
		t.Fatal("expected weak password to be rejected")
	}
	if _, ok := os.created["weakpwuser"]; ok {
		t.Fatal("OS account must not be created when password validation fails")
	}
}

func TestCreateAccount_InvalidUsernameRejected(t *testing.T) {
	repo := newFakeAccountRepo()
	os := newFakeSSHManager()
	svc := NewAccountService(repo, os)

	_, err := svc.CreateAccount(context.Background(), "admin-1", models.CreateAccountRequest{Username: "root", Password: strongPassword})
	if err == nil {
		t.Fatal("expected reserved username 'root' to be rejected")
	}
}

func TestCreateAccount_RollsBackDBRowOnOSFailure(t *testing.T) {
	repo := newFakeAccountRepo()
	os := newFakeSSHManager()
	os.failCreateUser = "willfail01"
	svc := NewAccountService(repo, os)

	_, err := svc.CreateAccount(context.Background(), "admin-1", models.CreateAccountRequest{Username: "willfail01", Password: strongPassword})
	if err == nil {
		t.Fatal("expected error when OS account creation fails")
	}
	// The DB row must be marked deleted, not left as a live "active" ghost
	// account with no real Linux user behind it.
	got, _ := repo.GetByUsername(context.Background(), "willfail01")
	if got == nil {
		t.Fatal("expected the row to still exist (soft-deleted)")
	}
	if got.Status != models.StatusDeleted {
		t.Fatalf("expected status deleted after rollback, got %s", got.Status)
	}
}

func TestDisableThenEnable(t *testing.T) {
	repo := newFakeAccountRepo()
	os := newFakeSSHManager()
	svc := NewAccountService(repo, os)

	acc, err := svc.CreateAccount(context.Background(), "admin-1", models.CreateAccountRequest{Username: "togglemeuser", Password: strongPassword})
	if err != nil {
		t.Fatal(err)
	}

	if err := svc.Disable(context.Background(), "admin-1", acc.ID); err != nil {
		t.Fatal(err)
	}
	if !os.disabled["togglemeuser"] {
		t.Fatal("expected OS DisableAccount to have been called")
	}
	got, _ := repo.GetByID(context.Background(), acc.ID)
	if got.Status != models.StatusDisabled {
		t.Fatalf("expected disabled status, got %s", got.Status)
	}

	if err := svc.Enable(context.Background(), "admin-1", acc.ID); err != nil {
		t.Fatal(err)
	}
	if !os.enabled["togglemeuser"] {
		t.Fatal("expected OS EnableAccount to have been called")
	}
	got, _ = repo.GetByID(context.Background(), acc.ID)
	if got.Status != models.StatusActive {
		t.Fatalf("expected active status after enable, got %s", got.Status)
	}
}

func TestDelete_CallsOSAndSoftDeletes(t *testing.T) {
	repo := newFakeAccountRepo()
	os := newFakeSSHManager()
	svc := NewAccountService(repo, os)

	acc, err := svc.CreateAccount(context.Background(), "admin-1", models.CreateAccountRequest{Username: "deletemeuser", Password: strongPassword})
	if err != nil {
		t.Fatal(err)
	}
	if err := svc.Delete(context.Background(), "admin-1", acc.ID); err != nil {
		t.Fatal(err)
	}
	if !os.deleted["deletemeuser"] {
		t.Fatal("expected OS DeleteAccount to have been called")
	}
	got, _ := repo.GetByID(context.Background(), acc.ID)
	if got.Status != models.StatusDeleted {
		t.Fatalf("expected deleted status, got %s", got.Status)
	}
}

func TestReapExpired_DisablesPastDueAccounts(t *testing.T) {
	repo := newFakeAccountRepo()
	os := newFakeSSHManager()
	svc := NewAccountService(repo, os)

	past := time.Now().Add(-time.Hour)
	acc, err := svc.CreateAccount(context.Background(), "admin-1", models.CreateAccountRequest{
		Username: "expiredaccount", Password: strongPassword, ExpiresAt: &past,
	})
	if err != nil {
		t.Fatal(err)
	}

	n, err := svc.ReapExpired(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatalf("expected 1 account reaped, got %d", n)
	}
	got, _ := repo.GetByID(context.Background(), acc.ID)
	if got.Status != models.StatusExpired {
		t.Fatalf("expected expired status, got %s", got.Status)
	}
	if !os.disabled["expiredaccount"] {
		t.Fatal("expected OS-level disable to have been called on reap")
	}
}

func TestUpdate_RenameChecksCollision(t *testing.T) {
	repo := newFakeAccountRepo()
	os := newFakeSSHManager()
	svc := NewAccountService(repo, os)

	_, err := svc.CreateAccount(context.Background(), "admin-1", models.CreateAccountRequest{Username: "firstuser", Password: strongPassword})
	if err != nil {
		t.Fatal(err)
	}
	second, err := svc.CreateAccount(context.Background(), "admin-1", models.CreateAccountRequest{Username: "seconduser", Password: strongPassword})
	if err != nil {
		t.Fatal(err)
	}

	taken := "firstuser"
	_, err = svc.Update(context.Background(), "admin-1", second.ID, models.UpdateAccountRequest{NewUsername: &taken})
	if !errors.Is(err, ErrUsernameTaken) {
		t.Fatalf("expected ErrUsernameTaken, got %v", err)
	}
}
