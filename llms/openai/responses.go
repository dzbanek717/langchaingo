package openai

import (
	"context"
	"fmt"

	"github.com/tmc/langchaingo/llms"
	"github.com/tmc/langchaingo/llms/openai/internal/openaiclient"
)

// GenerateContentWithResponsesAPI uses the modern OpenAI Responses API (/v1/responses)
// which supports file inputs via file_url, file_id, or file_data.
//
// Use this method when you need to:
// - Send PDF, TXT, or DOCX files via URL (including presigned URLs)
// - Reference files uploaded to /v1/files with purpose="user_data"
//
// For InputFileContent parts, use llms.InputFilePart() or llms.InputFileIDPart()
//
// Reference: https://platform.openai.com/docs/api-reference/responses
func (o *LLM) GenerateContentWithResponsesAPI(ctx context.Context, messages []llms.MessageContent, options ...llms.CallOption) (*llms.ContentResponse, error) {
	if o.CallbacksHandler != nil {
		o.CallbacksHandler.HandleLLMGenerateContentStart(ctx, messages)
	}

	opts := llms.CallOptions{}
	for _, opt := range options {
		opt(&opts)
	}

	// Convert llms.MessageContent to ResponseMessage format
	responseMessages := make([]*openaiclient.ResponseMessage, 0, len(messages))
	for _, mc := range messages {
		msg := &openaiclient.ResponseMessage{}

		switch mc.Role {
		case llms.ChatMessageTypeSystem:
			msg.Role = RoleSystem
		case llms.ChatMessageTypeAI:
			msg.Role = RoleAssistant
		case llms.ChatMessageTypeHuman:
			msg.Role = RoleUser
		case llms.ChatMessageTypeGeneric:
			msg.Role = RoleUser
		case llms.ChatMessageTypeTool:
			msg.Role = RoleTool
			// Extract tool call ID from ToolCallResponse
			if len(mc.Parts) == 1 {
				if p, ok := mc.Parts[0].(llms.ToolCallResponse); ok {
					msg.ToolCallID = p.ToolCallID
					msg.Content = []openaiclient.ResponseContent{
						{
							Type: "input_text",
							Text: p.Content,
						},
					}
				}
			}
			responseMessages = append(responseMessages, msg)
			continue
		default:
			return nil, fmt.Errorf("role %v not supported in Responses API", mc.Role)
		}

		// Convert content parts to ResponseContent format
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
				// Convert ImageURLContent to input_image for Responses API
				msg.Content = append(msg.Content, openaiclient.ResponseContent{
					Type:     "input_image",
					ImageURL: p.URL,
				})
			case llms.ToolCall:
				// Tool calls are handled separately
				msg.ToolCalls = append(msg.ToolCalls, openaiclient.ToolCall{
					ID:   p.ID,
					Type: openaiclient.ToolType(p.Type),
					Function: openaiclient.ToolFunction{
						Name:      p.FunctionCall.Name,
						Arguments: p.FunctionCall.Arguments,
					},
				})
			case llms.BinaryContent:
				// BinaryContent not directly supported in Responses API
				// Would need to be converted to base64 file_data
				return nil, fmt.Errorf("BinaryContent not yet supported in Responses API")
			}
		}

		responseMessages = append(responseMessages, msg)
	}

	// Build the request
	req := &openaiclient.ResponseRequest{
		Model:                  opts.Model,
		Input:                  responseMessages,
		StreamingFunc:          opts.StreamingFunc,
		StreamingReasoningFunc: opts.StreamingReasoningFunc,
		Temperature:            opts.Temperature,
		N:                      opts.N,
		StopWords:              opts.StopWords,
		Seed:                   opts.Seed,
		MaxTokens:              opts.MaxTokens,
		Metadata:               opts.Metadata,
		Store:                  false,
	}

	// Add tools
	for _, fn := range opts.Functions {
		req.Tools = append(req.Tools, openaiclient.Tool{
			Type: "function",
			Function: openaiclient.FunctionDefinition{
				Name:        fn.Name,
				Description: fn.Description,
				Parameters:  fn.Parameters,
				Strict:      fn.Strict,
			},
		})
	}

	for _, tool := range opts.Tools {
		t, err := toolFromTool(tool)
		if err != nil {
			return nil, fmt.Errorf("failed to convert llms tool to openai tool: %w", err)
		}
		req.Tools = append(req.Tools, t)
	}

	if opts.ToolChoice != nil {
		req.ToolChoice = opts.ToolChoice
	}

	// Make the request
	result, err := o.client.CreateResponse(ctx, req)
	if err != nil {
		return nil, err
	}

	// Responses API uses "output" array instead of "choices"
	if len(result.Output) == 0 {
		return nil, ErrEmptyResponse
	}

	// Convert response to llms.ContentResponse
	// The Responses API can have multiple messages in output, but we'll use the first one
	choices := make([]*llms.ContentChoice, 0, len(result.Output))
	for _, outputMsg := range result.Output {
		if outputMsg.Type != "message" {
			continue // Skip non-message outputs
		}

		// Extract text content from output message
		var content string
		for _, rc := range outputMsg.Content {
			// Responses API uses "output_text" in responses
			if rc.Type == "output_text" {
				content += rc.Text
			}
		}

		choice := &llms.ContentChoice{
			Content:    content,
			StopReason: outputMsg.Status, // "completed", "failed", etc.
			GenerationInfo: map[string]any{
				"CompletionTokens":  result.Usage.CompletionTokens,
				"PromptTokens":      result.Usage.PromptTokens,
				"TotalTokens":       result.Usage.TotalTokens,
				"ReasoningTokens":   result.Usage.CompletionTokensDetails.ReasoningTokens,
				"PromptAudioTokens": result.Usage.PromptTokensDetails.AudioTokens,
				// Standardized fields
				"ThinkingContent":                    "", // Responses API doesn't expose this separately yet
				"ThinkingTokens":                     result.Usage.CompletionTokensDetails.ReasoningTokens,
				"PromptCachedTokens":                 result.Usage.PromptTokensDetails.CachedTokens,
				"CompletionAudioTokens":              result.Usage.CompletionTokensDetails.AudioTokens,
				"CompletionReasoningTokens":          result.Usage.CompletionTokensDetails.ReasoningTokens,
				"CompletionAcceptedPredictionTokens": result.Usage.CompletionTokensDetails.AcceptedPredictionTokens,
				"CompletionRejectedPredictionTokens": result.Usage.CompletionTokensDetails.RejectedPredictionTokens,
			},
		}

		// Handle tool calls from the output message
		for _, tc := range outputMsg.ToolCalls {
			choice.ToolCalls = append(choice.ToolCalls, llms.ToolCall{
				ID:   tc.ID,
				Type: string(tc.Type),
				FunctionCall: &llms.FunctionCall{
					Name:      tc.Function.Name,
					Arguments: tc.Function.Arguments,
				},
			})
		}

		// Populate legacy single-function call field for backwards compatibility
		if len(choice.ToolCalls) > 0 {
			choice.FuncCall = choice.ToolCalls[0].FunctionCall
		}

		choices = append(choices, choice)
	}

	response := &llms.ContentResponse{Choices: choices}
	if o.CallbacksHandler != nil {
		o.CallbacksHandler.HandleLLMGenerateContentEnd(ctx, response)
	}

	return response, nil
}
