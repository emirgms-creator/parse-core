package extractor

import (
	"encoding/json"
	"reflect"
	"testing"
)

func TestCleanValue(t *testing.T) {
	tests := []struct {
		input    string
		expected string
		isClean  bool
	}{
		{"", "", false},
		{"ERROR", "", false},
		{"Unknown", "", false},
		{"N/A", "", false},
		{"NA", "", false},
		{"NULL", "", false},
		{"NONE", "", false},
		{"-", "", false},
		{"  undefined  ", "", false},
		{"12345", "12345", true},
		{"Acme Corp", "Acme Corp", true},
	}

	for _, tt := range tests {
		got, clean := CleanValue(tt.input)
		if got != tt.expected || clean != tt.isClean {
			t.Errorf("CleanValue(%q) = (%q, %t); want (%q, %t)", tt.input, got, clean, tt.expected, tt.isClean)
		}
	}
}

func TestParseList(t *testing.T) {
	tests := []struct {
		input    string
		expected []string
	}{
		{"", nil},
		{"Item A", []string{"Item A"}},
		{"Item A, Item B", []string{"Item A", "Item B"}},
		{"Item A; Item B; Item C", []string{"Item A", "Item B", "Item C"}},
		{"Item A|Item B|Item C", []string{"Item A", "Item B", "Item C"}},
		{`["Item A", "Item B"]`, []string{"Item A", "Item B"}},
	}

	for _, tt := range tests {
		got := ParseList(tt.input)
		if !reflect.DeepEqual(got, tt.expected) {
			t.Errorf("ParseList(%q) = %v; want %v", tt.input, got, tt.expected)
		}
	}
}

func TestCleanRow(t *testing.T) {
	fields := []SchemaField{
		{Name: "invoice_number", IsArray: false},
		{Name: "total_amount", IsArray: false},
		{Name: "vendor_name", IsArray: false},
		{Name: "line_items", IsArray: true},
	}

	t.Run("Clean Row (Fast-Pass Success)", func(t *testing.T) {
		row := Row{
			Index: 1,
			Columns: []Column{
				{Name: "Invoice Number", Value: "INV-1001"},
				{Name: "Total Amount", Value: "$250.00"},
				{Name: "Vendor Name", Value: "Acme Supplies"},
				{Name: "Line Items", Value: "Widget A, Gadget B"},
			},
		}

		res, ok := CleanRow(row, fields)
		if !ok {
			t.Fatalf("expected CleanRow to succeed (Fast-Pass)")
		}

		var data map[string]interface{}
		if err := json.Unmarshal(res, &data); err != nil {
			t.Fatalf("failed to unmarshal result JSON: %v", err)
		}

		if data["invoice_number"] != "INV-1001" {
			t.Errorf("expected invoice_number to be INV-1001, got %v", data["invoice_number"])
		}
		if data["total_amount"] != "$250.00" {
			t.Errorf("expected total_amount to be $250.00, got %v", data["total_amount"])
		}
		if data["vendor_name"] != "Acme Supplies" {
			t.Errorf("expected vendor_name to be Acme Supplies, got %v", data["vendor_name"])
		}

		items, ok := data["line_items"].([]interface{})
		if !ok || len(items) != 2 || items[0] != "Widget A" || items[1] != "Gadget B" {
			t.Errorf("expected line_items to be [Widget A, Gadget B], got %v", data["line_items"])
		}
	})

	t.Run("Dirty Metadata (Fast-Pass Success with null)", func(t *testing.T) {
		row := Row{
			Index: 2,
			Columns: []Column{
				{Name: "Invoice Number", Value: "ERROR"},
				{Name: "Total Amount", Value: "$120.00"},
				{Name: "Vendor Name", Value: "Acme Corp"},
				{Name: "Line Items", Value: "UNKNOWN"},
			},
		}

		res, ok := CleanRow(row, fields)
		if !ok {
			t.Fatalf("expected CleanRow to succeed (Fast-Pass)")
		}

		var data map[string]interface{}
		if err := json.Unmarshal(res, &data); err != nil {
			t.Fatalf("failed to unmarshal result JSON: %v", err)
		}

		if data["invoice_number"] != nil {
			t.Errorf("expected invoice_number to be null (nil), got %v", data["invoice_number"])
		}
		if data["total_amount"] != "$120.00" {
			t.Errorf("expected total_amount to be $120.00, got %v", data["total_amount"])
		}
		if data["vendor_name"] != "Acme Corp" {
			t.Errorf("expected vendor_name to be Acme Corp, got %v", data["vendor_name"])
		}
		
		items, ok := data["line_items"].([]interface{})
		if !ok || len(items) != 0 {
			t.Errorf("expected line_items to be empty array, got %v", data["line_items"])
		}
	})

	t.Run("Complex Metadata Text (AI-Pass Fallback)", func(t *testing.T) {
		row := Row{
			Index: 3,
			Columns: []Column{
				{Name: "Invoice Number", Value: "This field represents a long natural language description that contains instructions to look up the invoice number later in the registry."},
				{Name: "Total Amount", Value: "$120.00"},
				{Name: "Vendor Name", Value: "Acme Corp"},
				{Name: "Line Items", Value: "Widget A"},
			},
		}

		_, ok := CleanRow(row, fields)
		if ok {
			t.Error("expected CleanRow to trigger AI-Pass fallback due to complex text in metadata field")
		}
	})

	t.Run("Missing Semantic Field (AI-Pass Fallback)", func(t *testing.T) {
		genericFields := []SchemaField{
			{Name: "title", IsArray: false},
			{Name: "summary", IsArray: false},
			{Name: "key_points", IsArray: true},
		}

		row := Row{
			Index: 4,
			Columns: []Column{
				{Name: "Title", Value: "Project Alpha Updates"},
				// Summary column is completely missing, which is a required semantic field
				{Name: "Key Points", Value: "Goal reached, On budget"},
			},
		}

		_, ok := CleanRow(row, genericFields)
		if ok {
			t.Error("expected CleanRow to trigger AI-Pass fallback due to missing semantic summary field")
		}
	})
}
