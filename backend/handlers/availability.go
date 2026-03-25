package handlers

import (
	"encoding/json"
	"net/http"

	"kyron-medical/services"
)

func HandleAvailability(w http.ResponseWriter, r *http.Request) {
	doctorID := r.URL.Query().Get("doctorId")
	if doctorID == "" {
		http.Error(w, "doctorId required", http.StatusBadRequest)
		return
	}

	if services.GetDoctorByID(doctorID) == nil {
		http.Error(w, "doctor not found", http.StatusNotFound)
		return
	}

	slots := services.GenerateAvailability(doctorID)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"slots": slots})
}
