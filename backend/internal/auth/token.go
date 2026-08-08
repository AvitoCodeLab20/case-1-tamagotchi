package auth

import (
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

// MinJWTSecretLength is the shortest signing key accepted for HS256. A key
// shorter than the hash output weakens the signature, so the service refuses to
// start with one.
const MinJWTSecretLength = 32

// accessTokenAudience labels tokens minted for the game API. Verifying it stops
// a token issued for another audience by the same key from being accepted here.
const accessTokenAudience = "tamagotchi-api" //nolint:gosec // an audience label, not a credential

// Claims is the payload of an access token. Only the user id travels in the
// token; everything else is read from the database when a handler needs it, so
// a stale token cannot carry stale authorisation data.
type Claims struct {
	jwt.RegisteredClaims
}

// UserID returns the subject of the token as a UUID.
func (claims Claims) UserID() (uuid.UUID, error) {
	userID, err := uuid.Parse(claims.Subject)
	if err != nil {
		return uuid.Nil, fmt.Errorf("parse token subject: %w", err)
	}

	return userID, nil
}

// TokenIssuer signs and verifies HS256 access tokens.
type TokenIssuer struct {
	secret []byte
	issuer string
	ttl    time.Duration
	parser *jwt.Parser
	now    func() time.Time
}

// NewTokenIssuer builds an issuer. The parser is configured once so that every
// verification enforces the same algorithm, issuer, audience, and expiry.
func NewTokenIssuer(secret []byte, issuer string, ttl time.Duration) (*TokenIssuer, error) {
	if len(secret) < MinJWTSecretLength {
		return nil, fmt.Errorf("JWT secret must be at least %d bytes, got %d", MinJWTSecretLength, len(secret))
	}
	if issuer == "" {
		return nil, errors.New("JWT issuer must not be empty")
	}
	if ttl <= 0 {
		return nil, errors.New("access token TTL must be positive")
	}

	parser := jwt.NewParser(
		// Pinning the algorithm is what stops the "alg: none" and
		// "HS256 signed with the RS256 public key" confusion attacks.
		jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}),
		jwt.WithIssuer(issuer),
		jwt.WithAudience(accessTokenAudience),
		jwt.WithExpirationRequired(),
	)

	return &TokenIssuer{
		secret: secret,
		issuer: issuer,
		ttl:    ttl,
		parser: parser,
		now:    time.Now,
	}, nil
}

// TTL reports the lifetime of the tokens this issuer mints.
func (issuer *TokenIssuer) TTL() time.Duration {
	return issuer.ttl
}

// Issue mints a signed access token for the user and reports when it expires.
func (issuer *TokenIssuer) Issue(userID uuid.UUID) (string, time.Time, error) {
	issuedAt := issuer.now().UTC()
	expiresAt := issuedAt.Add(issuer.ttl)

	claims := Claims{
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   userID.String(),
			Issuer:    issuer.issuer,
			Audience:  jwt.ClaimStrings{accessTokenAudience},
			ID:        uuid.NewString(),
			IssuedAt:  jwt.NewNumericDate(issuedAt),
			NotBefore: jwt.NewNumericDate(issuedAt),
			ExpiresAt: jwt.NewNumericDate(expiresAt),
		},
	}

	signed, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(issuer.secret)
	if err != nil {
		return "", time.Time{}, fmt.Errorf("sign access token: %w", err)
	}

	return signed, expiresAt, nil
}

// Parse verifies the signature and the registered claims of an access token.
// Every failure collapses into ErrInvalidAccessToken so the API cannot leak why
// a token was rejected.
func (issuer *TokenIssuer) Parse(token string) (Claims, error) {
	claims := Claims{}

	parsed, err := issuer.parser.ParseWithClaims(token, &claims, func(*jwt.Token) (any, error) {
		return issuer.secret, nil
	})
	if err != nil {
		return Claims{}, fmt.Errorf("%w: %w", ErrInvalidAccessToken, err)
	}
	if !parsed.Valid {
		return Claims{}, ErrInvalidAccessToken
	}
	if _, err = claims.UserID(); err != nil {
		return Claims{}, ErrInvalidAccessToken
	}

	return claims, nil
}
