package llms

import (
	"encoding/json"
	"testing"
)

func TestInputFileContentMarshaling(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		content  InputFileContent
		wantType string
		wantURL  string
		wantID   string
	}{
		{
			name: "file URL",
			content: InputFileContent{
				FileURL: "https://example.com/document.pdf",
			},
			wantType: "input_file",
			wantURL:  "https://example.com/document.pdf",
		},
		{
			name: "presigned S3 URL",
			content: InputFileContent{
				FileURL: "https://bucket.s3.amazonaws.com/doc.pdf?X-Amz-Algorithm=...",
			},
			wantType: "input_file",
			wantURL:  "https://bucket.s3.amazonaws.com/doc.pdf?X-Amz-Algorithm=...",
		},
		{
			name: "file ID",
			content: InputFileContent{
				FileID: "file-abc123",
			},
			wantType: "input_file",
			wantID:   "file-abc123",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// Test marshaling
			data, err := json.Marshal(tt.content)
			if err != nil {
				t.Fatalf("Failed to marshal InputFileContent: %v", err)
			}

			var got map[string]interface{}
			if err := json.Unmarshal(data, &got); err != nil {
				t.Fatalf("Failed to unmarshal result: %v", err)
			}

			if got["type"] != tt.wantType {
				t.Errorf("Type mismatch: got %q, want %q", got["type"], tt.wantType)
			}

			if tt.wantURL != "" {
				if got["file_url"] != tt.wantURL {
					t.Errorf("FileURL mismatch: got %q, want %q", got["file_url"], tt.wantURL)
				}
			}

			if tt.wantID != "" {
				if got["file_id"] != tt.wantID {
					t.Errorf("FileID mismatch: got %q, want %q", got["file_id"], tt.wantID)
				}
			}

			// Test unmarshaling
			var unmarshaled InputFileContent
			if err := json.Unmarshal(data, &unmarshaled); err != nil {
				t.Fatalf("Failed to unmarshal InputFileContent: %v", err)
			}

			if unmarshaled.FileURL != tt.content.FileURL {
				t.Errorf("Unmarshaled FileURL mismatch: got %q, want %q", unmarshaled.FileURL, tt.content.FileURL)
			}

			if unmarshaled.FileID != tt.content.FileID {
				t.Errorf("Unmarshaled FileID mismatch: got %q, want %q", unmarshaled.FileID, tt.content.FileID)
			}
		})
	}
}

func TestInputFileContentHelpers(t *testing.T) {
	t.Parallel()

	t.Run("InputFilePart", func(t *testing.T) {
		t.Parallel()
		url := "https://example.com/document.pdf"
		content := InputFilePart(url)

		if content.FileURL != url {
			t.Errorf("FileURL mismatch: got %q, want %q", content.FileURL, url)
		}

		if content.FileID != "" {
			t.Errorf("Expected empty FileID, got %q", content.FileID)
		}
	})

	t.Run("InputFileIDPart", func(t *testing.T) {
		t.Parallel()
		fileID := "file-abc123"
		content := InputFileIDPart(fileID)

		if content.FileID != fileID {
			t.Errorf("FileID mismatch: got %q, want %q", content.FileID, fileID)
		}

		if content.FileURL != "" {
			t.Errorf("Expected empty FileURL, got %q", content.FileURL)
		}
	})
}

func TestInputFileContentString(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		content InputFileContent
		want    string
	}{
		{
			name: "URL",
			content: InputFileContent{
				FileURL: "https://example.com/doc.pdf",
			},
			want: "https://example.com/doc.pdf",
		},
		{
			name: "ID",
			content: InputFileContent{
				FileID: "file-abc123",
			},
			want: "file-abc123",
		},
		{
			name: "URL takes precedence",
			content: InputFileContent{
				FileURL: "https://example.com/doc.pdf",
				FileID:  "file-abc123",
			},
			want: "https://example.com/doc.pdf",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := tt.content.String(); got != tt.want {
				t.Errorf("String() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestInputFileContentIsPart(t *testing.T) {
	t.Parallel()

	// Test that InputFileContent implements ContentPart interface
	var _ ContentPart = InputFileContent{}
}

func TestInputFileContentUnmarshalError(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		json    string
		wantErr string
	}{
		{
			name:    "missing type",
			json:    `{"file_url": "https://example.com/doc.pdf"}`,
			wantErr: "invalid or missing \"type\" field",
		},
		{
			name:    "wrong type",
			json:    `{"type": "input_text", "file_url": "https://example.com/doc.pdf"}`,
			wantErr: "invalid or missing \"type\" field",
		},
		{
			name:    "missing both file_url and file_id",
			json:    `{"type": "input_file"}`,
			wantErr: "must have either file_url or file_id",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			var content InputFileContent
			err := json.Unmarshal([]byte(tt.json), &content)
			if err == nil {
				t.Fatal("Expected error, got nil")
			}
			if tt.wantErr != "" && !contains(err.Error(), tt.wantErr) {
				t.Errorf("Error message %q does not contain %q", err.Error(), tt.wantErr)
			}
		})
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		(len(s) > 0 && len(substr) > 0 && findSubstring(s, substr)))
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
