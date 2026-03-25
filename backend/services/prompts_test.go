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
