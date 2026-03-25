package services

import (
	"testing"
)

func TestInitAI_DefaultsToClaude(t *testing.T) {
	t.Setenv("AI_PROVIDER", "")
	t.Setenv("ANTHROPIC_API_KEY", "test-key")

	if err := InitAI(); err != nil {
		t.Fatalf("InitAI() error = %v", err)
	}
	if AI == nil {
		t.Fatal("AI should not be nil after InitAI")
	}
	if _, ok := AI.(*ClaudeService); !ok {
		t.Errorf("expected *ClaudeService, got %T", AI)
	}
}

func TestInitAI_ClaudeExplicit(t *testing.T) {
	t.Setenv("AI_PROVIDER", "claude")
	t.Setenv("ANTHROPIC_API_KEY", "test-key")

	if err := InitAI(); err != nil {
		t.Fatalf("InitAI() error = %v", err)
	}
	if _, ok := AI.(*ClaudeService); !ok {
		t.Errorf("expected *ClaudeService, got %T", AI)
	}
}

func TestInitAI_Gemini(t *testing.T) {
	t.Setenv("AI_PROVIDER", "gemini")
	t.Setenv("GEMINI_API_KEY", "test-gemini-key")

	if err := InitAI(); err != nil {
		t.Fatalf("InitAI() error = %v", err)
	}
	if AI == nil {
		t.Fatal("AI should not be nil after InitAI")
	}
	if _, ok := AI.(*GeminiService); !ok {
		t.Errorf("expected *GeminiService, got %T", AI)
	}
}

func TestInitAI_SetsGlobalAI(t *testing.T) {
	t.Setenv("AI_PROVIDER", "claude")
	t.Setenv("ANTHROPIC_API_KEY", "k")

	if err := InitAI(); err != nil {
		t.Fatal(err)
	}
	prev := AI

	t.Setenv("AI_PROVIDER", "gemini")
	t.Setenv("GEMINI_API_KEY", "gk")

	if err := InitAI(); err != nil {
		t.Fatal(err)
	}
	if AI == prev {
		t.Error("InitAI should replace the global AI instance")
	}
}
