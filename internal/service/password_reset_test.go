package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/dgrco/quikslate/internal/auth"
	"github.com/dgrco/quikslate/internal/domain"
	"github.com/dgrco/quikslate/internal/infra/mail/mailtest"
)

// Tests for the password reset flow. The failures worth catching here are all
// silent ones: a reset endpoint that answers differently for a registered and
// an unregistered address is an account enumeration oracle while still
// returning 200, a token stored unhashed reads as a working feature until the
// table leaks, and a password written to the wrong user id fails only for the
// person locked out of their account.
//
// The fake records the calls it received in order rather than only answering
// them, because most of these bugs are about which call happened, against
// which id, with what value -- none of which the returned error reveals.

const (
	resetUserId    = "user-1"
	resetTokenId   = "prt-1" // deliberately != resetUserId, see TestUsePasswordResetTokenSuccess
	resetUserEmail = "user@example.com"
	resetOldPass   = "old-password-1"
	resetNewPass   = "new-password-1"
)

// resetCall is one recorded repo call. Only the fields these tests assert on
// are captured; op is the method name.
type resetCall struct {
	op     string
	userId string
	hash   string
	email  string
}

// fakeResetRepo is a domain.Repo test double. domain.Repo embeds every
// sub-repo, so the nil embedded interface supplies the rest of the method
// set; only the methods the reset flow reaches are overridden. Any other call
// nil-panics, which is the intent: it surfaces an unexpected dependency
// instead of quietly returning a zero value.
type fakeResetRepo struct {
	domain.Repo

	user        domain.User
	userErr     error // GetUserByEmail
	userByIdErr error // GetUserById
	token       domain.PasswordResetToken
	tokenErr    error // GetPasswordResetToken
	useErr      error // UsePasswordResetToken

	// Recorded calls, in order.
	calls []resetCall

	// Recorded writes.
	storedPassword *string
	committed      bool
}

// WithTx returns the same fake so calls made through the tx-scoped repo land
// in one ordered log alongside the rest.
func (f *fakeResetRepo) WithTx(tx domain.Tx) domain.Repo { return f }

func (f *fakeResetRepo) BeginTransaction(ctx context.Context) (domain.Tx, error) {
	f.calls = append(f.calls, resetCall{op: "BeginTransaction"})
	return &fakeTx{repo: f}, nil
}

func (f *fakeResetRepo) GetUserByEmail(ctx context.Context, email string) (domain.User, error) {
	f.calls = append(f.calls, resetCall{op: "GetUserByEmail", email: email})
	return f.user, f.userErr
}

func (f *fakeResetRepo) GetUserById(ctx context.Context, id string) (domain.User, error) {
	f.calls = append(f.calls, resetCall{op: "GetUserById", userId: id})
	return f.user, f.userByIdErr
}

func (f *fakeResetRepo) CreatePasswordResetToken(
	ctx context.Context, userId, tokenHash string, expiresAt time.Time,
) (domain.PasswordResetToken, error) {
	f.calls = append(f.calls, resetCall{op: "CreatePasswordResetToken", userId: userId, hash: tokenHash})
	return domain.PasswordResetToken{Id: resetTokenId, UserId: userId, TokenHash: tokenHash}, nil
}

func (f *fakeResetRepo) GetPasswordResetToken(ctx context.Context, tokenHash string) (domain.PasswordResetToken, error) {
	f.calls = append(f.calls, resetCall{op: "GetPasswordResetToken", hash: tokenHash})
	return f.token, f.tokenErr
}

func (f *fakeResetRepo) UsePasswordResetToken(ctx context.Context, tokenHash string) error {
	f.calls = append(f.calls, resetCall{op: "UsePasswordResetToken", hash: tokenHash})
	return f.useErr
}

func (f *fakeResetRepo) RevokePasswordResetTokensByUserId(ctx context.Context, userId string) error {
	f.calls = append(f.calls, resetCall{op: "RevokePasswordResetTokensByUserId", userId: userId})
	return nil
}

func (f *fakeResetRepo) RevokeRefreshTokensByUserId(ctx context.Context, userId string) error {
	f.calls = append(f.calls, resetCall{op: "RevokeRefreshTokensByUserId", userId: userId})
	return nil
}

func (f *fakeResetRepo) UpdateUser(ctx context.Context, id string, update domain.UserUpdate) error {
	f.calls = append(f.calls, resetCall{op: "UpdateUser", userId: id})
	f.storedPassword = update.Password
	return nil
}

// fakeTx records only Commit. Rollback is deliberately not asserted on: the
// services defer it unconditionally, so it runs on the success path too and
// proves nothing.
type fakeTx struct{ repo *fakeResetRepo }

func (t *fakeTx) Commit(ctx context.Context) error   { t.repo.committed = true; return nil }
func (t *fakeTx) Rollback(ctx context.Context) error { return nil }

// find returns the first recorded call with the given op, or nil.
func (f *fakeResetRepo) find(op string) *resetCall {
	for i := range f.calls {
		if f.calls[i].op == op {
			return &f.calls[i]
		}
	}
	return nil
}

func (f *fakeResetRepo) called(op string) bool { return f.find(op) != nil }

// indexOf returns the position of the first call with the given op, or -1.
func (f *fakeResetRepo) indexOf(op string) int {
	return slices.IndexFunc(f.calls, func(c resetCall) bool { return c.op == op })
}

// wrote reports whether any mutating repo method ran. The enumeration tests
// assert this stays false: quietly reporting success while having written
// rows for a nonexistent user is the failure mode worth catching.
func (f *fakeResetRepo) wrote() bool {
	for _, op := range []string{
		"CreatePasswordResetToken", "RevokePasswordResetTokensByUserId",
		"UsePasswordResetToken", "RevokeRefreshTokensByUserId", "UpdateUser",
	} {
		if f.called(op) {
			return true
		}
	}
	return false
}

// sha256Hex is an independent reimplementation of the service's token
// hashing. Calling the service's own hashToken here would move with the bug
// it is meant to catch.
func sha256Hex(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])
}

func newFakeResetRepo(t *testing.T) *fakeResetRepo {
	t.Helper()
	hash, err := auth.HashPassword(resetOldPass)
	if err != nil {
		t.Fatalf("hashing the fixture password: %v", err)
	}
	return &fakeResetRepo{
		user: domain.User{Id: resetUserId, Email: resetUserEmail, Password: hash},
		token: domain.PasswordResetToken{
			Id:        resetTokenId,
			UserId:    resetUserId,
			ExpiresAt: time.Now().Add(15 * time.Minute),
		},
	}
}

// An address with no account must be indistinguishable from one with an
// account. Returning the repo's ErrNotFound turns the endpoint into an
// enumeration oracle, and merely swallowing the error without returning
// early is worse: execution falls through with a zero-valued user, so the
// writes below run with an empty string where a UUID belongs.
func TestSendPasswordResetTokenUnknownEmailIsSilent(t *testing.T) {
	repo := newFakeResetRepo(t)
	repo.userErr = domain.ErrNotFound
	mailer := mailtest.NewMailer()
	svc := NewAuthService(repo, mailer, "test-secret")

	err := svc.SendPasswordResetToken(context.Background(), resetUserEmail)
	if err != nil {
		t.Fatalf("unknown email should not error, got %v", err)
	}
	if repo.wrote() {
		t.Errorf("unknown email must not write; calls were %v", repo.calls)
	}
	if mailer.DidSendMail() {
		t.Error("the mailer sent to an unknown email")
	}
}

// The quiet path above is only for "no such account". A failing database
// still has to surface, or every reset silently no-ops during an outage.
func TestSendPasswordResetTokenPropagatesRealErrors(t *testing.T) {
	repo := newFakeResetRepo(t)
	repo.userErr = errors.New("connection refused")
	mailer := mailtest.NewMailer()
	svc := NewAuthService(repo, mailer, "test-secret")

	err := svc.SendPasswordResetToken(context.Background(), resetUserEmail)
	if err == nil {
		t.Error("a database failure must not be reported as success")
	}
	if mailer.DidSendMail() {
		t.Error("mail was sent despite a database failure")
	}
}

// The token is a bearer credential: the plaintext goes in the email and only
// its hash may reach the table, so a database leak can't be replayed. Storing
// the raw token behaves identically in every functional test.
func TestSendPasswordResetTokenStoresOnlyTheHash(t *testing.T) {
	repo := newFakeResetRepo(t)
	mailer := mailtest.NewMailer()
	svc := NewAuthService(repo, mailer, "test-secret")

	err := svc.SendPasswordResetToken(context.Background(), resetUserEmail)
	if err != nil {
		t.Fatalf("SendPasswordResetToken: %v", err)
	}
	mailCall := mailer.Find("SendForgotPassword")
	if mailCall == nil {
		t.Fatal("SendForgotPassword not called")
	}
	if mailCall.Token == "" {
		t.Fatal("a known address must yield a token")
	}

	resetCall := repo.find("CreatePasswordResetToken")
	if resetCall == nil {
		t.Fatal("no token was persisted")
	}
	if resetCall.hash == mailCall.Token {
		t.Fatal("the plaintext token was stored; only its hash may be persisted")
	}
	if want := sha256Hex(mailCall.Token); resetCall.hash != want {
		t.Errorf("stored hash = %q, want sha256 of the emailed token %q", resetCall.hash, want)
	}
	if resetCall.userId != resetUserId {
		t.Errorf("token stored against user %q, want %q", resetCall.userId, resetUserId)
	}
}

// Only one token may be outstanding per user, and the order is what enforces
// it. Creating before revoking deletes the token that was just issued, so
// every reset link in the mail is dead on arrival -- while this function
// stillreturns it with no error.
func TestSendPasswordResetTokenRevokesBeforeIssuing(t *testing.T) {
	repo := newFakeResetRepo(t)
	mailer := mailtest.NewMailer()
	svc := NewAuthService(repo, mailer, "test-secret")

	if err := svc.SendPasswordResetToken(context.Background(), resetUserEmail); err != nil {
		t.Fatalf("SendPasswordResetToken: %v", err)
	}

	revoked := repo.indexOf("RevokePasswordResetTokensByUserId")
	created := repo.indexOf("CreatePasswordResetToken")
	if revoked == -1 || created == -1 {
		t.Fatalf("expected both a revoke and a create, got %v", repo.calls)
	}
	if revoked > created {
		t.Errorf("revoke ran after create, which deletes the new token; calls were %v", repo.calls)
	}
}

// Registration lowercases before storing and the lookup matches exactly, so
// without normalization here a user who capitalizes their address falls into
// the deliberately-silent no-account path and simply never receives mail.
func TestSendPasswordResetTokenNormalizesEmail(t *testing.T) {
	repo := newFakeResetRepo(t)
	mailer := mailtest.NewMailer()
	svc := NewAuthService(repo, mailer, "test-secret")

	mixed := "User@Example.COM"
	if err := svc.SendPasswordResetToken(context.Background(), mixed); err != nil {
		t.Fatalf("SendPasswordResetToken: %v", err)
	}

	userLookup := repo.find("GetUserByEmail")
	if userLookup == nil {
		t.Fatal("no lookup was performed")
	}
	if userLookup.email != strings.ToLower(mixed) {
		t.Errorf("looked up %q, want the normalized %q", userLookup.email, strings.ToLower(mixed))
	}
}

// The reset email must be sent to the stored user's email, never to a requested email.
// If the requested email maps to a real account via loose matching logic, and it is
// sent to the requested email, then an attacker could get the reset link if they own
// that email.
func TestSendPasswordResetTokenEmailsStoredUserEmail(t *testing.T) {
	repo := newFakeResetRepo(t)
	mailer := mailtest.NewMailer()
	svc := NewAuthService(repo, mailer, "test-secret")

	// repo.user is returned from GetUserByEmail regardless of the email, so this acts like a match
	email := "email.that.does.not.equal.yet.matches.stored.user@example.com"
	if err := svc.SendPasswordResetToken(context.Background(), email); err != nil {
		t.Fatalf("SendPasswordResetToken: %v", err)
	}

	mailerCall := mailer.Find("SendForgotPassword")
	if mailerCall == nil {
		t.Fatal("no mailer call was performed")
	}
	if mailerCall.ReceiverEmail != repo.user.Email {
		t.Errorf("mailer sent to %q, expected it to send to %q", mailerCall.ReceiverEmail, repo.user.Email)
	}
}

// A token that never existed and one already used or expired must be reported
// identically, and as an invalid *token* rather than the repo's bare
// ErrNotFound -- which the handler renders as a 404 on what is a valid route.
func TestUsePasswordResetTokenRejectsUnusableTokens(t *testing.T) {
	cases := []struct {
		name  string
		setup func(*fakeResetRepo)
	}{
		{
			name:  "no such token",
			setup: func(f *fakeResetRepo) { f.tokenErr = domain.ErrNotFound },
		},
		{
			// The conditional UPDATE reports zero rows affected for a token
			// that is already used or past its expiry.
			name:  "already used or expired",
			setup: func(f *fakeResetRepo) { f.useErr = domain.ErrNotFound },
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			repo := newFakeResetRepo(t)
			tc.setup(repo)
			mailer := mailtest.NewMailer()
			svc := NewAuthService(repo, mailer, "test-secret")

			err := svc.UsePasswordResetToken(context.Background(), resetNewPass, "some-token")
			if !errors.Is(err, domain.ErrInvalidPasswordResetToken) {
				t.Errorf("got %v, want ErrInvalidPasswordResetToken", err)
			}
			if errors.Is(err, domain.ErrNotFound) {
				t.Error("the repo's ErrNotFound leaked out untranslated")
			}
			if repo.committed {
				t.Error("committed despite rejecting the token")
			}
			if repo.storedPassword != nil {
				t.Error("changed the password despite rejecting the token")
			}
		})
	}
}

// Reusing the current password must be refused, and refused without burning
// the token -- otherwise a mistyped attempt costs the user their reset link.
func TestUsePasswordResetTokenRejectsUnchangedPassword(t *testing.T) {
	repo := newFakeResetRepo(t)
	mailer := mailtest.NewMailer()
	svc := NewAuthService(repo, mailer, "test-secret")

	err := svc.UsePasswordResetToken(context.Background(), resetOldPass, "some-token")
	if !errors.Is(err, domain.ErrSamePassword) {
		t.Errorf("got %v, want ErrSamePassword", err)
	}
	if repo.committed {
		t.Error("committed despite rejecting the password")
	}
	if repo.called("UsePasswordResetToken") {
		t.Error("consumed the token on a rejected attempt")
	}
}

// A password below the length floor must be rejected before any database work
// begins; the reset path is otherwise a way around the bound that
// registration enforces.
func TestUsePasswordResetTokenValidatesPasswordFirst(t *testing.T) {
	repo := newFakeResetRepo(t)
	mailer := mailtest.NewMailer()
	svc := NewAuthService(repo, mailer, "test-secret")

	if err := svc.UsePasswordResetToken(context.Background(), "short", "some-token"); err == nil {
		t.Fatal("a too-short password must be rejected")
	}
	if len(repo.calls) != 0 {
		t.Errorf("touched the database before validating; calls were %v", repo.calls)
	}
}

// The success path, asserted end to end. The user id is the sharp edge: the
// token row carries both its own id and the user's, and reading the wrong one
// points the password write at an id that does not identify a user.
func TestUsePasswordResetTokenSuccess(t *testing.T) {
	repo := newFakeResetRepo(t)
	mailer := mailtest.NewMailer()
	svc := NewAuthService(repo, mailer, "test-secret")

	presented := "the-emailed-token"
	if err := svc.UsePasswordResetToken(context.Background(), resetNewPass, presented); err != nil {
		t.Fatalf("UsePasswordResetToken: %v", err)
	}

	// Looked up by hash, never by the plaintext that arrived in the request.
	got := repo.find("GetPasswordResetToken")
	if got == nil {
		t.Fatal("the token was never looked up")
	}
	if want := sha256Hex(presented); got.hash != want {
		t.Errorf("looked up hash %q, want %q", got.hash, want)
	}

	// Every user-scoped call must use the token's UserId, not its own Id.
	for _, op := range []string{"GetUserById", "UpdateUser", "RevokeRefreshTokensByUserId"} {
		call := repo.find(op)
		if call == nil {
			t.Errorf("%s was never called", op)
			continue
		}
		if call.userId != resetUserId {
			t.Errorf("%s used id %q, want the token's UserId %q", op, call.userId, resetUserId)
		}
	}

	if !repo.called("UsePasswordResetToken") {
		t.Error("the token was never consumed")
	}

	// The new password must be stored as a verifiable hash, never plaintext.
	if repo.storedPassword == nil {
		t.Fatal("no password was written")
	}
	stored := *repo.storedPassword
	if stored == resetNewPass {
		t.Fatal("the plaintext password was stored")
	}
	if !auth.CheckPassword(resetNewPass, stored) {
		t.Error("the stored hash does not verify against the new password")
	}

	if !repo.committed {
		t.Error("the transaction was never committed")
	}
}
