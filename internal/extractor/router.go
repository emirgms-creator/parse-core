package extractor

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

// SchemaField defines a field in the target schema.
type SchemaField struct {
	Name    string
	IsArray bool
}

// GetSchemaFields returns the fields of a schema based on its name or file path.
func GetSchemaFields(schemaName string) ([]SchemaField, error) {
	switch schemaName {
	case "generic":
		return []SchemaField{
			{Name: "title", IsArray: false},
			{Name: "summary", IsArray: false},
			{Name: "key_points", IsArray: true},
		}, nil
	case "contract":
		return []SchemaField{
			{Name: "parties_involved", IsArray: true},
			{Name: "effective_date", IsArray: false},
			{Name: "termination_clauses", IsArray: true},
			{Name: "obligations", IsArray: true},
		}, nil
	case "invoice":
		return []SchemaField{
			{Name: "invoice_number", IsArray: false},
			{Name: "total_amount", IsArray: false},
			{Name: "vendor_name", IsArray: false},
			{Name: "line_items", IsArray: true},
		}, nil
	}

	// Try reading as custom JSON file path
	data, err := os.ReadFile(schemaName)
	if err != nil {
		return nil, fmt.Errorf("failed to read custom schema file: %w", err)
	}

	var check map[string]interface{}
	if err := json.Unmarshal(data, &check); err != nil {
		return nil, fmt.Errorf("invalid JSON in custom schema file: %w", err)
	}

	var fields []SchemaField
	for k, v := range check {
		isArray := false
		if _, ok := v.([]interface{}); ok {
			isArray = true
		}
		fields = append(fields, SchemaField{
			Name:    k,
			IsArray: isArray,
		})
	}
	return fields, nil
}

// GetAliases returns common CSV header name mappings for known schema fields.
func GetAliases(fieldName string) []string {
	fieldNameLower := strings.ToLower(strings.TrimSpace(fieldName))
	switch fieldNameLower {
	case "invoice_number":
		return []string{"invoice_number", "invoice number", "invoice_num", "invoice num", "inv_num", "inv num", "invoice#", "invoice_id", "invoice id", "id"}
	case "total_amount":
		return []string{"total_amount", "total amount", "total", "amount", "total_due", "total due", "price", "sum"}
	case "vendor_name":
		return []string{"vendor_name", "vendor name", "vendor", "company", "company_name", "company name", "supplier", "seller"}
	case "line_items":
		return []string{"line_items", "line items", "items", "description", "details"}
	case "parties_involved":
		return []string{"parties_involved", "parties involved", "parties", "party_a", "party_b", "contract_parties", "signatories"}
	case "effective_date":
		return []string{"effective_date", "effective date", "date", "start_date", "start date", "contract_date"}
	case "termination_clauses":
		return []string{"termination_clauses", "termination clauses", "termination", "exit_terms"}
	case "obligations":
		return []string{"obligations", "deliverables", "promises", "duties"}
	case "title":
		return []string{"title", "subject", "name", "topic"}
	case "summary":
		return []string{"summary", "description", "abstract"}
	case "key_points":
		return []string{"key_points", "key points", "bullet_points", "bullets", "points", "highlights"}
	default:
		return []string{fieldName}
	}
}

// CleanValue checks if a string is a standard dirty marker or empty, and cleans it.
// Returns the cleaned string (empty if dirty/empty) and a boolean indicating if it is clean.
func CleanValue(val string) (string, bool) {
	trimmed := strings.TrimSpace(val)
	upper := strings.ToUpper(trimmed)
	switch upper {
	case "ERROR", "UNKNOWN", "N/A", "NA", "NULL", "NONE", "", "-", "UNDEFINED":
		return "", false
	}
	return trimmed, true
}

// ParseList splits a string value into a slice of cleaned strings.
func ParseList(val string) []string {
	val = strings.TrimSpace(val)
	if val == "" {
		return nil
	}

	// Check if it's a JSON array
	if strings.HasPrefix(val, "[") && strings.HasSuffix(val, "]") {
		var list []string
		if err := json.Unmarshal([]byte(val), &list); err == nil {
			return list
		}
	}

	// Try splitting by common delimiters
	var parts []string
	if strings.Contains(val, "|") {
		parts = strings.Split(val, "|")
	} else if strings.Contains(val, ";") {
		parts = strings.Split(val, ";")
	} else if strings.Contains(val, ",") {
		parts = strings.Split(val, ",")
	} else {
		parts = []string{val}
	}

	var cleaned []string
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed != "" {
			cleaned = append(cleaned, trimmed)
		}
	}
	return cleaned
}

// isSemanticField returns true if the field requires natural language extraction or generation.
func isSemanticField(name string) bool {
	nameLower := strings.ToLower(name)
	switch nameLower {
	case "summary", "key_points", "obligations", "termination_clauses", "key points", "termination clauses":
		return true
	}
	return false
}

// isMetadataField returns true if the field represents structured metadata.
func isMetadataField(name string) bool {
	nameLower := strings.ToLower(name)
	switch nameLower {
	case "invoice_number", "total_amount", "vendor_name", "effective_date", "invoice number", "total amount", "vendor name", "effective date":
		return true
	}
	return false
}

// isComplexText returns true if a string contains complex sentences or a large number of words.
func isComplexText(val string) bool {
	trimmed := strings.TrimSpace(val)
	if len(trimmed) > 150 {
		return true
	}
	words := strings.Fields(trimmed)
	if len(words) > 15 {
		return true
	}
	if strings.Contains(trimmed, ". ") || strings.Contains(trimmed, "; ") {
		return true
	}
	return false
}

// CleanRow attempts to map a parsed row into the target schema deterministically.
// Returns the JSON raw message representation and true if successful (Fast-Pass).
// Returns nil and false if LLM inference is needed (AI-Pass).
func CleanRow(row Row, fields []SchemaField) (json.RawMessage, bool) {
	outputMap := make(map[string]interface{})
	mappedFieldsCount := 0

	for _, field := range fields {
		aliases := GetAliases(field.Name)
		rawVal, found := row.GetWithAliases(aliases...)

		if !found {
			// If a core semantic field is missing, we must fallback to the LLM to write/extract it.
			if isSemanticField(field.Name) {
				return nil, false
			}
			if field.IsArray {
				outputMap[field.Name] = []string{}
			} else {
				outputMap[field.Name] = nil
			}
			continue
		}

		cleanedVal, isClean := CleanValue(rawVal)
		if !isClean {
			// If it's a semantic field and contains dirty placeholder, LLM needs to resolve it.
			if isSemanticField(field.Name) {
				return nil, false
			}
			if field.IsArray {
				outputMap[field.Name] = []string{}
			} else {
				outputMap[field.Name] = nil
			}
			mappedFieldsCount++
			continue
		}

		// Verify that a metadata field does not contain complex text
		if !field.IsArray && isMetadataField(field.Name) {
			if isComplexText(cleanedVal) {
				return nil, false
			}
		}

		if field.IsArray {
			outputMap[field.Name] = ParseList(cleanedVal)
		} else {
			outputMap[field.Name] = cleanedVal
		}
		mappedFieldsCount++
	}

	if mappedFieldsCount == 0 && len(fields) > 0 {
		return nil, false
	}

	jsonBytes, err := json.Marshal(outputMap)
	if err != nil {
		return nil, false
	}
	return json.RawMessage(jsonBytes), true
}

// GetSchemaFieldsFromRows dynamically constructs schema fields from the columns of the first row.
func GetSchemaFieldsFromRows(rows []Row) []SchemaField {
	if len(rows) == 0 {
		return nil
	}
	var fields []SchemaField
	for _, col := range rows[0].Columns {
		// Basic type heuristic: if column name contains "items", "list", "points", or "clauses", treat as array
		nameLower := strings.ToLower(col.Name)
		isArray := strings.Contains(nameLower, "items") ||
			strings.Contains(nameLower, "list") ||
			strings.Contains(nameLower, "points") ||
			strings.Contains(nameLower, "clauses") ||
			strings.Contains(nameLower, "parties") ||
			strings.Contains(nameLower, "obligations")

		fields = append(fields, SchemaField{
			Name:    col.Name,
			IsArray: isArray,
		})
	}
	return fields
}

// GenerateSystemPromptForFields constructs a strict data extraction prompt for the LLM using the dynamic fields.
func GenerateSystemPromptForFields(fields []SchemaField) string {
	templateMap := make(map[string]interface{})
	for _, f := range fields {
		if f.IsArray {
			templateMap[f.Name] = []string{"List of items"}
		} else {
			templateMap[f.Name] = "Extracted value"
		}
	}
	compactBytes, _ := json.Marshal(templateMap)

	return fmt.Sprintf(`You are a privacy-first, local data ingestion and structuring engine. 
Your task is to analyze the unstructured text provided, ignore noise (headers, footers, page numbers), and extract the key information strictly matching the requested JSON structure.

Extract the following fields based on this structure: %s
If a field is not found in the text, return null. Do not invent or hallucinate data.

You must respond ONLY with a JSON object matching this structure.
Do not include any conversational preamble, postscript, or explanation. Output ONLY the raw JSON object itself.`, string(compactBytes))
}
