package extractor

import (
	"fmt"
	"os"
)

// TextExtractor extracts text from plain text (.txt) and Markdown (.md) documents.
type TextExtractor struct{}

// ExtractText reads the text file content directly from disk.
func (e *TextExtractor) ExtractText(filePath string) (string, error) {
	content, err := os.ReadFile(filePath)
	if err != nil {
		return "", fmt.Errorf("failed to read file %q: %w", filePath, err)
	}
	return string(content), nil
}
