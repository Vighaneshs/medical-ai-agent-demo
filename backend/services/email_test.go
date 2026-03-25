package services

import (
	"strings"
	"testing"
	"time"

	"kyron-medical/models"
)

func TestRemoveChars(t *testing.T) {
	tests := []struct {
		s, chars, want string
	}{
		{"2026-04-01", "-", "20260401"},
		{"10:00", ":", "1000"},
		{"2026-04-01", "", "2026-04-01"},
		{"hello", "xyz", "hello"},
		{"aabbcc", "b", "aacc"},
		{"", "-:", ""},
	}
	for _, tc := range tests {
		t.Run(tc.s+"/"+tc.chars, func(t *testing.T) {
			got := removeChars(tc.s, tc.chars)
			if got != tc.want {
				t.Errorf("removeChars(%q, %q) = %q, want %q", tc.s, tc.chars, got, tc.want)
			}
		})
	}
}

func TestReplaceAll(t *testing.T) {
	tests := []struct {
		s, old, new_, want string
	}{
		{"hello world", "world", "Go", "hello Go"},
		{"aaa", "a", "b", "bbb"},
		{"no match", "xyz", "abc", "no match"},
		{"", "a", "b", ""},
		{"abcabc", "abc", "", ""},
	}
	for _, tc := range tests {
		t.Run(tc.s, func(t *testing.T) {
			got := replaceAll(tc.s, tc.old, tc.new_)
			if got != tc.want {
				t.Errorf("replaceAll(%q, %q, %q) = %q, want %q", tc.s, tc.old, tc.new_, got, tc.want)
			}
		})
	}
}

func testAppointment() *models.Appointment {
	return &models.Appointment{
		ID: "abc12345-0000-0000-0000-000000000000",
		Doctor: models.Doctor{
			Name:      "Dr. Lisa Thompson",
			Specialty: "Neurology",
		},
		Slot: models.TimeSlot{
			Date:      "2026-04-01",
			StartTime: "10:00",
			EndTime:   "11:00",
		},
		Patient: models.PatientInfo{
			FirstName: "Alex",
			Email:     "alex@test.com",
		},
	}
}

func TestBuildConfirmationHTML_ContainsKeyFields(t *testing.T) {
	appt := testAppointment()
	html := buildConfirmationHTML(appt)

	checks := []string{
		"Alex",
		"Dr. Lisa Thompson",
		"Neurology",
		"Wednesday, April 1, 2026",
		"10:00",
		"11:00",
		"abc12345",
		"Kyron Medical",
		"1250 Healthcare Blvd",
	}
	for _, want := range checks {
		if !strings.Contains(html, want) {
			t.Errorf("confirmation HTML missing %q", want)
		}
	}
}

func TestBuildConfirmationHTML_GoogleCalendarLink(t *testing.T) {
	appt := testAppointment()
	html := buildConfirmationHTML(appt)
	if !strings.Contains(html, "calendar.google.com") {
		t.Error("confirmation HTML should contain Google Calendar link")
	}
	// Date should be formatted without dashes: 20260401
	if !strings.Contains(html, "20260401") {
		t.Error("calendar link should contain date without dashes")
	}
}

func TestBuildReminderHTML_ContainsKeyFields(t *testing.T) {
	appt := testAppointment()
	html := buildReminderHTML(appt)

	checks := []string{
		"Alex",
		"Dr. Lisa Thompson",
		"Wednesday, April 1, 2026",
		"10:00",
		"Kyron Medical",
	}
	for _, want := range checks {
		if !strings.Contains(html, want) {
			t.Errorf("reminder HTML missing %q", want)
		}
	}
}

func TestScheduleReminder_SkipsPastAppointments(t *testing.T) {
	appt := &models.Appointment{
		ID: "past-0000-0000-0000-000000000000",
		Slot: models.TimeSlot{
			Date:      "2020-01-01", // past
			StartTime: "09:00",
		},
		Patient: models.PatientInfo{Email: "test@test.com"},
	}
	// Should not panic; reminder time is in the past so AfterFunc is skipped
	ScheduleReminder(appt)
}

func TestScheduleReminder_SchedulesFutureAppointment(t *testing.T) {
	// Appointment 25 hours from now (reminder fires in 1 hour)
	future := time.Now().Add(25 * time.Hour)
	appt := &models.Appointment{
		ID: "future-0000-0000-0000-000000000000",
		Slot: models.TimeSlot{
			Date:      future.Format("2006-01-02"),
			StartTime: future.Format("15:04"),
			EndTime:   future.Add(time.Hour).Format("15:04"),
		},
		Patient: models.PatientInfo{Email: "future@test.com"},
	}
	// Should not panic — just schedules a time.AfterFunc
	ScheduleReminder(appt)
}

func TestSendConfirmationSMS_NoCredentials(t *testing.T) {
	t.Setenv("TWILIO_ACCOUNT_SID", "")
	t.Setenv("TWILIO_AUTH_TOKEN", "")
	t.Setenv("TWILIO_PHONE_NUMBER", "")

	appt := testAppointment()
	appt.Patient.Phone = "555-000-0001"
	// Should not panic — just logs and returns
	SendConfirmationSMS(appt)
}

func TestSendConfirmationEmail_NoCredentials(t *testing.T) {
	t.Setenv("RESEND_API_KEY", "")
	t.Setenv("RESEND_FROM_EMAIL", "")

	appt := testAppointment()
	// Should not panic — just logs and returns
	SendConfirmationEmail(appt)
}
