package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHandleAvailability_MissingDoctorID(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/api/availability", nil)
	w := httptest.NewRecorder()
	HandleAvailability(w, r)
	if w.Code != http.StatusBadRequest {
		t.Errorf("got status %d, want 400 for missing doctorId", w.Code)
	}
}

func TestHandleAvailability_InvalidDoctorID(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/api/availability?doctorId=dr-nobody", nil)
	w := httptest.NewRecorder()
	HandleAvailability(w, r)
	if w.Code != http.StatusNotFound {
		t.Errorf("got status %d, want 404 for unknown doctorId", w.Code)
	}
}

func TestHandleAvailability_ValidDoctorID(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/api/availability?doctorId=dr-mitchell", nil)
	w := httptest.NewRecorder()
	HandleAvailability(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("got status %d, want 200", w.Code)
	}

	var resp map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("could not decode response: %v", err)
	}

	slots, ok := resp["slots"].([]interface{})
	if !ok {
		t.Fatalf("response missing 'slots' array, got: %v", resp)
	}
	if len(slots) == 0 {
		t.Error("expected at least one slot in availability response")
	}
}

func TestHandleAvailability_ContentType(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/api/availability?doctorId=dr-patel", nil)
	w := httptest.NewRecorder()
	HandleAvailability(w, r)

	ct := w.Header().Get("Content-Type")
	if ct != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", ct)
	}
}

func TestHandleAvailability_AllDoctors(t *testing.T) {
	doctorIDs := []string{"dr-mitchell", "dr-patel", "dr-chen", "dr-rodriguez", "dr-thompson"}
	for _, id := range doctorIDs {
		t.Run(id, func(t *testing.T) {
			r := httptest.NewRequest(http.MethodGet, "/api/availability?doctorId="+id, nil)
			w := httptest.NewRecorder()
			HandleAvailability(w, r)
			if w.Code != http.StatusOK {
				t.Errorf("doctor %q: got status %d, want 200", id, w.Code)
			}
		})
	}
}
