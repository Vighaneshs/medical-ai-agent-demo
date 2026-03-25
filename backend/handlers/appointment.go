package handlers

import (
	"encoding/json"
	"net/http"

	"kyron-medical/services"
)

func HandleAppointment(w http.ResponseWriter, r *http.Request) {
	sessionID := r.URL.Query().Get("sessionId")
	if sessionID == "" {
		http.Error(w, "missing sessionId", http.StatusBadRequest)
		return
	}

	sess := services.Store.Get(sessionID)
	if sess == nil || sess.Appointment == nil {
		http.Error(w, "no appointment found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(sess.Appointment)
}
