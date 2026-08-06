package response

import (
	"encoding/json"
	"net/http"
)

type errorResponse struct {
	Error string `json:"error"`
}

// WriteJSON writes data as a JSON body with the given status code.
func WriteJSON(w http.ResponseWriter, data any, status int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

// WriteError writes msg as a {"error": msg} JSON body with the given status
// code.
func WriteError(w http.ResponseWriter, msg string, status int) {
	WriteJSON(w, errorResponse{Error: msg}, status)
}
