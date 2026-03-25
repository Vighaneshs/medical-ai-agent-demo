package handlers

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/google/uuid"
	"kyron-medical/models"
	"kyron-medical/services"
)

type ChatHandler struct {
	sessions *services.SessionStore
}

func NewChatHandler(sessions *services.SessionStore) *ChatHandler {
	return &ChatHandler{sessions: sessions}
}

func (h *ChatHandler) HandleChat(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming not supported", http.StatusInternalServerError)
		return
	}

	var req models.ChatRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.SessionID == "" || req.Message == "" {
		log.Printf("[chat] bad request: err=%v sessionID=%q message=%q", err, req.SessionID, req.Message)
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}

	log.Printf("[chat] session=%s state=%s message=%q", req.SessionID, "", req.Message)

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")

	sess := h.sessions.GetOrCreate(req.SessionID)
	log.Printf("[chat] session=%s state=%s history_len=%d", sess.ID, sess.State, len(sess.Messages))

	if services.IsEmergency(req.Message) {
		log.Printf("[chat] session=%s EMERGENCY detected", sess.ID)
		payload := services.EmergencySSEPayload()
		fmt.Fprintf(w, "data: %s\n\n", payload)
		flusher.Flush()
		return
	}

	h.sessions.AppendMessage(sess, "user", req.Message)

	systemPrompt := services.Build(sess)
	log.Printf("[chat] session=%s system_prompt_len=%d preview=%q", sess.ID, len(systemPrompt), truncateStr(systemPrompt, 200))
	ctx := r.Context()

	textChunks := make(chan string, 100)
	toolResults := make(chan []services.ToolCallResult, 1)

	log.Printf("[chat] session=%s calling AI.Stream with %d messages", sess.ID, len(sess.Messages))
	go services.AI.Stream(ctx, systemPrompt, sess.Messages, textChunks, toolResults)

	var assistantText string
	chunkCount := 0
	for chunk := range textChunks {
		if ctx.Err() != nil {
			log.Printf("[chat] session=%s context cancelled during stream", sess.ID)
			return
		}
		chunkCount++
		assistantText += chunk
		data, _ := json.Marshal(models.SSEChunk{Text: chunk})
		fmt.Fprintf(w, "data: %s\n\n", data)
		flusher.Flush()
	}
	log.Printf("[chat] session=%s stream done: %d chunks, text_len=%d response=%q",
		sess.ID, chunkCount, len(assistantText), truncateStr(assistantText, 300))

	calls := <-toolResults
	log.Printf("[chat] session=%s tool_calls=%d", sess.ID, len(calls))
	for _, c := range calls {
		log.Printf("[chat] session=%s   tool=%s input=%v", sess.ID, c.ToolName, c.Input)
	}

	newState := h.executeToolCalls(sess, calls, r)
	log.Printf("[chat] session=%s new_state=%s", sess.ID, newState)

	// Gemini often makes tool-only turns (no text). Loop up to 3 times until we
	// get visible text or run out of tool-only continuations to follow.
	for attempt := 1; attempt <= 3 && assistantText == "" && len(calls) > 0; attempt++ {
		if ctx.Err() != nil {
			return
		}
		log.Printf("[chat] session=%s auto-continue attempt=%d state=%s", sess.ID, attempt, sess.State)
		sp := services.Build(sess)
		tc := make(chan string, 100)
		tr := make(chan []services.ToolCallResult, 1)
		go services.AI.Stream(ctx, sp, sess.Messages, tc, tr)
		for chunk := range tc {
			if ctx.Err() != nil {
				return
			}
			assistantText += chunk
			data, _ := json.Marshal(models.SSEChunk{Text: chunk})
			fmt.Fprintf(w, "data: %s\n\n", data)
			flusher.Flush()
		}
		calls = <-tr
		if len(calls) > 0 {
			log.Printf("[chat] session=%s continuation attempt=%d produced %d tool calls", sess.ID, attempt, len(calls))
			newState = h.executeToolCalls(sess, calls, r)
		}
		log.Printf("[chat] session=%s continuation attempt=%d done: text_len=%d new_state=%s", sess.ID, attempt, len(assistantText), newState)
	}

	if assistantText != "" {
		h.sessions.AppendMessage(sess, "assistant", assistantText)
	}
	h.sessions.Save(sess)

	doneEvent := models.SSEDone{Done: true, NewState: string(newState)}
	if sess.MatchedDoctor != nil {
		doneEvent.DoctorID = sess.MatchedDoctor.ID
	}
	if sess.SelectedSlot != nil {
		doneEvent.SelectedSlot = sess.SelectedSlot
	}
	if sess.Appointment != nil {
		doneEvent.AppointmentID = sess.Appointment.ID
	}

	done, _ := json.Marshal(doneEvent)
	fmt.Fprintf(w, "data: %s\n\n", done)
	flusher.Flush()
}

func (h *ChatHandler) executeToolCalls(sess *models.Session, calls []services.ToolCallResult, r *http.Request) models.SessionState {
	for _, call := range calls {
		switch call.ToolName {
		case "begin_intake":
			if sess.State == models.StateGreeting || sess.State == models.StateBooked ||
				sess.State == models.StatePrescription || sess.State == models.StateHours {
				sess.State = models.StateIntake
			} else {
				log.Printf("[chat] session=%s WARN begin_intake: ignored in state=%s", sess.ID, sess.State)
			}

		case "begin_prescription":
			sess.State = models.StatePrescription

		case "show_office_info":
			sess.State = models.StateHours

		case "collect_intake":
			if sess.State != models.StateIntake {
				log.Printf("[chat] session=%s WARN collect_intake: ignored in state=%s (expected INTAKE)", sess.ID, sess.State)
				break
			}
			sess.PatientInfo = models.PatientInfo{
				FirstName:      strField(call.Input, "firstName"),
				LastName:       strField(call.Input, "lastName"),
				DOB:            strField(call.Input, "dob"),
				Phone:          strField(call.Input, "phone"),
				Email:          strField(call.Input, "email"),
				ReasonForVisit: strField(call.Input, "reasonForVisit"),
			}
			sess.PhoneNumber = sess.PatientInfo.Phone
			sess.State = models.StateMatching

		case "confirm_doctor":
			if sess.State != models.StateMatching {
				log.Printf("[chat] session=%s WARN confirm_doctor: ignored in state=%s (expected MATCHING)", sess.ID, sess.State)
				break
			}
			doctorID := strField(call.Input, "doctorId")
			if doc := services.GetDoctorByID(doctorID); doc != nil {
				sess.MatchedDoctor = doc
				sess.State = models.StateScheduling
			} else {
				log.Printf("[chat] session=%s WARN confirm_doctor: unknown doctorId=%q — state stays %s (valid IDs: %v)",
					sess.ID, doctorID, sess.State, services.DoctorIDs())
			}

		case "select_slot":
			if sess.State != models.StateScheduling {
				log.Printf("[chat] session=%s WARN select_slot: ignored in state=%s (expected SCHEDULING)", sess.ID, sess.State)
				break
			}
			date := strField(call.Input, "date")
			startTime := strField(call.Input, "startTime")
			if date != "" && startTime != "" && sess.MatchedDoctor != nil && services.IsSlotBooked(sess.MatchedDoctor.ID, date, startTime) {
				log.Printf("[chat] session=%s WARN select_slot: slot already booked doctor=%s date=%s time=%s",
					sess.ID, sess.MatchedDoctor.ID, date, startTime)
			}
			if date != "" && startTime != "" && sess.MatchedDoctor != nil && !services.IsSlotBooked(sess.MatchedDoctor.ID, date, startTime) {
				endTime := services.GenerateAvailability(sess.MatchedDoctor.ID)
				var endT string
				for _, s := range endTime {
					if s.Date == date && s.StartTime == startTime {
						endT = s.EndTime
						break
					}
				}
				sess.SelectedSlot = &models.TimeSlot{
					DoctorID:  sess.MatchedDoctor.ID,
					Date:      date,
					StartTime: startTime,
					EndTime:   endT,
					Available: true,
				}
				sess.State = models.StateConfirming
			}

		case "confirm_booking":
			log.Printf("[chat] session=%s confirm_booking: state=%s matchedDoctor=%v selectedSlot=%v",
				sess.ID, sess.State, sess.MatchedDoctor != nil, sess.SelectedSlot != nil)
			if sess.State != models.StateConfirming {
				log.Printf("[chat] session=%s WARN confirm_booking: ignored in state=%s (expected CONFIRMING)", sess.ID, sess.State)
				break
			}
			if sess.SelectedSlot == nil || sess.MatchedDoctor == nil {
				log.Printf("[chat] session=%s WARN confirm_booking: nil pointer — doctor=%v slot=%v — cannot book",
					sess.ID, sess.MatchedDoctor, sess.SelectedSlot)
				break
			}
			smsOptIn := boolField(call.Input, "smsOptIn")
			sess.PatientInfo.SMSOptIn = smsOptIn

			apptID := uuid.New().String()
			appt := &models.Appointment{
				ID:        apptID,
				SessionID: sess.ID,
				Doctor:    *sess.MatchedDoctor,
				Slot:      *sess.SelectedSlot,
				Patient:   sess.PatientInfo,
				BookedAt:  time.Now(),
			}
			sess.Appointment = appt
			sess.State = models.StateBooked

			services.BookSlot(sess.SelectedSlot.DoctorID, sess.SelectedSlot.Date, sess.SelectedSlot.StartTime)

			go services.SendConfirmationEmail(appt)
			if smsOptIn {
				go services.SendConfirmationSMS(appt)
			}
			go services.ScheduleReminder(appt)
			log.Printf("[chat] session=%s confirm_booking: BOOKED appt=%s smsOptIn=%v", sess.ID, apptID, smsOptIn)

		case "log_prescription_request":
			sess.State = models.StatePrescription
		}
	}
	return sess.State
}

func truncateStr(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}

func strField(m map[string]interface{}, key string) string {
	if v, ok := m[key]; ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

func boolField(m map[string]interface{}, key string) bool {
	if v, ok := m[key]; ok {
		if b, ok := v.(bool); ok {
			return b
		}
	}
	return false
}
