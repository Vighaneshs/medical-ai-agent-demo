package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"kyron-medical/models"
	"kyron-medical/services"
)

// ─── Test helpers ────────────────────────────────────────────────────────────

// flushRecorder wraps httptest.ResponseRecorder and implements http.Flusher.
type flushRecorder struct {
	*httptest.ResponseRecorder
}

func (f *flushRecorder) Flush() {}

// mockAI implements services.AIProvider with configurable outputs.
type mockAI struct {
	chunks      []string
	toolResults []services.ToolCallResult
	summary     string
}

func (m *mockAI) Stream(
	ctx context.Context,
	_ string,
	_ []models.ChatMessage,
	textChunks chan<- string,
	toolResults chan<- []services.ToolCallResult,
) {
	defer close(textChunks)
	defer close(toolResults)
	for _, c := range m.chunks {
		select {
		case <-ctx.Done():
			return
		case textChunks <- c:
		}
	}
	toolResults <- m.toolResults
}

func (m *mockAI) Summarize(_ context.Context, _ []models.ChatMessage) string {
	return m.summary
}

// setupHandlerTest initialises an in-memory session store and injects a mock AI.
func setupHandlerTest(t *testing.T, ai services.AIProvider) *ChatHandler {
	t.Helper()
	if err := services.InitSessionStore(":memory:"); err != nil {
		t.Fatalf("InitSessionStore: %v", err)
	}
	services.AI = ai
	return NewChatHandler(services.Store)
}

// chatRequest builds an HTTP POST request to /api/chat.
func chatRequest(t *testing.T, sessionID, message string) *http.Request {
	t.Helper()
	body, _ := json.Marshal(map[string]string{"sessionId": sessionID, "message": message})
	r := httptest.NewRequest(http.MethodPost, "/api/chat", bytes.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	return r
}

// parseSSEEvents splits a raw SSE body into parsed JSON objects.
func parseSSEEvents(t *testing.T, body string) []map[string]interface{} {
	t.Helper()
	var events []map[string]interface{}
	for _, line := range strings.Split(body, "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		var ev map[string]interface{}
		if err := json.Unmarshal([]byte(line[6:]), &ev); err == nil {
			events = append(events, ev)
		}
	}
	return events
}

// ─── strField / boolField ────────────────────────────────────────────────────

func TestStrField(t *testing.T) {
	m := map[string]interface{}{
		"name":   "Alice",
		"number": 42,
		"nil":    nil,
	}
	if got := strField(m, "name"); got != "Alice" {
		t.Errorf("strField name = %q, want Alice", got)
	}
	if got := strField(m, "number"); got != "" {
		t.Errorf("strField number = %q, want empty (not a string)", got)
	}
	if got := strField(m, "missing"); got != "" {
		t.Errorf("strField missing key = %q, want empty", got)
	}
	if got := strField(m, "nil"); got != "" {
		t.Errorf("strField nil value = %q, want empty", got)
	}
}

func TestBoolField(t *testing.T) {
	m := map[string]interface{}{
		"yes":  true,
		"no":   false,
		"str":  "true",
		"num":  1,
	}
	if !boolField(m, "yes") {
		t.Error("boolField yes = false, want true")
	}
	if boolField(m, "no") {
		t.Error("boolField no = true, want false")
	}
	if boolField(m, "str") {
		t.Error("boolField str = true, want false (not a bool)")
	}
	if boolField(m, "missing") {
		t.Error("boolField missing = true, want false")
	}
}

// ─── HandleChat — HTTP layer ──────────────────────────────────────────────────

func TestHandleChat_InvalidJSON(t *testing.T) {
	h := setupHandlerTest(t, &mockAI{})
	w := &flushRecorder{httptest.NewRecorder()}
	r := httptest.NewRequest(http.MethodPost, "/api/chat", strings.NewReader("not json"))
	r.Header.Set("Content-Type", "application/json")
	h.HandleChat(w, r)
	if w.Code != http.StatusBadRequest {
		t.Errorf("got status %d, want 400 for invalid JSON", w.Code)
	}
}

func TestHandleChat_MissingSessionID(t *testing.T) {
	h := setupHandlerTest(t, &mockAI{})
	w := &flushRecorder{httptest.NewRecorder()}
	body, _ := json.Marshal(map[string]string{"message": "hello"})
	r := httptest.NewRequest(http.MethodPost, "/api/chat", bytes.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	h.HandleChat(w, r)
	if w.Code != http.StatusBadRequest {
		t.Errorf("got status %d, want 400 for missing sessionId", w.Code)
	}
}

func TestHandleChat_EmergencyMessage(t *testing.T) {
	h := setupHandlerTest(t, &mockAI{chunks: []string{"This should not appear"}})
	w := &flushRecorder{httptest.NewRecorder()}
	h.HandleChat(w, chatRequest(t, "emergency-sess", "I have severe chest pain"))

	body := w.Body.String()
	events := parseSSEEvents(t, body)
	if len(events) == 0 {
		t.Fatal("expected at least one SSE event for emergency")
	}
	found := false
	for _, ev := range events {
		if v, ok := ev["emergency"].(bool); ok && v {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("emergency SSE event not found in: %s", body)
	}
}

func TestHandleChat_StreamsTextChunks(t *testing.T) {
	ai := &mockAI{chunks: []string{"Hello ", "there ", "patient."}, toolResults: nil}
	h := setupHandlerTest(t, ai)
	w := &flushRecorder{httptest.NewRecorder()}
	h.HandleChat(w, chatRequest(t, "stream-sess", "Hello"))

	body := w.Body.String()
	events := parseSSEEvents(t, body)

	var textParts []string
	for _, ev := range events {
		if text, ok := ev["text"].(string); ok && text != "" {
			textParts = append(textParts, text)
		}
	}
	combined := strings.Join(textParts, "")
	if !strings.Contains(combined, "Hello") {
		t.Errorf("streamed text %q should contain Hello", combined)
	}
}

func TestHandleChat_DoneEventContainsNewState(t *testing.T) {
	ai := &mockAI{
		chunks: []string{"Sure, I can help with that!"},
		toolResults: []services.ToolCallResult{
			{ToolName: "begin_intake", Input: map[string]interface{}{}},
		},
	}
	h := setupHandlerTest(t, ai)
	w := &flushRecorder{httptest.NewRecorder()}
	h.HandleChat(w, chatRequest(t, "state-sess", "I want to book"))

	events := parseSSEEvents(t, w.Body.String())
	var doneEvent map[string]interface{}
	for _, ev := range events {
		if v, ok := ev["done"].(bool); ok && v {
			doneEvent = ev
			break
		}
	}
	if doneEvent == nil {
		t.Fatal("no done event found in SSE stream")
	}
	if doneEvent["newState"] != "INTAKE" {
		t.Errorf("newState = %v, want INTAKE", doneEvent["newState"])
	}
}

// ─── executeToolCalls — state machine transitions ────────────────────────────

func TestExecuteToolCalls_BeginIntake(t *testing.T) {
	setupHandlerTest(t, &mockAI{})
	sess := services.Store.GetOrCreate("begin-intake-test")
	sess.State = models.StateGreeting

	calls := []services.ToolCallResult{{ToolName: "begin_intake", Input: map[string]interface{}{}}}
	newState, _ := executeToolCalls(sess, calls)

	if newState != models.StateIntake {
		t.Errorf("begin_intake: state = %q, want INTAKE", newState)
	}
}

func TestExecuteToolCalls_BeginPrescription(t *testing.T) {
	setupHandlerTest(t, &mockAI{})
	sess := services.Store.GetOrCreate("begin-presc-test")
	calls := []services.ToolCallResult{{ToolName: "begin_prescription", Input: map[string]interface{}{}}}
	newState, _ := executeToolCalls(sess, calls)
	if newState != models.StatePrescription {
		t.Errorf("begin_prescription: state = %q, want PRESCRIPTION", newState)
	}
}

func TestExecuteToolCalls_ShowOfficeInfo(t *testing.T) {
	setupHandlerTest(t, &mockAI{})
	sess := services.Store.GetOrCreate("office-info-test")
	calls := []services.ToolCallResult{{ToolName: "show_office_info", Input: map[string]interface{}{}}}
	newState, _ := executeToolCalls(sess, calls)
	if newState != models.StateHours {
		t.Errorf("show_office_info: state = %q, want HOURS", newState)
	}
}

func TestExecuteToolCalls_CollectIntake(t *testing.T) {
	setupHandlerTest(t, &mockAI{})
	sess := services.Store.GetOrCreate("collect-intake-test")
	sess.State = models.StateIntake

	calls := []services.ToolCallResult{{
		ToolName: "collect_intake",
		Input: map[string]interface{}{
			"firstName":      "Alex",
			"lastName":       "Johnson",
			"dob":            "1990-03-05",
			"phone":          "555-123-4567",
			"email":          "alex@test.com",
			"reasonForVisit": "migraines",
		},
	}}
	newState, _ := executeToolCalls(sess, calls)

	if newState != models.StateMatching {
		t.Errorf("collect_intake: state = %q, want MATCHING", newState)
	}
	if sess.PatientInfo.FirstName != "Alex" {
		t.Errorf("FirstName = %q, want Alex", sess.PatientInfo.FirstName)
	}
	if sess.PatientInfo.Email != "alex@test.com" {
		t.Errorf("Email = %q, want alex@test.com", sess.PatientInfo.Email)
	}
	if sess.PhoneNumber != "555-123-4567" {
		t.Errorf("PhoneNumber = %q, want 555-123-4567", sess.PhoneNumber)
	}
}

func TestExecuteToolCalls_ConfirmDoctor(t *testing.T) {
	setupHandlerTest(t, &mockAI{})
	sess := services.Store.GetOrCreate("confirm-doc-test")
	sess.State = models.StateMatching

	calls := []services.ToolCallResult{{
		ToolName: "confirm_doctor",
		Input:    map[string]interface{}{"doctorId": "dr-thompson"},
	}}
	newState, _ := executeToolCalls(sess, calls)

	if newState != models.StateScheduling {
		t.Errorf("confirm_doctor: state = %q, want SCHEDULING", newState)
	}
	if sess.MatchedDoctor == nil || sess.MatchedDoctor.ID != "dr-thompson" {
		t.Errorf("MatchedDoctor = %v, want dr-thompson", sess.MatchedDoctor)
	}
}

func TestExecuteToolCalls_ConfirmDoctor_InvalidID(t *testing.T) {
	setupHandlerTest(t, &mockAI{})
	sess := services.Store.GetOrCreate("confirm-doc-invalid")
	sess.State = models.StateMatching

	calls := []services.ToolCallResult{{
		ToolName: "confirm_doctor",
		Input:    map[string]interface{}{"doctorId": "dr-nobody"},
	}}
	newState, errs := executeToolCalls(sess, calls)

	if newState != models.StateMatching {
		t.Errorf("invalid doctor: state = %q, want MATCHING unchanged", newState)
	}
	if len(errs) == 0 {
		t.Error("expected a tool error for invalid doctorId, got none")
	}
	if len(errs) > 0 && !strings.Contains(errs[0], "dr-nobody") {
		t.Errorf("error message should mention the bad doctor ID, got: %q", errs[0])
	}
}

func TestExecuteToolCalls_SelectSlot(t *testing.T) {
	setupHandlerTest(t, &mockAI{})
	sess := services.Store.GetOrCreate("select-slot-test")
	sess.State = models.StateScheduling
	sess.MatchedDoctor = services.GetDoctorByID("dr-mitchell")

	// Get a real available slot
	slots := services.GenerateAvailability("dr-mitchell")
	var available *models.TimeSlot
	for i := range slots {
		if slots[i].Available {
			available = &slots[i]
			break
		}
	}
	if available == nil {
		t.Skip("no available slots for test")
	}

	calls := []services.ToolCallResult{{
		ToolName: "select_slot",
		Input: map[string]interface{}{
			"date":      available.Date,
			"startTime": available.StartTime,
		},
	}}
	newState, _ := executeToolCalls(sess, calls)

	if newState != models.StateConfirming {
		t.Errorf("select_slot: state = %q, want CONFIRMING", newState)
	}
	if sess.SelectedSlot == nil {
		t.Fatal("SelectedSlot should be set")
	}
	if sess.SelectedSlot.Date != available.Date {
		t.Errorf("SelectedSlot.Date = %q, want %q", sess.SelectedSlot.Date, available.Date)
	}
}

// TestExecuteToolCalls_SelectSlot_AlreadyBooked verifies that select_slot for a
// pre-booked slot returns a tool error and leaves state unchanged.
func TestExecuteToolCalls_SelectSlot_AlreadyBooked(t *testing.T) {
	setupHandlerTest(t, &mockAI{})
	sess := services.Store.GetOrCreate("select-slot-booked")
	sess.State = models.StateScheduling
	sess.MatchedDoctor = services.GetDoctorByID("dr-mitchell")

	// Find an available slot and pre-book it
	slots := services.GenerateAvailability("dr-mitchell")
	var target *models.TimeSlot
	for i := range slots {
		if slots[i].Available {
			target = &slots[i]
			break
		}
	}
	if target == nil {
		t.Skip("no available slots for dr-mitchell")
	}
	services.BookSlot("dr-mitchell", target.Date, target.StartTime)

	calls := []services.ToolCallResult{{
		ToolName: "select_slot",
		Input: map[string]interface{}{
			"date":      target.Date,
			"startTime": target.StartTime,
		},
	}}
	newState, errs := executeToolCalls(sess, calls)

	if newState != models.StateScheduling {
		t.Errorf("already-booked slot: state = %q, want SCHEDULING unchanged", newState)
	}
	if len(errs) == 0 {
		t.Error("expected a tool error for already-booked slot, got none")
	}
	if len(errs) > 0 && !strings.Contains(errs[0], "already booked") {
		t.Errorf("error should mention 'already booked', got: %q", errs[0])
	}
}

func TestExecuteToolCalls_ConfirmBooking(t *testing.T) {
	setupHandlerTest(t, &mockAI{})
	sess := services.Store.GetOrCreate("confirm-booking-test")
	sess.State = models.StateConfirming
	sess.MatchedDoctor = services.GetDoctorByID("dr-patel")
	sess.SelectedSlot = &models.TimeSlot{
		DoctorID:  "dr-patel",
		Date:      "2099-06-01", // far future to avoid collision
		StartTime: "09:00",
		EndTime:   "10:00",
	}
	sess.PatientInfo = models.PatientInfo{
		FirstName: "Alice",
		LastName:  "Test",
		Email:     "alice@test.com",
		Phone:     "555-000-1111",
	}

	calls := []services.ToolCallResult{{
		ToolName: "confirm_booking",
		Input:    map[string]interface{}{"smsOptIn": false},
	}}
	newState, _ := executeToolCalls(sess, calls)

	if newState != models.StateBooked {
		t.Errorf("confirm_booking: state = %q, want BOOKED", newState)
	}
	if sess.Appointment == nil {
		t.Fatal("Appointment should be set after confirm_booking")
	}
	if sess.Appointment.Doctor.ID != "dr-patel" {
		t.Errorf("Appointment.Doctor.ID = %q, want dr-patel", sess.Appointment.Doctor.ID)
	}
}

func TestExecuteToolCalls_LogPrescriptionRequest(t *testing.T) {
	setupHandlerTest(t, &mockAI{})
	sess := services.Store.GetOrCreate("presc-log-test")

	calls := []services.ToolCallResult{{
		ToolName: "log_prescription_request",
		Input: map[string]interface{}{
			"medication":   "Lisinopril",
			"pharmacyName": "CVS Pharmacy",
		},
	}}
	newState, _ := executeToolCalls(sess, calls)
	if newState != models.StatePrescription {
		t.Errorf("log_prescription_request: state = %q, want PRESCRIPTION", newState)
	}
}

func TestExecuteToolCalls_EmptyCalls(t *testing.T) {
	setupHandlerTest(t, &mockAI{})
	sess := services.Store.GetOrCreate("empty-calls-test")
	sess.State = models.StateGreeting

	newState, _ := executeToolCalls(sess, nil)
	if newState != models.StateGreeting {
		t.Errorf("empty calls: state = %q, want GREETING (unchanged)", newState)
	}
}

// ─── TestExecuteToolCalls_StateGuards ────────────────────────────────────────

// TestExecuteToolCalls_StateGuards verifies that each state-gated tool is
// silently ignored when the session is in a state that does not permit it.
func TestExecuteToolCalls_StateGuards(t *testing.T) {
	tests := []struct {
		name          string
		setupState    models.SessionState
		toolName      string
		extraSetup    func(sess *models.Session)
		wantState     models.SessionState
		wantNilDoctor bool // if true, MatchedDoctor must still be nil after the call
	}{
		{
			name:       "collect_intake ignored in MATCHING",
			setupState: models.StateMatching,
			toolName:   "collect_intake",
			extraSetup: func(sess *models.Session) {
				// Provide all fields so the tool would normally succeed
				// collect_intake is only allowed in INTAKE, not MATCHING
			},
			wantState:     models.StateMatching,
			wantNilDoctor: false,
		},
		{
			name:       "confirm_doctor ignored in SCHEDULING",
			setupState: models.StateScheduling,
			toolName:   "confirm_doctor",
			extraSetup: func(sess *models.Session) {
				sess.MatchedDoctor = services.GetDoctorByID("dr-patel")
			},
			wantState: models.StateScheduling,
		},
		{
			name:       "select_slot ignored in CONFIRMING",
			setupState: models.StateConfirming,
			toolName:   "select_slot",
			extraSetup: func(sess *models.Session) {
				sess.MatchedDoctor = services.GetDoctorByID("dr-mitchell")
			},
			wantState: models.StateConfirming,
		},
		{
			name:       "confirm_booking ignored in SCHEDULING",
			setupState: models.StateScheduling,
			toolName:   "confirm_booking",
			extraSetup: func(sess *models.Session) {
				sess.MatchedDoctor = services.GetDoctorByID("dr-patel")
				sess.SelectedSlot = &models.TimeSlot{
					DoctorID: "dr-patel", Date: "2099-01-01",
					StartTime: "09:00", EndTime: "10:00",
				}
			},
			wantState: models.StateScheduling,
		},
		{
			name:       "begin_intake ignored in MATCHING",
			setupState: models.StateMatching,
			toolName:   "begin_intake",
			extraSetup: nil,
			wantState:  models.StateMatching,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			setupHandlerTest(t, &mockAI{})
			sess := services.Store.GetOrCreate("state-guard-" + tc.name)
			sess.State = tc.setupState
			if tc.extraSetup != nil {
				tc.extraSetup(sess)
			}
			origPatientInfo := sess.PatientInfo

			calls := []services.ToolCallResult{{
				ToolName: tc.toolName,
				Input: map[string]interface{}{
					// Provide inputs that would succeed if the tool were allowed
					"firstName":      "Blocked",
					"lastName":       "User",
					"dob":            "1990-01-01",
					"phone":          "555-999-0000",
					"email":          "blocked@test.com",
					"reasonForVisit": "testing",
					"doctorId":       "dr-thompson",
					"date":           "2099-06-01",
					"startTime":      "09:00",
					"smsOptIn":       false,
				},
			}}

			newState, _ := executeToolCalls(sess, calls)

			if newState != tc.wantState {
				t.Errorf("state after blocked tool call = %q, want %q", newState, tc.wantState)
			}

			// collect_intake specifically must not update PatientInfo when ignored
			if tc.toolName == "collect_intake" {
				if sess.PatientInfo.FirstName != origPatientInfo.FirstName {
					t.Errorf("collect_intake (blocked): PatientInfo.FirstName changed to %q, want unchanged %q",
						sess.PatientInfo.FirstName, origPatientInfo.FirstName)
				}
			}
		})
	}
}

// ─── TestExecuteToolCalls_ConfirmBooking_NilGuards ───────────────────────────

// TestExecuteToolCalls_ConfirmBooking_NilGuards checks that confirm_booking
// does not panic and stays in CONFIRMING when MatchedDoctor or SelectedSlot
// is nil, and succeeds when both are set.
func TestExecuteToolCalls_ConfirmBooking_NilGuards(t *testing.T) {
	t.Run("nil MatchedDoctor stays CONFIRMING", func(t *testing.T) {
		setupHandlerTest(t, &mockAI{})
		sess := services.Store.GetOrCreate("confirm-booking-nil-doctor")
		sess.State = models.StateConfirming
		sess.MatchedDoctor = nil
		sess.SelectedSlot = &models.TimeSlot{
			DoctorID: "dr-patel", Date: "2099-07-01",
			StartTime: "10:00", EndTime: "11:00",
		}

		calls := []services.ToolCallResult{{
			ToolName: "confirm_booking",
			Input:    map[string]interface{}{"smsOptIn": false},
		}}
		newState, errs := executeToolCalls(sess, calls)

		if newState != models.StateConfirming {
			t.Errorf("nil MatchedDoctor: state = %q, want CONFIRMING", newState)
		}
		if sess.Appointment != nil {
			t.Error("nil MatchedDoctor: Appointment should not be created")
		}
		if len(errs) == 0 {
			t.Error("expected a tool error for nil MatchedDoctor, got none")
		}
	})

	t.Run("nil SelectedSlot stays CONFIRMING", func(t *testing.T) {
		setupHandlerTest(t, &mockAI{})
		sess := services.Store.GetOrCreate("confirm-booking-nil-slot")
		sess.State = models.StateConfirming
		sess.MatchedDoctor = services.GetDoctorByID("dr-patel")
		sess.SelectedSlot = nil

		calls := []services.ToolCallResult{{
			ToolName: "confirm_booking",
			Input:    map[string]interface{}{"smsOptIn": false},
		}}
		newState, errs := executeToolCalls(sess, calls)

		if newState != models.StateConfirming {
			t.Errorf("nil SelectedSlot: state = %q, want CONFIRMING", newState)
		}
		if sess.Appointment != nil {
			t.Error("nil SelectedSlot: Appointment should not be created")
		}
		if len(errs) == 0 {
			t.Error("expected a tool error for nil SelectedSlot, got none")
		}
	})

	t.Run("both set succeeds and transitions to BOOKED", func(t *testing.T) {
		setupHandlerTest(t, &mockAI{})
		sess := services.Store.GetOrCreate("confirm-booking-nil-both-set")
		sess.State = models.StateConfirming
		sess.MatchedDoctor = services.GetDoctorByID("dr-patel")
		sess.SelectedSlot = &models.TimeSlot{
			DoctorID:  "dr-patel",
			Date:      "2099-08-01",
			StartTime: "14:00",
			EndTime:   "15:00",
		}
		sess.PatientInfo = models.PatientInfo{
			FirstName: "Test",
			LastName:  "Booking",
			Email:     "testbooking@example.com",
			Phone:     "555-111-2222",
		}

		calls := []services.ToolCallResult{{
			ToolName: "confirm_booking",
			Input:    map[string]interface{}{"smsOptIn": true},
		}}
		newState, _ := executeToolCalls(sess, calls)

		if newState != models.StateBooked {
			t.Errorf("both set: state = %q, want BOOKED", newState)
		}
		if sess.Appointment == nil {
			t.Fatal("both set: Appointment should be created")
		}
		if sess.Appointment.Doctor.ID != "dr-patel" {
			t.Errorf("Appointment.Doctor.ID = %q, want dr-patel", sess.Appointment.Doctor.ID)
		}
		if !sess.PatientInfo.SMSOptIn {
			t.Error("SMSOptIn should be true")
		}
	})
}

// ─── TestExecuteToolCalls_BeginIntake_ValidStates ────────────────────────────

// TestExecuteToolCalls_BeginIntake_ValidStates ensures begin_intake transitions
// to INTAKE from allowed states (GREETING, BOOKED, PRESCRIPTION, HOURS) and
// stays put in mid-flow states (MATCHING, SCHEDULING, CONFIRMING, INTAKE).
func TestExecuteToolCalls_BeginIntake_ValidStates(t *testing.T) {
	allowed := []models.SessionState{
		models.StateGreeting,
		models.StateBooked,
		models.StatePrescription,
		models.StateHours,
	}
	blocked := []models.SessionState{
		models.StateMatching,
		models.StateScheduling,
		models.StateConfirming,
		models.StateIntake,
	}

	calls := []services.ToolCallResult{{
		ToolName: "begin_intake",
		Input:    map[string]interface{}{},
	}}

	for _, state := range allowed {
		t.Run("allowed from "+string(state), func(t *testing.T) {
			setupHandlerTest(t, &mockAI{})
			sess := services.Store.GetOrCreate("begin-intake-allowed-" + string(state))
			sess.State = state

			newState, _ := executeToolCalls(sess, calls)
			if newState != models.StateIntake {
				t.Errorf("begin_intake from %s: state = %q, want INTAKE", state, newState)
			}
		})
	}

	for _, state := range blocked {
		t.Run("blocked from "+string(state), func(t *testing.T) {
			setupHandlerTest(t, &mockAI{})
			sess := services.Store.GetOrCreate("begin-intake-blocked-" + string(state))
			sess.State = state
			if state == models.StateScheduling {
				sess.MatchedDoctor = services.GetDoctorByID("dr-mitchell")
			}

			newState, _ := executeToolCalls(sess, calls)
			if newState != state {
				t.Errorf("begin_intake from %s (blocked): state = %q, want unchanged %s",
					state, newState, state)
			}
		})
	}
}

// ─── TestExecuteToolCalls_MultipleTools_LastWins ─────────────────────────────

// TestExecuteToolCalls_MultipleTools_LastWins verifies that when two tools are
// executed in sequence, state guards prevent a wrong-state tool from
// overwriting the state set by a valid preceding tool.
//
// Scenario:
//   1. select_slot fires: session is in SCHEDULING → transitions to CONFIRMING
//   2. collect_intake fires: session is now in CONFIRMING (not INTAKE) → ignored
//
// Final state must be CONFIRMING, not the result of collect_intake (MATCHING).
func TestExecuteToolCalls_MultipleTools_LastWins(t *testing.T) {
	setupHandlerTest(t, &mockAI{})
	sess := services.Store.GetOrCreate("multi-tool-last-wins")
	sess.State = models.StateScheduling
	sess.MatchedDoctor = services.GetDoctorByID("dr-mitchell")

	// Find a real available slot so select_slot can succeed
	slots := services.GenerateAvailability("dr-mitchell")
	var available *models.TimeSlot
	for i := range slots {
		if slots[i].Available {
			available = &slots[i]
			break
		}
	}
	if available == nil {
		t.Skip("no available slots for dr-mitchell")
	}

	calls := []services.ToolCallResult{
		{
			ToolName: "select_slot",
			Input: map[string]interface{}{
				"date":      available.Date,
				"startTime": available.StartTime,
			},
		},
		{
			// This tool expects INTAKE state; it should be ignored because
			// after select_slot the session is now CONFIRMING.
			ToolName: "collect_intake",
			Input: map[string]interface{}{
				"firstName":      "Ignored",
				"lastName":       "Patient",
				"dob":            "1990-01-01",
				"phone":          "555-000-9999",
				"email":          "ignored@test.com",
				"reasonForVisit": "should not matter",
			},
		},
	}

	newState, _ := executeToolCalls(sess, calls)

	if newState != models.StateConfirming {
		t.Errorf("final state = %q, want CONFIRMING (collect_intake should be blocked after select_slot)", newState)
	}
	// PatientInfo should NOT have been overwritten by the blocked collect_intake
	if sess.PatientInfo.FirstName == "Ignored" {
		t.Error("PatientInfo.FirstName was overwritten by blocked collect_intake call")
	}
}
