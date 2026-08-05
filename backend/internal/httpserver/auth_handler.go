package httpserver

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/google/uuid"

	"github.com/AvitoCodeLab20/case-1-tamagotchi/backend/internal/auth"
)

// authService is the part of auth.Service the transport layer uses.
type authService interface {
	Register(ctx context.Context, input auth.RegisterInput) (auth.TokenPair, error)
	Login(ctx context.Context, input auth.LoginInput) (auth.TokenPair, error)
	Refresh(ctx context.Context, refreshToken string) (auth.TokenPair, error)
	Logout(ctx context.Context, refreshToken string) error
	LogoutAll(ctx context.Context, userID uuid.UUID) error
	UserByID(ctx context.Context, userID uuid.UUID) (auth.User, error)
	ParseAccessToken(token string) (uuid.UUID, error)
}

type registerRequest struct {
	Email       string `json:"email"`
	DisplayName string `json:"display_name"`
	Password    string `json:"password"`
}

type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type refreshRequest struct {
	RefreshToken string `json:"refresh_token"`
}

type userResponse struct {
	ID          string     `json:"id"`
	Email       string     `json:"email"`
	DisplayName string     `json:"display_name"`
	Status      string     `json:"status"`
	CreatedAt   time.Time  `json:"created_at"`
	LastLoginAt *time.Time `json:"last_login_at,omitempty"`
}

// tokenResponse follows the OAuth 2.0 bearer-token field names so any standard
// HTTP client can consume it without a custom mapping.
type tokenResponse struct {
	AccessToken           string       `json:"access_token"`
	TokenType             string       `json:"token_type"`
	ExpiresIn             int          `json:"expires_in"`
	ExpiresAt             time.Time    `json:"expires_at"`
	RefreshToken          string       `json:"refresh_token"`
	RefreshTokenExpiresAt time.Time    `json:"refresh_token_expires_at"`
	User                  userResponse `json:"user"`
}

func newUserResponse(user auth.User) userResponse {
	return userResponse{
		ID:          user.ID.String(),
		Email:       user.Email,
		DisplayName: user.DisplayName,
		Status:      user.Status,
		CreatedAt:   user.CreatedAt,
		LastLoginAt: user.LastLoginAt,
	}
}

func newTokenResponse(pair auth.TokenPair, now time.Time) tokenResponse {
	return tokenResponse{
		AccessToken:           pair.AccessToken,
		TokenType:             "Bearer",
		ExpiresIn:             int(pair.AccessTokenExpiresAt.Sub(now).Round(time.Second).Seconds()),
		ExpiresAt:             pair.AccessTokenExpiresAt,
		RefreshToken:          pair.RefreshToken,
		RefreshTokenExpiresAt: pair.RefreshTokenExpiresAt,
		User:                  newUserResponse(pair.User),
	}
}

// registerHandler creates an account and returns a signed-in session.
func registerHandler(service authService, logger *slog.Logger) http.HandlerFunc {
	return func(response http.ResponseWriter, request *http.Request) {
		payload := registerRequest{}
		if err := decodeJSON(response, request, &payload); err != nil {
			writeError(response, http.StatusBadRequest, codeBadRequest, err.Error())

			return
		}

		pair, err := service.Register(request.Context(), auth.RegisterInput{
			Email:       payload.Email,
			DisplayName: payload.DisplayName,
			Password:    payload.Password,
		})
		if err != nil {
			writeAuthError(response, logger, "register", err)

			return
		}

		writeJSON(response, http.StatusCreated, newTokenResponse(pair, time.Now()))
	}
}

// loginHandler exchanges an email and password for a token pair.
func loginHandler(service authService, logger *slog.Logger) http.HandlerFunc {
	return func(response http.ResponseWriter, request *http.Request) {
		payload := loginRequest{}
		if err := decodeJSON(response, request, &payload); err != nil {
			writeError(response, http.StatusBadRequest, codeBadRequest, err.Error())

			return
		}

		pair, err := service.Login(request.Context(), auth.LoginInput{
			Email:    payload.Email,
			Password: payload.Password,
		})
		if err != nil {
			writeAuthError(response, logger, "login", err)

			return
		}

		writeJSON(response, http.StatusOK, newTokenResponse(pair, time.Now()))
	}
}

// refreshHandler rotates a refresh token into a new pair.
func refreshHandler(service authService, logger *slog.Logger) http.HandlerFunc {
	return func(response http.ResponseWriter, request *http.Request) {
		payload := refreshRequest{}
		if err := decodeJSON(response, request, &payload); err != nil {
			writeError(response, http.StatusBadRequest, codeBadRequest, err.Error())

			return
		}
		if payload.RefreshToken == "" {
			writeFieldError(response, "refresh_token", "refresh token is required")

			return
		}

		pair, err := service.Refresh(request.Context(), payload.RefreshToken)
		if err != nil {
			writeAuthError(response, logger, "refresh", err)

			return
		}

		writeJSON(response, http.StatusOK, newTokenResponse(pair, time.Now()))
	}
}

// logoutHandler revokes the presented refresh token. It answers 204 even for an
// unknown token: a client that is signing out cannot act on the difference, and
// a different answer would turn the endpoint into a token oracle.
func logoutHandler(service authService, logger *slog.Logger) http.HandlerFunc {
	return func(response http.ResponseWriter, request *http.Request) {
		payload := refreshRequest{}
		if err := decodeJSON(response, request, &payload); err != nil {
			writeError(response, http.StatusBadRequest, codeBadRequest, err.Error())

			return
		}
		if payload.RefreshToken == "" {
			writeFieldError(response, "refresh_token", "refresh token is required")

			return
		}

		if err := service.Logout(request.Context(), payload.RefreshToken); err != nil {
			writeInternalError(response, logger, "logout", err)

			return
		}

		response.WriteHeader(http.StatusNoContent)
	}
}

// logoutAllHandler revokes every session of the authenticated user.
func logoutAllHandler(service authService, logger *slog.Logger) http.HandlerFunc {
	return func(response http.ResponseWriter, request *http.Request) {
		userID, ok := userIDFromContext(request.Context())
		if !ok {
			writeInternalError(response, logger, "logout all", errors.New("handler reached without requireAuth"))

			return
		}

		if err := service.LogoutAll(request.Context(), userID); err != nil {
			writeInternalError(response, logger, "logout all", err)

			return
		}

		response.WriteHeader(http.StatusNoContent)
	}
}

// currentUserHandler returns the account behind the presented access token.
func currentUserHandler(service authService, logger *slog.Logger) http.HandlerFunc {
	return func(response http.ResponseWriter, request *http.Request) {
		userID, ok := userIDFromContext(request.Context())
		if !ok {
			writeInternalError(response, logger, "current user", errors.New("handler reached without requireAuth"))

			return
		}

		user, err := service.UserByID(request.Context(), userID)
		if err != nil {
			writeAuthError(response, logger, "current user", err)

			return
		}

		writeJSON(response, http.StatusOK, newUserResponse(user))
	}
}

// writeAuthError maps a domain error to its HTTP answer. Anything unrecognised
// is a 500 with the cause in the log and not in the response.
func writeAuthError(response http.ResponseWriter, logger *slog.Logger, operation string, err error) {
	validationError := auth.ValidationError{}

	switch {
	case errors.As(err, &validationError):
		writeFieldError(response, validationError.Field, validationError.Message)
	case errors.Is(err, auth.ErrInvalidCredentials):
		writeError(response, http.StatusUnauthorized, codeInvalidCredentials, "invalid email or password")
	case errors.Is(err, auth.ErrEmailTaken):
		writeError(response, http.StatusConflict, codeEmailTaken, "this email is already registered")
	case errors.Is(err, auth.ErrUserNotActive):
		writeError(response, http.StatusForbidden, codeUserNotActive, "account is not active")
	case errors.Is(err, auth.ErrInvalidRefreshToken):
		writeError(response, http.StatusUnauthorized, codeUnauthorized, "refresh token is invalid or expired")
	case errors.Is(err, auth.ErrInvalidAccessToken), errors.Is(err, auth.ErrUserNotFound):
		writeError(response, http.StatusUnauthorized, codeUnauthorized, "access token is invalid or expired")
	default:
		writeInternalError(response, logger, operation, err)
	}
}
