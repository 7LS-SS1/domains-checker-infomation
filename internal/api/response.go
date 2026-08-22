package api

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"domainmonitor/internal/i18n"
)

type errorBody struct {
	Error errorPayload `json:"error"`
}

type errorPayload struct {
	Code      string           `json:"code"`
	Message   string           `json:"message"`
	Messages  i18n.Translation `json:"messages"`
	Locale    i18n.Locale      `json:"locale"`
	RequestID string           `json:"request_id"`
	Details   map[string]any   `json:"details,omitempty"`
}

type dataBody struct {
	Data any `json:"data"`
}

func writeData(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(dataBody{Data: data})
}

func writeError(w http.ResponseWriter, r *http.Request, status int, code string, details map[string]any) {
	locale := i18n.FromContext(r.Context(), i18n.Thai)
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(errorBody{Error: errorPayload{
		Code:      code,
		Message:   i18n.Message(code, locale),
		Messages:  i18n.Messages(code),
		Locale:    locale,
		RequestID: RequestID(r.Context()),
		Details:   details,
	}})
}

func decodeJSON(w http.ResponseWriter, r *http.Request, destination any) bool {
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(destination); err != nil {
		writeError(w, r, http.StatusBadRequest, "INVALID_JSON", map[string]any{"reason": safeJSONError(err)})
		return false
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		writeError(w, r, http.StatusBadRequest, "INVALID_JSON", map[string]any{"reason": "multiple JSON values are not allowed"})
		return false
	}
	return true
}

func safeJSONError(err error) string {
	var syntaxError *json.SyntaxError
	if errors.As(err, &syntaxError) {
		return "invalid JSON syntax"
	}
	var typeError *json.UnmarshalTypeError
	if errors.As(err, &typeError) {
		return "invalid value type for field " + typeError.Field
	}
	if errors.Is(err, io.EOF) {
		return "request body is required"
	}
	if len(err.Error()) > 120 {
		return "invalid JSON payload"
	}
	return err.Error()
}
