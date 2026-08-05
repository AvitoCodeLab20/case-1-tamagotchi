package httpserver

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
)

// maxRequestBodyBytes caps a decoded request body. Auth payloads are a few
// hundred bytes, so anything larger is either a bug or an attempt to make the
// server allocate.
const maxRequestBodyBytes = 8 << 10

// Error codes returned in the error envelope. Clients branch on the code, never
// on the message, so messages stay free to change.
const (
	codeBadRequest         = "bad_request"
	codeValidationFailed   = "validation_failed"
	codeInvalidCredentials = "invalid_credentials" //nolint:gosec // an error code, not a credential
	codeEmailTaken         = "email_taken"
	codeUserNotActive      = "user_not_active"
	codeUnauthorized       = "unauthorized"
	codeRateLimited        = "rate_limited"
	codeInternalError      = "internal_error"
)

// errorResponse is the single error shape of the API.
type errorResponse struct {
	Error errorBody `json:"error"`
}

type errorBody struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Field   string `json:"field,omitempty"`
}

// decodeJSON reads a JSON request body into target, rejecting anything the
// handler would otherwise have to guess about: wrong content type, unknown
// fields, oversized bodies, and trailing garbage.
func decodeJSON(response http.ResponseWriter, request *http.Request, target any) error {
	contentType := request.Header.Get("Content-Type")
	if contentType != "" {
		mediaType, _, _ := strings.Cut(contentType, ";")
		if strings.TrimSpace(mediaType) != "application/json" {
			return errors.New("Content-Type must be application/json")
		}
	}

	request.Body = http.MaxBytesReader(response, request.Body, maxRequestBodyBytes)

	decoder := json.NewDecoder(request.Body)
	decoder.DisallowUnknownFields()

	if err := decoder.Decode(target); err != nil {
		var maxBytesError *http.MaxBytesError
		if errors.As(err, &maxBytesError) {
			return fmt.Errorf("request body must not exceed %d bytes", maxRequestBodyBytes)
		}

		return fmt.Errorf("malformed JSON body: %w", err)
	}

	// A second value in the stream means the client sent something other than
	// the single object this endpoint documents.
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return errors.New("request body must contain a single JSON object")
	}

	return nil
}

// writeJSON writes a success payload.
func writeJSON(response http.ResponseWriter, status int, payload any) {
	response.Header().Set("Content-Type", "application/json")
	response.WriteHeader(status)

	_ = json.NewEncoder(response).Encode(payload)
}

// writeError writes the error envelope.
func writeError(response http.ResponseWriter, status int, code, message string) {
	writeJSON(response, status, errorResponse{Error: errorBody{Code: code, Message: message}})
}

// writeFieldError writes the error envelope for a rejected input field.
func writeFieldError(response http.ResponseWriter, field, message string) {
	writeJSON(response, http.StatusBadRequest, errorResponse{
		Error: errorBody{Code: codeValidationFailed, Message: message, Field: field},
	})
}

// writeInternalError logs the cause and answers with a generic message, so an
// internal failure never reaches the client as a database or driver string.
func writeInternalError(response http.ResponseWriter, logger *slog.Logger, operation string, err error) {
	logger.Error("request failed", "operation", operation, "error", err)
	writeError(response, http.StatusInternalServerError, codeInternalError, "internal server error")
}
