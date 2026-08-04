package auth

import (
	"errors"
	"strings"
	"testing"

	"golang.org/x/crypto/bcrypt"
)

func TestPasswordHasherRoundTrip(t *testing.T) {
	hasher := newTestHasher(t)

	hash, err := hasher.Hash("correct horse battery")
	if err != nil {
		t.Fatalf("Hash() error = %v", err)
	}
	if hash == "correct horse battery" {
		t.Fatal("Hash() returned the password unchanged")
	}

	if err = hasher.Verify(hash, "correct horse battery"); err != nil {
		t.Errorf("Verify() with the right password error = %v", err)
	}
	if err = hasher.Verify(hash, "wrong horse battery"); !errors.Is(err, ErrInvalidCredentials) {
		t.Errorf("Verify() with a wrong password error = %v, want ErrInvalidCredentials", err)
	}
}

func TestPasswordHasherSaltsEachHash(t *testing.T) {
	hasher := newTestHasher(t)

	first, err := hasher.Hash("same password")
	if err != nil {
		t.Fatalf("Hash() error = %v", err)
	}

	second, err := hasher.Hash("same password")
	if err != nil {
		t.Fatalf("Hash() error = %v", err)
	}

	if first == second {
		t.Error("two hashes of the same password are identical, so bcrypt was not salted")
	}
}

func TestPasswordHasherRejectsBadCost(t *testing.T) {
	if _, err := NewPasswordHasher(bcrypt.MaxCost + 1); err == nil {
		t.Error("NewPasswordHasher() error = nil, want an out-of-range cost error")
	}
}

func TestVerifyReportsMalformedHash(t *testing.T) {
	hasher := newTestHasher(t)

	err := hasher.Verify("not-a-bcrypt-hash", "any password")
	if err == nil {
		t.Fatal("Verify() error = nil, want a hash parsing error")
	}
	// A broken stored hash is an operational fault, not a wrong password: the
	// service must log it rather than answer "invalid credentials".
	if errors.Is(err, ErrInvalidCredentials) {
		t.Error("Verify() reported a malformed hash as invalid credentials")
	}
}

func TestValidatePassword(t *testing.T) {
	testCases := []struct {
		name     string
		password string
		wantErr  bool
	}{
		{name: "minimum length", password: strings.Repeat("a", MinPasswordLength)},
		{name: "maximum length", password: strings.Repeat("a", MaxPasswordLength)},
		{name: "too short", password: strings.Repeat("a", MinPasswordLength-1), wantErr: true},
		{name: "too long", password: strings.Repeat("a", MaxPasswordLength+1), wantErr: true},
		{name: "empty", password: "", wantErr: true},
		// 37 Cyrillic runes are 74 bytes, past the bcrypt limit.
		{name: "cyrillic past the byte limit", password: strings.Repeat("п", 37), wantErr: true},
		{name: "cyrillic within the byte limit", password: strings.Repeat("п", 20)},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			err := ValidatePassword(testCase.password)

			if testCase.wantErr && err == nil {
				t.Fatal("ValidatePassword() error = nil, want a validation error")
			}
			if !testCase.wantErr && err != nil {
				t.Fatalf("ValidatePassword() error = %v", err)
			}

			if testCase.wantErr {
				validationError := ValidationError{}
				if !errors.As(err, &validationError) || validationError.Field != "password" {
					t.Errorf("error = %v, want a ValidationError on the password field", err)
				}
			}
		})
	}
}

// newTestHasher builds a hasher at the cheapest cost so the suite stays fast.
func newTestHasher(t *testing.T) *PasswordHasher {
	t.Helper()

	hasher, err := NewPasswordHasher(bcrypt.MinCost)
	if err != nil {
		t.Fatalf("NewPasswordHasher() error = %v", err)
	}

	return hasher
}
