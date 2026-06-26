package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// Client handles communication with the local Ollama LLM engine.
type Client struct {
	BaseURL    string
	Model      string
	HTTPClient *http.Client
}

// NewClient creates a new Client configured to call Ollama.
func NewClient(baseURL string, model string) *Client {
	if baseURL == "" {
		baseURL = "http://localhost:11434"
	}
	if model == "" {
		model = "phi-4"
	}
	return &Client{
		BaseURL: baseURL,
		Model:   model,
		HTTPClient: &http.Client{
			Timeout: 120 * time.Second, // Local LLM generation can be slow on CPU/older GPUs
		},
	}
}

// GenerateRequest defines the payload schema for Ollama's /api/generate endpoint.
type GenerateRequest struct {
	Model   string                 `json:"model"`
	Prompt  string                 `json:"prompt"`
	System  string                 `json:"system,omitempty"`
	Format  string                 `json:"format,omitempty"`
	Stream  bool                   `json:"stream"`
	Options map[string]interface{} `json:"options,omitempty"`
}

// GenerateResponse defines the expected response payload from Ollama's /api/generate endpoint.
type GenerateResponse struct {
	Model     string    `json:"model"`
	CreatedAt time.Time `json:"created_at"`
	Response  string    `json:"response"`
	Done      bool      `json:"done"`
}

// ParseText sends raw text to Ollama, injecting the resolved systemPrompt,
// and returns the raw validated JSON response payload.
// Sets temperature to 0.0 to enforce strict deterministic extraction.
// Implements robust retry logic with exponential backoff on HTTP status/network errors.
func (c *Client) ParseText(ctx context.Context, text string, systemPrompt string) (json.RawMessage, error) {
	url := fmt.Sprintf("%s/api/generate", c.BaseURL)

	reqPayload := GenerateRequest{
		Model:  c.Model,
		Prompt: text,
		System: systemPrompt,
		Format: "json", // Enforce JSON formatting at the Ollama engine level
		Stream: false,
		Options: map[string]interface{}{
			"temperature": 0.0, // Enforce zero creativity / strict deterministic output
		},
	}

	reqBytes, err := json.Marshal(reqPayload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request payload: %w", err)
	}

	var resp *http.Response
	maxRetries := 3
	backoff := 1 * time.Second

	for i := 0; i < maxRetries; i++ {
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(reqBytes))
		if err != nil {
			return nil, fmt.Errorf("failed to build http request: %w", err)
		}
		req.Header.Set("Content-Type", "application/json")

		resp, err = c.HTTPClient.Do(req)
		if err == nil && resp.StatusCode == http.StatusOK {
			break
		}

		// Handle retry fallback limits
		if i == maxRetries-1 {
			if err != nil {
				return nil, fmt.Errorf("http request to Ollama failed after %d retries: %w", maxRetries, err)
			}
			bodyBytes, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			return nil, fmt.Errorf("ollama API returned status %s after %d retries: %s", resp.Status, maxRetries, string(bodyBytes))
		}

		// Close body of intermediate failed response to prevent resource leaks
		if resp != nil && resp.Body != nil {
			resp.Body.Close()
		}

		// Backoff pause respecting context completion
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(backoff):
			backoff *= 2
		}
	}
	defer resp.Body.Close()

	var genResp GenerateResponse
	if err := json.NewDecoder(resp.Body).Decode(&genResp); err != nil {
		return nil, fmt.Errorf("failed to decode Ollama response JSON: %w", err)
	}

	// Validate that the output string is indeed structurally valid JSON
	rawJSON := json.RawMessage(genResp.Response)
	if !json.Valid(rawJSON) {
		return nil, fmt.Errorf("ollama returned invalid JSON representation: %q", genResp.Response)
	}

	return rawJSON, nil
}
