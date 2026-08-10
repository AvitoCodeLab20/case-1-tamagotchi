package httpserver

import (
	"context"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
	"github.com/google/uuid"

	"github.com/AvitoCodeLab20/case-1-tamagotchi/backend/internal/pet"
)

const (
	petStateEventType      = "pet.state"
	webSocketAuthTimeout   = 5 * time.Second
	webSocketWriteTimeout  = 5 * time.Second
	webSocketPingInterval  = 30 * time.Second
	webSocketReadLimit     = 8 << 10
	petStateSubscriberSize = 1
)

type petStatePublisher interface {
	Publish(userID uuid.UUID, value pet.Pet)
}

type petStateHub struct {
	mu          sync.Mutex
	subscribers map[uuid.UUID]map[chan pet.Pet]struct{}
	closed      bool
}

func newPetStateHub() *petStateHub {
	return &petStateHub{
		subscribers: make(map[uuid.UUID]map[chan pet.Pet]struct{}),
	}
}
func (hub *petStateHub) Subscribe(userID uuid.UUID) (<-chan pet.Pet, func()) {
	hub.mu.Lock()
	defer hub.mu.Unlock()

	updates := make(chan pet.Pet, petStateSubscriberSize)
	if hub.closed {
		close(updates)

		return updates, func() {}
	}

	if hub.subscribers[userID] == nil {
		hub.subscribers[userID] = make(map[chan pet.Pet]struct{})
	}
	hub.subscribers[userID][updates] = struct{}{}

	return updates, func() {
		hub.unsubscribe(userID, updates)
	}
}

func (hub *petStateHub) unsubscribe(userID uuid.UUID, updates chan pet.Pet) {
	hub.mu.Lock()
	defer hub.mu.Unlock()

	userSubscribers, ok := hub.subscribers[userID]
	if !ok {
		return
	}
	if _, ok = userSubscribers[updates]; !ok {
		return
	}

	delete(userSubscribers, updates)
	close(updates)

	if len(userSubscribers) == 0 {
		delete(hub.subscribers, userID)
	}
}

func (hub *petStateHub) Publish(userID uuid.UUID, value pet.Pet) {
	hub.mu.Lock()
	defer hub.mu.Unlock()

	for updates := range hub.subscribers[userID] {
		select {
		case updates <- value:
			continue
		default:
		}

		select {
		case <-updates:
		default:
		}

		select {
		case updates <- value:
		default:
		}
	}
}

func (hub *petStateHub) Close() {
	hub.mu.Lock()
	defer hub.mu.Unlock()

	if hub.closed {
		return
	}
	hub.closed = true

	for userID, userSubscribers := range hub.subscribers {
		for updates := range userSubscribers {
			close(updates)
		}
		delete(hub.subscribers, userID)
	}
}

type petStateData struct {
	Level        int       `json:"level"`
	Experience   int64     `json:"experience"`
	Health       int       `json:"health"`
	Hunger       int       `json:"hunger"`
	Happiness    int       `json:"happiness"`
	Energy       int       `json:"energy"`
	StateVersion int64     `json:"state_version"`
	UpdatedAt    time.Time `json:"updated_at"`
}

func newPetStateData(value pet.Pet) petStateData {
	return petStateData{
		Level:        value.Level,
		Experience:   value.Experience,
		Health:       value.Health,
		Hunger:       value.Hunger,
		Happiness:    value.Happiness,
		Energy:       value.Energy,
		StateVersion: value.StateVersion,
		UpdatedAt:    value.UpdatedAt,
	}
}

func petStateWebSocketHandler(
	petService petService,
	tickets *webSocketTicketStore,
	hub *petStateHub,
	originPatterns []string,
	logger *slog.Logger,
) http.HandlerFunc {
	return func(response http.ResponseWriter, request *http.Request) {
		connection, err := websocket.Accept(response, request, &websocket.AcceptOptions{
			OriginPatterns: originPatterns,
		})
		if err != nil {
			logger.Warn("accept pet state websocket", "error", err)

			return
		}
		defer func() {
			if err := connection.CloseNow(); err != nil {
				logger.Error("close websocket connection", "error", err)
			}
		}()
		connection.SetReadLimit(webSocketReadLimit)

		ticket := strings.TrimSpace(
			request.URL.Query().Get("ticket"),
		)

		userID, err := tickets.Consume(ticket)
		if err != nil {
			writeError(
				response,
				http.StatusUnauthorized,
				codeUnauthorized,
				"websocket ticket is invalid or expired",
			)

			return
		}

		updates, unsubscribe := hub.Subscribe(userID)
		defer unsubscribe()

		connectionContext := connection.CloseRead(context.Background())

		currentPet, err := petService.Get(connectionContext, userID)
		if err != nil {
			logger.Error("load initial pet state", "user_id", userID, "error", err)
			_ = connection.Close(websocket.StatusInternalError, "failed to load pet state")

			return
		}

		if err = writePetState(connection, currentPet); err != nil {
			return
		}

		pingTicker := time.NewTicker(webSocketPingInterval)
		defer pingTicker.Stop()

		for {
			select {
			case <-connectionContext.Done():
				return

			case value, open := <-updates:
				if !open {
					_ = connection.Close(websocket.StatusGoingAway, "server shutting down")

					return
				}

				if err = writePetState(connection, value); err != nil {
					return
				}

			case <-pingTicker.C:
				pingContext, cancel := context.WithTimeout(
					context.Background(),
					webSocketWriteTimeout,
				)
				err = connection.Ping(pingContext)
				cancel()
				if err != nil {
					return
				}
			}
		}
	}
}

func writePetState(connection *websocket.Conn, value pet.Pet) error {
	writeContext, cancel := context.WithTimeout(
		context.Background(),
		webSocketWriteTimeout,
	)
	defer cancel()

	return wsjson.Write(
		writeContext,
		connection,
		newPetStateData(value),
	)
}
