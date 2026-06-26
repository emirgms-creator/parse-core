package llm

import (
	"encoding/json"
	"fmt"
	"os"
)

// Schema defines the prompt rules and JSON structure expected for a specific document format.
type Schema struct {
	Name         string
	SystemPrompt string
}

// Schemas contains predefined data extraction schemas tailored to specific document types.
var Schemas = map[string]Schema{
	"generic": {
		Name: "generic",
		SystemPrompt: `You are a privacy-first, local data ingestion and structuring engine. 
Your task is to analyze the unstructured text provided, ignore noise (headers, footers, page numbers), and extract the key information.

You must respond ONLY with a JSON object matching this schema:
{
  "title": "A descriptive, concise title representing the core subject of the text",
  "summary": "A short summary (1-2 sentences) of the text's contents",
  "key_points": ["List", "of", "important", "facts", "or", "points", "extracted"]
}

Do not include any conversational preamble, postscript, or explanation. Output ONLY the raw JSON object itself.`,
	},
	"contract": {
		Name: "contract",
		SystemPrompt: `You are a privacy-first, local contract analysis and structuring engine. 
Your task is to analyze the unstructured contract text provided, ignore noise (headers, footers, page numbers), and extract the core contract terms.

You must respond ONLY with a JSON object matching this schema:
{
  "parties_involved": ["Name of Party A", "Name of Party B"],
  "effective_date": "YYYY-MM-DD or standard date string representation",
  "termination_clauses": ["List of key clauses related to contract termination, duration, or exit terms"],
  "obligations": ["Summary of key deliverables, obligations, or promises for each party"]
}

Do not include any conversational preamble, postscript, or explanation. Output ONLY the raw JSON object itself.`,
	},
	"invoice": {
		Name: "invoice",
		SystemPrompt: `You are a privacy-first, local invoice parsing and structuring engine. 
Your task is to analyze the unstructured invoice text provided, ignore noise (headers, footers, page numbers), and extract invoice billing items.

You must respond ONLY with a JSON object matching this schema:
{
  "invoice_number": "The unique reference or invoice number identifier",
  "total_amount": "Total due amount as a string (with currency symbol)",
  "vendor_name": "The billing vendor, supplier, or company name",
  "line_items": ["List of individual items billed, quantity, and their costs"]
}

Do not include any conversational preamble, postscript, or explanation. Output ONLY the raw JSON object itself.`,
	},
}

// ValidateSchema verifies whether the given schema name exists in the predefined set.
func ValidateSchema(name string) bool {
	_, exists := Schemas[name]
	return exists
}

// ResolveSystemPrompt resolves the system prompt for a predefined schema name
// or dynamically loads and compiles a custom system prompt from a local JSON schema file.
func ResolveSystemPrompt(schemaInput string) (string, error) {
	// 1. Check if it matches a predefined schema
	if schema, exists := Schemas[schemaInput]; exists {
		return schema.SystemPrompt, nil
	}

	// 2. Otherwise, treat schemaInput as a local file path containing custom JSON schema
	data, err := os.ReadFile(schemaInput)
	if err != nil {
		return "", fmt.Errorf("failed to read schema file: %w", err)
	}

	// Verify the loaded file is valid JSON
	var check map[string]interface{}
	if err := json.Unmarshal(data, &check); err != nil {
		return "", fmt.Errorf("invalid JSON format in schema file %q: %w", schemaInput, err)
	}

	// Marshal back to a compact, single-line JSON string for LLM readability
	compactBytes, err := json.Marshal(check)
	if err != nil {
		return "", fmt.Errorf("failed to format schema JSON: %w", err)
	}
	compactJSON := string(compactBytes)

	// Construct dynamic strict data extraction prompt
	customPrompt := fmt.Sprintf(`You are a privacy-first, local data ingestion and structuring engine. 
Your task is to analyze the unstructured text provided, ignore noise (headers, footers, page numbers), and extract the key information strictly matching the requested JSON structure.

Extract the following fields based on this structure: %s
If a field is not found in the text, return null. Do not invent or hallucinate data.

You must respond ONLY with a JSON object matching this structure.
Do not include any conversational preamble, postscript, or explanation. Output ONLY the raw JSON object itself.`, compactJSON)

	return customPrompt, nil
}
