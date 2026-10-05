package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/dgrco/quikslate/internal/domain"
	"github.com/dgrco/quikslate/internal/infra/mail/mailtest"
	"github.com/dgrco/quikslate/internal/service"
)

// Tests for the two password reset endpoints. The service-level tests cover
// the logic; what is asserted here is the part that only exists at the HTTP
// boundary -- that the response an attacker can observe carries no more than
// it should.

const (
	handlerUserId = "user-1"
	handlerEmail  = "user@example.com"
)

// fakeAuthRepo is a domain.Repo test double covering only the methods the
// reset endpoints reach. Everything else nil-panics via the embedded
// interface, surfacing an unexpected dependency rather than a zero value.
type fakeAuthRepo struct {
	domain.Repo

	user    domain.User
	userErr error
	useErr  error
	token   domain.PasswordResetToken
}

func (f *fakeAuthRepo) WithTx(tx domain.Tx) domain.Repo { return f }

func (f *fakeAuthRepo) BeginTransaction(ctx context.Context) (domain.Tx, error) {
	return noopTx{}, nil
}

func (f *fakeAuthRepo) GetUserByEmail(ctx context.Context, email string) (domain.User, error) {
	return f.user, f.userErr
}

func (f *fakeAuthRepo) GetUserById(ctx context.Context, id string) (domain.User, error) {
	return f.user, nil
}

func (f *fakeAuthRepo) CreatePasswordResetToken(
	ctx context.Context, userId, tokenHash string, expiresAt time.Time,
) (domain.PasswordResetToken, error) {
	return domain.PasswordResetToken{UserId: userId, TokenHash: tokenHash}, nil
}

func (f *fakeAuthRepo) GetPasswordResetToken(ctx context.Context, tokenHash string) (domain.PasswordResetToken, error) {
	return f.token, nil
}

func (f *fakeAuthRepo) UsePasswordResetToken(ctx context.Context, tokenHash string) error {
	return f.useErr
}

func (f *fakeAuthRepo) RevokePasswordResetTokensByUserId(ctx context.Context, userId string) error {
	return nil
}

func (f *fakeAuthRepo) RevokeRefreshTokensByUserId(ctx context.Context, userId string) error {
	return nil
}

func (f *fakeAuthRepo) UpdateUser(ctx context.Context, id string, update domain.UserUpdate) error {
	return nil
}

type noopTx struct{}

func (noopTx) Commit(ctx context.Context) error   { return nil }
func (noopTx) Rollback(ctx context.Context) error { return nil }

func newAuthHandler(repo domain.Repo) *AuthHandler {
	fakeMailer := mailtest.NewMailer()
	authService := service.NewAuthService(repo, fakeMailer, "test-secret")
	return NewAuthHandler(authService, "test-secret", true)
}

func postJSON(t *testing.T, h http.HandlerFunc, body any) *httptest.ResponseRecorder {
	t.Helper()
	raw, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("marshalling request body: %v", err)
	}
	req := httptest.NewRequest(http.MethodPost, "/v1/auth/forgot-password", bytes.NewReader(raw))
	rec := httptest.NewRecorder()
	h(rec, req)
	return rec
}

// The whole point of the endpoint's quiet no-account path. If a registered
// and an unregistered address produce responses that differ in any observable
// way -- status, body, or the presence of a token -- the endpoint becomes a
// way to test whether someone has an account here, which for many services is
// itself the sensitive fact.
func TestForgotPasswordDoesNotRevealAccountExistence(t *testing.T) {
	known := &fakeAuthRepo{user: domain.User{Id: handlerUserId, Email: handlerEmail}}
	unknown := &fakeAuthRepo{userErr: domain.ErrNotFound}

	knownRec := postJSON(t, newAuthHandler(known).ForgotPassword, forgotPasswordRequest{Email: handlerEmail})
	unknownRec := postJSON(t, newAuthHandler(unknown).ForgotPassword, forgotPasswordRequest{Email: "nobody@example.com"})

	if knownRec.Code != http.StatusAccepted {
		t.Errorf("known address: status = %d, want %d", knownRec.Code, http.StatusAccepted)
	}
	if knownRec.Code != unknownRec.Code {
		t.Errorf("status differs by account existence: known %d, unknown %d", knownRec.Code, unknownRec.Code)
	}
	if knownRec.Body.String() != unknownRec.Body.String() {
		t.Errorf("body differs by account existence:\n known:   %s unknown: %s",
			knownRec.Body.String(), unknownRec.Body.String())
	}
}

// The reset token is the credential that authorizes the password change. It
// belongs in the email and nowhere else: returning it in the response body
// would let anyone reset any account by asking.
func TestForgotPasswordNeverReturnsTheToken(t *testing.T) {
	repo := &fakeAuthRepo{user: domain.User{Id: handlerUserId, Email: handlerEmail}}
	var captured string
	repoWithCapture := &capturingRepo{fakeAuthRepo: repo, hash: &captured}

	rec := postJSON(t, newAuthHandler(repoWithCapture).ForgotPassword, forgotPasswordRequest{Email: handlerEmail})

	if captured == "" {
		t.Fatal("no token was issued, so this test proves nothing")
	}
	// The body must contain neither the stored hash nor anything hex-ish of
	// token length that could be the plaintext.
	body := rec.Body.String()
	if strings.Contains(body, captured) {
		t.Errorf("the token hash appeared in the response body: %s", body)
	}
	var resp map[string]any
	if err := json.Unmarshal([]byte(body), &resp); err != nil {
		t.Fatalf("decoding response: %v", err)
	}
	for k, v := range resp {
		if s, ok := v.(string); ok && len(s) >= 64 {
			t.Errorf("field %q looks like a leaked token: %q", k, s)
		}
	}
}

// capturingRepo records the hash that was persisted, so the test above can
// assert the response does not contain it.
type capturingRepo struct {
	*fakeAuthRepo
	hash *string
}

// WithTx must return the outer type: the embedded fake's WithTx returns the
// inner value, which would route the tx-scoped write past the override below.
func (c *capturingRepo) WithTx(tx domain.Tx) domain.Repo { return c }

func (c *capturingRepo) CreatePasswordResetToken(
	ctx context.Context, userId, tokenHash string, expiresAt time.Time,
) (domain.PasswordResetToken, error) {
	*c.hash = tokenHash
	return domain.PasswordResetToken{UserId: userId, TokenHash: tokenHash}, nil
}

// Every refresh token for the user is revoked server-side during a reset, so
// the cookie this browser still holds is already dead. Leaving it set means
// the failure surfaces later as a confusing 401 on the next refresh instead
// of a clean logged-out state now.
func TestResetPasswordClearsTheRefreshCookie(t *testing.T) {
	repo := &fakeAuthRepo{
		user:  domain.User{Id: handlerUserId, Email: handlerEmail, Password: "$2a$10$notarealhashatall"},
		token: domain.PasswordResetToken{Id: "prt-1", UserId: handlerUserId},
	}
	h := newAuthHandler(repo)

	raw, _ := json.Marshal(resetPasswordRequest{Token: "the-token", NewPassword: "new-password-1"})
	req := httptest.NewRequest(http.MethodPost, "/v1/auth/reset-password", bytes.NewReader(raw))
	rec := httptest.NewRecorder()
	h.ResetPassword(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body %s", rec.Code, rec.Body.String())
	}

	var cleared bool
	for _, c := range rec.Result().Cookies() {
		if c.Name == "refresh_token" {
			cleared = c.MaxAge < 0
			if c.Path != "/v1/auth" {
				t.Errorf("clearing cookie has Path %q, want /v1/auth; a mismatched path leaves the original in place", c.Path)
			}
		}
	}
	if !cleared {
		t.Error("the refresh_token cookie was not expired")
	}
}

// An unusable token has to reach the client as a 400 about the token, not the
// 404 the repository's bare ErrNotFound would produce on a route that exists.
func TestResetPasswordRejectsUnusableToken(t *testing.T) {
	repo := &fakeAuthRepo{
		user:   domain.User{Id: handlerUserId, Email: handlerEmail, Password: "$2a$10$notarealhashatall"},
		token:  domain.PasswordResetToken{Id: "prt-1", UserId: handlerUserId},
		useErr: domain.ErrNotFound, // already used or expired
	}
	h := newAuthHandler(repo)

	raw, _ := json.Marshal(resetPasswordRequest{Token: "spent-token", NewPassword: "new-password-1"})
	req := httptest.NewRequest(http.MethodPost, "/v1/auth/reset-password", bytes.NewReader(raw))
	rec := httptest.NewRecorder()
	h.ResetPassword(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400; body %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "invalid password reset token") {
		t.Errorf("body = %s, want the invalid-token message", rec.Body.String())
	}
}
