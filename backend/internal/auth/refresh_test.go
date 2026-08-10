package auth

import (
	"crypto/sha256"
	"encoding/base64"
	"testing"
)

func TestNewRefreshTokenIsUnpredictable(t *testing.T) {
	const samples = 100

	seen := make(map[string]struct{}, samples)

	for range samples {
		token, digest, err := newRefreshToken()
		if err != nil {
			t.Fatalf("newRefreshToken() error = %v", err)
		}

		raw, err := base64.RawURLEncoding.DecodeString(token)
		if err != nil {
			t.Fatalf("token is not raw base64url: %v", err)
		}
		if len(raw) != refreshTokenBytes {
			t.Fatalf("token entropy = %d bytes, want %d", len(raw), refreshTokenBytes)
		}
		if len(digest) != sha256.Size {
			t.Fatalf("digest = %d bytes, want %d", len(digest), sha256.Size)
		}

		if _, duplicate := seen[token]; duplicate {
			t.Fatal("newRefreshToken() returned a duplicate token")
		}
		seen[token] = struct{}{}
	}
}

func TestNewRefreshTokenDigestMatchesToken(t *testing.T) {
	token, digest, err := newRefreshToken()
	if err != nil {
		t.Fatalf("newRefreshToken() error = %v", err)
	}

	// The digest is the lookup key, so it must be reproducible from the
	// plaintext the client sends back.
	if string(hashRefreshToken(token)) != string(digest) {
		t.Error("hashRefreshToken() does not reproduce the digest returned by newRefreshToken()")
	}
	if string(digest) == token {
		t.Error("the stored digest equals the plaintext token")
	}
}
