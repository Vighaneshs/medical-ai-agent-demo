package services

import (
	"testing"
	"time"

	"kyron-medical/models"
)

// newTestStore creates an in-memory SQLite session store for testing.
func newTestStore(t *testing.T) *SessionStore {
	t.Helper()
	if err := InitSessionStore(":memory:"); err != nil {
		t.Fatalf("InitSessionStore: %v", err)
	}
	return Store
}

func TestInitSessionStore_Success(t *testing.T) {
	store := newTestStore(t)
	if store == nil {
		t.Fatal("expected non-nil store")
	}
}

func TestDBPath_Default(t *testing.T) {
	t.Setenv("DB_PATH", "")
	if got := DBPath(); got != "./kyron.db" {
		t.Errorf("DBPath() = %q, want ./kyron.db", got)
	}
}

func TestDBPath_EnvOverride(t *testing.T) {
	t.Setenv("DB_PATH", "/tmp/test.db")
	if got := DBPath(); got != "/tmp/test.db" {
		t.Errorf("DBPath() = %q, want /tmp/test.db", got)
	}
}

func TestGetOrCreate_NewSession(t *testing.T) {
	store := newTestStore(t)
	sess := store.GetOrCreate("new-session-1")
	if sess == nil {
		t.Fatal("expected non-nil session")
	}
	if sess.ID != "new-session-1" {
		t.Errorf("got ID %q, want new-session-1", sess.ID)
	}
	if sess.State != models.StateGreeting {
		t.Errorf("new session state = %q, want GREETING", sess.State)
	}
}

func TestGetOrCreate_ExistingFromCache(t *testing.T) {
	store := newTestStore(t)
	sess1 := store.GetOrCreate("cached-session")
	sess2 := store.GetOrCreate("cached-session")
	// Should be the same pointer (from cache)
	if sess1 != sess2 {
		t.Error("expected same session pointer from cache on second call")
	}
}

func TestGetOrCreate_PersistsToDBAndLoadsBack(t *testing.T) {
	store := newTestStore(t)
	sess := store.GetOrCreate("persist-test")
	sess.State = models.StateIntake
	sess.PatientInfo = models.PatientInfo{FirstName: "Alice", Email: "alice@test.com"}
	store.Save(sess)

	// Clear cache and reload from DB
	store.cache.Delete("persist-test")

	loaded := store.GetOrCreate("persist-test")
	if loaded.State != models.StateIntake {
		t.Errorf("loaded state = %q, want INTAKE", loaded.State)
	}
	if loaded.PatientInfo.FirstName != "Alice" {
		t.Errorf("loaded FirstName = %q, want Alice", loaded.PatientInfo.FirstName)
	}
}

func TestSave_UpdatesState(t *testing.T) {
	store := newTestStore(t)
	sess := store.GetOrCreate("save-state-test")
	sess.State = models.StateMatching
	store.Save(sess)

	store.cache.Delete("save-state-test")
	loaded := store.GetOrCreate("save-state-test")
	if loaded.State != models.StateMatching {
		t.Errorf("saved state = MATCHING, loaded = %q", loaded.State)
	}
}

func TestSave_UpdatesLastActivityAt(t *testing.T) {
	store := newTestStore(t)
	sess := store.GetOrCreate("activity-test")
	before := time.Now().Add(-time.Second)
	store.Save(sess)
	if sess.LastActivityAt.Before(before) {
		t.Error("Save should update LastActivityAt")
	}
}

func TestAppendMessage_AddsToSession(t *testing.T) {
	store := newTestStore(t)
	sess := store.GetOrCreate("msg-test")
	store.AppendMessage(sess, "user", "Hello there")
	store.AppendMessage(sess, "assistant", "Hi! How can I help?")

	if len(sess.Messages) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(sess.Messages))
	}
	if sess.Messages[0].Role != "user" || sess.Messages[0].Content != "Hello there" {
		t.Errorf("first message wrong: %+v", sess.Messages[0])
	}
	if sess.Messages[1].Role != "assistant" {
		t.Errorf("second message role = %q, want assistant", sess.Messages[1].Role)
	}
}

func TestAppendMessage_PersistedToDB(t *testing.T) {
	store := newTestStore(t)
	sess := store.GetOrCreate("msg-persist-test")
	store.AppendMessage(sess, "user", "Persisted message")
	store.Save(sess)

	// Reload from DB
	store.cache.Delete("msg-persist-test")
	loaded := store.GetOrCreate("msg-persist-test")

	if len(loaded.Messages) == 0 {
		t.Fatal("expected messages to be loaded from DB")
	}
	found := false
	for _, m := range loaded.Messages {
		if m.Content == "Persisted message" {
			found = true
			break
		}
	}
	if !found {
		t.Error("persisted message not found after DB reload")
	}
}

func TestGetByPhone_ReturnsMatchingSession(t *testing.T) {
	store := newTestStore(t)
	sess := store.GetOrCreate("phone-lookup-test")
	sess.PhoneNumber = "555-999-8888"
	store.Save(sess)

	found := store.GetByPhone("555-999-8888")
	if found == nil {
		t.Fatal("expected to find session by phone")
	}
	if found.ID != "phone-lookup-test" {
		t.Errorf("found wrong session: %q", found.ID)
	}
}

func TestGetByPhone_ReturnsNilForUnknownPhone(t *testing.T) {
	store := newTestStore(t)
	found := store.GetByPhone("000-000-0000")
	if found != nil {
		t.Errorf("expected nil for unknown phone, got session %q", found.ID)
	}
}

func TestSave_PersistsMatchedDoctorAndSlot(t *testing.T) {
	store := newTestStore(t)
	sess := store.GetOrCreate("doctor-slot-test")
	sess.MatchedDoctor = GetDoctorByID("dr-chen")
	sess.SelectedSlot = &models.TimeSlot{
		DoctorID:  "dr-chen",
		Date:      "2026-05-01",
		StartTime: "14:00",
		EndTime:   "15:00",
		Available: false,
	}
	store.Save(sess)

	store.cache.Delete("doctor-slot-test")
	loaded := store.GetOrCreate("doctor-slot-test")

	if loaded.MatchedDoctor == nil || loaded.MatchedDoctor.ID != "dr-chen" {
		t.Error("matched doctor not persisted correctly")
	}
	if loaded.SelectedSlot == nil || loaded.SelectedSlot.Date != "2026-05-01" {
		t.Error("selected slot not persisted correctly")
	}
}
