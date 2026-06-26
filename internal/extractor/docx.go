package extractor

import (
	"archive/zip"
	"encoding/xml"
	"fmt"
	"io"
	"strings"
)

// DocxExtractor extracts text from Microsoft Word (.docx) documents.
type DocxExtractor struct{}

// ExtractText decompresses the DOCX zip package, extracts word/document.xml, and parses w:t text nodes.
func (e *DocxExtractor) ExtractText(filePath string) (string, error) {
	reader, err := zip.OpenReader(filePath)
	if err != nil {
		return "", fmt.Errorf("failed to open docx file %q: %w", filePath, err)
	}
	defer reader.Close()

	var documentXML io.ReadCloser
	for _, file := range reader.File {
		if file.Name == "word/document.xml" {
			documentXML, err = file.Open()
			if err != nil {
				return "", fmt.Errorf("failed to open word/document.xml in docx: %w", err)
			}
			break
		}
	}

	if documentXML == nil {
		return "", fmt.Errorf("word/document.xml not found in docx archive: %s", filePath)
	}
	defer documentXML.Close()

	return parseDocXML(documentXML)
}

// parseDocXML parses the Word Document XML and outputs a reconstructed plain text string.
func parseDocXML(r io.Reader) (string, error) {
	var result strings.Builder
	decoder := xml.NewDecoder(r)

	for {
		token, err := decoder.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", fmt.Errorf("failed parsing XML token: %w", err)
		}

		switch se := token.(type) {
		case xml.StartElement:
			if se.Name.Local == "t" {
				var textVal string
				if err := decoder.DecodeElement(&textVal, &se); err != nil {
					return "", fmt.Errorf("failed parsing XML text element: %w", err)
				}
				result.WriteString(textVal)
			} else if se.Name.Local == "p" {
				// Translate Word paragraph boundaries into double newlines for semantic chunking
				if result.Len() > 0 {
					current := result.String()
					if !strings.HasSuffix(current, "\n\n") {
						if strings.HasSuffix(current, "\n") {
							result.WriteString("\n")
						} else {
							result.WriteString("\n\n")
						}
					}
				}
			} else if se.Name.Local == "br" || se.Name.Local == "cr" {
				result.WriteString("\n")
			}
		}
	}

	return result.String(), nil
}
