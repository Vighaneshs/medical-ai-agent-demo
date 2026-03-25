package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"time"

	"kyron-medical/models"
	"kyron-medical/services"
)

type VoiceHandler struct {
	sessions *services.SessionStore
}

func NewVoiceHandler(sessions *services.SessionStore) *VoiceHandler {
	return &VoiceHandler{sessions: sessions}
}

// vapiLLMConfig returns the Vapi model block for the active AI provider.
func vapiLLMConfig(systemPrompt string) map[string]interface{} {
	provider, model := "anthropic", services.ActiveModel()
	if os.Getenv("AI_PROVIDER") == "gemini" {
		provider, model = "google", services.ActiveModel()
	}
	return map[string]interface{}{
		"provider": provider,
		"model":    model,
		"messages": []map[string]string{
			{"role": "system", "content": systemPrompt},
		},
	}
}

func (h *VoiceHandler) HandleInitiate(w http.ResponseWriter, r *http.Request) {
	var req models.VoiceInitiateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.SessionID == "" {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}

	sess := h.sessions.GetOrCreate(req.SessionID)

	summary := services.AI.Summarize(context.Background(), sess.Messages)
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
			"model":        vapiLLMConfig(systemPrompt),
		},
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

// HandleCallPhone initiates an outbound phone call to the patient via Vapi's REST API.
func (h *VoiceHandler) HandleCallPhone(w http.ResponseWriter, r *http.Request) {
	var req struct {
		SessionID string `json:"sessionId"`
		Phone     string `json:"phone"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.SessionID == "" || req.Phone == "" {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}

	vapiKey := os.Getenv("VAPI_PRIVATE_KEY")
	phoneNumberID := os.Getenv("VAPI_PHONE_NUMBER_ID")
	assistantID := os.Getenv("VAPI_ASSISTANT_ID")
	if vapiKey == "" || phoneNumberID == "" || assistantID == "" {
		http.Error(w, "phone call not configured — set VAPI_PRIVATE_KEY and VAPI_PHONE_NUMBER_ID", http.StatusServiceUnavailable)
		return
	}

	sess := h.sessions.GetOrCreate(req.SessionID)
	summary := services.AI.Summarize(context.Background(), sess.Messages)
	if summary != "" {
		sess.ChatSummary = summary
		h.sessions.Save(sess)
	}
	systemPrompt := services.Build(sess)

	firstName := sess.PatientInfo.FirstName
	firstMessage := "Hi, this is Kyron calling from Kyron Medical. How can I help you today?"
	if firstName != "" {
		firstMessage = "Hi " + firstName + ", this is Kyron from Kyron Medical calling to continue our conversation. How can I help?"
	}

	payload := map[string]interface{}{
		"assistantId": assistantID,
		"assistantOverrides": map[string]interface{}{
			"firstMessage": firstMessage,
			"model":        vapiLLMConfig(systemPrompt),
		},
		"customer":      map[string]interface{}{"number": req.Phone},
		"phoneNumberId": phoneNumberID,
	}

	body, _ := json.Marshal(payload)
	httpReq, err := http.NewRequestWithContext(r.Context(), "POST", "https://api.vapi.ai/call/phone", bytes.NewReader(body))
	if err != nil {
		http.Error(w, "failed to build request", http.StatusInternalServerError)
		return
	}
	httpReq.Header.Set("Authorization", "Bearer "+vapiKey)
	httpReq.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(httpReq)
	if err != nil {
		log.Printf("[voice] call-phone http error: %v", err)
		http.Error(w, "failed to place call", http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		var vapiErr map[string]interface{}
		json.NewDecoder(resp.Body).Decode(&vapiErr)
		log.Printf("[voice] call-phone vapi error: status=%d body=%v", resp.StatusCode, vapiErr)
		http.Error(w, "vapi rejected the call request", http.StatusBadGateway)
		return
	}

	log.Printf("[voice] call-phone initiated: session=%s phone=%s", req.SessionID, req.Phone)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "calling"})
}

func (h *VoiceHandler) HandleWebhook(w http.ResponseWriter, r *http.Request) {
	var payload map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, "invalid payload", http.StatusBadRequest)
		return
	}

	callerPhone := ""
	if call, ok := payload["call"].(map[string]interface{}); ok {
		if customer, ok := call["customer"].(map[string]interface{}); ok {
			callerPhone, _ = customer["number"].(string)
		}
	}

	overrides := map[string]interface{}{}

	if callerPhone != "" {
		if sess := h.sessions.GetByPhone(callerPhone); sess != nil {
			summary := services.AI.Summarize(context.Background(), sess.Messages)
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
				"model":        vapiLLMConfig(systemPrompt),
			}
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"assistantId":        os.Getenv("VAPI_ASSISTANT_ID"),
		"assistantOverrides": overrides,
	})
}
