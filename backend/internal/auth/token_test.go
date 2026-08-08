package auth

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

// testSecret is a fixture signing key, long enough to pass the length check.
const testSecret = "test-signing-key-that-is-long-enough"

func TestTokenIssuerRoundTrip(t *testing.T) {
	issuer := newTestIssuer(t, time.Hour)
	userID := uuid.New()

	token, expiresAt, err := issuer.Issue(userID)
	if err != nil {
		t.Fatalf("Issue() error = %v", err)
	}
	if !expiresAt.After(time.Now()) {
		t.Errorf("expiresAt = %s, want a future timestamp", expiresAt)
	}

	claims, err := issuer.Parse(token)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	parsedUserID, err := claims.UserID()
	if err != nil {
		t.Fatalf("UserID() error = %v", err)
	}
	if parsedUserID != userID {
		t.Errorf("UserID() = %s, want %s", parsedUserID, userID)
	}
}

func TestTokenIssuerIssuesUniqueTokenIDs(t *testing.T) {
	issuer := newTestIssuer(t, time.Hour)
	userID := uuid.New()

	first, _, err := issuer.Issue(userID)
	if err != nil {
		t.Fatalf("Issue() error = %v", err)
	}

	second, _, err := issuer.Issue(userID)
	if err != nil {
		t.Fatalf("Issue() error = %v", err)
	}

	firstClaims, err := issuer.Parse(first)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	secondClaims, err := issuer.Parse(second)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	// A distinct jti is what makes a future deny-list able to revoke one token
	// rather than every token of the user.
	if firstClaims.ID == secondClaims.ID {
		t.Error("two tokens share the same jti")
	}
}

func TestTokenIssuerRejectsExpiredToken(t *testing.T) {
	issuer := newTestIssuer(t, time.Minute)
	// Issue the token as if it had been minted two hours ago.
	issuer.now = func() time.Time { return time.Now().Add(-2 * time.Hour) }

	token, _, err := issuer.Issue(uuid.New())
	if err != nil {
		t.Fatalf("Issue() error = %v", err)
	}

	issuer.now = time.Now

	if _, err = issuer.Parse(token); !errors.Is(err, ErrInvalidAccessToken) {
		t.Errorf("Parse() error = %v, want ErrInvalidAccessToken", err)
	}
}

func TestTokenIssuerRejectsForeignSignature(t *testing.T) {
	issuer := newTestIssuer(t, time.Hour)

	other, err := NewTokenIssuer([]byte("a-completely-different-signing-key-32"), "avito-tamagotchi", time.Hour)
	if err != nil {
		t.Fatalf("NewTokenIssuer() error = %v", err)
	}

	token, _, err := other.Issue(uuid.New())
	if err != nil {
		t.Fatalf("Issue() error = %v", err)
	}

	if _, err = issuer.Parse(token); !errors.Is(err, ErrInvalidAccessToken) {
		t.Errorf("Parse() error = %v, want ErrInvalidAccessToken", err)
	}
}

// TestTokenIssuerRejectsUnsignedToken covers the classic "alg: none" attack: a
// token whose header claims no signature is required must never be accepted.
func TestTokenIssuerRejectsUnsignedToken(t *testing.T) {
	issuer := newTestIssuer(t, time.Hour)

	claims := Claims{
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   uuid.NewString(),
			Issuer:    "avito-tamagotchi",
			Audience:  jwt.ClaimStrings{accessTokenAudience},
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
		},
	}

	unsigned, err := jwt.NewWithClaims(jwt.SigningMethodNone, claims).
		SignedString(jwt.UnsafeAllowNoneSignatureType)
	if err != nil {
		t.Fatalf("build unsigned token: %v", err)
	}

	if _, err = issuer.Parse(unsigned); !errors.Is(err, ErrInvalidAccessToken) {
		t.Errorf("Parse() error = %v, want ErrInvalidAccessToken", err)
	}
}

func TestTokenIssuerRejectsForeignIssuerAndAudience(t *testing.T) {
	issuer := newTestIssuer(t, time.Hour)

	testCases := []struct {
		name     string
		issuer   string
		audience string
	}{
		{name: "foreign issuer", issuer: "someone-else", audience: accessTokenAudience},
		{name: "foreign audience", issuer: "avito-tamagotchi", audience: "another-api"},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			claims := Claims{
				RegisteredClaims: jwt.RegisteredClaims{
					Subject:   uuid.NewString(),
					Issuer:    testCase.issuer,
					Audience:  jwt.ClaimStrings{testCase.audience},
					ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
				},
			}

			// Signed with the same key the service trusts, so only the issuer
			// and audience checks can reject it.
			token, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(testSecret))
			if err != nil {
				t.Fatalf("sign token: %v", err)
			}

			if _, err = issuer.Parse(token); !errors.Is(err, ErrInvalidAccessToken) {
				t.Errorf("Parse() error = %v, want ErrInvalidAccessToken", err)
			}
		})
	}
}

func TestTokenIssuerRejectsNonUUIDSubject(t *testing.T) {
	issuer := newTestIssuer(t, time.Hour)

	claims := Claims{
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   "not-a-uuid",
			Issuer:    "avito-tamagotchi",
			Audience:  jwt.ClaimStrings{accessTokenAudience},
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
		},
	}

	token, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(testSecret))
	if err != nil {
		t.Fatalf("sign token: %v", err)
	}

	if _, err = issuer.Parse(token); !errors.Is(err, ErrInvalidAccessToken) {
		t.Errorf("Parse() error = %v, want ErrInvalidAccessToken", err)
	}
}

func TestNewTokenIssuerValidatesArguments(t *testing.T) {
	testCases := []struct {
		name   string
		secret string
		issuer string
		ttl    time.Duration
	}{
		{name: "short secret", secret: strings.Repeat("a", MinJWTSecretLength-1), issuer: "avito", ttl: time.Hour},
		{name: "empty issuer", secret: testSecret, issuer: "", ttl: time.Hour},
		{name: "zero ttl", secret: testSecret, issuer: "avito", ttl: 0},
		{name: "negative ttl", secret: testSecret, issuer: "avito", ttl: -time.Hour},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			if _, err := NewTokenIssuer([]byte(testCase.secret), testCase.issuer, testCase.ttl); err == nil {
				t.Error("NewTokenIssuer() error = nil, want a validation error")
			}
		})
	}
}

func newTestIssuer(t *testing.T, ttl time.Duration) *TokenIssuer {
	t.Helper()

	issuer, err := NewTokenIssuer([]byte(testSecret), "avito-tamagotchi", ttl)
	if err != nil {
		t.Fatalf("NewTokenIssuer() error = %v", err)
	}

	return issuer
}
