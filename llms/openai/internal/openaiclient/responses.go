package openaiclient

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

// ResponseRequest is a request to the OpenAI Responses API (/v1/responses).
// This is the modern API that supports file inputs via file_url, file_id, or file_data.
//
// The Responses API uses "input" instead of "messages" and supports:
// - input_text: Regular text content
// - input_file: File references (PDF, TXT, DOCX) via URL, ID, or base64 data
//
// Reference: https://platform.openai.com/docs/api-reference/responses
type ResponseRequest struct {
	Model       string             `json:"model"`
	Input       []*ResponseMessage `json:"input"` // Uses "input" instead of "messages"
	Temperature float64            `json:"temperature,omitempty"`
	TopP        float64            `json:"top_p,omitempty"`
	MaxTokens   int                `json:"max_tokens,omitempty"`
	N           int                `json:"n,omitempty"`
	StopWords   []string           `json:"stop,omitempty"`
	Stream      bool               `json:"stream,omitempty"`
	Seed        int                `json:"seed,omitempty"`
	Tools       []Tool             `json:"tools,omitempty"`
	ToolChoice  any                `json:"tool_choice,omitempty"`
	Metadata    map[string]any     `json:"metadata,omitempty"`

	// Store controls whether the response is stored by OpenAI.
	Store bool `json:"store,omitempty"`

	// Streaming functions (not sent to API)
	StreamingFunc          func(ctx context.Context, chunk []byte) error                 `json:"-"`
	StreamingReasoningFunc func(ctx context.Context, reasoningChunk, chunk []byte) error `json:"-"`
}

// ResponseMessage represents a message in the Responses API.
// Content can contain input_text and input_file parts.
type ResponseMessage struct {
	Role    string            `json:"role"`
	Content []ResponseContent `json:"content,omitempty"`

	// For assistant messages with tool calls
	ToolCalls []ToolCall `json:"tool_calls,omitempty"`

	// For tool response messages
	ToolCallID string `json:"tool_call_id,omitempty"`
}

// ResponseContent represents content in a ResponseMessage.
// Can be input_text, input_file, or input_image type.
type ResponseContent struct {
	Type string `json:"type"` // "input_text", "input_file", or "input_image"

	// For input_text
	Text string `json:"text,omitempty"`

	// For input_file (documents: PDF, TXT, DOCX)
	FileURL string `json:"file_url,omitempty"` // External URL (including presigned URLs)
	FileID  string `json:"file_id,omitempty"`  // File ID from /v1/files upload
	// FileData string `json:"file_data,omitempty"` // Base64-encoded file data (future)

	// For input_image (images)
	ImageURL string `json:"image_url,omitempty"` // Image URL
}

// ResponseCompletionResponse is the response from the Responses API.
// The Responses API uses "output" instead of "choices" and has a different structure.
type ResponseCompletionResponse struct {
	ID        string                   `json:"id,omitempty"`
	Object    string                   `json:"object,omitempty"`
	CreatedAt int64                    `json:"created_at,omitempty"`
	Status    string                   `json:"status,omitempty"`
	Model     string                   `json:"model,omitempty"`
	Output    []*ResponseOutputMessage `json:"output,omitempty"`
	Usage     ChatUsage                `json:"usage,omitempty"`

	// Legacy fields for backward compatibility (not used in Responses API)
	Choices           []*ResponseCompletionChoice `json:"choices,omitempty"`
	SystemFingerprint string                      `json:"system_fingerprint,omitempty"`
}

// ResponseOutputMessage represents a message in the output array.
type ResponseOutputMessage struct {
	ID        string            `json:"id,omitempty"`
	Type      string            `json:"type"` // "message"
	Status    string            `json:"status,omitempty"`
	Role      string            `json:"role"` // "assistant"
	Content   []ResponseContent `json:"content"`
	ToolCalls []ToolCall        `json:"tool_calls,omitempty"` // Tool calls requested by the model
}

// ResponseCompletionChoice represents a choice in the response (legacy).
type ResponseCompletionChoice struct {
	Index        int             `json:"index"`
	Message      ResponseMessage `json:"message"`
	FinishReason FinishReason    `json:"finish_reason"`
}

// StreamedResponsePayload is a chunk from the streaming response.
type StreamedResponsePayload struct {
	ID      string  `json:"id,omitempty"`
	Created float64 `json:"created,omitempty"`
	Model   string  `json:"model,omitempty"`
	Object  string  `json:"object,omitempty"`
	Choices []struct {
		Index float64 `json:"index,omitempty"`
		Delta struct {
			Role             string      `json:"role,omitempty"`
			Content          string      `json:"content,omitempty"`
			ToolCalls        []*ToolCall `json:"tool_calls,omitempty"`
			ReasoningContent string      `json:"reasoning_content,omitempty"`
		} `json:"delta,omitempty"`
		FinishReason FinishReason `json:"finish_reason,omitempty"`
	} `json:"choices,omitempty"`
	SystemFingerprint string `json:"system_fingerprint"`
	Usage             *Usage `json:"usage,omitempty"`
	Error             error  `json:"-"`
}

// CreateResponse makes a request to the /v1/responses endpoint.
func (c *Client) CreateResponse(ctx context.Context, req *ResponseRequest) (*ResponseCompletionResponse, error) {
	if req.StreamingFunc != nil || req.StreamingReasoningFunc != nil {
		req.Stream = true
	}

	// Build request payload
	payloadBytes, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}

	// Build HTTP request
	body := bytes.NewReader(payloadBytes)
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.buildURL("/responses", req.Model), body)
	if err != nil {
		return nil, err
	}

	c.setHeaders(httpReq)

	// Send request
	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, sanitizeHTTPError(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		msg := fmt.Sprintf("API returned unexpected status code: %d", resp.StatusCode)

		var errResp errorMessage
		if err := json.NewDecoder(resp.Body).Decode(&errResp); err != nil {
			return nil, fmt.Errorf("%s", msg)
		}

		return nil, fmt.Errorf("%s: %s", msg, errResp.Error.Message)
	}

	if req.StreamingFunc != nil || req.StreamingReasoningFunc != nil {
		return parseStreamingResponseResponse(ctx, resp, req)
	}

	// Parse response
	var response ResponseCompletionResponse
	return &response, json.NewDecoder(resp.Body).Decode(&response)
}

func parseStreamingResponseResponse(ctx context.Context, r *http.Response, payload *ResponseRequest) (*ResponseCompletionResponse, error) {
	scanner := bufio.NewScanner(r.Body)
	responseChan := make(chan StreamedResponsePayload)

	readerCtx, cancelReader := context.WithCancel(ctx)
	defer cancelReader()

	go func() {
		defer close(responseChan)
		for scanner.Scan() {
			select {
			case <-readerCtx.Done():
				return
			default:
			}

			line := scanner.Text()
			if line == "" {
				continue
			}

			// Skip SSE comment lines
			if strings.HasPrefix(line, ":") {
				continue
			}

			// Only process lines that start with "data:"
			if !strings.HasPrefix(line, "data:") {
				continue
			}

			data := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
			if data == "[DONE]" {
				return
			}

			var streamPayload StreamedResponsePayload
			if err := json.Unmarshal([]byte(data), &streamPayload); err != nil {
				continue
			}

			select {
			case <-readerCtx.Done():
				return
			case responseChan <- streamPayload:
			}
		}

		if err := scanner.Err(); err != nil {
			select {
			case <-readerCtx.Done():
				return
			case responseChan <- StreamedResponsePayload{Error: fmt.Errorf("error reading streaming response: %w", err)}:
			}
		}
	}()

	return combineStreamingResponseResponse(readerCtx, payload, responseChan)
}

func combineStreamingResponseResponse(
	ctx context.Context,
	payload *ResponseRequest,
	responseChan chan StreamedResponsePayload,
) (*ResponseCompletionResponse, error) {
	response := ResponseCompletionResponse{
		Output: []*ResponseOutputMessage{
			{
				Type:    "message",
				Role:    "assistant",
				Content: []ResponseContent{},
			},
		},
	}

	var contentBuilder strings.Builder
	var reasoningBuilder strings.Builder

	for streamResponse := range responseChan {
		if streamResponse.Error != nil {
			return nil, streamResponse.Error
		}

		if streamResponse.Usage != nil {
			response.Usage.CompletionTokens = streamResponse.Usage.CompletionTokens
			response.Usage.PromptTokens = streamResponse.Usage.PromptTokens
			response.Usage.TotalTokens = streamResponse.Usage.TotalTokens
			response.Usage.PromptTokensDetails = streamResponse.Usage.PromptTokensDetails
			response.Usage.CompletionTokensDetails = streamResponse.Usage.CompletionTokensDetails
		}

		if len(streamResponse.Choices) == 0 {
			continue
		}

		choice := streamResponse.Choices[0]
		chunk := []byte(choice.Delta.Content)
		reasoningChunk := []byte(choice.Delta.ReasoningContent)

		contentBuilder.WriteString(choice.Delta.Content)
		reasoningBuilder.WriteString(choice.Delta.ReasoningContent)

		response.Output[0].Status = string(choice.FinishReason)

		// TODO: Handle tool calls in streaming for Responses API
		// if len(choice.Delta.ToolCalls) > 0 { ... }

		if payload.StreamingFunc != nil {
			if err := payload.StreamingFunc(ctx, chunk); err != nil {
				return nil, fmt.Errorf("streaming func returned an error: %w", err)
			}
		}

		if payload.StreamingReasoningFunc != nil {
			if err := payload.StreamingReasoningFunc(ctx, reasoningChunk, chunk); err != nil {
				return nil, fmt.Errorf("streaming reasoning func returned an error: %w", err)
			}
		}
	}

	// Set final content
	if contentBuilder.Len() > 0 {
		response.Output[0].Content = []ResponseContent{
			{
				Type: "output_text",
				Text: contentBuilder.String(),
			},
		}
		response.Output[0].Status = "completed"
	}

	return &response, nil
}
