package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
)

// refreshTokenBytes is the entropy of a refresh token before encoding. 256 bits
// makes guessing infeasible, which matters because the token is a bearer
// credential with a much longer life than an access token.
const refreshTokenBytes = 32

// newRefreshToken returns a fresh opaque token together with the digest stored
// in the database. The plaintext is handed to the client exactly once; a dump
// of refresh_sessions therefore cannot be replayed.
func newRefreshToken() (token string, digest []byte, err error) {
	raw := make([]byte, refreshTokenBytes)
	if _, err = rand.Read(raw); err != nil {
		return "", nil, fmt.Errorf("generate refresh token: %w", err)
	}

	token = base64.RawURLEncoding.EncodeToString(raw)

	return token, hashRefreshToken(token), nil
}

// hashRefreshToken derives the lookup digest of a refresh token.
//
// SHA-256 rather than bcrypt is deliberate: the token is 256 random bits, so it
// is not guessable by dictionary attack, and an unsalted digest is what lets the
// database find the session by a unique index in a single query.
func hashRefreshToken(token string) []byte {
	digest := sha256.Sum256([]byte(token))

	return digest[:]
}
