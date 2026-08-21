package handler

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/dgrco/quikslate/internal/domain"
)

// handleServiceError is the single place domain errors become HTTP responses,
// so a missing case is invisible at the call site: the error still reaches
// this function and still produces a response, just a 500 with the message
// swallowed into the log. These cases assert the literal status and body text
// rather than the ERR_* constants, since asserting a constant against itself
// passes no matter what either becomes.
func TestHandleServiceError(t *testing.T) {
	cases := []struct {
		name       string
		err        error
		wantStatus int
		wantBody   string
	}{
		{
			name:       "unusable password reset token",
			err:        domain.ErrInvalidPasswordResetToken,
			wantStatus: http.StatusBadRequest,
			wantBody:   "invalid password reset token",
		},
		{
			// Wrapped on the way up in places; errors.Is has to see through it.
			name:       "wrapped password reset token error",
			err:        fmt.Errorf("resetting password: %w", domain.ErrInvalidPasswordResetToken),
			wantStatus: http.StatusBadRequest,
			wantBody:   "invalid password reset token",
		},
		{
			// A *ValidationError carries a client-facing message, so the body
			// is the message itself rather than a fixed constant.
			name:       "reusing the current password",
			err:        domain.ErrSamePassword,
			wantStatus: http.StatusBadRequest,
			wantBody:   "new password must be different from your current password",
		},
		{
			name:       "unknown errors stay opaque",
			err:        errors.New("pq: connection refused on 10.0.0.4:5432"),
			wantStatus: http.StatusInternalServerError,
			wantBody:   "internal server error",
		},
	}

	// The 500 branch logs; keep it out of the test output.
	log.SetOutput(io.Discard)
	defer log.SetOutput(nil)

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			handleServiceError(rec, tc.err, "test")

			if rec.Code != tc.wantStatus {
				t.Errorf("status = %d, want %d", rec.Code, tc.wantStatus)
			}

			var body struct {
				Error string `json:"error"`
			}
			if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
				t.Fatalf("decoding response body: %v", err)
			}
			if body.Error != tc.wantBody {
				t.Errorf("error = %q, want %q", body.Error, tc.wantBody)
			}
		})
	}
}

// The opaque-500 case above only means something if the internals really are
// withheld. A handler that echoes the error string leaks connection strings,
// table names, and query fragments to unauthenticated callers.
func TestHandleServiceErrorHidesInternalDetail(t *testing.T) {
	log.SetOutput(io.Discard)
	defer log.SetOutput(nil)

	secret := "pq: password authentication failed for user \"quikslate_api\""
	rec := httptest.NewRecorder()
	handleServiceError(rec, errors.New(secret), "test")

	if body := rec.Body.String(); strings.Contains(body, "quikslate_api") || strings.Contains(body, "pq:") {
		t.Errorf("internal error detail reached the client: %s", body)
	}
}
