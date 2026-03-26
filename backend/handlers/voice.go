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

	"github.com/google/uuid"

	"kyron-medical/models"
	"kyron-medical/services"
)

// vapiToolDefinitions returns a Vapi-compatible tool list for all 8 session
// state tools. Each tool has a server URL so Vapi POSTs calls to our handler.
func vapiToolDefinitions(toolURL string) []map[string]interface{} {
	server := map[string]interface{}{"url": toolURL, "timeoutSeconds": 20}

	defs := []struct {
		name        string
		description string
		properties  map[string]interface{}
		required    []interface{}
	}{
		{
			name:        "begin_intake",
			description: "Start the appointment scheduling intake flow when the patient wants to book",
			properties:  map[string]interface{}{},
			required:    []interface{}{},
		},
		{
			name:        "begin_prescription",
			description: "Start a prescription refill inquiry flow",
			properties:  map[string]interface{}{},
			required:    []interface{}{},
		},
		{
			name:        "show_office_info",
			description: "Switch to office hours and location information flow",
			properties:  map[string]interface{}{},
			required:    []interface{}{},
		},
		{
			name:        "collect_intake",
			description: "Save patient intake information once all 6 fields are confirmed",
			properties: map[string]interface{}{
				"firstName":      map[string]string{"type": "string"},
				"lastName":       map[string]string{"type": "string"},
				"dob":            map[string]interface{}{"type": "string", "description": "YYYY-MM-DD"},
				"phone":          map[string]string{"type": "string"},
				"email":          map[string]string{"type": "string"},
				"reasonForVisit": map[string]string{"type": "string"},
			},
			required: []interface{}{"firstName", "lastName", "dob", "phone", "email", "reasonForVisit"},
		},
		{
			name:        "confirm_doctor",
			description: "Confirm the matched doctor after the patient agrees",
			properties:  map[string]interface{}{"doctorId": map[string]string{"type": "string"}},
			required:    []interface{}{"doctorId"},
		},
		{
			name:        "select_slot",
			description: "Record the patient's chosen appointment slot",
			properties: map[string]interface{}{
				"date":      map[string]interface{}{"type": "string", "description": "YYYY-MM-DD"},
				"startTime": map[string]interface{}{"type": "string", "description": "HH:MM"},
			},
			required: []interface{}{"date", "startTime"},
		},
		{
			name:        "confirm_booking",
			description: "Finalize the appointment booking",
			properties:  map[string]interface{}{"smsOptIn": map[string]string{"type": "boolean"}},
			required:    []interface{}{"smsOptIn"},
		},
		{
			name:        "log_prescription_request",
			description: "Log a prescription refill request",
			properties: map[string]interface{}{
				"medication":     map[string]string{"type": "string"},
				"prescriberName": map[string]string{"type": "string"},
				"pharmacyName":   map[string]string{"type": "string"},
				"pharmacyPhone":  map[string]string{"type": "string"},
			},
			required: []interface{}{"medication", "pharmacyName"},
		},
		{
			name:        "reject_doctor",
			description: "Call this when the patient explicitly rejects the matched doctor",
			properties:  map[string]interface{}{},
			required:    []interface{}{},
		},
		{
			name:        "cancel_scheduling",
			description: "Call this when the patient rejects all time slots or wants a different doctor",
			properties:  map[string]interface{}{},
			required:    []interface{}{},
		},
		{
			name:        "cancel_selection",
			description: "Call this when the patient rejects the final appointment summary and wants a different time",
			properties:  map[string]interface{}{},
			required:    []interface{}{},
		},
		{
			name:        "restart_booking_flow",
			description: "Call this if the patient wants to cancel everything and start completely from scratch",
			properties:  map[string]interface{}{},
			required:    []interface{}{},
		},
	}

	result := make([]map[string]interface{}, len(defs))
	for i, d := range defs {
		result[i] = map[string]interface{}{
			"type": "function",
			"function": map[string]interface{}{
				"name":        d.name,
				"description": d.description,
				"parameters": map[string]interface{}{
					"type":       "object",
					"properties": d.properties,
					"required":   d.required,
				},
			},
			"server": server,
		}
	}
	return result
}

// voiceToolResult returns a natural-language spoken sentence that Vapi feeds
// back to the LLM as the tool's return value.
func voiceToolResult(sess *models.Session, toolName string, errs []string) string {
	if len(errs) > 0 {
		return errs[0]
	}
	switch toolName {
	case "begin_intake":
		return "Done. Ask the patient for their first name, last name, date of birth, phone, email, and reason for visit."
	case "begin_prescription":
		return "Done. Ask the patient for their medication name and pharmacy details."
	case "show_office_info":
		return "Done. Provide the office hours and location to the patient."
	case "collect_intake":
		return fmt.Sprintf("Patient details saved for %s %s. Now find the best specialist for their concern.", sess.PatientInfo.FirstName, sess.PatientInfo.LastName)
	case "confirm_doctor":
		if sess.MatchedDoctor != nil {
			return fmt.Sprintf("Doctor confirmed: %s, %s. Present available time slots to the patient.", sess.MatchedDoctor.Name, sess.MatchedDoctor.Specialty)
		}
	case "select_slot":
		if sess.SelectedSlot != nil {
			return fmt.Sprintf("Slot saved: %s at %s. Ask the patient to confirm the booking and whether they want an SMS reminder.", sess.SelectedSlot.Date, sess.SelectedSlot.StartTime)
		}
	case "confirm_booking":
		if sess.Appointment != nil {
			return fmt.Sprintf("SUCCESS — appointment is now saved in our system. Confirmation number: %s. Tell the patient exactly this: their appointment is confirmed, their confirmation number is %s, and a confirmation email has been sent to %s. Then ask if there is anything else you can help with.", sess.Appointment.ID[:8], sess.Appointment.ID[:8], sess.Appointment.Patient.Email)
		}
		return "ERROR — booking failed, appointment was not saved. Apologise to the patient and offer to try again."
	case "log_prescription_request":
		return "Prescription request logged. Let the patient know their request has been received."
	case "reject_doctor":
		return "Doctor rejected. Ask the patient for different symptoms so we can find another specialist."
	case "cancel_scheduling":
		return "Scheduling cancelled. Ask the patient if they want to choose a different doctor."
	case "cancel_selection":
		return "Selection cancelled. Read the previous available time slots to the patient again."
	case "restart_booking_flow":
		return "Booking flow restarted. Ask the patient for their symptoms again."
	}
	return "Done."
}

type VoiceHandler struct {
	sessions *services.SessionStore
}

func NewVoiceHandler(sessions *services.SessionStore) *VoiceHandler {
	return &VoiceHandler{sessions: sessions}
}

// voicePreamble is prepended to every system prompt sent to Vapi.
// It overrides the chat-oriented tool-mention instructions so Claude never
// narrates tool names or "Tool call" phrases aloud through the TTS engine.
const voicePreamble = `VOICE MODE — YOU ARE SPEAKING ALOUD:
- NEVER say tool names (begin_intake, collect_intake, confirm_doctor, select_slot, confirm_booking, etc.) in your spoken responses.
- NEVER say "I'm calling a tool", "Tool call", or reference function names in any way.
- Tools MUST still be called — you just don't narrate them. The action does not happen unless the tool is invoked. Never skip a required tool call.
- Keep each spoken turn SHORT — 1–3 sentences maximum. Long responses are hard to listen to.
- Speak conversationally, as if on a phone call.

`

// vapiLLMConfig returns the Vapi model block for the active AI provider,
// including conversation history so the voice agent picks up mid-conversation.
func vapiLLMConfig(systemPrompt string, history []models.ChatMessage) map[string]interface{} {
	provider, model := "anthropic", services.ActiveModel()
	if os.Getenv("AI_PROVIDER") == "gemini" {
		provider, model = "google", services.ActiveModel()
	}

	// System prompt first (with voice-mode preamble), then last 20 turns of chat history as context
	msgs := []map[string]string{
		{"role": "system", "content": voicePreamble + systemPrompt},
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

	overrides := map[string]interface{}{
		"firstMessage": firstMessage,
		"model":        vapiLLMConfig(systemPrompt, sess.Messages),
		"metadata":     map[string]string{"sessionId": sess.ID},
	}
	if bu := os.Getenv("BACKEND_URL"); bu != "" {
		overrides["tools"] = vapiToolDefinitions(bu + "/api/voice/tool")
	} else {
		log.Printf("[voice] BACKEND_URL not set — tool webhooks disabled for this call")
	}
	resp := models.VoiceInitiateResponse{
		AssistantID:        os.Getenv("VAPI_ASSISTANT_ID"),
		AssistantOverrides: overrides,
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

	phoneOverrides := map[string]interface{}{
		"firstMessage": firstMessage,
		"model":        vapiLLMConfig(systemPrompt, sess.Messages),
	}
	if bu := os.Getenv("BACKEND_URL"); bu != "" {
		phoneOverrides["tools"] = vapiToolDefinitions(bu + "/api/voice/tool")
	}
	payload := map[string]interface{}{
		"assistantId":        assistantID,
		"assistantOverrides": phoneOverrides,
		"metadata":           map[string]string{"sessionId": req.SessionID},
		"customer":           map[string]interface{}{"number": req.Phone},
		"phoneNumberId":      phoneNumberID,
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
				"metadata":     map[string]string{"sessionId": sess.ID},
			}
			if bu := os.Getenv("BACKEND_URL"); bu != "" {
				overrides["tools"] = vapiToolDefinitions(bu + "/api/voice/tool")
			}
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"assistantId":        os.Getenv("VAPI_ASSISTANT_ID"),
		"assistantOverrides": overrides,
	})
}

// HandleToolCall handles Vapi server-side tool-call webhooks.
// Vapi POSTs here when the voice LLM invokes one of the tools defined in
// vapiToolDefinitions. We execute the tool via the shared executeToolCalls
// function, persist the session, and return a natural-language result that
// Vapi feeds back to the LLM so it can continue the conversation.
func (h *VoiceHandler) HandleToolCall(w http.ResponseWriter, r *http.Request) {
	var payload map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, "invalid payload", http.StatusBadRequest)
		return
	}

	msg, _ := payload["message"].(map[string]interface{})
	call, _ := msg["call"].(map[string]interface{})

	// Resolve session: prefer metadata.sessionId, fall back to caller phone.
	sessionID := ""
	if meta, ok := call["metadata"].(map[string]interface{}); ok {
		sessionID, _ = meta["sessionId"].(string)
	}
	if sessionID == "" {
		if customer, ok := call["customer"].(map[string]interface{}); ok {
			phone, _ := customer["number"].(string)
			if phone != "" {
				log.Printf("[voice/tool] inbound caller %s not found in memory, creating new session", phone)
				sessionID = uuid.New().String()
				sessTemp := h.sessions.GetOrCreate(sessionID)
				sessTemp.PhoneNumber = phone
				h.sessions.Save(sessTemp)
			}
		}
	}
	if sessionID == "" {
		log.Printf("[voice/tool] cannot identify session — no metadata.sessionId or phone match")
		http.Error(w, "cannot identify session", http.StatusBadRequest)
		return
	}

	sess := h.sessions.GetOrCreate(sessionID)

	// Parse the tool call list.
	rawList, _ := msg["toolCallList"].([]interface{})
	calls := make([]services.ToolCallResult, 0, len(rawList))
	ids := make([]string, 0, len(rawList))
	for _, raw := range rawList {
		tc, _ := raw.(map[string]interface{})
		ids = append(ids, func() string { s, _ := tc["id"].(string); return s }())
		fn, _ := tc["function"].(map[string]interface{})
		name, _ := fn["name"].(string)
		argsStr, _ := fn["arguments"].(string)
		var input map[string]interface{}
		if argsStr != "" {
			json.Unmarshal([]byte(argsStr), &input)
		}
		calls = append(calls, services.ToolCallResult{ToolName: name, Input: input})
		log.Printf("[voice/tool] session=%s tool=%s args=%s", sessionID, name, argsStr)
	}

	_, errs := executeToolCalls(sess, calls)
	h.sessions.Save(sess)

	results := make([]map[string]interface{}, len(calls))
	for i, c := range calls {
		result := voiceToolResult(sess, c.ToolName, errs)
		results[i] = map[string]interface{}{
			"toolCallId": ids[i],
			"result":     result,
		}
		log.Printf("[voice/tool] session=%s tool=%s result=%q", sessionID, c.ToolName, result)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"results": results})
}
