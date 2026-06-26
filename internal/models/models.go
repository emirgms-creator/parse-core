package models

// ParsedDocument represents the structured, semantic output extracted and parsed from unstructured document text.
type ParsedDocument struct {
	Title     string   `json:"title"`
	Summary   string   `json:"summary"`
	KeyPoints []string `json:"key_points"`
}
