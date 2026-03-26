package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/google/uuid"

	"kyron-medical/models"
	"kyron-medical/services"
)

// voiceToolResult returns a natural-language spoken sentence that Vapi feeds
// back to the LLM as the tool's return value.
func voiceToolResult(sess *models.Session, toolName string, errs []string) string {
	// select_slot and confirm_booking use session state as the authoritative
	// success indicator — not the shared errs slice, which may contain errors
	// from other tools in the same batch.
	switch toolName {
	case "select_slot":
		if sess.SelectedSlot != nil {
			return fmt.Sprintf("Slot saved: %s at %s. NEXT: read back the full booking summary — doctor name, date, and time. Ask: \"Shall I confirm this appointment? And would you like SMS reminders?\" When patient says yes, you MUST immediately call confirm_booking with smsOptIn=true or false. Do NOT say the appointment is confirmed until confirm_booking succeeds.", sess.SelectedSlot.Date, sess.SelectedSlot.StartTime)
		}
		// Slot was not saved — surface the specific error if we have one, otherwise
		// give the agent actionable guidance on format requirements.
		if len(errs) > 0 {
			return errs[0]
		}
		return "ERROR: Could not save the time slot — the date or time may be missing or in the wrong format. " +
			"Re-confirm the exact date (use YYYY-MM-DD format, e.g. \"2026-04-01\") and time (use HH:MM 24-hour format, e.g. \"09:00\") with the patient, then call select_slot again."
	case "confirm_booking":
		if sess.Appointment != nil {
			return fmt.Sprintf("SUCCESS. Appointment confirmed. Confirmation number: %s. Tell the patient: \"Your appointment is confirmed. Your confirmation number is %s and a confirmation email has been sent to %s.\" Ask if there is anything else you can help with.", sess.Appointment.ID[:8], sess.Appointment.ID[:8], sess.Appointment.Patient.Email)
		}
		if len(errs) > 0 {
			return errs[0]
		}
		return "ERROR — booking failed. Tell the patient there was an issue saving the appointment and ask if they would like to try again. If yes, call confirm_booking again."
	}

	if len(errs) > 0 {
		return errs[0]
	}
	switch toolName {
	case "begin_intake":
		return "Intake flow started. Collect ALL of the following from the patient one by one: first name, last name, date of birth (YYYY-MM-DD), phone number, email address, and reason for visit. Once you have ALL six fields confirmed, call collect_intake."
	case "begin_prescription":
		return "Prescription flow started. Ask for: medication name (required), prescribing doctor's name, pharmacy name (required), and pharmacy phone. Once collected, call log_prescription_request."
	case "show_office_info":
		return "Done. Tell the patient: our office is open Monday to Friday, 9 AM to 5 PM. We are located at 123 Medical Drive. Then ask if there is anything else you can help with."
	case "collect_intake":
		// Build the doctor list inline so the LLM has the IDs immediately in
		// its tool result context without needing to scan back through the system prompt.
		doctorLines := ""
		for _, d := range services.Doctors {
			doctorLines += fmt.Sprintf("\n  - %s → ID: \"%s\" (%s)", d.Name, d.ID, d.Specialty)
		}
		return fmt.Sprintf(
			"Intake saved for %s %s. Reason: %q. "+
				"NEXT: pick the best matching doctor for this patient's reason and tell them the doctor's name and specialty in 1 sentence. "+
				"Ask: \"Would you like to schedule with [Dr. Name]?\" "+
				"When patient says yes, call confirm_doctor with the EXACT ID from this list:%s",
			sess.PatientInfo.FirstName, sess.PatientInfo.LastName,
			sess.PatientInfo.ReasonForVisit,
			doctorLines,
		)
	case "confirm_doctor":
		if sess.MatchedDoctor != nil {
			return fmt.Sprintf("Doctor confirmed: %s (%s). NEXT: ask the patient what day of the week and approximate time of day works best for them (e.g. \"What day works best for you — earlier in the week or later? And do you prefer mornings or afternoons?\"). Do NOT read out the full list of slots. Once they express a preference, silently find the closest available slot from your system prompt that matches, then call select_slot with date (YYYY-MM-DD) and startTime (HH:MM). IMPORTANT: You MUST call select_slot and receive a successful result BEFORE saying anything about the slot being saved. Do not say 'I have you booked' or 'locked in' until select_slot returns success.", sess.MatchedDoctor.Name, sess.MatchedDoctor.Specialty)
		}
	case "log_prescription_request":
		return "Prescription request logged. Tell the patient their request has been received and the pharmacy will be contacted. Ask if there is anything else you can help with."
	case "reject_doctor":
		return "Doctor rejected. Tell the patient you will find another specialist. Ask them to describe their symptoms again or clarify what kind of doctor they are looking for, then pick a different doctor and present them."
	case "cancel_scheduling":
		return "Scheduling cancelled. Ask the patient if they would like to choose a different doctor or restart the booking process."
	case "cancel_selection":
		return "Slot selection cancelled. Ask the patient what day and time would work better for them, then find the nearest available slot and call select_slot again."
	case "restart_booking_flow":
		return "Booking restarted. Greet the patient fresh and ask how you can help them today."
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
- NEVER say tool names or reference function names in your spoken responses.
- NEVER say "I'm calling a tool" or "Tool call".
- Tools MUST still be called — you just don't narrate them. The action does not happen unless the tool is invoked. Never skip a required tool call.
- Keep each spoken turn SHORT — 1–3 sentences maximum.
- Speak conversationally, as if on a phone call.

BOOKING FLOW — always follow this sequence, using tool results to guide each step:
1. begin_intake → collect all 6 fields → collect_intake
2. confirm_doctor (use exact doctor ID from the doctor list)
3. Ask patient for their preferred day/time — do NOT list all slots. Once they express a preference, silently pick the nearest matching available slot from your system prompt and call select_slot. CRITICAL: You MUST call select_slot and receive a successful tool result before saying anything about the slot being saved. The slot is NOT saved until select_slot returns success. IMPORTANT: date MUST be YYYY-MM-DD (e.g. "2026-04-01") and startTime MUST be HH:MM 24-hour (e.g. "09:00") — look up the exact values from the available slots in your system prompt, do not guess.
4. After select_slot succeeds: read back the booking summary (doctor, date, time) and ask "Shall I confirm this appointment?"
5. confirm_booking (smsOptIn true/false) — MUST be called before telling patient they are booked. Do NOT say "confirmed" or "all set" until confirm_booking returns SUCCESS.
Never skip a step. The appointment does not exist until confirm_booking succeeds.

`

// vapiLLMConfig returns the Vapi model override block.
// Vapi requires both "provider" and "model" — set VAPI_PROVIDER and VAPI_MODEL
// in your .env to control which model Vapi uses for voice calls, e.g.:
//   VAPI_PROVIDER=anthropic  VAPI_MODEL=claude-3-5-sonnet-20241022
//   VAPI_PROVIDER=google     VAPI_MODEL=gemini-1.5-flash
func vapiLLMConfig(systemPrompt string, history []models.ChatMessage) map[string]interface{} {
	provider := os.Getenv("VAPI_PROVIDER")
	model := os.Getenv("VAPI_MODEL")

	if provider == "" {
		provider = "openai"
	}
	if model == "" {
		model = "gpt-5.2"
	}
	log.Printf("[voice] vapi model: provider=%s model=%s", provider, model)

	// System prompt first (with voice-mode preamble), then last 20 turns of chat history as context.
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
			"role":    m.Role,
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

	msg, _ := payload["message"].(map[string]interface{})
	msgType, _ := msg["type"].(string)

	var call map[string]interface{}
	if msg != nil {
		call, _ = msg["call"].(map[string]interface{})
	}
	if call == nil {
		call, _ = payload["call"].(map[string]interface{})
	}

	callerPhone := ""
	if call != nil {
		if customer, ok := call["customer"].(map[string]interface{}); ok {
			callerPhone, _ = customer["number"].(string)
		}
	}

	if msgType == "end-of-call-report" {
		if callerPhone != "" {
			if sess := h.sessions.GetByPhone(callerPhone); sess != nil {
				endedReason, _ := call["endedReason"].(string)
				if strings.Contains(endedReason, "error") || endedReason == "silence-timeout" || endedReason == "assistant-error" {
					sess.LastCallDroppedAt = time.Now()
				} else {
					sess.LastCallDroppedAt = time.Time{}
				}

				if artifact, ok := msg["artifact"].(map[string]interface{}); ok {
					if msgs, ok := artifact["messages"].([]interface{}); ok {
						for _, mRaw := range msgs {
							m, ok := mRaw.(map[string]interface{})
							if !ok { continue }
							role, _ := m["role"].(string)
							message, _ := m["message"].(string)
							
							if (role == "user" || role == "assistant" || role == "bot") && message != "" {
								if role == "bot" { role = "assistant" }
								h.sessions.AppendMessage(sess, role, "[Voice] " + message)
							}
						}
					} else if transcript, ok := artifact["transcript"].(string); ok && transcript != "" {
						h.sessions.AppendMessage(sess, "assistant", "**Voice Call Transcript:**\n" + transcript)
					}
				}

				h.sessions.Save(sess)
			}
		}
		w.WriteHeader(http.StatusOK)
		return
	}

	overrides := map[string]interface{}{}

	if callerPhone != "" {
		sess := h.sessions.GetByPhone(callerPhone)
		if sess == nil {
			log.Printf("[voice/webhook] inbound caller %s not found — creating new session", callerPhone)
			sessionID := uuid.New().String()
			sess = h.sessions.GetOrCreate(sessionID)
			sess.PhoneNumber = callerPhone
			h.sessions.Save(sess)
		}

		summary := services.AI.Summarize(context.Background(), sess.Messages)
		if summary != "" {
			sess.ChatSummary = summary
			h.sessions.Save(sess)
		}

		systemPrompt := services.Build(sess)

		firstMsg := buildVoiceFirstMessage(sess, true)

		// Check if the previous call was dropped recently (e.g. within 30 minutes)
		if !sess.LastCallDroppedAt.IsZero() && time.Since(sess.LastCallDroppedAt) < 30*time.Minute {
			firstMsg = "Looks like we got disconnected. Want to continue exactly where we left off?"
			sess.LastCallDroppedAt = time.Time{} // Reset after handling
			h.sessions.Save(sess)
		}

		overrides = map[string]interface{}{
			"firstMessage": firstMsg,
			"model":        vapiLLMConfig(systemPrompt, sess.Messages),
			"metadata":     map[string]string{"sessionId": sess.ID},
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"assistantId":        os.Getenv("VAPI_ASSISTANT_ID"),
		"assistantOverrides": overrides,
	})
}

// HandleRegisterCall stores a Vapi call ID → session ID mapping.
// The browser SDK fires call-start with the call ID; the frontend POSTs here
// so HandleToolCall can resolve the session without relying on metadata.
func (h *VoiceHandler) HandleRegisterCall(w http.ResponseWriter, r *http.Request) {
	var req struct {
		SessionID string `json:"sessionId"`
		CallID    string `json:"callId"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.SessionID == "" || req.CallID == "" {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}
	h.sessions.RegisterCallID(req.CallID, req.SessionID)
	log.Printf("[voice] registered call %s → session %s", req.CallID, req.SessionID)
	w.WriteHeader(http.StatusNoContent)
}

// HandleToolCall handles Vapi server-side tool-call webhooks.
// Vapi POSTs here when the voice LLM invokes one of the tools defined in
// vapiToolDefinitions. We execute the tool via the shared executeToolCalls
// function, persist the session, and return a natural-language result that
// Vapi feeds back to the LLM so it can continue the conversation.
func (h *VoiceHandler) HandleToolCall(w http.ResponseWriter, r *http.Request) {
	// Read body once so we can log the raw payload for debugging.
	rawBody, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "failed to read body", http.StatusBadRequest)
		return
	}
	log.Printf("[voice/tool] raw payload: %s", string(rawBody))

	var payload map[string]interface{}
	if err := json.Unmarshal(rawBody, &payload); err != nil {
		http.Error(w, "invalid payload", http.StatusBadRequest)
		return
	}

	// Vapi wraps the event in a "message" object, but the exact structure can
	// vary. Try message.call first, then top-level call as fallback.
	msg, _ := payload["message"].(map[string]interface{})
	var call map[string]interface{}
	if msg != nil {
		call, _ = msg["call"].(map[string]interface{})
	}
	if call == nil {
		call, _ = payload["call"].(map[string]interface{})
	}
	log.Printf("[voice/tool] msg keys=%v call keys=%v", mapKeys(msg), mapKeys(call))

	// Resolve session ID — try each source in reliability order:
	// 1. Query param (URL interpolation in Vapi dashboard)
	// 2. Call ID registered by the browser SDK via POST /api/voice/register-call
	// 3. Metadata fields in the payload body
	sessionID := r.URL.Query().Get("sessionId")

	if sessionID == "" {
		if callID, _ := call["id"].(string); callID != "" {
			if sess := h.sessions.GetByCallID(callID); sess != nil {
				sessionID = sess.ID
				log.Printf("[voice/tool] resolved session %s via call ID %s", sessionID, callID)
			}
		}
	}
	if sessionID == "" {
		for _, src := range []map[string]interface{}{
			nestedMap(call, "metadata"),
			nestedMap(msg, "metadata"),
			nestedMap(payload, "metadata"),
		} {
			if id, _ := src["sessionId"].(string); id != "" {
				sessionID = id
				break
			}
		}
	}

	// Phone-number fallback for inbound calls that never hit the webhook.
	if sessionID == "" && call != nil {
		if customer, ok := call["customer"].(map[string]interface{}); ok {
			if phone, _ := customer["number"].(string); phone != "" {
				if sess := h.sessions.GetByPhone(phone); sess != nil {
					sessionID = sess.ID
					log.Printf("[voice/tool] resolved session %s via phone %s", sessionID, phone)
				} else {
					log.Printf("[voice/tool] inbound caller %s not found — creating new session", phone)
					sessionID = uuid.New().String()
					sessTemp := h.sessions.GetOrCreate(sessionID)
					sessTemp.PhoneNumber = phone
					h.sessions.Save(sessTemp)
				}
			}
		}
	}

	if sessionID == "" {
		log.Printf("[voice/tool] cannot identify session — payload keys: %v", mapKeys(payload))
		http.Error(w, "cannot identify session", http.StatusBadRequest)
		return
	}

	sess := h.sessions.GetOrCreate(sessionID)

	// Parse the tool call list. Vapi uses "toolCallList" in the message body.
	var rawList []interface{}
	if msg != nil {
		rawList, _ = msg["toolCallList"].([]interface{})
	}
	if len(rawList) == 0 {
		rawList, _ = payload["toolCallList"].([]interface{})
	}
	log.Printf("[voice/tool] session=%s tool_calls=%d", sessionID, len(rawList))

	calls := make([]services.ToolCallResult, 0, len(rawList))
	ids := make([]string, 0, len(rawList))
	for _, raw := range rawList {
		tc, _ := raw.(map[string]interface{})
		ids = append(ids, func() string { s, _ := tc["id"].(string); return s }())
		fn, _ := tc["function"].(map[string]interface{})
		name, _ := fn["name"].(string)
		argsStr, isStr := fn["arguments"].(string)
		var input map[string]interface{}
		if isStr && argsStr != "" {
			json.Unmarshal([]byte(argsStr), &input)
		} else if argsMap, isMap := fn["arguments"].(map[string]interface{}); isMap {
			input = argsMap
		}
		// Voice-specific fix: confirm_doctor often arrives with empty doctorId because
		// the LLM omits the field even when an enum is configured. Resolve it here
		// before it reaches executeToolCalls so the shared handler always gets a valid ID.
		if name == "confirm_doctor" {
			input = voiceResolveDoctorID(input, sess, sessionID)
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

	// Inject updated system prompt into the active call so the LLM always has
	// the correct state context — this fixes the "fixed system prompt" problem.
	if callID, _ := call["id"].(string); callID != "" {
		go injectSystemMessage(callID, services.Build(sess))
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"results": results})
}

// injectSystemMessage pushes an updated system prompt into an active Vapi call.
// This keeps the LLM's context in sync as session state changes mid-call —
// e.g. after collect_intake the LLM gets the MATCHING state instructions,
// after confirm_doctor it gets SCHEDULING instructions with available slots.
func injectSystemMessage(callID, systemPrompt string) {
	vapiKey := os.Getenv("VAPI_PRIVATE_KEY")
	if vapiKey == "" {
		return
	}
	body, _ := json.Marshal(map[string]interface{}{
		"role":    "system",
		"content": voicePreamble + systemPrompt,
	})
	url := "https://api.vapi.ai/call/" + callID + "/message"
	req, err := http.NewRequestWithContext(context.Background(), "POST", url, bytes.NewReader(body))
	if err != nil {
		log.Printf("[voice/inject] failed to build request: %v", err)
		return
	}
	req.Header.Set("Authorization", "Bearer "+vapiKey)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		log.Printf("[voice/inject] call=%s error: %v", callID, err)
		return
	}
	defer resp.Body.Close()
	log.Printf("[voice/inject] call=%s status=%d", callID, resp.StatusCode)
}

// mapKeys returns the keys of a map for logging; nil-safe.
func mapKeys(m map[string]interface{}) []string {
	if m == nil {
		return nil
	}
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

// nestedMap returns m[key] as a map; returns an empty map (never nil) so
// callers can safely access fields without a nil check.
func nestedMap(m map[string]interface{}, key string) map[string]interface{} {
	if m == nil {
		return map[string]interface{}{}
	}
	v, _ := m[key].(map[string]interface{})
	if v == nil {
		return map[string]interface{}{}
	}
	return v
}

// voiceResolveDoctorID ensures the confirm_doctor tool input always has a valid doctorId.
// The voice LLM frequently omits or empties this field even when Vapi enums are configured.
// Resolution order:
//  1. doctorId already valid — return as-is
//  2. doctorName field provided (if added to the Vapi tool schema) — last-name match
//  3. Keyword match against patient's reason for visit
func voiceResolveDoctorID(input map[string]interface{}, sess *models.Session, sessionID string) map[string]interface{} {
	if input == nil {
		input = map[string]interface{}{}
	}

	doctorID, _ := input["doctorId"].(string)
	if services.GetDoctorByID(doctorID) != nil {
		return input // already valid
	}

	// Try doctorName if the Vapi tool passes it
	if name, _ := input["doctorName"].(string); name != "" {
		if doc := services.GetDoctorByID(name); doc != nil {
			log.Printf("[voice/tool] session=%s confirm_doctor: resolved %q via doctorName → %s", sessionID, name, doc.ID)
			input["doctorId"] = doc.ID
			return input
		}
	}

	// Keyword match from reason for visit — covers the common case where the LLM
	// presented the right doctor by name but sent an empty doctorId.
	reason := sess.PatientInfo.ReasonForVisit
	if reason == "" {
		return input
	}
	lower := strings.ToLower(reason)
	best, bestCount := -1, 0
	for i, d := range services.Doctors {
		count := 0
		for _, kw := range d.Keywords {
			if strings.Contains(lower, strings.ToLower(kw)) {
				count++
			}
		}
		if count > bestCount {
			bestCount = count
			best = i
		}
	}
	if best >= 0 && bestCount > 0 {
		doc := &services.Doctors[best]
		log.Printf("[voice/tool] session=%s confirm_doctor: doctorId=%q unresolved — inferred %s from reason %q", sessionID, doctorID, doc.ID, reason)
		input["doctorId"] = doc.ID
	}
	return input
}
