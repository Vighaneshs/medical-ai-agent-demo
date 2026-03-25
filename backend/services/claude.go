package services

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
	"github.com/anthropics/anthropic-sdk-go/packages/param"
	"kyron-medical/models"
)

type ToolCallResult struct {
	ToolName string
	Input    map[string]interface{}
}

type ClaudeService struct {
	client *anthropic.Client
}

var Claude *ClaudeService

func InitClaude() {
	client := anthropic.NewClient(option.WithAPIKey(os.Getenv("ANTHROPIC_API_KEY")))
	Claude = &ClaudeService{client: &client}
}

func (c *ClaudeService) Stream(
	ctx context.Context,
	systemPrompt string,
	messages []models.ChatMessage,
	textChunks chan<- string,
	toolResults chan<- []ToolCallResult,
) {
	defer close(textChunks)
	defer close(toolResults)

	anthropicMessages := make([]anthropic.MessageParam, len(messages))
	for i, m := range messages {
		if m.Role == "user" {
			anthropicMessages[i] = anthropic.NewUserMessage(anthropic.NewTextBlock(m.Content))
		} else {
			anthropicMessages[i] = anthropic.NewAssistantMessage(anthropic.NewTextBlock(m.Content))
		}
	}

	stream := c.client.Messages.NewStreaming(ctx, anthropic.MessageNewParams{
		Model:     anthropic.ModelClaudeSonnet4_6,
		MaxTokens: 1024,
		System:    []anthropic.TextBlockParam{{Text: systemPrompt}},
		Messages:  anthropicMessages,
		Tools:     buildTools(),
	})

	acc := anthropic.Message{}
	for stream.Next() {
		event := stream.Current()
		acc.Accumulate(event)

		if delta, ok := event.AsAny().(anthropic.ContentBlockDeltaEvent); ok {
			if text, ok := delta.Delta.AsAny().(anthropic.TextDelta); ok && text.Text != "" {
				select {
				case <-ctx.Done():
					return
				case textChunks <- text.Text:
				}
			}
		}
	}

	if err := stream.Err(); err != nil && ctx.Err() == nil {
		select {
		case textChunks <- "\n\nI'm having trouble connecting right now. Please try again.":
		default:
		}
	}

	var calls []ToolCallResult
	for _, block := range acc.Content {
		if block.Type == "tool_use" {
			var input map[string]interface{}
			json.Unmarshal(block.Input, &input)
			calls = append(calls, ToolCallResult{
				ToolName: block.Name,
				Input:    input,
			})
		}
	}

	toolResults <- calls
}

func (c *ClaudeService) Summarize(ctx context.Context, messages []models.ChatMessage) string {
	if len(messages) == 0 {
		return ""
	}

	limit := len(messages)
	if limit > 20 {
		limit = 20
	}
	var history strings.Builder
	for _, m := range messages[len(messages)-limit:] {
		history.WriteString(fmt.Sprintf("%s: %s\n", m.Role, m.Content))
	}

	resp, err := c.client.Messages.New(ctx, anthropic.MessageNewParams{
		Model:     anthropic.ModelClaudeHaiku4_5,
		MaxTokens: 150,
		Messages: []anthropic.MessageParam{
			anthropic.NewUserMessage(anthropic.NewTextBlock(
				"Summarize this medical appointment chat in 2 sentences for AI context continuity:\n\n" + history.String(),
			)),
		},
	})
	if err != nil || len(resp.Content) == 0 {
		return ""
	}
	return resp.Content[0].Text
}

func buildTools() []anthropic.ToolUnionParam {
	tools := []struct {
		name        string
		description string
		properties  map[string]interface{}
		required    []string
	}{
		{
			name:        "begin_intake",
			description: "Start the appointment scheduling intake flow when patient wants to book",
			properties:  map[string]interface{}{},
			required:    []string{},
		},
		{
			name:        "begin_prescription",
			description: "Start a prescription refill inquiry flow",
			properties:  map[string]interface{}{},
			required:    []string{},
		},
		{
			name:        "show_office_info",
			description: "Switch to office hours and location information flow",
			properties:  map[string]interface{}{},
			required:    []string{},
		},
		{
			name:        "collect_intake",
			description: "Save patient intake information once all fields are collected",
			properties: map[string]interface{}{
				"firstName":      map[string]string{"type": "string"},
				"lastName":       map[string]string{"type": "string"},
				"dob":            map[string]string{"type": "string", "description": "YYYY-MM-DD"},
				"phone":          map[string]string{"type": "string"},
				"email":          map[string]string{"type": "string"},
				"reasonForVisit": map[string]string{"type": "string"},
			},
			required: []string{"firstName", "lastName", "dob", "phone", "email", "reasonForVisit"},
		},
		{
			name:        "confirm_doctor",
			description: "Confirm the matched doctor after patient agrees",
			properties:  map[string]interface{}{"doctorId": map[string]string{"type": "string"}},
			required:    []string{"doctorId"},
		},
		{
			name:        "select_slot",
			description: "Record the patient's chosen appointment slot",
			properties: map[string]interface{}{
				"date":      map[string]string{"type": "string", "description": "YYYY-MM-DD"},
				"startTime": map[string]string{"type": "string", "description": "HH:MM"},
			},
			required: []string{"date", "startTime"},
		},
		{
			name:        "confirm_booking",
			description: "Finalize the appointment booking",
			properties:  map[string]interface{}{"smsOptIn": map[string]string{"type": "boolean"}},
			required:    []string{"smsOptIn"},
		},
		{
			name:        "log_prescription_request",
			description: "Log a prescription refill request",
			properties: map[string]interface{}{
				"medication":    map[string]string{"type": "string"},
				"prescriberName": map[string]string{"type": "string"},
				"pharmacyName":  map[string]string{"type": "string"},
				"pharmacyPhone": map[string]string{"type": "string"},
			},
			required: []string{"medication", "pharmacyName"},
		},
	}

	result := make([]anthropic.ToolUnionParam, len(tools))
	for i, t := range tools {
		tp := anthropic.ToolParam{
			Name:        t.name,
			Description: param.NewOpt(t.description),
			InputSchema: anthropic.ToolInputSchemaParam{
				Properties: t.properties,
				Required:   t.required,
			},
		}
		result[i] = anthropic.ToolUnionParam{OfTool: &tp}
	}
	return result
}
