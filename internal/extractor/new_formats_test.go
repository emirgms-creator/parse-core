package extractor

import (
	"os"
	"testing"

	"github.com/xuri/excelize/v2"
)

func TestExcelExtractor(t *testing.T) {
	// Create temporary Excel file
	f := excelize.NewFile()
	defer f.Close()

	sheet := "Sheet1"
	headers := []string{"Transaction ID", "Item", "Quantity"}
	row1 := []string{"TXN_001", "Coffee", "2"}
	row2 := []string{"TXN_002", "Tea", "1"}

	for colIdx, val := range headers {
		cell, _ := excelize.CoordinatesToCellName(colIdx+1, 1)
		_ = f.SetCellValue(sheet, cell, val)
	}
	for colIdx, val := range row1 {
		cell, _ := excelize.CoordinatesToCellName(colIdx+1, 2)
		_ = f.SetCellValue(sheet, cell, val)
	}
	for colIdx, val := range row2 {
		cell, _ := excelize.CoordinatesToCellName(colIdx+1, 3)
		_ = f.SetCellValue(sheet, cell, val)
	}

	tmpFile, err := os.CreateTemp("", "test_excel_*.xlsx")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	tmpPath := tmpFile.Name()
	tmpFile.Close()
	defer os.Remove(tmpPath)

	if err := f.SaveAs(tmpPath); err != nil {
		t.Fatalf("failed to save Excel: %v", err)
	}

	// Test extraction
	ext := &ExcelExtractor{}
	rows, err := ext.ExtractRows(tmpPath)
	if err != nil {
		t.Fatalf("ExtractRows failed: %v", err)
	}

	if len(rows) != 2 {
		t.Fatalf("expected 2 rows, got %d", len(rows))
	}

	// Verify row 1
	r1 := rows[0]
	if r1.Index != 1 {
		t.Errorf("expected index 1, got %d", r1.Index)
	}
	val, _ := r1.Get("Item")
	if val != "Coffee" {
		t.Errorf("expected Coffee, got %s", val)
	}

	// Verify row 2
	r2 := rows[1]
	val, _ = r2.Get("Quantity")
	if val != "1" {
		t.Errorf("expected 1, got %s", val)
	}
}

func TestJsonExtractor(t *testing.T) {
	t.Run("JSON Array", func(t *testing.T) {
		content := `[
			{"Transaction ID": "TXN_101", "Item": "Cake", "Quantity": 3},
			{"Transaction ID": "TXN_102", "Item": "Cookie", "Quantity": 5}
		]`
		tmpFile, err := os.CreateTemp("", "test_json_*.json")
		if err != nil {
			t.Fatalf("failed to create temp file: %v", err)
		}
		tmpPath := tmpFile.Name()
		defer os.Remove(tmpPath)
		_ = os.WriteFile(tmpPath, []byte(content), 0644)
		tmpFile.Close()

		ext := &JsonExtractor{}
		rows, err := ext.ExtractRows(tmpPath)
		if err != nil {
			t.Fatalf("ExtractRows failed: %v", err)
		}

		if len(rows) != 2 {
			t.Fatalf("expected 2 rows, got %d", len(rows))
		}

		if rows[0].Index != 1 || rows[1].Index != 2 {
			t.Errorf("unexpected indices: %d, %d", rows[0].Index, rows[1].Index)
		}

		val, _ := rows[0].Get("Item")
		if val != "Cake" {
			t.Errorf("expected Cake, got %s", val)
		}

		qty, _ := rows[1].Get("Quantity")
		if qty != "5" {
			t.Errorf("expected 5, got %s", qty)
		}
	})

	t.Run("JSON Lines (JSONL)", func(t *testing.T) {
		content := `{"Transaction ID": "TXN_201", "Item": "Latte", "Quantity": 1}
{"Transaction ID": "TXN_202", "Item": "Mocha", "Quantity": 2}`
		tmpFile, err := os.CreateTemp("", "test_jsonl_*.jsonl")
		if err != nil {
			t.Fatalf("failed to create temp file: %v", err)
		}
		tmpPath := tmpFile.Name()
		defer os.Remove(tmpPath)
		_ = os.WriteFile(tmpPath, []byte(content), 0644)
		tmpFile.Close()

		ext := &JsonExtractor{}
		rows, err := ext.ExtractRows(tmpPath)
		if err != nil {
			t.Fatalf("ExtractRows failed: %v", err)
		}

		if len(rows) != 2 {
			t.Fatalf("expected 2 rows, got %d", len(rows))
		}

		val, _ := rows[0].Get("Item")
		if val != "Latte" {
			t.Errorf("expected Latte, got %s", val)
		}

		val2, _ := rows[1].Get("Item")
		if val2 != "Mocha" {
			t.Errorf("expected Mocha, got %s", val2)
		}
	})
}

func TestXmlExtractor(t *testing.T) {
	content := `<records>
	<row>
		<TransactionID>TXN_301</TransactionID>
		<Item>Sandwich</Item>
		<Quantity>2</Quantity>
	</row>
	<row>
		<TransactionID>TXN_302</TransactionID>
		<Item>Soup</Item>
		<Quantity>1</Quantity>
	</row>
</records>`

	tmpFile, err := os.CreateTemp("", "test_xml_*.xml")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	tmpPath := tmpFile.Name()
	defer os.Remove(tmpPath)
	_ = os.WriteFile(tmpPath, []byte(content), 0644)
	tmpFile.Close()

	ext := &XmlExtractor{}
	rows, err := ext.ExtractRows(tmpPath)
	if err != nil {
		t.Fatalf("ExtractRows failed: %v", err)
	}

	if len(rows) != 2 {
		t.Fatalf("expected 2 rows, got %d", len(rows))
	}

	// Verify Row 1
	val, _ := rows[0].Get("Item")
	if val != "Sandwich" {
		t.Errorf("expected Sandwich, got %s", val)
	}
	id, _ := rows[0].Get("TransactionID")
	if id != "TXN_301" {
		t.Errorf("expected TXN_301, got %s", id)
	}

	// Verify Row 2
	val2, _ := rows[1].Get("Item")
	if val2 != "Soup" {
		t.Errorf("expected Soup, got %s", val2)
	}
}

func TestRegisterNewFormats(t *testing.T) {
	exts := []string{".xlsx", ".json", ".jsonl", ".xml"}
	for _, ext := range exts {
		docExt, err := NewExtractor(ext)
		if err != nil {
			t.Errorf("NewExtractor(%q) returned error: %v", ext, err)
		}
		if _, ok := docExt.(RowExtractor); !ok {
			t.Errorf("NewExtractor(%q) does not implement RowExtractor", ext)
		}
	}
}
