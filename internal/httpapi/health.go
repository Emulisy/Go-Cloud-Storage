package httpapi

import "net/http"

// health reports process liveness only; it does not probe storage readiness.
func health(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}
