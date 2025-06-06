package helper

import (
	"encoding/json"
	"log"
	"net/http"
)

// ErrorResponse defines the standard JSON error structure.
type ErrorResponse struct {
	Error  string `json:"error"`
	Detail any    `json:"detail,omitempty"`
}

func ResponseJson(w http.ResponseWriter, code int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	if data != nil {
		if err := json.NewEncoder(w).Encode(data); err != nil {
			log.Fatalf("Error encoding JSON response: %v\n", err)
		}
	}
}

func ResponseErrorJson(w http.ResponseWriter, code int, message string, detail any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	response := ErrorResponse{Error: message}
	if detail != nil {
		response.Detail = detail
	}
	if encodeErr := json.NewEncoder(w).Encode(response); encodeErr != nil {
		log.Fatalf("Error encoding JSON error response: %v\n", encodeErr)
	}
}

// SimpleMessage is a basic struct for simple JSON responses.
type SimpleMessage struct {
	Message string `json:"message"`
}
