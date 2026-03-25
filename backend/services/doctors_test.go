package services

import (
	"strings"
	"testing"
	"time"
)

func TestGetDoctorByID(t *testing.T) {
	tests := []struct {
		id       string
		wantNil  bool
		wantName string
	}{
		{"dr-mitchell", false, "Dr. Sarah Mitchell"},
		{"dr-rodriguez", false, "Dr. James Rodriguez"},
		{"dr-patel", false, "Dr. Priya Patel"},
		{"dr-chen", false, "Dr. David Chen"},
		{"dr-thompson", false, "Dr. Lisa Thompson"},
		{"dr-unknown", true, ""},
		{"", true, ""},
		{"DR-MITCHELL", false, "Dr. Sarah Mitchell"}, // normalized to lowercase
	}

	for _, tc := range tests {
		t.Run(tc.id, func(t *testing.T) {
			doc := GetDoctorByID(tc.id)
			if tc.wantNil {
				if doc != nil {
					t.Errorf("GetDoctorByID(%q) = %v, want nil", tc.id, doc)
				}
				return
			}
			if doc == nil {
				t.Fatalf("GetDoctorByID(%q) = nil, want %q", tc.id, tc.wantName)
			}
			if doc.Name != tc.wantName {
				t.Errorf("got name %q, want %q", doc.Name, tc.wantName)
			}
		})
	}
}

func TestGetDoctorByID_ReturnsPointerToSliceElement(t *testing.T) {
	doc := GetDoctorByID("dr-patel")
	if doc == nil {
		t.Fatal("expected non-nil doctor")
	}
	if doc.ID != "dr-patel" {
		t.Errorf("got id %q, want dr-patel", doc.ID)
	}
	if doc.Specialty != "Dermatology" {
		t.Errorf("got specialty %q, want Dermatology", doc.Specialty)
	}
}

func TestBookSlot_IsSlotBooked(t *testing.T) {
	// Use unique IDs to avoid interference with other tests
	doctorID := "test-doc-bookslot"
	date := "2099-01-15"
	startTime := "09:00"

	if IsSlotBooked(doctorID, date, startTime) {
		t.Fatal("slot should not be booked before BookSlot call")
	}

	BookSlot(doctorID, date, startTime)

	if !IsSlotBooked(doctorID, date, startTime) {
		t.Error("slot should be booked after BookSlot call")
	}
}

func TestIsSlotBooked_DifferentKeys(t *testing.T) {
	BookSlot("test-doc-diff", "2099-02-01", "10:00")

	// Different doctor
	if IsSlotBooked("other-doc", "2099-02-01", "10:00") {
		t.Error("different doctorID should not be booked")
	}
	// Different date
	if IsSlotBooked("test-doc-diff", "2099-02-02", "10:00") {
		t.Error("different date should not be booked")
	}
	// Different time
	if IsSlotBooked("test-doc-diff", "2099-02-01", "11:00") {
		t.Error("different time should not be booked")
	}
}

func TestGenerateAvailability_Count(t *testing.T) {
	slots := GenerateAvailability("dr-mitchell")
	if len(slots) == 0 {
		t.Fatal("expected non-empty slots")
	}
}

func TestGenerateAvailability_WeekdaysOnly(t *testing.T) {
	slots := GenerateAvailability("dr-rodriguez")
	for _, s := range slots {
		d, err := time.Parse("2006-01-02", s.Date)
		if err != nil {
			t.Fatalf("invalid date %q: %v", s.Date, err)
		}
		w := d.Weekday()
		if w != time.Monday && w != time.Wednesday && w != time.Friday {
			t.Errorf("slot on %v (%s) — only Mon/Wed/Fri expected", w, s.Date)
		}
	}
}

func TestGenerateAvailability_DateRange(t *testing.T) {
	slots := GenerateAvailability("dr-patel")
	today := time.Now().Truncate(24 * time.Hour)
	earliest := today.AddDate(0, 0, 7)
	latest := today.AddDate(0, 0, 60)

	for _, s := range slots {
		d, _ := time.Parse("2006-01-02", s.Date)
		if d.Before(earliest) {
			t.Errorf("slot date %s is before today+7 (%s)", s.Date, earliest.Format("2006-01-02"))
		}
		if d.After(latest) {
			t.Errorf("slot date %s is after today+60 (%s)", s.Date, latest.Format("2006-01-02"))
		}
	}
}

func TestGenerateAvailability_ValidStartTimes(t *testing.T) {
	valid := map[string]bool{
		"09:00": true, "10:00": true, "11:00": true,
		"13:00": true, "14:00": true, "15:00": true, "16:00": true,
	}
	slots := GenerateAvailability("dr-chen")
	for _, s := range slots {
		if !valid[s.StartTime] {
			t.Errorf("unexpected start time %q", s.StartTime)
		}
		if s.StartTime == "12:00" {
			t.Error("12:00 (lunch) should not appear")
		}
	}
}

func TestGenerateAvailability_EndTimesMatch(t *testing.T) {
	endFor := map[string]string{
		"09:00": "10:00", "10:00": "11:00", "11:00": "12:00",
		"13:00": "14:00", "14:00": "15:00", "15:00": "16:00", "16:00": "17:00",
	}
	for _, s := range GenerateAvailability("dr-thompson") {
		want, ok := endFor[s.StartTime]
		if !ok {
			t.Errorf("unexpected start time %q", s.StartTime)
			continue
		}
		if s.EndTime != want {
			t.Errorf("start %q: got endTime %q, want %q", s.StartTime, s.EndTime, want)
		}
	}
}

func TestGenerateAvailability_DoctorIDSet(t *testing.T) {
	slots := GenerateAvailability("dr-mitchell")
	for _, s := range slots {
		if s.DoctorID != "dr-mitchell" {
			t.Errorf("expected doctorID dr-mitchell, got %q", s.DoctorID)
		}
	}
}

func TestGenerateAvailability_SomeAvailable(t *testing.T) {
	slots := GenerateAvailability("dr-rodriguez")
	var available int
	for _, s := range slots {
		if s.Available {
			available++
		}
	}
	if available == 0 {
		t.Error("expected some available slots")
	}
	// djb2 blocks ~20%, so at least 70% should be available
	pct := float64(available) / float64(len(slots))
	if pct < 0.70 {
		t.Errorf("only %.0f%% of slots available, expected ≥70%%", pct*100)
	}
}

func TestFormatDateReadable(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"2026-04-01", "Wednesday, April 1, 2026"},
		{"2026-01-05", "Monday, January 5, 2026"},
		{"2026-12-25", "Friday, December 25, 2026"},
		{"bad-date", "bad-date"}, // returns input on error
	}

	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			got := FormatDateReadable(tc.input)
			if got != tc.want {
				t.Errorf("FormatDateReadable(%q) = %q, want %q", tc.input, got, tc.want)
			}
		})
	}
}

func TestDoctorListForPrompt_ContainsAllDoctors(t *testing.T) {
	list := DoctorListForPrompt()
	expected := []string{
		"Dr. Sarah Mitchell", "dr-mitchell", "Cardiology",
		"Dr. James Rodriguez", "dr-rodriguez", "Orthopedics",
		"Dr. Priya Patel", "dr-patel", "Dermatology",
		"Dr. David Chen", "dr-chen", "Gastroenterology",
		"Dr. Lisa Thompson", "dr-thompson", "Neurology",
	}
	for _, want := range expected {
		if !strings.Contains(list, want) {
			t.Errorf("DoctorListForPrompt missing %q", want)
		}
	}
}
