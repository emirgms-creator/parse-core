package extractor

import (
	"fmt"
	"strings"

	"github.com/xuri/excelize/v2"
)

// ExcelExtractor parses Excel spreadsheets (.xlsx) and converts each row into key-value structure or sentences.
type ExcelExtractor struct{}

// ExtractRows parses an Excel spreadsheet and extracts rows.
func (e *ExcelExtractor) ExtractRows(filePath string) ([]Row, error) {
	f, err := excelize.OpenFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open Excel file %q: %w", filePath, err)
	}
	defer f.Close()

	// Get first sheet name
	sheets := f.GetSheetList()
	if len(sheets) == 0 {
		return nil, fmt.Errorf("no sheets found in Excel file %q", filePath)
	}
	firstSheet := sheets[0]

	rowsList, err := f.GetRows(firstSheet)
	if err != nil {
		return nil, fmt.Errorf("failed to get rows from sheet %q in %q: %w", firstSheet, filePath, err)
	}

	if len(rowsList) == 0 {
		return nil, nil
	}

	var headers []string
	startRow := 0

	// If there is only one row, treat it as data without headers.
	// Otherwise, assume the first row represents the column headers.
	if len(rowsList) > 1 {
		headers = rowsList[0]
		startRow = 1
	}

	var rows []Row
	for i := startRow; i < len(rowsList); i++ {
		rowVals := rowsList[i]
		var columns []Column
		maxCols := len(rowVals)
		if len(headers) > maxCols {
			maxCols = len(headers)
		}
		for j := 0; j < maxCols; j++ {
			headerName := fmt.Sprintf("Col%d", j+1)
			if j < len(headers) && strings.TrimSpace(headers[j]) != "" {
				headerName = strings.TrimSpace(headers[j])
			}
			val := ""
			if j < len(rowVals) {
				val = rowVals[j]
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

// ExtractText reads an Excel file and translates rows into sentences.
func (e *ExcelExtractor) ExtractText(filePath string) (string, error) {
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
