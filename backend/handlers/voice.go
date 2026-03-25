package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"os"

	"kyron-medical/models"
	"kyron-medical/services"
)

type VoiceHandler struct {
	sessions *services.SessionStore
}

func NewVoiceHandler(sessions *services.SessionStore) *VoiceHandler {
	return &VoiceHandler{sessions: sessions}
}

func (h *VoiceHandler) HandleInitiate(w http.ResponseWriter, r *http.Request) {
	var req models.VoiceInitiateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.SessionID == "" {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}

	sess := h.sessions.GetOrCreate(req.SessionID)

	// Generate a short summary of the chat for voice context
	summary := services.Claude.Summarize(context.Background(), sess.Messages)
	if summary != "" {
		sess.ChatSummary = summary
		h.sessions.Save(sess)
	}

	systemPrompt := services.Build(sess)

	firstName := sess.PatientInfo.FirstName
	reason := sess.PatientInfo.ReasonForVisit
	firstMessage := "Hi, I'm Kyron, the AI care coordinator for Kyron Medical. How can I help you today?"
	if firstName != "" && reason != "" {
		firstMessage = "Hi " + firstName + ", I'm Kyron continuing our conversation. I see you're here about " + reason + ". How can I help?"
	} else if firstName != "" {
		firstMessage = "Hi " + firstName + ", I'm Kyron. I have your information from our chat — how can I help?"
	}

	resp := models.VoiceInitiateResponse{
		AssistantID: os.Getenv("VAPI_ASSISTANT_ID"),
		AssistantOverrides: map[string]interface{}{
			"firstMessage": firstMessage,
			"model": map[string]interface{}{
				"provider": "anthropic",
				"model":    "claude-sonnet-4-6",
				"messages": []map[string]string{
					{"role": "system", "content": systemPrompt},
				},
			},
		},
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func (h *VoiceHandler) HandleWebhook(w http.ResponseWriter, r *http.Request) {
	var payload map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, "invalid payload", http.StatusBadRequest)
		return
	}

	// Extract caller phone number from Vapi webhook
	callerPhone := ""
	if call, ok := payload["call"].(map[string]interface{}); ok {
		if customer, ok := call["customer"].(map[string]interface{}); ok {
			callerPhone, _ = customer["number"].(string)
		}
	}

	overrides := map[string]interface{}{}

	if callerPhone != "" {
		if sess := h.sessions.GetByPhone(callerPhone); sess != nil {
			summary := services.Claude.Summarize(context.Background(), sess.Messages)
			if summary != "" {
				sess.ChatSummary = summary
				h.sessions.Save(sess)
			}

			systemPrompt := services.Build(sess)
			firstName := sess.PatientInfo.FirstName

			firstMessage := "Welcome back to Kyron Medical."
			if firstName != "" {
				firstMessage = "Hi " + firstName + ", welcome back. I remember you from our previous conversation — let me pick up right where we left off."
			}

			overrides = map[string]interface{}{
				"firstMessage": firstMessage,
				"model": map[string]interface{}{
					"provider": "anthropic",
					"model":    "claude-sonnet-4-6",
					"messages": []map[string]string{
						{"role": "system", "content": systemPrompt},
					},
				},
			}
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"assistantId":         os.Getenv("VAPI_ASSISTANT_ID"),
		"assistantOverrides":  overrides,
	})
}
