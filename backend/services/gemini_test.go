package services

import (
	"testing"

	"kyron-medical/models"
)

func TestMsgsToGeminiContents_RoleConversion(t *testing.T) {
	messages := []models.ChatMessage{
		{Role: "user", Content: "Hello"},
		{Role: "assistant", Content: "Hi there"},
		{Role: "user", Content: "I need an appointment"},
	}

	contents := msgsToGeminiContents(messages)

	if len(contents) != 3 {
		t.Fatalf("expected 3 contents, got %d", len(contents))
	}
	if string(contents[0].Role) != "user" {
		t.Errorf("content[0] role = %q, want user", contents[0].Role)
	}
	// "assistant" must be converted to "model"
	if string(contents[1].Role) != "model" {
		t.Errorf("content[1] role = %q, want model (assistant→model)", contents[1].Role)
	}
	if string(contents[2].Role) != "user" {
		t.Errorf("content[2] role = %q, want user", contents[2].Role)
	}
}

func TestMsgsToGeminiContents_PreservesContent(t *testing.T) {
	messages := []models.ChatMessage{
		{Role: "user", Content: "My knee hurts"},
	}
	contents := msgsToGeminiContents(messages)
	if len(contents) == 0 {
		t.Fatal("expected at least one content")
	}
	if len(contents[0].Parts) == 0 || contents[0].Parts[0].Text != "My knee hurts" {
		t.Errorf("content text not preserved")
	}
}

func TestMsgsToGeminiContents_EmptyMessages(t *testing.T) {
	contents := msgsToGeminiContents(nil)
	if len(contents) != 0 {
		t.Errorf("expected empty contents for nil input, got %d", len(contents))
	}
}

func TestBuildGeminiTools_ToolCount(t *testing.T) {
	tool := buildGeminiTools()
	if tool == nil {
		t.Fatal("buildGeminiTools returned nil")
	}
	if len(tool.FunctionDeclarations) != 8 {
		t.Errorf("expected 8 function declarations, got %d", len(tool.FunctionDeclarations))
	}
}

func TestBuildGeminiTools_RequiredToolNames(t *testing.T) {
	tool := buildGeminiTools()
	names := map[string]bool{}
	for _, f := range tool.FunctionDeclarations {
		names[f.Name] = true
	}

	required := []string{
		"begin_intake", "begin_prescription", "show_office_info",
		"collect_intake", "confirm_doctor", "select_slot",
		"confirm_booking", "log_prescription_request",
	}
	for _, name := range required {
		if !names[name] {
			t.Errorf("missing function declaration: %q", name)
		}
	}
}

func TestBuildGeminiTools_CollectIntakeRequiredFields(t *testing.T) {
	tool := buildGeminiTools()
	var collectIntake *struct {
		Required []string
	}
	for _, f := range tool.FunctionDeclarations {
		if f.Name == "collect_intake" {
			if f.Parameters == nil {
				t.Fatal("collect_intake has no parameters schema")
			}
			required := f.Parameters.Required
			expected := []string{"firstName", "lastName", "dob", "phone", "email", "reasonForVisit"}
			if len(required) != len(expected) {
				t.Errorf("collect_intake required fields: got %v, want %v", required, expected)
			}
			_ = collectIntake
			return
		}
	}
	t.Error("collect_intake not found in tool declarations")
}

func TestBuildGeminiTools_ConfirmBookingHasSmsOptIn(t *testing.T) {
	tool := buildGeminiTools()
	for _, f := range tool.FunctionDeclarations {
		if f.Name == "confirm_booking" {
			if f.Parameters == nil {
				t.Fatal("confirm_booking has no parameters")
			}
			if _, ok := f.Parameters.Properties["smsOptIn"]; !ok {
				t.Error("confirm_booking missing smsOptIn property")
			}
			return
		}
	}
	t.Error("confirm_booking not found")
}
