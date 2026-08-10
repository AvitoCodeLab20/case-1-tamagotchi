package httpserver

import (
	"crypto/rand"
	"encoding/base64"
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

const (
	webSocketTicketTTL       = 30 * time.Second
	webSocketTicketByteCount = 32
)

var errInvalidWebSocketTicket = errors.New(
	"invalid or expired websocket ticket",
)

type webSocketTicket struct {
	userID    uuid.UUID
	expiresAt time.Time
}

type webSocketTicketStore struct {
	mu      sync.Mutex
	tickets map[string]webSocketTicket
	ttl     time.Duration
	now     func() time.Time
}

func newWebSocketTicketStore(
	ttl time.Duration,
) *webSocketTicketStore {
	return &webSocketTicketStore{
		tickets: make(map[string]webSocketTicket),
		ttl:     ttl,
		now:     time.Now,
	}
}

func (store *webSocketTicketStore) Issue(
	userID uuid.UUID,
) (string, time.Time, error) {
	randomBytes := make([]byte, webSocketTicketByteCount)

	if _, err := rand.Read(randomBytes); err != nil {
		return "", time.Time{}, err
	}

	ticket := base64.RawURLEncoding.EncodeToString(randomBytes)
	now := store.now().UTC()
	expiresAt := now.Add(store.ttl)

	store.mu.Lock()
	defer store.mu.Unlock()

	store.removeExpiredLocked(now)

	store.tickets[ticket] = webSocketTicket{
		userID:    userID,
		expiresAt: expiresAt,
	}

	return ticket, expiresAt, nil
}

func (store *webSocketTicketStore) Consume(
	ticket string,
) (uuid.UUID, error) {
	ticket = strings.TrimSpace(ticket)
	if ticket == "" {
		return uuid.Nil, errInvalidWebSocketTicket
	}

	store.mu.Lock()
	defer store.mu.Unlock()

	value, ok := store.tickets[ticket]
	if !ok {
		return uuid.Nil, errInvalidWebSocketTicket
	}

	// Удаляем сразу, поэтому ticket одноразовый.
	delete(store.tickets, ticket)

	if !store.now().UTC().Before(value.expiresAt) {
		return uuid.Nil, errInvalidWebSocketTicket
	}

	return value.userID, nil
}

func (store *webSocketTicketStore) removeExpiredLocked(
	now time.Time,
) {
	for ticket, value := range store.tickets {
		if !now.Before(value.expiresAt) {
			delete(store.tickets, ticket)
		}
	}
}

type webSocketTicketResponse struct {
	Ticket    string    `json:"ticket"`
	ExpiresAt time.Time `json:"expires_at"`
}

func webSocketTicketHandler(
	tickets *webSocketTicketStore,
	logger *slog.Logger,
) http.HandlerFunc {
	return func(
		response http.ResponseWriter,
		request *http.Request,
	) {
		userID, ok := userIDFromContext(request.Context())
		if !ok {
			writeInternalError(
				response,
				logger,
				"create websocket ticket",
				errors.New(
					"authenticated user is missing from context",
				),
			)

			return
		}

		ticket, expiresAt, err := tickets.Issue(userID)
		if err != nil {
			writeInternalError(
				response,
				logger,
				"create websocket ticket",
				err,
			)

			return
		}

		response.Header().Set("Cache-Control", "no-store")

		writeJSON(response, http.StatusOK, webSocketTicketResponse{
			Ticket:    ticket,
			ExpiresAt: expiresAt,
		})
	}
}
