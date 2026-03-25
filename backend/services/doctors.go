package services

import (
	"fmt"
	"strings"
	"time"

	"kyron-medical/models"
)

// ─── Hardcoded Doctors ────────────────────────────────────────────────────────

var Doctors = []models.Doctor{
	{
		ID:            "dr-mitchell",
		Name:          "Dr. Sarah Mitchell",
		Specialty:     "Cardiology",
		Keywords:      []string{"heart", "chest", "chest pain", "cardiovascular", "palpitations", "blood pressure", "arrhythmia", "shortness of breath", "irregular heartbeat", "cardiac"},
		Bio:           "Board-certified cardiologist with 15+ years at Kyron Medical, specializing in preventive cardiology and arrhythmia management.",
		ImageInitials: "SM",
		Phone:         "(555) 201-0001",
	},
	{
		ID:            "dr-rodriguez",
		Name:          "Dr. James Rodriguez",
		Specialty:     "Orthopedics",
		Keywords:      []string{"knee", "hip", "shoulder", "back", "spine", "bones", "joints", "fracture", "sports injury", "wrist", "ankle", "elbow", "musculoskeletal", "arthritis", "tendon"},
		Bio:           "Orthopedic surgeon specializing in joint replacement, sports medicine, and minimally invasive spine procedures.",
		ImageInitials: "JR",
		Phone:         "(555) 201-0002",
	},
	{
		ID:            "dr-patel",
		Name:          "Dr. Priya Patel",
		Specialty:     "Dermatology",
		Keywords:      []string{"skin", "rash", "acne", "moles", "hair loss", "nails", "eczema", "psoriasis", "itching", "lesion", "dermatitis", "hives", "sunburn", "birthmark", "wart"},
		Bio:           "Dermatologist focused on medical and cosmetic skin conditions, with expertise in skin cancer screening and chronic skin disorders.",
		ImageInitials: "PP",
		Phone:         "(555) 201-0003",
	},
	{
		ID:            "dr-chen",
		Name:          "Dr. David Chen",
		Specialty:     "Gastroenterology",
		Keywords:      []string{"stomach", "digestive", "bowel", "colon", "gut", "nausea", "acid reflux", "bloating", "IBS", "diarrhea", "constipation", "abdominal", "heartburn", "ulcer", "liver", "colonoscopy"},
		Bio:           "Gastroenterologist with expertise in inflammatory bowel disease, colonoscopy, and complex digestive disorders.",
		ImageInitials: "DC",
		Phone:         "(555) 201-0004",
	},
	{
		ID:            "dr-thompson",
		Name:          "Dr. Lisa Thompson",
		Specialty:     "Neurology",
		Keywords:      []string{"brain", "head", "headaches", "migraines", "nerves", "dizziness", "numbness", "seizure", "memory", "tremor", "MS", "stroke", "tingling", "vertigo", "concussion", "neuropathy"},
		Bio:           "Neurologist specializing in headache disorders, movement conditions, and multiple sclerosis management.",
		ImageInitials: "LT",
		Phone:         "(555) 201-0005",
	},
}

// GetDoctorByID returns a doctor by ID, or nil if not found.
func GetDoctorByID(id string) *models.Doctor {
	// Exact match first
	for i := range Doctors {
		if Doctors[i].ID == id {
			return &Doctors[i]
		}
	}
	// Normalize: lowercase, replace underscores/spaces/dots with dashes, collapse repeats
	r := strings.NewReplacer("_", "-", " ", "-", ".", "")
	norm := strings.ToLower(r.Replace(id))
	for i := range Doctors {
		if Doctors[i].ID == norm {
			return &Doctors[i]
		}
	}
	// Last-name match — handles "dr-david-chen", "david-chen", "chen", etc.
	// Doctor names are "Dr. Firstname Lastname" → last word is the last name.
	for i := range Doctors {
		parts := strings.Fields(Doctors[i].Name)
		lastName := strings.ToLower(parts[len(parts)-1]) // e.g. "chen"
		if strings.Contains(norm, lastName) {
			return &Doctors[i]
		}
	}
	return nil
}

// DoctorIDs returns all valid doctor IDs — used in debug logs.
func DoctorIDs() []string {
	ids := make([]string, len(Doctors))
	for i, d := range Doctors {
		ids[i] = d.ID
	}
	return ids
}

// ─── Availability Generator ───────────────────────────────────────────────────

// bookedSlots tracks confirmed appointments (doctorId|date|startTime)
var bookedSlots = map[string]bool{}

// BookSlot marks a slot as booked.
func BookSlot(doctorID, date, startTime string) {
	bookedSlots[slotKey(doctorID, date, startTime)] = true
}

// IsSlotBooked checks if a slot is already booked.
func IsSlotBooked(doctorID, date, startTime string) bool {
	return bookedSlots[slotKey(doctorID, date, startTime)]
}

func slotKey(doctorID, date, startTime string) string {
	return doctorID + "|" + date + "|" + startTime
}

// djb2 hash for deterministic pre-blocking
func djb2(s string) uint32 {
	var h uint32 = 5381
	for _, c := range s {
		h = ((h << 5) + h) + uint32(c)
	}
	return h
}

var slotHours = []string{"09:00", "10:00", "11:00", "13:00", "14:00", "15:00", "16:00"}

// GenerateAvailability returns available slots for a doctor from today+7 to today+60 days.
// Mon/Wed/Fri only, 09:00–16:00 hourly (no 12:00 lunch slot).
// ~20% of slots are deterministically pre-blocked to simulate real schedules.
func GenerateAvailability(doctorID string) []models.TimeSlot {
	today := time.Now().Truncate(24 * time.Hour)
	start := today.AddDate(0, 0, 7)
	end := today.AddDate(0, 0, 60)

	var slots []models.TimeSlot

	for d := start; !d.After(end); d = d.AddDate(0, 0, 1) {
		weekday := d.Weekday()
		if weekday != time.Monday && weekday != time.Wednesday && weekday != time.Friday {
			continue
		}

		dateStr := d.Format("2006-01-02")

		for _, hour := range slotHours {
			// Deterministic pre-blocking: ~20% of slots blocked
			key := doctorID + dateStr + hour
			blocked := djb2(key)%5 == 0

			// Also check runtime booked slots
			alreadyBooked := IsSlotBooked(doctorID, dateStr, hour)

			endHour := endTimeForSlot(hour)

			slots = append(slots, models.TimeSlot{
				DoctorID:  doctorID,
				Date:      dateStr,
				StartTime: hour,
				EndTime:   endHour,
				Available: !blocked && !alreadyBooked,
			})
		}
	}

	return slots
}

func endTimeForSlot(start string) string {
	hourMap := map[string]string{
		"09:00": "10:00",
		"10:00": "11:00",
		"11:00": "12:00",
		"13:00": "14:00",
		"14:00": "15:00",
		"15:00": "16:00",
		"16:00": "17:00",
	}
	if end, ok := hourMap[start]; ok {
		return end
	}
	return ""
}

// FormatDateReadable returns "Wednesday, April 2, 2026"
func FormatDateReadable(dateStr string) string {
	t, err := time.Parse("2006-01-02", dateStr)
	if err != nil {
		return dateStr
	}
	return t.Format("Monday, January 2, 2006")
}

// DoctorListForPrompt returns a formatted string of all doctors for injection into system prompt.
func DoctorListForPrompt() string {
	result := ""
	for _, d := range Doctors {
		result += fmt.Sprintf("- %s (ID: %s) — %s\n  Keywords: %v\n  Bio: %s\n\n",
			d.Name, d.ID, d.Specialty, d.Keywords, d.Bio)
	}
	return result
}
