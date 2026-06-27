package extractor

import (
	"encoding/xml"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
)

// XmlExtractor parses XML documents (.xml) and extracts structured rows.
type XmlExtractor struct{}

// ExtractRows parses an XML file and extracts child elements under the root node as rows.
func (e *XmlExtractor) ExtractRows(filePath string) ([]Row, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open XML file %q: %w", filePath, err)
	}
	defer file.Close()

	decoder := xml.NewDecoder(file)
	var rows []Row
	var currentFields map[string]string
	var currentFieldName string
	var inRow bool
	var depth int

	for {
		token, err := decoder.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("XML parsing error: %w", err)
		}

		switch t := token.(type) {
		case xml.StartElement:
			depth++
			if depth == 2 {
				// We entered a child element under the root, representing a row record
				inRow = true
				currentFields = make(map[string]string)
			} else if depth == 3 && inRow {
				// We entered a field element inside the row
				currentFieldName = t.Name.Local
				if _, exists := currentFields[currentFieldName]; !exists {
					currentFields[currentFieldName] = ""
				}
			}

		case xml.CharData:
			if inRow && depth == 3 && currentFieldName != "" {
				val := strings.TrimSpace(string(t))
				if val != "" {
					currentFields[currentFieldName] += val
				}
			}

		case xml.EndElement:
			if depth == 2 && inRow {
				// We reached the end of the row record
				// Convert to Row structure
				var keys []string
				for k := range currentFields {
					keys = append(keys, k)
				}
				sort.Strings(keys)

				var columns []Column
				for _, key := range keys {
					columns = append(columns, Column{
						Name:  key,
						Value: currentFields[key],
					})
				}

				rows = append(rows, Row{
					Index:   len(rows) + 1,
					Columns: columns,
				})
				inRow = false
				currentFieldName = ""
			} else if depth == 3 {
				currentFieldName = ""
			}
			depth--
		}
	}

	return rows, nil
}

// ExtractText translates XML rows into formatted sentence blocks.
func (e *XmlExtractor) ExtractText(filePath string) (string, error) {
	rows, err := e.ExtractRows(filePath)
	if err != nil {
		return "", err
	}

	var builder strings.Builder
	for _, row := range rows {
		var cols []string
		for _, col := range row.Columns {
			cols = append(cols, fmt.Sprintf("%s=%s", col.Name, col.Value))
		}
		builder.WriteString(fmt.Sprintf("Row %d: %s.\n", row.Index, strings.Join(cols, ", ")))
	}

	return builder.String(), nil
}
