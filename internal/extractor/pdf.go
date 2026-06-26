package extractor

import (
	"bytes"
	"fmt"

	"github.com/ledongthuc/pdf"
)

// PdfExtractor extracts text from PDF documents.
type PdfExtractor struct{}

// ExtractText opens a PDF file and pulls out its full raw text representation.
func (e *PdfExtractor) ExtractText(filePath string) (string, error) {
	f, r, err := pdf.Open(filePath)
	if err != nil {
		return "", fmt.Errorf("failed to open PDF file %q: %w", filePath, err)
	}
	defer f.Close()

	textReader, err := r.GetPlainText()
	if err != nil {
		return "", fmt.Errorf("failed to extract plain text from PDF: %w", err)
	}

	var buf bytes.Buffer
	if _, err := buf.ReadFrom(textReader); err != nil {
		return "", fmt.Errorf("failed to read text from PDF reader: %w", err)
	}

	return buf.String(), nil
}
