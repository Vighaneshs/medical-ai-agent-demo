package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHandleDoctors_ReturnsOK(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/api/doctors", nil)
	w := httptest.NewRecorder()
	HandleDoctors(w, r)

	if w.Code != http.StatusOK {
		t.Errorf("got status %d, want 200", w.Code)
	}
}

func TestHandleDoctors_ContentType(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/api/doctors", nil)
	w := httptest.NewRecorder()
	HandleDoctors(w, r)

	ct := w.Header().Get("Content-Type")
	if ct != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", ct)
	}
}

func TestHandleDoctors_ReturnsFiveDoctors(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/api/doctors", nil)
	w := httptest.NewRecorder()
	HandleDoctors(w, r)

	var resp map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("could not decode response: %v", err)
	}

	doctors, ok := resp["doctors"].([]interface{})
	if !ok {
		t.Fatalf("response missing 'doctors' array")
	}
	if len(doctors) != 5 {
		t.Errorf("got %d doctors, want 5", len(doctors))
	}
}

func TestHandleDoctors_EachDoctorHasRequiredFields(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/api/doctors", nil)
	w := httptest.NewRecorder()
	HandleDoctors(w, r)

	var resp map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("could not decode response: %v", err)
	}

	doctors, _ := resp["doctors"].([]interface{})
	for i, d := range doctors {
		doc, ok := d.(map[string]interface{})
		if !ok {
			t.Errorf("doctor[%d] is not an object", i)
			continue
		}
		for _, field := range []string{"id", "name", "specialty"} {
			if v, ok := doc[field].(string); !ok || v == "" {
				t.Errorf("doctor[%d] missing or empty field %q", i, field)
			}
		}
	}
}

func TestHandleDoctors_ContainsAllKnownIDs(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/api/doctors", nil)
	w := httptest.NewRecorder()
	HandleDoctors(w, r)

	var resp map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("could not decode response: %v", err)
	}

	doctors, _ := resp["doctors"].([]interface{})
	ids := map[string]bool{}
	for _, d := range doctors {
		if doc, ok := d.(map[string]interface{}); ok {
			if id, ok := doc["id"].(string); ok {
				ids[id] = true
			}
		}
	}

	expected := []string{"dr-mitchell", "dr-patel", "dr-chen", "dr-rodriguez", "dr-thompson"}
	for _, want := range expected {
		if !ids[want] {
			t.Errorf("doctor %q not found in response", want)
		}
	}
}
