package services

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"

	"google.golang.org/genai"
	"kyron-medical/models"
)

func geminiModel() string {
	if m := os.Getenv("GEMINI_MODEL"); m != "" {
		return m
	}
	return "gemini-2.0-flash"
}

type GeminiService struct {
	client *genai.Client
}

func newGeminiService() (*GeminiService, error) {
	client, err := genai.NewClient(context.Background(), &genai.ClientConfig{
		APIKey:  os.Getenv("GEMINI_API_KEY"),
		Backend: genai.BackendGeminiAPI,
	})
	if err != nil {
		return nil, fmt.Errorf("create gemini client: %w", err)
	}
	return &GeminiService{client: client}, nil
}

func (g *GeminiService) Stream(
	ctx context.Context,
	systemPrompt string,
	messages []models.ChatMessage,
	textChunks chan<- string,
	toolResults chan<- []ToolCallResult,
) {
	defer close(textChunks)
	defer close(toolResults)

	contents := msgsToGeminiContents(messages)
	config := &genai.GenerateContentConfig{
		SystemInstruction: genai.NewContentFromText(systemPrompt, "user"),
		MaxOutputTokens:   1024,
		Tools:             []*genai.Tool{buildGeminiTools()},
	}

	if len(contents) == 0 {
		log.Printf("[gemini] no contents after filtering — sending fallback")
		select {
		case textChunks <- "I didn't catch that. Could you say that again?":
		default:
		}
		return
	}
	log.Printf("[gemini] starting stream: model=%s contents=%d", geminiModel(), len(contents))

	var allParts []*genai.Part
	respCount := 0

	for resp, err := range g.client.Models.GenerateContentStream(ctx, geminiModel(), contents, config) {
		if err != nil {
			log.Printf("[gemini] stream error after %d responses: %v", respCount, err)
			if ctx.Err() == nil {
				select {
				case textChunks <- "\n\nI'm having trouble connecting right now. Please try again.":
				default:
				}
			}
			break
		}
		respCount++

		if len(resp.Candidates) == 0 {
			log.Printf("[gemini] resp #%d: no candidates", respCount)
			continue
		}
		if resp.Candidates[0].Content == nil {
			log.Printf("[gemini] resp #%d: nil content (finish_reason=%v)", respCount, resp.Candidates[0].FinishReason)
			continue
		}

		parts := resp.Candidates[0].Content.Parts
		log.Printf("[gemini] resp #%d: %d parts", respCount, len(parts))
		for i, part := range parts {
			if part.Thought {
				log.Printf("[gemini]   part[%d] THOUGHT (skipped) len=%d", i, len(part.Text))
				continue // never send internal thinking tokens to the user
			}
			if part.Text != "" {
				log.Printf("[gemini]   part[%d] text=%q", i, truncate(part.Text, 80))
			}
			if part.FunctionCall != nil {
				log.Printf("[gemini]   part[%d] function_call=%s args=%v", i, part.FunctionCall.Name, part.FunctionCall.Args)
			}
			allParts = append(allParts, part)
			if part.Text != "" {
				select {
				case <-ctx.Done():
					toolResults <- nil
					return
				case textChunks <- part.Text:
				}
			}
		}
	}
	log.Printf("[gemini] stream complete: %d responses, %d total parts", respCount, len(allParts))

	var calls []ToolCallResult
	for _, part := range allParts {
		if part.FunctionCall != nil {
			input := make(map[string]interface{})
			for k, v := range part.FunctionCall.Args {
				input[k] = v
			}
			calls = append(calls, ToolCallResult{
				ToolName: part.FunctionCall.Name,
				Input:    input,
			})
		}
	}
	log.Printf("[gemini] tool calls extracted: %d", len(calls))
	toolResults <- calls
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}

func (g *GeminiService) Summarize(ctx context.Context, messages []models.ChatMessage) string {
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

	prompt := "Summarize this medical appointment chat in 2 sentences for AI context continuity:\n\n" + history.String()
	resp, err := g.client.Models.GenerateContent(ctx, geminiModel(),
		[]*genai.Content{genai.NewContentFromText(prompt, "user")},
		nil,
	)
	if err != nil || len(resp.Candidates) == 0 || resp.Candidates[0].Content == nil {
		return ""
	}
	for _, part := range resp.Candidates[0].Content.Parts {
		if part.Text != "" {
			return part.Text
		}
	}
	return ""
}

// msgsToGeminiContents converts chat history to Gemini's Content slice.
// Gemini uses "user" and "model" roles (not "assistant").
func msgsToGeminiContents(messages []models.ChatMessage) []*genai.Content {
	var contents []*genai.Content
	for _, m := range messages {
		if strings.TrimSpace(m.Content) == "" {
			continue
		}
		role := m.Role
		if role == "assistant" {
			role = "model"
		}
		// Gemini requires strictly alternating user/model turns.
		// When the AI makes a tool-only call (no text), no assistant message is
		// stored, so consecutive user messages appear. Merge them into one turn.
		if len(contents) > 0 && string(contents[len(contents)-1].Role) == role {
			last := contents[len(contents)-1]
			last.Parts = append(last.Parts, &genai.Part{Text: "\n" + m.Content})
			log.Printf("[gemini] merged consecutive %s messages to maintain alternation", role)
		} else {
			contents = append(contents, genai.NewContentFromText(m.Content, genai.Role(role)))
		}
	}
	return contents
}

// buildGeminiTools returns a single Tool wrapping all function declarations.
func buildGeminiTools() *genai.Tool {
	str := func(desc string) *genai.Schema {
		return &genai.Schema{Type: genai.TypeString, Description: desc}
	}
	bool_ := func(desc string) *genai.Schema {
		return &genai.Schema{Type: genai.TypeBoolean, Description: desc}
	}
	obj := func(props map[string]*genai.Schema, required []string) *genai.Schema {
		return &genai.Schema{Type: genai.TypeObject, Properties: props, Required: required}
	}

	decls := []*genai.FunctionDeclaration{
		{
			Name:        "begin_intake",
			Description: "Start the appointment scheduling intake flow when patient wants to book",
		},
		{
			Name:        "begin_prescription",
			Description: "Start a prescription refill inquiry flow",
		},
		{
			Name:        "show_office_info",
			Description: "Switch to office hours and location information flow",
		},
		{
			Name:        "collect_intake",
			Description: "Save patient intake information once all fields are collected",
			Parameters: obj(map[string]*genai.Schema{
				"firstName":      str("Patient first name"),
				"lastName":       str("Patient last name"),
				"dob":            str("Date of birth in YYYY-MM-DD format"),
				"phone":          str("Phone number"),
				"email":          str("Email address"),
				"reasonForVisit": str("Reason for the appointment"),
			}, []string{"firstName", "lastName", "dob", "phone", "email", "reasonForVisit"}),
		},
		{
			Name:        "confirm_doctor",
			Description: "Confirm the matched doctor after patient agrees",
			Parameters:  obj(map[string]*genai.Schema{"doctorId": str("Doctor ID")}, []string{"doctorId"}),
		},
		{
			Name:        "select_slot",
			Description: "Record the patient's chosen appointment slot",
			Parameters: obj(map[string]*genai.Schema{
				"date":      str("Appointment date in YYYY-MM-DD format"),
				"startTime": str("Start time in HH:MM format"),
			}, []string{"date", "startTime"}),
		},
		{
			Name:        "confirm_booking",
			Description: "Finalize the appointment booking",
			Parameters:  obj(map[string]*genai.Schema{"smsOptIn": bool_("Whether to send SMS reminders")}, []string{"smsOptIn"}),
		},
		{
			Name:        "log_prescription_request",
			Description: "Log a prescription refill request",
			Parameters: obj(map[string]*genai.Schema{
				"medication":     str("Medication name"),
				"prescriberName": str("Prescribing doctor's name"),
				"pharmacyName":   str("Pharmacy name"),
				"pharmacyPhone":  str("Pharmacy phone number"),
			}, []string{"medication", "pharmacyName"}),
		},
	}

	return &genai.Tool{FunctionDeclarations: decls}
}
