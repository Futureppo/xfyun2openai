package api

import (
	"context"
	crand "crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"fmt"
	"net/http"
	"strings"
	"time"

	"xfyun2openai/internal/openai"
)

type contextKey string

const requestIDKey contextKey = "request_id"

func withRequestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestID := newRequestID()
		w.Header().Set("X-Request-ID", requestID)
		ctx := context.WithValue(r.Context(), requestIDKey, requestID)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func withAuth(apiKeys []string, next http.Handler) http.Handler {
	if len(apiKeys) == 0 {
		return next
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token := bearerToken(r.Header.Get("Authorization"))
		if token == "" {
			writeOpenAIError(w, openai.NewHTTPError(http.StatusUnauthorized, "missing Authorization bearer token", "authentication_error", "authorization", "missing_authorization"))
			return
		}
		if !tokenAllowed(token, apiKeys) {
			writeOpenAIError(w, openai.NewHTTPError(http.StatusUnauthorized, "invalid Authorization bearer token", "authentication_error", "authorization", "invalid_authorization"))
			return
		}

		next.ServeHTTP(w, r)
	})
}

func withMethod(method string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != method {
			writeOpenAIError(w, openai.NewHTTPError(http.StatusMethodNotAllowed, "method not allowed", "invalid_request_error", "method", "method_not_allowed"))
			return
		}
		next.ServeHTTP(w, r)
	})
}

func RequestIDFromContext(ctx context.Context) string {
	if requestID, ok := ctx.Value(requestIDKey).(string); ok {
		return requestID
	}
	return "req-unknown"
}

func newRequestID() string {
	var suffix [4]byte
	if _, err := crand.Read(suffix[:]); err != nil {
		return fmt.Sprintf("req-%d", time.Now().UnixNano())
	}
	return fmt.Sprintf("req-%d-%s", time.Now().UnixNano(), hex.EncodeToString(suffix[:]))
}

func bearerToken(header string) string {
	const prefix = "Bearer "
	if !strings.HasPrefix(header, prefix) {
		return ""
	}
	return strings.TrimSpace(strings.TrimPrefix(header, prefix))
}

func tokenAllowed(token string, allowed []string) bool {
	for _, candidate := range allowed {
		if subtle.ConstantTimeCompare([]byte(token), []byte(candidate)) == 1 {
			return true
		}
	}
	return false
}
