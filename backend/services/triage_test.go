package services

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestIsEmergency(t *testing.T) {
	tests := []struct {
		text     string
		want     bool
	}{
		// True positives — each pattern
		{"I have chest pain", true},
		{"severe chest tightness", true},
		{"I think I'm having a heart attack", true},
		{"I can't breathe at all", true},
		{"I cant breathe", true},
		{"having difficulty breathing", true},
		{"trouble breathing right now", true},
		{"I think I had a stroke", true},
		{"he is unresponsive", true},
		{"she became unconscious", true},
		{"I'm bleeding heavily", true},
		{"I am heavily bleeding", true},
		{"having suicidal thoughts", true},
		{"I want to kill myself", true},
		{"I want to hurt myself", true},
		{"I think I overdosed", true},
		{"the baby is not breathing", true},
		{"he's about to stop breathing", true},
		{"I collapsed at work", true},
		// Case insensitivity
		{"CHEST PAIN", true},
		{"HeArT AtTaCk", true},
		{"SUICIDAL", true},
		// False negatives — regular messages
		{"I have a headache", false},
		{"I'd like to book an appointment", false},
		{"Hello, how are you?", false},
		{"I need a prescription refill", false},
		{"migraine for three days", false},
		{"back pain", false},
		{"my knee hurts", false},
		{"", false},
		// Word boundary: stroke as compound word should not match
		{"working on my backstroke", false},
		// 'bleed' without 'heavily' should not match
		{"my finger is bleeding a little", false},
	}

	for _, tc := range tests {
		t.Run(tc.text, func(t *testing.T) {
			if got := IsEmergency(tc.text); got != tc.want {
				t.Errorf("IsEmergency(%q) = %v, want %v", tc.text, got, tc.want)
			}
		})
	}
}

func TestEmergencySSEPayload_IsValidJSON(t *testing.T) {
	payload := EmergencySSEPayload()
	if len(payload) == 0 {
		t.Fatal("expected non-empty payload")
	}

	var result map[string]interface{}
	if err := json.Unmarshal(payload, &result); err != nil {
		t.Fatalf("payload is not valid JSON: %v", err)
	}
}

func TestEmergencySSEPayload_Fields(t *testing.T) {
	var result struct {
		Text      string `json:"text"`
		Emergency bool   `json:"emergency"`
	}
	if err := json.Unmarshal(EmergencySSEPayload(), &result); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if !result.Emergency {
		t.Error("emergency field must be true")
	}
	if result.Text == "" {
		t.Error("text field must not be empty")
	}
	if !strings.Contains(result.Text, "911") {
		t.Error("emergency text must mention 911")
	}
}
