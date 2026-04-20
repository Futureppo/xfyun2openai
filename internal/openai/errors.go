package openai

import (
	"encoding/json"
	"net/http"
)

type ErrorResponse struct {
	Error ErrorBody `json:"error"`
}

type ErrorBody struct {
	Message string `json:"message"`
	Type    string `json:"type"`
	Param   string `json:"param,omitempty"`
	Code    string `json:"code,omitempty"`
}

type HTTPError struct {
	Status int
	Body   ErrorBody
}

func (e *HTTPError) Error() string {
	return e.Body.Message
}

func NewHTTPError(status int, message, errType, param, code string) *HTTPError {
	return &HTTPError{
		Status: status,
		Body: ErrorBody{
			Message: message,
			Type:    errType,
			Param:   param,
			Code:    code,
		},
	}
}

func WriteError(w http.ResponseWriter, err *HTTPError) {
	writeJSON(w, err.Status, ErrorResponse{Error: err.Body})
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}
