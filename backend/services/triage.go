package services

import (
	"encoding/json"
	"regexp"
)

// emergencyPatterns are checked BEFORE any Claude API call.
// If matched, a hardcoded safety response is returned immediately.
// No external calls, no automation — purely informational text in the chat.
var emergencyPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)chest\s+pain`),
	regexp.MustCompile(`(?i)can'?t\s+breath`),
	regexp.MustCompile(`(?i)difficulty\s+breath`),
	regexp.MustCompile(`(?i)trouble\s+breath`),
	regexp.MustCompile(`(?i)severe\s+chest`),
	regexp.MustCompile(`(?i)heart\s+attack`),
	regexp.MustCompile(`(?i)\bstroke\b`),
	regexp.MustCompile(`(?i)unresponsive`),
	regexp.MustCompile(`(?i)unconscious`),
	regexp.MustCompile(`(?i)bleed(ing)?\s+heavy`),
	regexp.MustCompile(`(?i)heavil?y\s+bleed`),
	regexp.MustCompile(`(?i)suicid`),
	regexp.MustCompile(`(?i)want\s+to\s+(hurt|kill)\s+myself`),
	regexp.MustCompile(`(?i)overdos`),
	regexp.MustCompile(`(?i)not\s+breathing`),
	regexp.MustCompile(`(?i)stop\s+breathing`),
	regexp.MustCompile(`(?i)collapsed`),
}

// emergencyMessage is displayed in the chat UI as a styled warning panel.
// IMPORTANT: This is purely informational text — no automated calls, no 911 dialing,
// no external API calls. It is a demo application.
const emergencyMessage = `⚠️ **This sounds like it may be a medical emergency.**

Please **call 911** or go to your nearest emergency room immediately.

Do not wait for an appointment — your safety is the priority.

---
*If this was a mistake, type "not an emergency" to continue scheduling.*`

// IsEmergency checks if a message contains emergency keywords.
func IsEmergency(text string) bool {
	for _, pattern := range emergencyPatterns {
		if pattern.MatchString(text) {
			return true
		}
	}
	return false
}

// EmergencySSEPayload returns the SSE-encoded emergency response payload.
// The frontend renders this as a special red/amber warning message.
func EmergencySSEPayload() []byte {
	type emergencyEvent struct {
		Text      string `json:"text"`
		Emergency bool   `json:"emergency"`
	}
	b, _ := json.Marshal(emergencyEvent{Text: emergencyMessage, Emergency: true})
	return b
}
