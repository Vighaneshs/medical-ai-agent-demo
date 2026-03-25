package handlers

import (
	"encoding/json"
	"net/http"

	"kyron-medical/services"
)

func HandleDoctors(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"doctors": services.Doctors})
}
