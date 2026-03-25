package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"kyron-medical/models"
	"kyron-medical/services"
)

// ─── vapiLLMConfig ────────────────────────────────────────────────────────────

func TestVapiLLMConfig_DefaultsToAnthropic(t *testing.T) {
	t.Setenv("AI_PROVIDER", "")
	cfg := vapiLLMConfig("test prompt")

	if cfg["provider"] != "anthropic" {
		t.Errorf("provider = %q, want anthropic", cfg["provider"])
	}
	if cfg["model"] != "claude-sonnet-4-6" {
		t.Errorf("model = %q, want claude-sonnet-4-6", cfg["model"])
	}
}

func TestVapiLLMConfig_ClaudeExplicit(t *testing.T) {
	t.Setenv("AI_PROVIDER", "claude")
	cfg := vapiLLMConfig("test prompt")

	if cfg["provider"] != "anthropic" {
		t.Errorf("provider = %q, want anthropic", cfg["provider"])
	}
}

func TestVapiLLMConfig_Gemini(t *testing.T) {
	t.Setenv("AI_PROVIDER", "gemini")
	cfg := vapiLLMConfig("test prompt")

	if cfg["provider"] != "google" {
		t.Errorf("provider = %q, want google", cfg["provider"])
	}
	if cfg["model"] != "gemini-2.0-flash" {
		t.Errorf("model = %q, want gemini-2.0-flash", cfg["model"])
	}
}

func TestVapiLLMConfig_ContainsSystemPrompt(t *testing.T) {
	t.Setenv("AI_PROVIDER", "claude")
	cfg := vapiLLMConfig("You are a helpful assistant.")

	msgs, ok := cfg["messages"].([]map[string]string)
	if !ok || len(msgs) == 0 {
		t.Fatal("messages not set in vapiLLMConfig")
	}
	if msgs[0]["role"] != "system" {
		t.Errorf("messages[0].role = %q, want system", msgs[0]["role"])
	}
	if msgs[0]["content"] != "You are a helpful assistant." {
		t.Errorf("messages[0].content = %q, want system prompt", msgs[0]["content"])
	}
}

// ─── HandleInitiate ──────────────────────────────────────────────────────────

func setupVoiceHandlerTest(t *testing.T) *VoiceHandler {
	t.Helper()
	if err := services.InitSessionStore(":memory:"); err != nil {
		t.Fatalf("InitSessionStore: %v", err)
	}
	services.AI = &mockAI{summary: ""}
	return NewVoiceHandler(services.Store)
}

func TestHandleInitiate_InvalidJSON(t *testing.T) {
	h := setupVoiceHandlerTest(t)
	r := httptest.NewRequest(http.MethodPost, "/api/voice/initiate", bytes.NewReader([]byte("not json")))
	w := httptest.NewRecorder()
	h.HandleInitiate(w, r)
	if w.Code != http.StatusBadRequest {
		t.Errorf("got status %d, want 400 for invalid JSON", w.Code)
	}
}

func TestHandleInitiate_MissingSessionID(t *testing.T) {
	h := setupVoiceHandlerTest(t)
	body, _ := json.Marshal(map[string]string{"sessionId": ""})
	r := httptest.NewRequest(http.MethodPost, "/api/voice/initiate", bytes.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.HandleInitiate(w, r)
	if w.Code != http.StatusBadRequest {
		t.Errorf("got status %d, want 400 for empty sessionId", w.Code)
	}
}

func TestHandleInitiate_ValidRequest(t *testing.T) {
	h := setupVoiceHandlerTest(t)
	body, _ := json.Marshal(models.VoiceInitiateRequest{SessionID: "voice-test-sess"})
	r := httptest.NewRequest(http.MethodPost, "/api/voice/initiate", bytes.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.HandleInitiate(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("got status %d, want 200", w.Code)
	}

	var resp models.VoiceInitiateResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("could not decode response: %v", err)
	}
	if resp.AssistantOverrides == nil {
		t.Error("assistantOverrides should not be nil")
	}
}

func TestHandleInitiate_DefaultFirstMessage(t *testing.T) {
	h := setupVoiceHandlerTest(t)
	body, _ := json.Marshal(models.VoiceInitiateRequest{SessionID: "voice-default-msg"})
	r := httptest.NewRequest(http.MethodPost, "/api/voice/initiate", bytes.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.HandleInitiate(w, r)

	var resp models.VoiceInitiateResponse
	json.NewDecoder(w.Body).Decode(&resp)

	firstMsg, _ := resp.AssistantOverrides["firstMessage"].(string)
	if firstMsg == "" {
		t.Error("firstMessage should not be empty")
	}
}

func TestHandleInitiate_PersonalisedFirstMessage(t *testing.T) {
	h := setupVoiceHandlerTest(t)

	// Pre-populate session with patient info
	sess := services.Store.GetOrCreate("voice-personalised")
	sess.PatientInfo = models.PatientInfo{
		FirstName:      "Maria",
		ReasonForVisit: "back pain",
	}
	services.Store.Save(sess)

	body, _ := json.Marshal(models.VoiceInitiateRequest{SessionID: "voice-personalised"})
	r := httptest.NewRequest(http.MethodPost, "/api/voice/initiate", bytes.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.HandleInitiate(w, r)

	var resp models.VoiceInitiateResponse
	json.NewDecoder(w.Body).Decode(&resp)

	firstMsg, _ := resp.AssistantOverrides["firstMessage"].(string)
	if firstMsg == "" {
		t.Fatal("firstMessage should not be empty")
	}
	// Should mention the patient's name and reason
	if len(firstMsg) < 10 {
		t.Errorf("firstMessage seems too short for personalised greeting: %q", firstMsg)
	}
}

// ─── HandleWebhook ───────────────────────────────────────────────────────────

func TestHandleWebhook_InvalidJSON(t *testing.T) {
	h := setupVoiceHandlerTest(t)
	r := httptest.NewRequest(http.MethodPost, "/api/voice/webhook", bytes.NewReader([]byte("bad")))
	w := httptest.NewRecorder()
	h.HandleWebhook(w, r)
	if w.Code != http.StatusBadRequest {
		t.Errorf("got status %d, want 400 for invalid JSON", w.Code)
	}
}

func TestHandleWebhook_UnknownPhone_EmptyOverrides(t *testing.T) {
	h := setupVoiceHandlerTest(t)

	payload := map[string]interface{}{
		"call": map[string]interface{}{
			"customer": map[string]interface{}{
				"number": "+10000000000",
			},
		},
	}
	body, _ := json.Marshal(payload)
	r := httptest.NewRequest(http.MethodPost, "/api/voice/webhook", bytes.NewReader(body))
	w := httptest.NewRecorder()
	h.HandleWebhook(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("got status %d, want 200", w.Code)
	}

	var resp map[string]interface{}
	json.NewDecoder(w.Body).Decode(&resp)

	overrides, ok := resp["assistantOverrides"].(map[string]interface{})
	if !ok {
		t.Fatal("assistantOverrides missing from webhook response")
	}
	if len(overrides) != 0 {
		t.Errorf("expected empty overrides for unknown phone, got: %v", overrides)
	}
}

func TestHandleWebhook_KnownPhone_HasOverrides(t *testing.T) {
	h := setupVoiceHandlerTest(t)

	// Store a session with a known phone number
	sess := services.Store.GetOrCreate("webhook-phone-test")
	sess.PhoneNumber = "+15550001234"
	sess.PatientInfo = models.PatientInfo{FirstName: "Jordan"}
	services.Store.Save(sess)

	payload := map[string]interface{}{
		"call": map[string]interface{}{
			"customer": map[string]interface{}{
				"number": "+15550001234",
			},
		},
	}
	body, _ := json.Marshal(payload)
	r := httptest.NewRequest(http.MethodPost, "/api/voice/webhook", bytes.NewReader(body))
	w := httptest.NewRecorder()
	h.HandleWebhook(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("got status %d, want 200", w.Code)
	}

	var resp map[string]interface{}
	json.NewDecoder(w.Body).Decode(&resp)

	overrides, ok := resp["assistantOverrides"].(map[string]interface{})
	if !ok {
		t.Fatal("assistantOverrides missing from webhook response")
	}
	if len(overrides) == 0 {
		t.Error("expected non-empty overrides for known phone number")
	}
	if _, hasFirst := overrides["firstMessage"]; !hasFirst {
		t.Error("overrides should contain firstMessage for known caller")
	}
}

func TestHandleWebhook_NoCallField_EmptyOverrides(t *testing.T) {
	h := setupVoiceHandlerTest(t)

	body, _ := json.Marshal(map[string]interface{}{"type": "status-update"})
	r := httptest.NewRequest(http.MethodPost, "/api/voice/webhook", bytes.NewReader(body))
	w := httptest.NewRecorder()
	h.HandleWebhook(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("got status %d, want 200", w.Code)
	}

	var resp map[string]interface{}
	json.NewDecoder(w.Body).Decode(&resp)

	overrides, _ := resp["assistantOverrides"].(map[string]interface{})
	if len(overrides) != 0 {
		t.Errorf("expected empty overrides when no call field, got: %v", overrides)
	}
}
