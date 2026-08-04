package auth

import (
	"errors"
	"fmt"

	"golang.org/x/crypto/bcrypt"
)

const (
	// MinPasswordLength is the shortest password the service accepts.
	MinPasswordLength = 8

	// MaxPasswordLength matches the bcrypt input limit. Longer passwords are
	// rejected instead of being silently truncated to 72 bytes.
	MaxPasswordLength = 72

	// MinBcryptCost is the lowest cost the configuration accepts. Tests may use
	// bcrypt.MinCost directly, but a running service must not.
	MinBcryptCost = 10

	// DefaultBcryptCost is the cost used when BCRYPT_COST is not configured.
	DefaultBcryptCost = 12

	// MaxBcryptCost caps the configurable cost. Each step doubles the work, and
	// above this the login endpoint becomes a denial of service against itself.
	MaxBcryptCost = 15
)

// PasswordHasher hashes and verifies user passwords with bcrypt.
type PasswordHasher struct {
	cost int

	// placeholder is a hash of a value nobody can supply. Verifying against it
	// keeps the timing of a login with an unknown email close to the timing of
	// a login with a wrong password, so responses do not leak which emails are
	// registered.
	placeholder []byte
}

// NewPasswordHasher builds a hasher with the given bcrypt cost.
func NewPasswordHasher(cost int) (*PasswordHasher, error) {
	if cost < bcrypt.MinCost || cost > bcrypt.MaxCost {
		return nil, fmt.Errorf("bcrypt cost %d is outside the supported range %d..%d", cost, bcrypt.MinCost, bcrypt.MaxCost)
	}

	placeholder, err := bcrypt.GenerateFromPassword([]byte("placeholder-for-constant-time-login"), cost)
	if err != nil {
		return nil, fmt.Errorf("generate placeholder hash: %w", err)
	}

	return &PasswordHasher{cost: cost, placeholder: placeholder}, nil
}

// Hash returns the bcrypt hash of a password that already passed ValidatePassword.
func (hasher *PasswordHasher) Hash(password string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), hasher.cost)
	if err != nil {
		return "", fmt.Errorf("hash password: %w", err)
	}

	return string(hash), nil
}

// Verify reports whether the password matches the stored hash. It returns
// ErrInvalidCredentials for a mismatch so callers cannot accidentally surface
// the difference between a wrong password and a malformed hash.
func (hasher *PasswordHasher) Verify(hash, password string) error {
	if err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)); err != nil {
		if errors.Is(err, bcrypt.ErrMismatchedHashAndPassword) {
			return ErrInvalidCredentials
		}

		return fmt.Errorf("compare password hash: %w", err)
	}

	return nil
}

// VerifyPlaceholder burns the same amount of CPU as Verify. It is called when
// no user matches the submitted email, so both branches of a failed login cost
// the same.
func (hasher *PasswordHasher) VerifyPlaceholder() {
	_ = bcrypt.CompareHashAndPassword(hasher.placeholder, []byte("placeholder-comparison"))
}

// ValidatePassword checks the password policy before hashing.
func ValidatePassword(password string) error {
	// bcrypt operates on bytes, so the limit is measured in bytes rather than
	// runes: a Cyrillic password reaches 72 bytes after 36 characters.
	switch length := len(password); {
	case length < MinPasswordLength:
		return ValidationError{
			Field:   "password",
			Message: fmt.Sprintf("password must be at least %d characters long", MinPasswordLength),
		}
	case length > MaxPasswordLength:
		return ValidationError{
			Field:   "password",
			Message: fmt.Sprintf("password must not exceed %d bytes", MaxPasswordLength),
		}
	}

	return nil
}
