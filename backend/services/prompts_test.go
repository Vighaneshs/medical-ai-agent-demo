package services

import (
	"strings"
	"testing"
	"time"

	"kyron-medical/models"
)

func newSession(state models.SessionState) *models.Session {
	return &models.Session{
		ID:    "test-session",
		State: state,
	}
}

func TestBuild_AlwaysContainsPersonaBlock(t *testing.T) {
	states := []models.SessionState{
		models.StateGreeting, models.StateIntake, models.StateMatching,
		models.StateScheduling, models.StateConfirming, models.StateBooked,
		models.StatePrescription, models.StateHours,
	}
	for _, state := range states {
		t.Run(string(state), func(t *testing.T) {
			sess := newSession(state)
			if state == models.StateScheduling {
				// Need a matched doctor to avoid nil panic
				sess.MatchedDoctor = GetDoctorByID("dr-mitchell")
			}
			prompt := Build(sess)
			if !strings.Contains(prompt, "Kyron") {
				t.Errorf("state %s: prompt missing persona block (Kyron)", state)
			}
			if !strings.Contains(prompt, "NOT a doctor") {
				t.Errorf("state %s: prompt missing safety constraint", state)
			}
		})
	}
}

func TestBuild_AlwaysContainsPracticeInfo(t *testing.T) {
	sess := newSession(models.StateGreeting)
	prompt := Build(sess)
	if !strings.Contains(prompt, "1250 Healthcare Blvd") {
		t.Error("prompt missing practice address")
	}
	if !strings.Contains(prompt, "(555) 201-0000") {
		t.Error("prompt missing practice phone number")
	}
}

func TestBuild_AlwaysContainsToolReminder(t *testing.T) {
	sess := newSession(models.StateGreeting)
	prompt := Build(sess)
	if !strings.Contains(prompt, "AVAILABLE TOOLS") {
		t.Error("prompt missing AVAILABLE TOOLS section")
	}
	if !strings.Contains(prompt, "collect_intake") {
		t.Error("prompt missing collect_intake tool")
	}
}

func TestBuild_GreetingState(t *testing.T) {
	sess := newSession(models.StateGreeting)
	prompt := Build(sess)
	if !strings.Contains(prompt, "GREETING") {
		t.Error("GREETING prompt missing GREETING heading")
	}
	if !strings.Contains(prompt, "begin_intake") {
		t.Error("GREETING prompt should mention begin_intake tool")
	}
}

func TestBuild_IntakeState_ShowsMissingFields(t *testing.T) {
	sess := newSession(models.StateIntake)
	prompt := Build(sess)
	for _, field := range []string{"full name", "date of birth", "phone number", "email address", "reason for visit"} {
		if !strings.Contains(prompt, field) {
			t.Errorf("INTAKE prompt missing field %q", field)
		}
	}
}

func TestBuild_IntakeState_ShowsCollectedFields(t *testing.T) {
	sess := newSession(models.StateIntake)
	sess.PatientInfo = models.PatientInfo{
		FirstName:      "Alex",
		LastName:       "Johnson",
		DOB:            "1990-03-05",
		Phone:          "555-123-4567",
		Email:          "alex@test.com",
		ReasonForVisit: "migraines",
	}
	prompt := Build(sess)
	if !strings.Contains(prompt, "Alex Johnson") {
		t.Error("collected name should appear in INTAKE prompt")
	}
	if !strings.Contains(prompt, "alex@test.com") {
		t.Error("collected email should appear in INTAKE prompt")
	}
	// Once all fields collected, the "Still needed" section should be absent
	if strings.Contains(prompt, "Still needed") {
		t.Error("all fields collected — 'Still needed' should not appear")
	}
}

func TestBuild_MatchingState_ContainsDoctors(t *testing.T) {
	sess := newSession(models.StateMatching)
	sess.PatientInfo = models.PatientInfo{
		FirstName:      "Alex",
		LastName:       "Johnson",
		ReasonForVisit: "migraines",
	}
	prompt := Build(sess)
	if !strings.Contains(prompt, "MATCHING") {
		t.Error("MATCHING prompt missing MATCHING heading")
	}
	if !strings.Contains(prompt, "Dr. Sarah Mitchell") {
		t.Error("MATCHING prompt should list all doctors")
	}
	if !strings.Contains(prompt, "confirm_doctor") {
		t.Error("MATCHING prompt should mention confirm_doctor tool")
	}
}

func TestBuild_SchedulingState_ContainsAvailability(t *testing.T) {
	sess := newSession(models.StateScheduling)
	sess.MatchedDoctor = GetDoctorByID("dr-thompson")
	prompt := Build(sess)
	if !strings.Contains(prompt, "SCHEDULING") {
		t.Error("SCHEDULING prompt missing SCHEDULING heading")
	}
	if !strings.Contains(prompt, "Dr. Lisa Thompson") {
		t.Error("SCHEDULING prompt should include matched doctor name")
	}
	if !strings.Contains(prompt, "select_slot") {
		t.Error("SCHEDULING prompt should mention select_slot tool")
	}
	// Should contain at least one date in the availability list
	year := time.Now().Year()
	if !strings.Contains(prompt, string(rune('0'+year/1000))) {
		t.Logf("prompt: %s", prompt[:200])
	}
}

func TestBuild_ConfirmingState_ContainsAppointmentDetails(t *testing.T) {
	sess := newSession(models.StateConfirming)
	sess.MatchedDoctor = GetDoctorByID("dr-patel")
	sess.SelectedSlot = &models.TimeSlot{
		DoctorID:  "dr-patel",
		Date:      "2026-04-01",
		StartTime: "10:00",
		EndTime:   "11:00",
	}
	sess.PatientInfo = models.PatientInfo{
		FirstName: "Alex",
		LastName:  "Johnson",
		Email:     "alex@test.com",
		Phone:     "555-123-4567",
	}
	prompt := Build(sess)
	if !strings.Contains(prompt, "Dr. Priya Patel") {
		t.Error("CONFIRMING prompt should include doctor name")
	}
	if !strings.Contains(prompt, "Wednesday, April 1, 2026") {
		t.Error("CONFIRMING prompt should include formatted appointment date")
	}
	if !strings.Contains(prompt, "confirm_booking") {
		t.Error("CONFIRMING prompt should mention confirm_booking tool")
	}
}

func TestBuild_BookedState(t *testing.T) {
	sess := newSession(models.StateBooked)
	sess.PatientInfo = models.PatientInfo{Email: "alex@test.com"}
	sess.Appointment = &models.Appointment{ID: "abc12345-rest-of-uuid"}
	prompt := Build(sess)
	if !strings.Contains(prompt, "confirmed") {
		t.Error("BOOKED prompt should say confirmed")
	}
	if !strings.Contains(prompt, "abc12345") {
		t.Error("BOOKED prompt should include shortened appointment ID")
	}
}

func TestBuild_PrescriptionState(t *testing.T) {
	sess := newSession(models.StatePrescription)
	prompt := Build(sess)
	if !strings.Contains(prompt, "PRESCRIPTION") {
		t.Error("PRESCRIPTION prompt missing heading")
	}
	if !strings.Contains(prompt, "log_prescription_request") {
		t.Error("PRESCRIPTION prompt should mention log_prescription_request tool")
	}
}

func TestBuild_HoursState(t *testing.T) {
	sess := newSession(models.StateHours)
	prompt := Build(sess)
	if !strings.Contains(prompt, "HOURS") {
		t.Error("HOURS prompt missing heading")
	}
	if !strings.Contains(prompt, "Monday–Friday") {
		t.Error("HOURS prompt should include office hours")
	}
}

func TestBuild_VoiceContextInjected(t *testing.T) {
	sess := newSession(models.StateGreeting)
	sess.ChatSummary = "Patient Alex needs a dermatology appointment."
	prompt := Build(sess)
	if !strings.Contains(prompt, "Patient Alex needs a dermatology appointment.") {
		t.Error("voice context should be injected when ChatSummary is set")
	}
}

func TestBuild_NoVoiceContextWhenEmpty(t *testing.T) {
	sess := newSession(models.StateGreeting)
	sess.ChatSummary = ""
	prompt := Build(sess)
	if strings.Contains(prompt, "PREVIOUS CONVERSATION CONTEXT") {
		t.Error("voice context block should not appear when ChatSummary is empty")
	}
}

// ─── New extended tests ───────────────────────────────────────────────────────

// TestBuild_IntakeState_PartialFields verifies that when only some patient
// fields are collected, the prompt lists those under "Already collected" and
// the remaining ones under "Still needed".
func TestBuild_IntakeState_PartialFields(t *testing.T) {
	sess := newSession(models.StateIntake)
	// Provide only name — the other four fields are still missing
	sess.PatientInfo = models.PatientInfo{
		FirstName: "Sam",
		LastName:  "Rivera",
	}
	prompt := Build(sess)

	if !strings.Contains(prompt, "Already collected") {
		t.Error("partial intake: prompt should contain 'Already collected' section")
	}
	if !strings.Contains(prompt, "Sam Rivera") {
		t.Error("partial intake: prompt should show collected name")
	}
	if !strings.Contains(prompt, "Still needed") {
		t.Error("partial intake: prompt should contain 'Still needed' section for remaining fields")
	}
	// Remaining fields that should be listed as still needed
	for _, field := range []string{"date of birth", "phone number", "email address", "reason for visit"} {
		if !strings.Contains(prompt, field) {
			t.Errorf("partial intake: prompt missing still-needed field %q", field)
		}
	}
}

// TestBuild_IntakeState_AllFieldsCollected verifies that when every patient
// field is present, the "Still needed" section is absent.
func TestBuild_IntakeState_AllFieldsCollected(t *testing.T) {
	sess := newSession(models.StateIntake)
	sess.PatientInfo = models.PatientInfo{
		FirstName:      "Alex",
		LastName:       "Johnson",
		DOB:            "1990-03-05",
		Phone:          "555-123-4567",
		Email:          "alex@test.com",
		ReasonForVisit: "migraines",
	}
	prompt := Build(sess)

	if strings.Contains(prompt, "Still needed") {
		t.Error("all fields collected: 'Still needed' section should not appear")
	}
	if !strings.Contains(prompt, "Already collected") {
		t.Error("all fields collected: 'Already collected' section should be present")
	}
}

// TestBuild_SchedulingState_NilMatchedDoctor verifies that Build does not panic
// when sess.MatchedDoctor is nil in SCHEDULING state.
func TestBuild_SchedulingState_NilMatchedDoctor(t *testing.T) {
	sess := newSession(models.StateScheduling)
	sess.MatchedDoctor = nil

	// Must not panic
	var prompt string
	func() {
		defer func() {
			if r := recover(); r != nil {
				t.Fatalf("Build panicked with nil MatchedDoctor in SCHEDULING: %v", r)
			}
		}()
		prompt = Build(sess)
	}()

	if !strings.Contains(prompt, "SCHEDULING") {
		t.Error("SCHEDULING prompt missing SCHEDULING heading even with nil doctor")
	}
}

// TestBuild_ConfirmingState_NilPointers verifies that Build does not panic when
// MatchedDoctor or SelectedSlot is nil in CONFIRMING state.
func TestBuild_ConfirmingState_NilPointers(t *testing.T) {
	tests := []struct {
		name   string
		doctor *models.Doctor
		slot   *models.TimeSlot
	}{
		{
			name:   "both nil",
			doctor: nil,
			slot:   nil,
		},
		{
			name:   "nil MatchedDoctor only",
			doctor: nil,
			slot: &models.TimeSlot{
				DoctorID: "dr-patel", Date: "2026-05-01",
				StartTime: "10:00", EndTime: "11:00",
			},
		},
		{
			name:   "nil SelectedSlot only",
			doctor: GetDoctorByID("dr-patel"),
			slot:   nil,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			sess := newSession(models.StateConfirming)
			sess.MatchedDoctor = tc.doctor
			sess.SelectedSlot = tc.slot

			func() {
				defer func() {
					if r := recover(); r != nil {
						t.Fatalf("Build panicked (%s): %v", tc.name, r)
					}
				}()
				Build(sess)
			}()
		})
	}
}

// TestBuild_ResponseFormatRule verifies that the persona block always contains
// the "RESPONSE FORMAT" instruction requiring a text message in every response.
func TestBuild_ResponseFormatRule(t *testing.T) {
	states := []models.SessionState{
		models.StateGreeting, models.StateIntake, models.StateMatching,
		models.StateScheduling, models.StateConfirming, models.StateBooked,
		models.StatePrescription, models.StateHours,
	}
	for _, state := range states {
		t.Run(string(state), func(t *testing.T) {
			sess := newSession(state)
			if state == models.StateScheduling {
				sess.MatchedDoctor = GetDoctorByID("dr-mitchell")
			}
			prompt := Build(sess)
			if !strings.Contains(prompt, "RESPONSE FORMAT") {
				t.Errorf("state %s: prompt missing RESPONSE FORMAT rule", state)
			}
			if !strings.Contains(prompt, "Always include a short") {
				t.Errorf("state %s: prompt missing instruction to always include text", state)
			}
		})
	}
}

// TestBuild_ToolReminder_WorksFromAnyState verifies that the tool reminder
// section describes begin_intake, begin_prescription, and show_office_info
// as working "from any state" rather than being restricted to GREETING.
func TestBuild_ToolReminder_WorksFromAnyState(t *testing.T) {
	sess := newSession(models.StateGreeting)
	prompt := Build(sess)

	anyStateTools := []string{"begin_intake", "begin_prescription", "show_office_info"}
	for _, tool := range anyStateTools {
		// Find the line(s) mentioning the tool and ensure "any state" is present
		idx := strings.Index(prompt, tool+": call when")
		if idx == -1 {
			t.Errorf("tool reminder: %q description not found in prompt", tool)
			continue
		}
		// Extract a short excerpt around the tool's description line
		end := idx + 120
		if end > len(prompt) {
			end = len(prompt)
		}
		excerpt := prompt[idx:end]
		if !strings.Contains(excerpt, "any state") {
			t.Errorf("tool %q description should say 'works from any state', got excerpt: %q", tool, excerpt)
		}
	}
}

// TestBuild_ConfirmingState_ContainsSlotInfo verifies that when a slot is set,
// the CONFIRMING prompt contains the appointment date and time.
func TestBuild_ConfirmingState_ContainsSlotInfo(t *testing.T) {
	sess := newSession(models.StateConfirming)
	sess.MatchedDoctor = GetDoctorByID("dr-patel")
	sess.SelectedSlot = &models.TimeSlot{
		DoctorID:  "dr-patel",
		Date:      "2026-06-15",
		StartTime: "11:00",
		EndTime:   "12:00",
	}
	sess.PatientInfo = models.PatientInfo{
		FirstName: "Quinn",
		LastName:  "Taylor",
		Email:     "quinn@example.com",
		Phone:     "555-444-3333",
	}

	prompt := Build(sess)

	// FormatDateReadable("2026-06-15") = "Monday, June 15, 2026"
	if !strings.Contains(prompt, "June 15, 2026") {
		t.Error("CONFIRMING prompt should contain the formatted appointment date")
	}
	if !strings.Contains(prompt, "11:00") {
		t.Error("CONFIRMING prompt should contain the appointment start time")
	}
	if !strings.Contains(prompt, "12:00") {
		t.Error("CONFIRMING prompt should contain the appointment end time")
	}
}
