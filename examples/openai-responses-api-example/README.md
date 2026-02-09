# OpenAI Responses API Example

This example demonstrates how to use OpenAI's modern **Responses API** (`/v1/responses`) with langchaingo to process documents (PDF, TXT, DOCX) and images using presigned URLs or file IDs.

## What is the Responses API?

The Responses API is OpenAI's modern endpoint that replaces the Chat Completions API for many use cases. It provides enhanced capabilities for file and image handling:

### Key Features

- **Direct URL Support**: Reference files via `file_url` (including presigned URLs from S3, GCS, Azure)
- **Image Support**: Reference images via `image_url`
- **File ID Support**: Use files uploaded to `/v1/files` with `purpose="user_data"`
- **Modern Structure**: Uses `input` instead of `messages`, with `input_text`, `input_file`, and `input_image` content types
- **Document Processing**: Automatically extracts both text and page images from PDFs

### Content Types

The Responses API uses different content types than Chat Completions:

**Request (input):**
- `input_text`: Regular text content (replaces `text`)
- `input_file`: Document file references (PDF, TXT, DOCX) via `file_url` or `file_id`
- `input_image`: Image references via `image_url`

**Response (output):**
- `output_text`: Text content returned by the model

## Installation

```bash
go get github.com/tmc/langchaingo
```

## Usage

### Basic Example with File URL

```go
import (
    "context"
    "github.com/tmc/langchaingo/llms"
    "github.com/tmc/langchaingo/llms/openai"
)

llm, _ := openai.New()

messages := []llms.MessageContent{
    {
        Role: llms.ChatMessageTypeHuman,
        Parts: []llms.ContentPart{
            llms.InputFilePart("https://example.com/document.pdf"),
            llms.TextPart("Summarize this document."),
        },
    },
}

response, err := llm.GenerateContentWithResponsesAPI(
    context.Background(),
    messages,
    llms.WithModel("gpt-4o"),
)
```

### Using Presigned S3 URLs

```go
// Generate presigned URL (AWS SDK example)
import (
    "github.com/aws/aws-sdk-go/service/s3"
    "time"
)

req, _ := s3Client.GetObjectRequest(&s3.GetObjectInput{
    Bucket: aws.String("my-bucket"),
    Key:    aws.String("documents/report.pdf"),
})
presignedURL, _ := req.Presign(15 * time.Minute)

// Use with Responses API
messages := []llms.MessageContent{
    {
        Role: llms.ChatMessageTypeHuman,
        Parts: []llms.ContentPart{
            llms.InputFilePart(presignedURL),
            llms.TextPart("Analyze this report."),
        },
    },
}

response, _ := llm.GenerateContentWithResponsesAPI(ctx, messages)
```

### Using File IDs

First, upload a file to OpenAI:

```bash
curl https://api.openai.com/v1/files \
  -H "Authorization: Bearer $OPENAI_API_KEY" \
  -F purpose="user_data" \
  -F file="@document.pdf"
```

Then use the file ID:

```go
messages := []llms.MessageContent{
    {
        Role: llms.ChatMessageTypeHuman,
        Parts: []llms.ContentPart{
            llms.InputFileIDPart("file-abc123"),
            llms.TextPart("What are the main points?"),
        },
    },
}

response, _ := llm.GenerateContentWithResponsesAPI(ctx, messages)
```

### Multiple Files

```go
messages := []llms.MessageContent{
    {
        Role: llms.ChatMessageTypeHuman,
        Parts: []llms.ContentPart{
            llms.TextPart("Compare these documents:"),
            llms.InputFilePart("https://example.com/doc1.pdf"),
            llms.InputFilePart("https://example.com/doc2.pdf"),
            llms.TextPart("What are the differences?"),
        },
    },
}
```

## API Comparison

### Chat Completions API (Legacy)

```json
{
  "model": "gpt-4o",
  "messages": [
    {
      "role": "user",
      "content": [
        { "type": "text", "text": "Analyze this." },
        { "type": "image_url", "image_url": {"url": "..."} }
      ]
    }
  ]
}
```

### Responses API (Modern)

**Document example:**
```json
{
  "model": "gpt-4o",
  "input": [
    {
      "role": "user",
      "content": [
        { "type": "input_text", "text": "What is in this file?" },
        { "type": "input_file", "file_url": "https://example.com/doc.pdf" }
      ]
    }
  ]
}
```

**Image example:**
```json
{
  "model": "gpt-4o",
  "input": [
    {
      "role": "user",
      "content": [
        { "type": "input_text", "text": "What is in this image?" },
        { "type": "input_image", "image_url": "https://example.com/image.jpg" }
      ]
    }
  ]
}
```

## Content Type Reference

### InputFileContent

The main content type for file references in the Responses API:

```go
// Using file URL (recommended for presigned URLs)
llms.InputFilePart("https://bucket.s3.amazonaws.com/doc.pdf?...")

// Using file ID (for uploaded files)
llms.InputFileIDPart("file-abc123")

// Direct construction
llms.InputFileContent{
    FileURL: "https://example.com/doc.pdf",
}

llms.InputFileContent{
    FileID: "file-abc123",
}
```

### JSON Marshaling

InputFileContent marshals to:

```json
{
  "type": "input_file",
  "file_url": "https://example.com/doc.pdf"
}
```

or

```json
{
  "type": "input_file",
  "file_id": "file-abc123"
}
```

## Backwards Compatibility

The library automatically converts content types to Responses API format:

- `TextContent` → `input_text`
- `ImageURLContent` → `input_image` with `image_url`
- `InputFileContent` → `input_file` with `file_url` or `file_id`

This means existing code using `llms.ImageURLPart()` for images will work with `GenerateContentWithResponsesAPI()`.

## Running the Example

1. Set your OpenAI API key:
   ```bash
   export OPENAI_API_KEY=your-api-key-here
   ```

2. Run the example:
   ```bash
   go run main.go
   ```

3. Optional environment variables:
   ```bash
   export OPENAI_FILE_ID=file-abc123  # For Example 2
   export S3_PRESIGNED_URL=https://...  # For Example 4
   ```

## Supported File Types

The Responses API supports:

- **PDF**: `.pdf` files
- **Text**: `.txt` files
- **Word**: `.docx` files

For PDFs, OpenAI extracts both:
- The text content
- An image of each page

This allows the model to understand both textual and visual elements.

## Best Practices

1. **Use Presigned URLs**: For cloud storage (S3, GCS, Azure), generate presigned URLs instead of uploading files to OpenAI
2. **Set Expiration**: Make presigned URLs valid for at least 15 minutes to account for processing time
3. **File Size**: Keep files under the model's context window limits
4. **Error Handling**: Always check for errors, especially with URL-based files
5. **Model Selection**: Use `gpt-4o` or later models for best document understanding

## Common Issues

### File Not Accessible

If you get errors about files not being accessible:

1. Ensure the URL is publicly accessible or properly presigned
2. Check that presigned URLs haven't expired
3. Verify CORS settings if using cloud storage
4. Test the URL in a browser first

### Invalid File Format

The Responses API only supports PDF, TXT, and DOCX. For other formats:

1. Convert to a supported format first
2. Or extract text and send as `input_text`

## Migration from Chat Completions

To migrate from Chat Completions API:

1. Replace `GenerateContent()` with `GenerateContentWithResponsesAPI()`
2. Use `llms.InputFilePart()` for document files (PDFs, DOCX, TXT)
3. Use `llms.ImageURLPart()` for images (automatically converted to input_image)
4. Update any file handling logic to use the new endpoint

The Responses API is fully compatible with existing llms.MessageContent structures.

## References

- [OpenAI Responses API Documentation](https://platform.openai.com/docs/api-reference/responses)
- [File Inputs Guide](https://platform.openai.com/docs/guides/pdf-files)
- [Migration Guide](https://platform.openai.com/docs/guides/migrate-to-responses)
- [Azure OpenAI Responses API](https://learn.microsoft.com/en-us/azure/ai-foundry/openai/how-to/responses)

## Sources

This implementation is based on:

- [Responses API Reference](https://platform.openai.com/docs/api-reference/responses)
- [File inputs documentation](https://platform.openai.com/docs/guides/pdf-files)
- [How to Process PDFs via URL with the OpenAI API](https://www.cometapi.com/how-to-process-pdfs-via-url-with-the-openai-api/)
