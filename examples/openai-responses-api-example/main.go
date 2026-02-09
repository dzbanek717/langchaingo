package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/tmc/langchaingo/llms"
	"github.com/tmc/langchaingo/llms/openai"
)

func main() {
	ctx := context.Background()

	// Initialize OpenAI LLM
	llm, err := openai.New()
	if err != nil {
		log.Fatal(err)
	}

	// Example 1: Using the Responses API with a file URL
	// This demonstrates using OpenAI's modern /v1/responses endpoint
	// which supports input_file content type with file_url
	fmt.Println("=== Example 1: PDF Analysis via URL ===")

	// Use a publicly accessible PDF for demonstration
	// In production, you would use presigned URLs from S3, GCS, etc.
	pdfURL := "https://www.berkshirehathaway.com/letters/2024ltr.pdf"

	messages1 := []llms.MessageContent{
		{
			Role: llms.ChatMessageTypeHuman,
			Parts: []llms.ContentPart{
				llms.InputFilePart(pdfURL),
				llms.TextPart("Please provide a brief summary of the key points in this letter."),
			},
		},
	}

	response1, err := llm.GenerateContentWithResponsesAPI(ctx, messages1, llms.WithModel("gpt-4o"))
	if err != nil {
		log.Printf("Example 1 error: %v\n", err)
	} else {
		fmt.Printf("Response: %s\n\n", response1.Choices[0].Content)
	}

	// Example 2: Using file_id (requires uploading file first)
	// This demonstrates using a file ID from OpenAI's /v1/files endpoint
	fmt.Println("=== Example 2: Using File ID ===")

	fileID := os.Getenv("OPENAI_FILE_ID") // Set this to a file you've uploaded
	if fileID == "" {
		fmt.Println("Skipping Example 2: OPENAI_FILE_ID not set")
		fmt.Println("To use file_id, first upload a file with purpose='user_data'")
	} else {
		messages2 := []llms.MessageContent{
			{
				Role: llms.ChatMessageTypeHuman,
				Parts: []llms.ContentPart{
					llms.InputFileIDPart(fileID),
					llms.TextPart("What are the main topics covered in this document?"),
				},
			},
		}

		response2, err := llm.GenerateContentWithResponsesAPI(ctx, messages2, llms.WithModel("gpt-4o"))
		if err != nil {
			log.Printf("Example 2 error: %v\n", err)
		} else {
			fmt.Printf("Response: %s\n\n", response2.Choices[0].Content)
		}
	}

	// Example 3: Multiple files and text in one message
	fmt.Println("=== Example 3: Multiple Files ===")

	// Using two Berkshire Hathaway shareholder letters from different years
	doc1URL := "https://www.berkshirehathaway.com/letters/2023ltr.pdf"
	doc2URL := "https://www.berkshirehathaway.com/letters/2024ltr.pdf"

	messages3 := []llms.MessageContent{
		{
			Role: llms.ChatMessageTypeHuman,
			Parts: []llms.ContentPart{
				llms.TextPart("Compare these two annual shareholder letters and highlight the main differences in themes or focus areas."),
				llms.InputFilePart(doc1URL),
				llms.InputFilePart(doc2URL),
			},
		},
	}

	response3, err := llm.GenerateContentWithResponsesAPI(ctx, messages3, llms.WithModel("gpt-4o"))
	if err != nil {
		log.Printf("Example 3 error: %v\n", err)
	} else {
		fmt.Printf("Response: %s\n\n", response3.Choices[0].Content)
	}

	// Example 4: Using presigned S3 URLs
	fmt.Println("=== Example 4: Presigned S3 URL ===")

	// In production, generate a presigned URL like this:
	// s3Client := s3.New(session.Must(session.NewSession()))
	// req, _ := s3Client.GetObjectRequest(&s3.GetObjectInput{
	//     Bucket: aws.String("my-bucket"),
	//     Key:    aws.String("documents/report.pdf"),
	// })
	// presignedURL, err := req.Presign(15 * time.Minute)

	presignedURL := os.Getenv("S3_PRESIGNED_URL")
	if presignedURL == "" {
		fmt.Println("Skipping Example 4: S3_PRESIGNED_URL not set")
		fmt.Println("Set S3_PRESIGNED_URL to test with presigned URLs")
	} else {
		messages4 := []llms.MessageContent{
			{
				Role: llms.ChatMessageTypeHuman,
				Parts: []llms.ContentPart{
					llms.InputFilePart(presignedURL),
					llms.TextPart("Analyze this document and extract the key insights."),
				},
			},
		}

		response4, err := llm.GenerateContentWithResponsesAPI(ctx, messages4, llms.WithModel("gpt-4o"))
		if err != nil {
			log.Printf("Example 4 error: %v\n", err)
		} else {
			fmt.Printf("Response: %s\n\n", response4.Choices[0].Content)
		}
	}

	// Example 5: Using ImageURLContent with Responses API
	// ImageURLContent is automatically converted to input_image when using Responses API
	fmt.Println("=== Example 5: ImageURLContent Compatibility ===")

	// Using a public domain image from Unsplash
	imageURL := "https://images.unsplash.com/photo-1506905925346-21bda4d32df4?w=800"
	messages5 := []llms.MessageContent{
		{
			Role: llms.ChatMessageTypeHuman,
			Parts: []llms.ContentPart{
				llms.ImageURLPart(imageURL),
				llms.TextPart("Describe what you see in this image."),
			},
		},
	}

	response5, err := llm.GenerateContentWithResponsesAPI(ctx, messages5, llms.WithModel("gpt-4o"))
	if err != nil {
		log.Printf("Example 5 error: %v\n", err)
	} else {
		fmt.Printf("Response: %s\n\n", response5.Choices[0].Content)
	}
}
