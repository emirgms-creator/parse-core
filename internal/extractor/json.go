package extractor

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
)

// JsonExtractor parses JSON files (.json, .jsonl) and extracts structured rows.
type JsonExtractor struct{}

// ExtractRows parses a JSON file (either array of objects or NDJSON) and extracts rows.
func (e *JsonExtractor) ExtractRows(filePath string) ([]Row, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read JSON file %q: %w", filePath, err)
	}

	trimmed := strings.TrimSpace(string(data))
	var rawObjects []map[string]interface{}

	// Scenario 1: JSON Array
	if strings.HasPrefix(trimmed, "[") && strings.HasSuffix(trimmed, "]") {
		var list []map[string]interface{}
		if err := json.Unmarshal(data, &list); err == nil {
			rawObjects = list
		}
	}

	// Scenario 2: JSON Lines (NDJSON/JSONL)
	if len(rawObjects) == 0 {
		scanner := bufio.NewScanner(strings.NewReader(trimmed))
		lineNum := 0
		for scanner.Scan() {
			lineNum++
			line := strings.TrimSpace(scanner.Text())
			if line == "" {
				continue
			}
			var obj map[string]interface{}
			if err := json.Unmarshal([]byte(line), &obj); err != nil {
				// If a line is malformed, we keep searching or return error
				return nil, fmt.Errorf("malformed JSON object at line %d: %w", lineNum, err)
			}
			rawObjects = append(rawObjects, obj)
		}
		if err := scanner.Err(); err != nil {
			return nil, fmt.Errorf("failed scanning JSON lines: %w", err)
		}
	}

	var rows []Row
	for i, obj := range rawObjects {
		// Sort keys to maintain deterministic column order
		var keys []string
		for k := range obj {
			keys = append(keys, k)
		}
		sort.Strings(keys)

		var columns []Column
		for _, key := range keys {
			val := obj[key]
			valStr := ""
			if val != nil {
				switch v := val.(type) {
				case string:
					valStr = v
				case []interface{}, map[string]interface{}:
					bytes, _ := json.Marshal(v)
					valStr = string(bytes)
				default:
					valStr = fmt.Sprintf("%v", v)
				}
			}
			columns = append(columns, Column{
				Name:  key,
				Value: valStr,
			})
		}
		rows = append(rows, Row{
			Index:   i + 1, // 1-based index matching standard row indices
			Columns: columns,
		})
	}

	return rows, nil
}

// ExtractText translates rows into formatted sentence blocks.
func (e *JsonExtractor) ExtractText(filePath string) (string, error) {
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
