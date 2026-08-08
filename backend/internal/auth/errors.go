package auth

import "errors"

var (
	// ErrInvalidCredentials is returned when the email is unknown or the
	// password does not match. The two cases share one error on purpose: the
	// API must not tell an attacker which emails are registered.
	ErrInvalidCredentials = errors.New("auth: invalid email or password")

	// ErrEmailTaken is returned when registration collides with an existing account.
	ErrEmailTaken = errors.New("auth: email is already registered")

	// ErrUserNotActive is returned when the account exists but is blocked or deleted.
	ErrUserNotActive = errors.New("auth: user is not active")

	// ErrUserNotFound is returned by the user repository when no row matches.
	ErrUserNotFound = errors.New("auth: user not found")

	// ErrInvalidAccessToken covers a malformed, expired, or wrongly signed JWT.
	ErrInvalidAccessToken = errors.New("auth: invalid access token")

	// ErrInvalidRefreshToken covers an unknown, expired, or already revoked
	// refresh token.
	ErrInvalidRefreshToken = errors.New("auth: invalid refresh token")

	// ErrSessionNotFound is returned by the session repository when no row matches.
	ErrSessionNotFound = errors.New("auth: refresh session not found")
)

// ValidationError reports a request field that failed the input policy. It is
// separate from the errors above so the transport layer can answer 400 instead
// of 401 and name the offending field.
type ValidationError struct {
	Field   string
	Message string
}

func (err ValidationError) Error() string {
	return err.Field + ": " + err.Message
}
