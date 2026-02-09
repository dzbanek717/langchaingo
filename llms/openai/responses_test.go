package openai

import (
	"encoding/json"
	"testing"

	"github.com/tmc/langchaingo/llms"
	"github.com/tmc/langchaingo/llms/openai/internal/openaiclient"
)

func TestResponsesAPIPayloadFormat(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name            string
		messages        []llms.MessageContent
		expectedPayload string
	}{
		{
			name: "input_text and input_file",
			messages: []llms.MessageContent{
				{
					Role: llms.ChatMessageTypeHuman,
					Parts: []llms.ContentPart{
						llms.TextPart("what is in this file?"),
						llms.InputFilePart("https://www.berkshirehathaway.com/letters/2024ltr.pdf"),
					},
				},
			},
			expectedPayload: `{
				"input": [
					{
						"role": "user",
						"content": [
							{"type": "input_text", "text": "what is in this file?"},
							{"type": "input_file", "file_url": "https://www.berkshirehathaway.com/letters/2024ltr.pdf"}
						]
					}
				]
			}`,
		},
		{
			name: "input_text and input_image",
			messages: []llms.MessageContent{
				{
					Role: llms.ChatMessageTypeHuman,
					Parts: []llms.ContentPart{
						llms.TextPart("what is in this image?"),
						llms.ImageURLPart("https://upload.wikimedia.org/wikipedia/commons/thumb/d/dd/Gfp-wisconsin-madison-the-nature-boardwalk.jpg/2560px-Gfp-wisconsin-madison-the-nature-boardwalk.jpg"),
					},
				},
			},
			expectedPayload: `{
				"input": [
					{
						"role": "user",
						"content": [
							{"type": "input_text", "text": "what is in this image?"},
							{"type": "input_image", "image_url": "https://upload.wikimedia.org/wikipedia/commons/thumb/d/dd/Gfp-wisconsin-madison-the-nature-boardwalk.jpg/2560px-Gfp-wisconsin-madison-the-nature-boardwalk.jpg"}
						]
					}
				]
			}`,
		},
		{
			name: "input_file with file_id",
			messages: []llms.MessageContent{
				{
					Role: llms.ChatMessageTypeHuman,
					Parts: []llms.ContentPart{
						llms.TextPart("Analyze this document"),
						llms.InputFileIDPart("file-abc123"),
					},
				},
			},
			expectedPayload: `{
				"input": [
					{
						"role": "user",
						"content": [
							{"type": "input_text", "text": "Analyze this document"},
							{"type": "input_file", "file_id": "file-abc123"}
						]
					}
				]
			}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// Convert messages to ResponseMessage format (same logic as in GenerateContentWithResponsesAPI)
			responseMessages := make([]*openaiclient.ResponseMessage, 0, len(tt.messages))
			for _, mc := range tt.messages {
				msg := &openaiclient.ResponseMessage{}

				switch mc.Role {
				case llms.ChatMessageTypeHuman:
					msg.Role = RoleUser
				case llms.ChatMessageTypeAI:
					msg.Role = RoleAssistant
				case llms.ChatMessageTypeSystem:
					msg.Role = RoleSystem
				}

				msg.Content = make([]openaiclient.ResponseContent, 0, len(mc.Parts))
				for _, part := range mc.Parts {
					switch p := part.(type) {
					case llms.TextContent:
						msg.Content = append(msg.Content, openaiclient.ResponseContent{
							Type: "input_text",
							Text: p.Text,
						})
					case llms.InputFileContent:
						rc := openaiclient.ResponseContent{
							Type: "input_file",
						}
						if p.FileURL != "" {
							rc.FileURL = p.FileURL
						}
						if p.FileID != "" {
							rc.FileID = p.FileID
						}
						msg.Content = append(msg.Content, rc)
					case llms.ImageURLContent:
						msg.Content = append(msg.Content, openaiclient.ResponseContent{
							Type:     "input_image",
							ImageURL: p.URL,
						})
					}
				}

				responseMessages = append(responseMessages, msg)
			}

			// Marshal to JSON
			payload := map[string]any{
				"input": responseMessages,
			}
			actualJSON, err := json.Marshal(payload)
			if err != nil {
				t.Fatalf("Failed to marshal payload: %v", err)
			}

			// Parse both JSONs for comparison
			var actual, expected map[string]any
			if err := json.Unmarshal(actualJSON, &actual); err != nil {
				t.Fatalf("Failed to unmarshal actual JSON: %v", err)
			}
			if err := json.Unmarshal([]byte(tt.expectedPayload), &expected); err != nil {
				t.Fatalf("Failed to unmarshal expected JSON: %v", err)
			}

			// Compare
			actualFormatted, _ := json.MarshalIndent(actual, "", "  ")
			expectedFormatted, _ := json.MarshalIndent(expected, "", "  ")

			if string(actualFormatted) != string(expectedFormatted) {
				t.Errorf("Payload mismatch:\nGot:\n%s\n\nExpected:\n%s", actualFormatted, expectedFormatted)
			}
		})
	}
}

func TestResponseContentMarshaling(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		content  openaiclient.ResponseContent
		expected string
	}{
		{
			name: "input_text",
			content: openaiclient.ResponseContent{
				Type: "input_text",
				Text: "Hello world",
			},
			expected: `{"type":"input_text","text":"Hello world"}`,
		},
		{
			name: "input_file with file_url",
			content: openaiclient.ResponseContent{
				Type:    "input_file",
				FileURL: "https://example.com/doc.pdf",
			},
			expected: `{"type":"input_file","file_url":"https://example.com/doc.pdf"}`,
		},
		{
			name: "input_file with file_id",
			content: openaiclient.ResponseContent{
				Type:   "input_file",
				FileID: "file-abc123",
			},
			expected: `{"type":"input_file","file_id":"file-abc123"}`,
		},
		{
			name: "input_image",
			content: openaiclient.ResponseContent{
				Type:     "input_image",
				ImageURL: "https://example.com/image.jpg",
			},
			expected: `{"type":"input_image","image_url":"https://example.com/image.jpg"}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			actual, err := json.Marshal(tt.content)
			if err != nil {
				t.Fatalf("Failed to marshal: %v", err)
			}

			if string(actual) != tt.expected {
				t.Errorf("Got %s, expected %s", actual, tt.expected)
			}
		})
	}
}
