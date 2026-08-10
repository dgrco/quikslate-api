package auth

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// Tests for access token signing and verification. Most of these are about
// what ValidateAccessToken must *reject*: a token it wrongly accepts is a
// full authentication bypass, since the userId it returns is what every
// downstream authz lookup keys off.

const testSecret = "test-secret-not-used-anywhere-real"

// signRaw builds a token from claims the exported API cannot produce, so the
// rejection paths can be exercised: an expired expiry, a foreign signing
// method, a different secret.
func signRaw(t *testing.T, method jwt.SigningMethod, claims jwt.Claims, key any) string {
	t.Helper()
	signed, err := jwt.NewWithClaims(method, claims).SignedString(key)
	if err != nil {
		t.Fatalf("failed to sign test token: %v", err)
	}
	return signed
}

// claimsFor builds valid claims for userId expiring expiry from now. A
// negative expiry produces an already-expired token.
func claimsFor(userId string, expiry time.Duration) AccessTokenClaims {
	return AccessTokenClaims{
		UserId: userId,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(expiry)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}
}

func TestGenerateAndValidateAccessToken(t *testing.T) {
	token, err := GenerateAccessToken("user-123", testSecret)
	if err != nil {
		t.Fatalf("GenerateAccessToken() error = %v", err)
	}

	claims, err := ValidateAccessToken(token, testSecret)
	if err != nil {
		t.Fatalf("ValidateAccessToken() error = %v, want nil", err)
	}
	if claims.UserId != "user-123" {
		t.Errorf("UserId = %q, want %q", claims.UserId, "user-123")
	}
}

// The token must carry userId and nothing authz-related. Roles and admin
// flags were deliberately removed from the claims so that a revoked role
// takes effect immediately instead of lingering until the token expires.
// If someone reintroduces a role claim, this fails and points at why that
// is not allowed.
func TestAccessTokenCarriesNoAuthorizationClaims(t *testing.T) {
	token, err := GenerateAccessToken("user-123", testSecret)
	if err != nil {
		t.Fatalf("GenerateAccessToken() error = %v", err)
	}

	// Parse into a bare map rather than AccessTokenClaims, which would
	// silently drop any field the struct does not name.
	var raw jwt.MapClaims
	if _, err := jwt.ParseWithClaims(token, &raw, func(*jwt.Token) (any, error) {
		return []byte(testSecret), nil
	}); err != nil {
		t.Fatalf("failed to parse token: %v", err)
	}

	allowed := map[string]bool{"user_id": true, "exp": true, "iat": true}
	for name := range raw {
		if !allowed[name] {
			t.Errorf("unexpected claim %q in access token: authorization state must be looked up per request, not carried in the token", name)
		}
	}
}

func TestValidateAccessTokenRejects(t *testing.T) {
	cases := []struct {
		name  string
		token func(t *testing.T) string
	}{
		{
			// The signature is what makes a token proof of anything. A token
			// signed with any other key is an unauthenticated stranger's
			// guess at the payload.
			name: "signed with a different secret",
			token: func(t *testing.T) string {
				return signRaw(t, jwt.SigningMethodHS256, claimsFor("user-123", time.Hour), []byte("attacker-secret"))
			},
		},
		{
			// Algorithm confusion. "alg":"none" declares the token
			// unsigned, so a parser that trusts the header would accept
			// arbitrary claims from anyone.
			//
			// Two independent things reject this today: the
			// SigningMethodHMAC check in ValidateAccessToken, and golang-jwt
			// itself, which refuses the none method unless the keyfunc hands
			// back jwt.UnsafeAllowNoneSignatureType. Deleting our check does
			// not make this test fail, so treat it as pinning the behavior
			// rather than that one line. It earns its place by catching a
			// library swap or a keyfunc rewrite that drops the outer layer.
			name: "unsigned token claiming alg none",
			token: func(t *testing.T) string {
				return signRaw(t, jwt.SigningMethodNone, claimsFor("user-123", time.Hour), jwt.UnsafeAllowNoneSignatureType)
			},
		},
		{
			// Expiry is the only thing bounding how long a stolen access
			// token stays useful, given they are never revoked directly.
			name: "expired token",
			token: func(t *testing.T) string {
				return signRaw(t, jwt.SigningMethodHS256, claimsFor("user-123", -time.Minute), []byte(testSecret))
			},
		},
		{
			name: "empty string",
			token: func(t *testing.T) string {
				return ""
			},
		},
		{
			name: "not a JWT at all",
			token: func(t *testing.T) string {
				return "definitely.not.a.jwt"
			},
		},
		{
			// The impersonation attempt this whole scheme exists to stop:
			// take a legitimately issued token, swap the claims for someone
			// else's userId, and keep the original signature. The signature
			// covers the header and payload verbatim, so it no longer
			// matches. Were it accepted, any user could become any other by
			// editing one field.
			name: "payload edited to impersonate another user",
			token: func(t *testing.T) string {
				valid, err := GenerateAccessToken("user-123", testSecret)
				if err != nil {
					t.Fatalf("GenerateAccessToken() error = %v", err)
				}
				parts := strings.Split(valid, ".")
				if len(parts) != 3 {
					t.Fatalf("expected a 3 segment JWT, got %d segments", len(parts))
				}
				forged, err := json.Marshal(claimsFor("victim-456", time.Hour))
				if err != nil {
					t.Fatalf("failed to marshal forged claims: %v", err)
				}
				parts[1] = base64.RawURLEncoding.EncodeToString(forged)
				return strings.Join(parts, ".")
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := ValidateAccessToken(tc.token(t), testSecret); err == nil {
				t.Error("ValidateAccessToken() error = nil, want a rejection")
			}
		})
	}
}

// Expiry rejection should be distinguishable from a bad signature: the
// handler layer turns the first into a 401 the client answers by refreshing,
// which is the whole basis of the refresh-and-retry flow in the frontend.
func TestValidateAccessTokenExpiredIsIdentifiable(t *testing.T) {
	expired := signRaw(t, jwt.SigningMethodHS256, claimsFor("user-123", -time.Minute), []byte(testSecret))

	_, err := ValidateAccessToken(expired, testSecret)
	if !errors.Is(err, jwt.ErrTokenExpired) {
		t.Errorf("ValidateAccessToken() error = %v, want it to wrap jwt.ErrTokenExpired", err)
	}
}

// A token minted right now must not already be expired, and must not outlive
// the 15 minute window the refresh flow is designed around.
//
// The window is written out literally instead of compared against
// accessTokenExpiry. Checking the constant against itself would hold no
// matter what it was changed to, so it would not notice someone widening the
// window to a day, which is exactly the change worth noticing: expiry is the
// only thing bounding how long a stolen access token keeps working. Changing
// the constant should require deliberately changing this number too.
func TestAccessTokenExpiry(t *testing.T) {
	const wantExpiry = 15 * time.Minute

	token, err := GenerateAccessToken("user-123", testSecret)
	if err != nil {
		t.Fatalf("GenerateAccessToken() error = %v", err)
	}

	claims, err := ValidateAccessToken(token, testSecret)
	if err != nil {
		t.Fatalf("ValidateAccessToken() error = %v", err)
	}

	// Lower bound allows for the time spent between signing and here, which
	// is microseconds in practice. Anything approaching the tolerance means
	// the expiry shrank, not that the machine was slow.
	expiresIn := time.Until(claims.ExpiresAt.Time)
	if expiresIn > wantExpiry || expiresIn < wantExpiry-time.Minute {
		t.Errorf("token expires in %v, want approximately %v", expiresIn, wantExpiry)
	}
}
