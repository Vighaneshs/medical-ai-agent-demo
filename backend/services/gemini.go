package services

import (
	"context"
	"fmt"
	"os"
	"strings"

	"google.golang.org/genai"
	"kyron-medical/models"
)

const geminiModel = "gemini-2.0-flash"

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

	var allParts []*genai.Part

	for resp, err := range g.client.Models.GenerateContentStream(ctx, geminiModel, contents, config) {
		if err != nil {
			if ctx.Err() == nil {
				select {
				case textChunks <- "\n\nI'm having trouble connecting right now. Please try again.":
				default:
				}
			}
			break
		}
		if len(resp.Candidates) == 0 || resp.Candidates[0].Content == nil {
			continue
		}
		for _, part := range resp.Candidates[0].Content.Parts {
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
	toolResults <- calls
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
	resp, err := g.client.Models.GenerateContent(ctx, geminiModel,
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
	contents := make([]*genai.Content, len(messages))
	for i, m := range messages {
		role := m.Role
		if role == "assistant" {
			role = "model"
		}
		contents[i] = genai.NewContentFromText(m.Content, genai.Role(role))
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
