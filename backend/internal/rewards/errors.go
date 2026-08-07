package rewards

import "errors"

var (
	ErrNotFound            = errors.New("rewards: reward not found")
	ErrNotAvailable        = errors.New("rewards: reward is not available")
	ErrSelectionExpired    = errors.New("rewards: selection has expired")
	ErrIdempotencyConflict = errors.New("rewards: idempotency key was reused")
)

type ValidationError struct {
	Field   string
	Message string
}

func (validationError ValidationError) Error() string {
	return validationError.Message
}
