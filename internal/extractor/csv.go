package extractor

import (
	"encoding/csv"
	"fmt"
	"os"
	"strings"
)

// CsvExtractor parses Comma Separated Values (.csv) and converts each row into a descriptive sentence.
type CsvExtractor struct{}

// ExtractRows reads a CSV file, identifies headers, and extracts the rows.
func (e *CsvExtractor) ExtractRows(filePath string) ([]Row, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open CSV file %q: %w", filePath, err)
	}
	defer f.Close()

	reader := csv.NewReader(f)
	reader.FieldsPerRecord = -1
	records, err := reader.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("failed to parse CSV records from %q: %w", filePath, err)
	}

	if len(records) == 0 {
		return nil, nil
	}

	var headers []string
	startRow := 0

	// If there is only one row, treat it as data without headers.
	// Otherwise, assume the first row represents the column headers.
	if len(records) > 1 {
		headers = records[0]
		startRow = 1
	}

	var rows []Row
	for i := startRow; i < len(records); i++ {
		row := records[i]
		var columns []Column
		for j, val := range row {
			headerName := fmt.Sprintf("Col%d", j+1)
			if j < len(headers) && strings.TrimSpace(headers[j]) != "" {
				headerName = strings.TrimSpace(headers[j])
			}
			columns = append(columns, Column{
				Name:  headerName,
				Value: val,
			})
		}
		rows = append(rows, Row{
			Index:   i,
			Columns: columns,
		})
	}

	return rows, nil
}

// ExtractText reads a CSV file, identifies headers, and translates rows into sentences.
func (e *CsvExtractor) ExtractText(filePath string) (string, error) {
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
