package pet

import (
	"time"

	"github.com/google/uuid"
)

type Pet struct {
	ID                uuid.UUID
	UserID            uuid.UUID
	Name              string
	Species           string
	Level             int
	Experience        int64
	Health            int
	Hunger            int
	Happiness         int
	Energy            int
	StateVersion      int64
	LastInteractionAt *time.Time
	CreatedAt         time.Time
	UpdatedAt         time.Time
}
