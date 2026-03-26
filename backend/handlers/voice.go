package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
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

// vapiLLMConfig returns the Vapi model block for the active AI provider,
// including conversation history so the voice agent picks up mid-conversation.
func vapiLLMConfig(systemPrompt string, history []models.ChatMessage) map[string]interface{} {
	provider, model := "anthropic", services.ActiveModel()
	if os.Getenv("AI_PROVIDER") == "gemini" {
		provider, model = "google", services.ActiveModel()
	}

	// System prompt first, then last 20 turns of chat history as context
	msgs := []map[string]string{
		{"role": "system", "content": systemPrompt},
	}
	start := 0
	if len(history) > 20 {
		start = len(history) - 20
	}
	for _, m := range history[start:] {
		if m.Content == "" {
			continue
		}
		msgs = append(msgs, map[string]string{
			"role":    m.Role, // "user" | "assistant" — Vapi normalises for each provider
			"content": m.Content,
		})
	}

	return map[string]interface{}{
		"provider": provider,
		"model":    model,
		"messages": msgs,
	}
}

// buildVoiceFirstMessage returns a short, natural spoken opener for Vapi.
// It should be one sentence — the LLM already has the full system prompt and
// conversation history via vapiLLMConfig and will handle the rest naturally.
func buildVoiceFirstMessage(sess *models.Session, isPhone bool) string {
	firstName := sess.PatientInfo.FirstName

	if firstName == "" {
		if isPhone {
			return "Hi, this is Kyron calling from Kyron Medical. How can I help you today?"
		}
		return "Hi, I'm Kyron, the AI care coordinator for Kyron Medical. How can I help you today?"
	}

	id := "I'm Kyron"
	if isPhone {
		id = "this is Kyron from Kyron Medical"
	}

	switch sess.State {
	case models.StateIntake:
		return fmt.Sprintf("Hi %s, %s — shall we carry on getting your details?", firstName, id)
	case models.StateMatching:
		if r := sess.PatientInfo.ReasonForVisit; r != "" {
			return fmt.Sprintf("Hi %s, %s — I'm still working on finding the right specialist for your %s. Ready to continue?", firstName, id, r)
		}
		return fmt.Sprintf("Hi %s, %s — shall we carry on finding you the right specialist?", firstName, id)
	case models.StateScheduling:
		if sess.MatchedDoctor != nil {
			return fmt.Sprintf("Hi %s, %s — shall we finish scheduling your appointment with %s?", firstName, id, sess.MatchedDoctor.Name)
		}
	case models.StateConfirming:
		if sess.MatchedDoctor != nil && sess.SelectedSlot != nil {
			return fmt.Sprintf("Hi %s, %s — ready to confirm your appointment with %s on %s?",
				firstName, id, sess.MatchedDoctor.Name, services.FormatDateReadable(sess.SelectedSlot.Date))
		}
	case models.StateBooked:
		return fmt.Sprintf("Hi %s, %s — your appointment is all set! Is there anything else I can help with?", firstName, id)
	case models.StatePrescription:
		return fmt.Sprintf("Hi %s, %s — shall we carry on with your prescription refill?", firstName, id)
	case models.StateHours:
		return fmt.Sprintf("Hi %s, %s — how can I help you today?", firstName, id)
	}

	return fmt.Sprintf("Hi %s, %s — great to connect. How can I help you today?", firstName, id)
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

	firstMessage := buildVoiceFirstMessage(sess, false)

	resp := models.VoiceInitiateResponse{
		AssistantID: os.Getenv("VAPI_ASSISTANT_ID"),
		AssistantOverrides: map[string]interface{}{
			"firstMessage": firstMessage,
			"model":        vapiLLMConfig(systemPrompt, sess.Messages),
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

	firstMessage := buildVoiceFirstMessage(sess, true)

	payload := map[string]interface{}{
		"assistantId": assistantID,
		"assistantOverrides": map[string]interface{}{
			"firstMessage": firstMessage,
			"model":        vapiLLMConfig(systemPrompt, sess.Messages),
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

			overrides = map[string]interface{}{
				"firstMessage": buildVoiceFirstMessage(sess, true),
				"model":        vapiLLMConfig(systemPrompt, sess.Messages),
			}
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"assistantId":        os.Getenv("VAPI_ASSISTANT_ID"),
		"assistantOverrides": overrides,
	})
}
