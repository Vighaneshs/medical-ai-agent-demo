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
	h := setupHandlerTest(t, &mockAI{})
	sess := services.Store.GetOrCreate("begin-intake-test")
	sess.State = models.StateGreeting

	calls := []services.ToolCallResult{{ToolName: "begin_intake", Input: map[string]interface{}{}}}
	newState := h.executeToolCalls(sess, calls, nil)

	if newState != models.StateIntake {
		t.Errorf("begin_intake: state = %q, want INTAKE", newState)
	}
}

func TestExecuteToolCalls_BeginPrescription(t *testing.T) {
	h := setupHandlerTest(t, &mockAI{})
	sess := services.Store.GetOrCreate("begin-presc-test")
	calls := []services.ToolCallResult{{ToolName: "begin_prescription", Input: map[string]interface{}{}}}
	newState := h.executeToolCalls(sess, calls, nil)
	if newState != models.StatePrescription {
		t.Errorf("begin_prescription: state = %q, want PRESCRIPTION", newState)
	}
}

func TestExecuteToolCalls_ShowOfficeInfo(t *testing.T) {
	h := setupHandlerTest(t, &mockAI{})
	sess := services.Store.GetOrCreate("office-info-test")
	calls := []services.ToolCallResult{{ToolName: "show_office_info", Input: map[string]interface{}{}}}
	newState := h.executeToolCalls(sess, calls, nil)
	if newState != models.StateHours {
		t.Errorf("show_office_info: state = %q, want HOURS", newState)
	}
}

func TestExecuteToolCalls_CollectIntake(t *testing.T) {
	h := setupHandlerTest(t, &mockAI{})
	sess := services.Store.GetOrCreate("collect-intake-test")

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
	newState := h.executeToolCalls(sess, calls, nil)

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
	h := setupHandlerTest(t, &mockAI{})
	sess := services.Store.GetOrCreate("confirm-doc-test")

	calls := []services.ToolCallResult{{
		ToolName: "confirm_doctor",
		Input:    map[string]interface{}{"doctorId": "dr-thompson"},
	}}
	newState := h.executeToolCalls(sess, calls, nil)

	if newState != models.StateScheduling {
		t.Errorf("confirm_doctor: state = %q, want SCHEDULING", newState)
	}
	if sess.MatchedDoctor == nil || sess.MatchedDoctor.ID != "dr-thompson" {
		t.Errorf("MatchedDoctor = %v, want dr-thompson", sess.MatchedDoctor)
	}
}

func TestExecuteToolCalls_ConfirmDoctor_InvalidID(t *testing.T) {
	h := setupHandlerTest(t, &mockAI{})
	sess := services.Store.GetOrCreate("confirm-doc-invalid")
	sess.State = models.StateMatching

	calls := []services.ToolCallResult{{
		ToolName: "confirm_doctor",
		Input:    map[string]interface{}{"doctorId": "dr-nobody"},
	}}
	newState := h.executeToolCalls(sess, calls, nil)

	// State should not change for invalid doctor
	if newState != models.StateMatching {
		t.Errorf("invalid doctor: state = %q, want MATCHING unchanged", newState)
	}
}

func TestExecuteToolCalls_SelectSlot(t *testing.T) {
	h := setupHandlerTest(t, &mockAI{})
	sess := services.Store.GetOrCreate("select-slot-test")
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
	newState := h.executeToolCalls(sess, calls, nil)

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

func TestExecuteToolCalls_ConfirmBooking(t *testing.T) {
	h := setupHandlerTest(t, &mockAI{})
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
	newState := h.executeToolCalls(sess, calls, nil)

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
	h := setupHandlerTest(t, &mockAI{})
	sess := services.Store.GetOrCreate("presc-log-test")

	calls := []services.ToolCallResult{{
		ToolName: "log_prescription_request",
		Input: map[string]interface{}{
			"medication":   "Lisinopril",
			"pharmacyName": "CVS Pharmacy",
		},
	}}
	newState := h.executeToolCalls(sess, calls, nil)
	if newState != models.StatePrescription {
		t.Errorf("log_prescription_request: state = %q, want PRESCRIPTION", newState)
	}
}

func TestExecuteToolCalls_EmptyCalls(t *testing.T) {
	h := setupHandlerTest(t, &mockAI{})
	sess := services.Store.GetOrCreate("empty-calls-test")
	sess.State = models.StateGreeting

	newState := h.executeToolCalls(sess, nil, nil)
	if newState != models.StateGreeting {
		t.Errorf("empty calls: state = %q, want GREETING (unchanged)", newState)
	}
}
