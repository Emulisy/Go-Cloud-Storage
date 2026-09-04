package httpapi

import (
	"encoding/json"
	"net/http"
)

func NewHandler() http.Handler {
	mux := http.NewServeMux()

	// TODO: Register health as:
	//
	// GET /healthz
	//
	// The handler function is health.
	mux.HandleFunc("GET /healthz", health)

	return mux
}

func health(w http.ResponseWriter, r *http.Request) {
	// TODO:
	// 1. Set Content-Type to application/json.
	// 2. Write status 200 OK.
	// 3. Encode {"status":"ok"} as JSON.
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	encoder := json.NewEncoder(w)
	err := encoder.Encode(map[string]string{"status": "ok"})
	if err != nil {
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
	}
}
