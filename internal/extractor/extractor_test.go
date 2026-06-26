package extractor

import (
	"archive/zip"
	"encoding/csv"
	"fmt"
	"os"
	"strings"
	"testing"
)

func TestExtractText(t *testing.T) {
	// The test assumes sample.pdf was downloaded to the root of the project.
	pdfPath := "../../sample.pdf"

	text, err := ExtractText(pdfPath)
	if err != nil {
		t.Fatalf("ExtractText failed: %v", err)
	}

	trimmed := strings.TrimSpace(text)
	expected := "Dummy PDF file"

	if !strings.Contains(trimmed, expected) {
		t.Errorf("expected text to contain %q, got %q", expected, trimmed)
	}
}

func TestSemanticChunking(t *testing.T) {
	t.Run("Fit Entirely", func(t *testing.T) {
		text := "Hello world. This is a small text."
		chunks := SemanticChunking(text, 100, 10)
		if len(chunks) != 1 {
			t.Fatalf("expected 1 chunk, got %d", len(chunks))
		}
		if chunks[0] != text {
			t.Errorf("expected %q, got %q", text, chunks[0])
		}
	})

	t.Run("Split By Paragraphs", func(t *testing.T) {
		text := "Paragraph one.\n\nParagraph two.\n\nParagraph three."
		chunks := SemanticChunking(text, 25, 5)
		
		if len(chunks) != 3 {
			t.Fatalf("expected 3 chunks, got %d: %v", len(chunks), chunks)
		}
		if chunks[0] != "Paragraph one." || chunks[1] != "Paragraph two." || chunks[2] != "Paragraph three." {
			t.Errorf("unexpected paragraph chunks: %v", chunks)
		}
	})

	t.Run("Split By Sentences with Overlap", func(t *testing.T) {
		text := "Sentence one. Sentence two. Sentence three. Sentence four."
		chunks := SemanticChunking(text, 35, 15)

		if len(chunks) != 3 {
			t.Fatalf("expected 3 chunks, got %d: %v", len(chunks), chunks)
		}
		if !strings.Contains(chunks[0], "Sentence one.") || !strings.Contains(chunks[0], "Sentence two.") {
			t.Errorf("chunk 0 incorrect: %q", chunks[0])
		}
		if !strings.Contains(chunks[1], "Sentence two.") || !strings.Contains(chunks[1], "Sentence three.") {
			t.Errorf("chunk 1 incorrect: %q", chunks[1])
		}
		if !strings.Contains(chunks[2], "Sentence three.") || !strings.Contains(chunks[2], "Sentence four.") {
			t.Errorf("chunk 2 incorrect: %q", chunks[2])
		}
	})

	t.Run("Word Splitting Fallback", func(t *testing.T) {
		textWithSpaces := "Supercalifragilisticexpialidocious word split fallback test"
		chunks := SemanticChunking(textWithSpaces, 20, 5)
		
		if len(chunks) == 0 {
			t.Fatal("expected at least 1 chunk")
		}
		if !strings.Contains(chunks[0], "Supercalifragilisticexpialidocious") {
			t.Errorf("expected chunk 0 to contain the long word, got: %q", chunks[0])
		}
	})
}

func TestMultiFormatExtractors(t *testing.T) {
	t.Run("Text Extractor", func(t *testing.T) {
		content := "This is a test plain text file."
		filePath := createTempText(t, content)
		defer os.Remove(filePath)

		ext, err := NewExtractor("txt")
		if err != nil {
			t.Fatalf("NewExtractor failed: %v", err)
		}

		extracted, err := ext.ExtractText(filePath)
		if err != nil {
			t.Fatalf("ExtractText failed: %v", err)
		}

		if extracted != content {
			t.Errorf("expected %q, got %q", content, extracted)
		}
	})

	t.Run("CSV Extractor", func(t *testing.T) {
		rows := [][]string{
			{"Name", "Role", "Team"},
			{"Bekir", "Lead", "Core"},
			{"Alex", "Dev", "UI"},
		}
		filePath := createTempCsv(t, rows)
		defer os.Remove(filePath)

		ext, err := NewExtractor(".csv")
		if err != nil {
			t.Fatalf("NewExtractor failed: %v", err)
		}

		extracted, err := ext.ExtractText(filePath)
		if err != nil {
			t.Fatalf("ExtractText failed: %v", err)
		}

		expected1 := "Row 1: Name=Bekir, Role=Lead, Team=Core."
		expected2 := "Row 2: Name=Alex, Role=Dev, Team=UI."

		if !strings.Contains(extracted, expected1) || !strings.Contains(extracted, expected2) {
			t.Errorf("extracted text format incorrect: %q", extracted)
		}
	})

	t.Run("DOCX Extractor", func(t *testing.T) {
		textContent := "Hello from Word document!"
		filePath := createTempDocx(t, textContent)
		defer os.Remove(filePath)

		ext, err := NewExtractor("docx")
		if err != nil {
			t.Fatalf("NewExtractor failed: %v", err)
		}

		extracted, err := ext.ExtractText(filePath)
		if err != nil {
			t.Fatalf("ExtractText failed: %v", err)
		}

		if !strings.Contains(extracted, textContent) {
			t.Errorf("expected to contain %q, got %q", textContent, extracted)
		}
	})
}

// Helper functions for TestMultiFormatExtractors

func createTempText(t *testing.T, content string) string {
	tmpFile, err := os.CreateTemp("", "test_text_*.txt")
	if err != nil {
		t.Fatalf("failed to create temp text: %v", err)
	}
	defer tmpFile.Close()

	if _, err := tmpFile.WriteString(content); err != nil {
		t.Fatalf("failed writing text: %v", err)
	}

	return tmpFile.Name()
}

func createTempCsv(t *testing.T, rows [][]string) string {
	tmpFile, err := os.CreateTemp("", "test_csv_*.csv")
	if err != nil {
		t.Fatalf("failed to create temp csv: %v", err)
	}
	defer tmpFile.Close()

	writer := csv.NewWriter(tmpFile)
	if err := writer.WriteAll(rows); err != nil {
		t.Fatalf("failed writing csv rows: %v", err)
	}

	return tmpFile.Name()
}

func createTempDocx(t *testing.T, textContent string) string {
	tmpFile, err := os.CreateTemp("", "test_doc_*.docx")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	defer tmpFile.Close()

	zipWriter := zip.NewWriter(tmpFile)
	defer zipWriter.Close()

	docXMLWriter, err := zipWriter.Create("word/document.xml")
	if err != nil {
		t.Fatalf("failed to create xml entry in zip: %v", err)
	}

	xmlContent := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">
	<w:body>
		<w:p>
			<w:r>
				<w:t>%s</w:t>
			</w:r>
		</w:p>
	</w:body>
</w:document>`, textContent)

	_, err = docXMLWriter.Write([]byte(xmlContent))
	if err != nil {
		t.Fatalf("failed writing xml contents: %v", err)
	}

	return tmpFile.Name()
}
