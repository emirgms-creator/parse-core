package extractor

import (
	"fmt"
	"path/filepath"
	"strings"
)

// DocumentExtractor defines the interface for pulling text from various file formats.
type DocumentExtractor interface {
	ExtractText(filePath string) (string, error)
}

// Column represents a single key-value field in a structured row.
type Column struct {
	Name  string
	Value string
}

// Row represents a parsed structured row from a document (like CSV).
type Row struct {
	Index   int
	Columns []Column
}

// Get finds a column value by name (case-insensitive, trimmed).
func (r Row) Get(name string) (string, bool) {
	nameLower := strings.ToLower(strings.TrimSpace(name))
	for _, col := range r.Columns {
		if strings.ToLower(strings.TrimSpace(col.Name)) == nameLower {
			return col.Value, true
		}
	}
	return "", false
}

// GetWithAliases finds a column value by trying multiple aliases.
func (r Row) GetWithAliases(aliases ...string) (string, bool) {
	for _, alias := range aliases {
		if val, ok := r.Get(alias); ok {
			return val, true
		}
	}
	return "", false
}

// RowExtractor is implemented by extractors that can yield raw structured rows,
// allowing the fast-pass router to bypass LLM processing.
type RowExtractor interface {
	DocumentExtractor
	ExtractRows(filePath string) ([]Row, error)
}

// NewExtractor returns the appropriate DocumentExtractor implementation based on the file extension.
func NewExtractor(ext string) (DocumentExtractor, error) {
	cleanExt := strings.ToLower(strings.TrimPrefix(ext, "."))
	switch cleanExt {
	case "pdf":
		return &PdfExtractor{}, nil
	case "txt", "md":
		return &TextExtractor{}, nil
	case "csv":
		return &CsvExtractor{}, nil
	case "xlsx":
		return &ExcelExtractor{}, nil
	case "json", "jsonl":
		return &JsonExtractor{}, nil
	case "xml":
		return &XmlExtractor{}, nil
	case "docx":
		return &DocxExtractor{}, nil
	default:
		return nil, fmt.Errorf("unsupported file format: %s. Supported formats are: .pdf, .docx, .txt, .md, .csv, .xlsx, .json, .jsonl, .xml", ext)
	}
}

// ExtractText is a compatibility helper that extracts text from a PDF file.
// It keeps existing unit tests functional without changes.
func ExtractText(pdfPath string) (string, error) {
	ext := &PdfExtractor{}
	return ext.ExtractText(pdfPath)
}

// ExtractSemanticChunks reads a file at the given path, dynamically determines its format,
// extracts the text, and segments it into semantic chunks of at most maxChunkSize.
func ExtractSemanticChunks(filePath string, maxChunkSize, overlapSize int) ([]string, error) {
	ext := filepath.Ext(filePath)
	docExt, err := NewExtractor(ext)
	if err != nil {
		return nil, fmt.Errorf("failed to create extractor for %q: %w", filePath, err)
	}

	rawText, err := docExt.ExtractText(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to extract text from %q: %w", filePath, err)
	}

	return SemanticChunking(rawText, maxChunkSize, overlapSize), nil
}
