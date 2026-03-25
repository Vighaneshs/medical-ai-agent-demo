package services

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"sync"
	"time"

	"github.com/google/uuid"
	"kyron-medical/models"
	_ "modernc.org/sqlite"
)

// SessionStore is a write-through cache: in-memory for fast reads,
// SQLite for persistence across restarts.
type SessionStore struct {
	db    *sql.DB
	cache sync.Map // map[string]*models.Session
}

var Store *SessionStore

// InitSessionStore opens (or creates) the SQLite database and runs migrations.
func InitSessionStore(dbPath string) error {
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return fmt.Errorf("open db: %w", err)
	}

	if err := db.Ping(); err != nil {
		return fmt.Errorf("ping db: %w", err)
	}

	// Enable WAL mode for better concurrent read/write performance
	if _, err := db.Exec(`PRAGMA journal_mode=WAL`); err != nil {
		return fmt.Errorf("wal mode: %w", err)
	}

	if err := migrate(db); err != nil {
		return fmt.Errorf("migrate: %w", err)
	}

	Store = &SessionStore{db: db}

	// Start background pruning goroutine
	go Store.pruneLoop()

	log.Printf("Session store initialized: %s", dbPath)
	return nil
}

func migrate(db *sql.DB) error {
	schema := `
	CREATE TABLE IF NOT EXISTS sessions (
		id              TEXT PRIMARY KEY,
		state           TEXT NOT NULL DEFAULT 'GREETING',
		patient_info    TEXT,
		matched_doctor  TEXT,
		selected_slot   TEXT,
		appointment     TEXT,
		phone_number    TEXT,
		chat_summary    TEXT,
		created_at      INTEGER NOT NULL,
		last_activity   INTEGER NOT NULL
	);
	CREATE INDEX IF NOT EXISTS idx_sessions_phone ON sessions(phone_number);

	CREATE TABLE IF NOT EXISTS messages (
		id          TEXT PRIMARY KEY,
		session_id  TEXT NOT NULL REFERENCES sessions(id) ON DELETE CASCADE,
		role        TEXT NOT NULL CHECK(role IN ('user', 'assistant')),
		content     TEXT NOT NULL,
		created_at  INTEGER NOT NULL
	);
	CREATE INDEX IF NOT EXISTS idx_messages_session ON messages(session_id);

	CREATE TABLE IF NOT EXISTS booked_slots (
		doctor_id   TEXT NOT NULL,
		date        TEXT NOT NULL,
		start_time  TEXT NOT NULL,
		session_id  TEXT NOT NULL REFERENCES sessions(id),
		PRIMARY KEY (doctor_id, date, start_time)
	);
	`
	_, err := db.Exec(schema)
	return err
}

// GetOrCreate retrieves a session from cache or DB, creating it if new.
func (s *SessionStore) GetOrCreate(id string) *models.Session {
	// Fast path: in-memory cache
	if cached, ok := s.cache.Load(id); ok {
		sess := cached.(*models.Session)
		sess.LastActivityAt = time.Now()
		return sess
	}

	// Try loading from DB
	sess, err := s.loadFromDB(id)
	if err == nil && sess != nil {
		sess.LastActivityAt = time.Now()
		s.cache.Store(id, sess)
		return sess
	}

	// Create new session
	sess = &models.Session{
		ID:             id,
		State:          models.StateGreeting,
		Messages:       []models.ChatMessage{},
		CreatedAt:      time.Now(),
		LastActivityAt: time.Now(),
	}
	s.cache.Store(id, sess)
	if err := s.writeToDB(sess); err != nil {
		log.Printf("warn: could not persist new session %s: %v", id, err)
	}
	return sess
}

// GetByPhone finds the most recent session for a given phone number (for voice callback).
func (s *SessionStore) GetByPhone(phone string) *models.Session {
	row := s.db.QueryRow(
		`SELECT id FROM sessions WHERE phone_number = ? ORDER BY last_activity DESC LIMIT 1`,
		phone,
	)
	var id string
	if err := row.Scan(&id); err != nil {
		return nil
	}
	return s.GetOrCreate(id)
}

// Save persists a session to cache and DB.
func (s *SessionStore) Save(sess *models.Session) {
	sess.LastActivityAt = time.Now()
	s.cache.Store(sess.ID, sess)
	if err := s.writeToDB(sess); err != nil {
		log.Printf("warn: could not save session %s: %v", sess.ID, err)
	}
}

// AppendMessage adds a message to a session and persists it.
func (s *SessionStore) AppendMessage(sess *models.Session, role, content string) {
	msg := models.ChatMessage{
		ID:        uuid.New().String(),
		Role:      role,
		Content:   content,
		CreatedAt: time.Now(),
	}
	sess.Messages = append(sess.Messages, msg)

	// Persist message to DB
	_, err := s.db.Exec(
		`INSERT INTO messages (id, session_id, role, content, created_at) VALUES (?, ?, ?, ?, ?)`,
		msg.ID, sess.ID, msg.Role, msg.Content, msg.CreatedAt.Unix(),
	)
	if err != nil {
		log.Printf("warn: could not persist message for session %s: %v", sess.ID, err)
	}
}

// ─── DB helpers ───────────────────────────────────────────────────────────────

func (s *SessionStore) writeToDB(sess *models.Session) error {
	patientJSON, _ := json.Marshal(sess.PatientInfo)
	doctorJSON, _ := json.Marshal(sess.MatchedDoctor)
	slotJSON, _ := json.Marshal(sess.SelectedSlot)
	apptJSON, _ := json.Marshal(sess.Appointment)

	_, err := s.db.Exec(`
		INSERT INTO sessions (id, state, patient_info, matched_doctor, selected_slot, appointment, phone_number, chat_summary, created_at, last_activity)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			state = excluded.state,
			patient_info = excluded.patient_info,
			matched_doctor = excluded.matched_doctor,
			selected_slot = excluded.selected_slot,
			appointment = excluded.appointment,
			phone_number = excluded.phone_number,
			chat_summary = excluded.chat_summary,
			last_activity = excluded.last_activity
	`,
		sess.ID, string(sess.State), string(patientJSON), string(doctorJSON),
		string(slotJSON), string(apptJSON), sess.PhoneNumber, sess.ChatSummary,
		sess.CreatedAt.Unix(), sess.LastActivityAt.Unix(),
	)
	return err
}

func (s *SessionStore) loadFromDB(id string) (*models.Session, error) {
	row := s.db.QueryRow(`
		SELECT id, state, patient_info, matched_doctor, selected_slot, appointment,
		       phone_number, chat_summary, created_at, last_activity
		FROM sessions WHERE id = ?`, id)

	var (
		sessID, state, patientJSON, doctorJSON, slotJSON, apptJSON string
		phoneNumber, chatSummary                                    string
		createdAt, lastActivity                                     int64
	)

	err := row.Scan(&sessID, &state, &patientJSON, &doctorJSON, &slotJSON, &apptJSON,
		&phoneNumber, &chatSummary, &createdAt, &lastActivity)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	sess := &models.Session{
		ID:             sessID,
		State:          models.SessionState(state),
		PhoneNumber:    phoneNumber,
		ChatSummary:    chatSummary,
		CreatedAt:      time.Unix(createdAt, 0),
		LastActivityAt: time.Unix(lastActivity, 0),
	}

	json.Unmarshal([]byte(patientJSON), &sess.PatientInfo)
	json.Unmarshal([]byte(doctorJSON), &sess.MatchedDoctor)
	json.Unmarshal([]byte(slotJSON), &sess.SelectedSlot)
	json.Unmarshal([]byte(apptJSON), &sess.Appointment)

	// Load last 50 messages from DB
	rows, err := s.db.Query(`
		SELECT id, role, content, created_at FROM messages
		WHERE session_id = ? ORDER BY created_at ASC LIMIT 50`, id)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var msg models.ChatMessage
			var ts int64
			rows.Scan(&msg.ID, &msg.Role, &msg.Content, &ts)
			msg.CreatedAt = time.Unix(ts, 0)
			sess.Messages = append(sess.Messages, msg)
		}
	}

	return sess, nil
}

// pruneLoop removes sessions older than 24 hours every 30 minutes.
func (s *SessionStore) pruneLoop() {
	ticker := time.NewTicker(30 * time.Minute)
	defer ticker.Stop()
	for range ticker.C {
		cutoff := time.Now().Add(-24 * time.Hour).Unix()
		result, err := s.db.Exec(`DELETE FROM sessions WHERE last_activity < ?`, cutoff)
		if err == nil {
			if n, _ := result.RowsAffected(); n > 0 {
				log.Printf("pruned %d expired sessions", n)
			}
		}
		// Also prune in-memory cache
		s.cache.Range(func(k, v interface{}) bool {
			if sess, ok := v.(*models.Session); ok {
				if time.Since(sess.LastActivityAt) > 24*time.Hour {
					s.cache.Delete(k)
				}
			}
			return true
		})
	}
}

// DBPath returns the path for the SQLite database file.
func DBPath() string {
	if path := os.Getenv("DB_PATH"); path != "" {
		return path
	}
	return "./kyron.db"
}
