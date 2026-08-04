package auth

import (
	"fmt"
	"net/mail"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
)

// User status values mirror the CHECK constraint on users.status.
const (
	StatusActive  = "active"
	StatusBlocked = "blocked"
	StatusDeleted = "deleted"
)

const (
	minDisplayNameLength = 2
	maxDisplayNameLength = 40
	maxEmailLength       = 254 // RFC 5321 limit on a forward path.
)

// User is an account as the auth package needs it. The password hash never
// leaves the repository layer in this struct, so it cannot be serialised by
// accident.
type User struct {
	ID          uuid.UUID
	Email       string
	DisplayName string
	Status      string
	LastLoginAt *time.Time
	CreatedAt   time.Time
}

// IsActive reports whether the account may authenticate.
func (user User) IsActive() bool {
	return user.Status == StatusActive
}

// Credentials is a stored account together with its password hash. Only the
// login path loads it.
type Credentials struct {
	User         User
	PasswordHash string
}

// RefreshSession is one issued refresh token. The plaintext token is never
// stored; TokenHash is the SHA-256 digest of it.
type RefreshSession struct {
	ID        uuid.UUID
	UserID    uuid.UUID
	TokenHash []byte
	ExpiresAt time.Time
	RevokedAt *time.Time
	CreatedAt time.Time
}

// CreateUserParams carries a validated, normalised account to the repository.
type CreateUserParams struct {
	Email        string
	DisplayName  string
	PasswordHash string
}

// RegisterInput is the raw registration request.
type RegisterInput struct {
	Email       string
	DisplayName string
	Password    string
}

// LoginInput is the raw login request.
type LoginInput struct {
	Email    string
	Password string
}

// TokenPair is what a successful register, login, or refresh hands back.
type TokenPair struct {
	AccessToken           string
	AccessTokenExpiresAt  time.Time
	RefreshToken          string
	RefreshTokenExpiresAt time.Time
	User                  User
}

// normaliseEmail trims and lowercases an address so it matches the
// users_email_lower_uidx unique index.
func normaliseEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}

// validateEmail checks the address against the format the users table accepts.
func validateEmail(email string) error {
	invalid := ValidationError{Field: "email", Message: "email is not a valid address"}

	if email == "" {
		return ValidationError{Field: "email", Message: "email is required"}
	}
	if len(email) > maxEmailLength {
		return ValidationError{
			Field:   "email",
			Message: fmt.Sprintf("email must not exceed %d characters", maxEmailLength),
		}
	}

	address, err := mail.ParseAddress(email)
	if err != nil {
		return invalid
	}
	// mail.ParseAddress accepts display-name forms such as `A <a@b.c>`; the
	// address must be bare so it round-trips through the unique index.
	if address.Address != email || address.Name != "" {
		return invalid
	}
	if strings.Count(email, "@") != 1 {
		return invalid
	}

	return nil
}

// validateDisplayName mirrors the char_length CHECK on users.display_name so a
// bad request fails as 400 instead of a constraint violation.
func validateDisplayName(displayName string) error {
	length := utf8.RuneCountInString(displayName)
	if length < minDisplayNameLength || length > maxDisplayNameLength {
		return ValidationError{
			Field:   "display_name",
			Message: fmt.Sprintf("display name must be between %d and %d characters", minDisplayNameLength, maxDisplayNameLength),
		}
	}

	return nil
}
