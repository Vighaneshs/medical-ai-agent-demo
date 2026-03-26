package models

import "time"

// ─── Session State Machine ────────────────────────────────────────────────────

type SessionState string

const (
	StateGreeting     SessionState = "GREETING"
	StateIntake       SessionState = "INTAKE"
	StateMatching     SessionState = "MATCHING"
	StateScheduling   SessionState = "SCHEDULING"
	StateConfirming   SessionState = "CONFIRMING"
	StateBooked       SessionState = "BOOKED"
	StatePrescription SessionState = "PRESCRIPTION"
	StateHours        SessionState = "HOURS"
)

// ─── Doctor ───────────────────────────────────────────────────────────────────

type Doctor struct {
	ID             string   `json:"id"`
	Name           string   `json:"name"`
	Specialty      string   `json:"specialty"`
	Keywords       []string `json:"keywords"`
	Bio            string   `json:"bio"`
	ImageInitials  string   `json:"imageInitials"`
	Phone          string   `json:"phone"`
}

// ─── Availability ─────────────────────────────────────────────────────────────

type TimeSlot struct {
	DoctorID  string `json:"doctorId"`
	Date      string `json:"date"`      // "2026-04-01"
	StartTime string `json:"startTime"` // "09:00"
	EndTime   string `json:"endTime"`   // "10:00"
	Available bool   `json:"available"`
}

// ─── Patient ──────────────────────────────────────────────────────────────────

type PatientInfo struct {
	FirstName      string `json:"firstName"`
	LastName       string `json:"lastName"`
	DOB            string `json:"dob"`   // "1990-05-15"
	Phone          string `json:"phone"`
	Email          string `json:"email"`
	ReasonForVisit string `json:"reasonForVisit"`
	SMSOptIn       bool   `json:"smsOptIn"`
}

// ─── Appointment ──────────────────────────────────────────────────────────────

type Appointment struct {
	ID                 string      `json:"id"`
	SessionID          string      `json:"sessionId"`
	Doctor             Doctor      `json:"doctor"`
	Slot               TimeSlot    `json:"slot"`
	Patient            PatientInfo `json:"patient"`
	BookedAt           time.Time   `json:"bookedAt"`
	ReminderScheduled  bool        `json:"reminderScheduled"`
}

// ─── Chat ─────────────────────────────────────────────────────────────────────

type ChatMessage struct {
	ID        string    `json:"id"`
	Role      string    `json:"role"` // "user" | "assistant"
	Content   string    `json:"content"`
	CreatedAt time.Time `json:"createdAt"`
}

// ─── Session ──────────────────────────────────────────────────────────────────

type Session struct {
	ID                string        `json:"id"`
	State             SessionState  `json:"state"`
	Messages          []ChatMessage `json:"messages"`
	PatientInfo       PatientInfo   `json:"patientInfo"`
	MatchedDoctor     *Doctor       `json:"matchedDoctor"`
	SelectedSlot      *TimeSlot     `json:"selectedSlot"`
	Appointment       *Appointment  `json:"appointment"`
	PhoneNumber       string        `json:"phoneNumber"` // for inbound call lookup
	ChatSummary       string        `json:"chatSummary"` // 2-sentence summary for voice handoff
	LastCallDroppedAt time.Time     `json:"lastCallDroppedAt"`
	CreatedAt         time.Time     `json:"createdAt"`
	LastActivityAt    time.Time     `json:"lastActivityAt"`
}

// ─── API Request / Response ───────────────────────────────────────────────────

type ChatRequest struct {
	SessionID string `json:"sessionId"`
	Message   string `json:"message"`
}

type BookRequest struct {
	SessionID   string      `json:"sessionId"`
	DoctorID    string      `json:"doctorId"`
	Slot        TimeSlot    `json:"slot"`
	PatientInfo PatientInfo `json:"patientInfo"`
}

type VoiceInitiateRequest struct {
	SessionID string `json:"sessionId"`
}

type VoiceInitiateResponse struct {
	AssistantID      string                 `json:"assistantId"`
	AssistantOverrides map[string]interface{} `json:"assistantOverrides"`
}

type SSEChunk struct {
	Text string `json:"text,omitempty"`
}

type SSEDone struct {
	Done          bool      `json:"done"`
	NewState      string    `json:"newState"`
	DoctorID      string    `json:"doctorId,omitempty"`
	SelectedSlot  *TimeSlot `json:"selectedSlot,omitempty"`
	AppointmentID string    `json:"appointmentId,omitempty"`
}

type SSEError struct {
	Error string `json:"error"`
}
