package handlers

import (
	"encoding/json"
	"net/http"

	"kyron-medical/services"
)

// HandleSession returns a lightweight snapshot of the current session state.
// The frontend uses this to reconcile chat UI after a voice call ends.
func HandleSession(w http.ResponseWriter, r *http.Request) {
	sessionID := r.URL.Query().Get("sessionId")
	if sessionID == "" {
		http.Error(w, "missing sessionId", http.StatusBadRequest)
		return
	}

	sess := services.Store.Get(sessionID)
	if sess == nil {
		http.Error(w, "session not found", http.StatusNotFound)
		return
	}

	resp := map[string]interface{}{
		"state":            string(sess.State),
		"patientFirstName": sess.PatientInfo.FirstName,
	}
	if sess.MatchedDoctor != nil {
		resp["doctorId"] = sess.MatchedDoctor.ID
	}
	if sess.SelectedSlot != nil {
		resp["selectedSlot"] = sess.SelectedSlot
	}
	if sess.Appointment != nil {
		resp["appointmentId"] = sess.Appointment.ID
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}
