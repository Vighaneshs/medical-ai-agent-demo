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
	log.Printf("[chat] session=%s stream done: %d chunks, text_len=%d", sess.ID, chunkCount, len(assistantText))

	calls := <-toolResults
	log.Printf("[chat] session=%s tool_calls=%d", sess.ID, len(calls))
	for _, c := range calls {
		log.Printf("[chat] session=%s   tool=%s input=%v", sess.ID, c.ToolName, c.Input)
	}

	newState := h.executeToolCalls(sess, calls, r)
	log.Printf("[chat] session=%s new_state=%s", sess.ID, newState)

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
			if sess.State == models.StateGreeting {
				sess.State = models.StateIntake
			}

		case "begin_prescription":
			sess.State = models.StatePrescription

		case "show_office_info":
			sess.State = models.StateHours

		case "collect_intake":
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
			doctorID := strField(call.Input, "doctorId")
			if doc := services.GetDoctorByID(doctorID); doc != nil {
				sess.MatchedDoctor = doc
				sess.State = models.StateScheduling
			}

		case "select_slot":
			date := strField(call.Input, "date")
			startTime := strField(call.Input, "startTime")
			if date != "" && startTime != "" && !services.IsSlotBooked(sess.MatchedDoctor.ID, date, startTime) {
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
			if sess.State == models.StateConfirming && sess.SelectedSlot != nil && sess.MatchedDoctor != nil {
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
			}

		case "log_prescription_request":
			sess.State = models.StatePrescription
		}
	}
	return sess.State
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
