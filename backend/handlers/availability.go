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

	all := services.GenerateAvailability(doctorID)
	available := make([]interface{}, 0, len(all))
	for _, s := range all {
		if s.Available {
			available = append(available, s)
		}
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"slots": available})
}
