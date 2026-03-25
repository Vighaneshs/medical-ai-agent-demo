package services

import (
	"context"
	"fmt"
	"os"

	"kyron-medical/models"
)

// ToolCallResult holds a single tool invocation from the AI model.
type ToolCallResult struct {
	ToolName string
	Input    map[string]interface{}
}

// AIProvider is the interface both Claude and Gemini backends implement.
// The chat handler only depends on this interface — switching providers
// requires only an env var change, no code changes.
type AIProvider interface {
	Stream(
		ctx context.Context,
		systemPrompt string,
		messages []models.ChatMessage,
		textChunks chan<- string,
		toolResults chan<- []ToolCallResult,
	)
	Summarize(ctx context.Context, messages []models.ChatMessage) string
}

// AI is the active provider. Set by InitAI() at startup.
var AI AIProvider

// InitAI reads AI_PROVIDER and initialises the correct backend.
// Defaults to Claude if AI_PROVIDER is unset.
func InitAI() error {
	switch os.Getenv("AI_PROVIDER") {
	case "gemini":
		svc, err := newGeminiService()
		if err != nil {
			return fmt.Errorf("gemini init: %w", err)
		}
		AI = svc
	default:
		AI = newClaudeService()
	}
	return nil
}
