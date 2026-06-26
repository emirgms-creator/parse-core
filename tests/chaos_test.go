package tests

import (
	"archive/zip"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

var binaryPath string

func TestMain(m *testing.M) {
	// Compile the parsecore binary to a temporary folder
	tmpDir, err := os.MkdirTemp("", "parsecore_test_bin_*")
	if err != nil {
		fmt.Printf("failed to create temp dir for binary: %v\n", err)
		os.Exit(1)
	}
	defer os.RemoveAll(tmpDir)

	binaryPath = filepath.Join(tmpDir, "parsecore.exe")

	cmd := exec.Command("go", "build", "-o", binaryPath, "../cmd/parsecore")
	cmd.Dir = "."
	if output, err := cmd.CombinedOutput(); err != nil {
		fmt.Printf("failed to compile binary: %v\nOutput:\n%s\n", err, string(output))
		os.Exit(1)
	}

	os.Exit(m.Run())
}

// 1. Giant TXT continuous string test (checks if semantic chunker terminates safely)
func TestChaosGiantTxt(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "parsecore_chaos_*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create a continuous string of 500,000 characters without spaces or periods
	giantWord := strings.Repeat("a", 500000)
	txtPath := filepath.Join(tmpDir, "giant.txt")
	if err := os.WriteFile(txtPath, []byte(giantWord), 0644); err != nil {
		t.Fatalf("failed to write giant txt file: %v", err)
	}

	outPath := filepath.Join(tmpDir, "out.json")

	// We only run extraction and check that the chunker completes
	// To avoid timeout or LLM failure on this huge chunk, we just test if the chunker resolves.
	// Since ExtractSemanticChunks is tested directly, we can execute the binary but we will target it.
	// Actually, running parsecore on a 500,000 word might take too long in Ollama, so we can check that
	// parsecore handles it or that the chunker segments it safely.
	// Let's run the binary but restrict the model or simulate to fail fast, OR just run the command.
	// Wait, let's execute the binary and check that it doesn't hang!
	// We can set a command timeout of 10 seconds.
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, binaryPath, "extract", "-i", txtPath, "-o", outPath, "-m", "phi4-mini:latest", "--url", "http://localhost:9999")
	output, err := cmd.CombinedOutput()

	// If it timed out, ctx.Err() will be context.DeadlineExceeded.
	// We assert that it did NOT timeout (meaning it did not hang or infinite loop).
	if ctx.Err() == context.DeadlineExceeded {
		t.Fatal("Chaos giant txt extraction timed out (infinite loop or hang suspected)")
	}

	// We expect the extraction to finish. It might fail at Ollama payload transfer (since 500,000 chars is too big for local context),
	// but the binary should exit cleanly with a code 0/1 without panics.
	outStr := string(output)
	if strings.Contains(outStr, "panic") {
		t.Errorf("binary panicked during giant txt parsing: %s", outStr)
	}
}

// 2. Corrupted DOCX zip package test (checks if parser handles malformed zip/xml gracefully)
func TestChaosCorruptedDocx(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "parsecore_chaos_*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	docxPath := filepath.Join(tmpDir, "corrupted.docx")
	
	// Write a corrupted docx zip
	f, err := os.Create(docxPath)
	if err != nil {
		t.Fatalf("failed to create corrupted docx: %v", err)
	}
	
	zw := zip.NewWriter(f)
	// Create word/document.xml with corrupted xml structure (unbalanced tag)
	w, err := zw.Create("word/document.xml")
	if err != nil {
		f.Close()
		t.Fatalf("failed to create docx xml writer: %v", err)
	}
	_, _ = w.Write([]byte("<w:document><w:body><w:p><w:t>Corrupted Docx File</w:p></w:document>")) // missing close tags for w:t and w:r
	
	zw.Close()
	f.Close()

	outPath := filepath.Join(tmpDir, "out.json")
	cmd := exec.Command(binaryPath, "extract", "-i", docxPath, "-o", outPath, "-m", "phi4-mini:latest")
	output, err := cmd.CombinedOutput()

	// It must exit with error 1 because the XML is malformed, but it must not panic.
	if err == nil {
		t.Error("expected error return from corrupted docx file, but got exit code 0")
	}

	outStr := string(output)
	if strings.Contains(outStr, "panic") {
		t.Errorf("binary panicked on corrupted docx: %s", outStr)
	}
}

// 3. Variable column CSV test (checks variable fields parsing tolerance)
func TestChaosInconsistentCsv(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "parsecore_chaos_*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	csvPath := filepath.Join(tmpDir, "inconsistent.csv")
	csvContent := "ColA,ColB,ColC\nValue1,Value2\nValue3,Value4,Value5,Value6\n"
	if err := os.WriteFile(csvPath, []byte(csvContent), 0644); err != nil {
		t.Fatalf("failed to write csv: %v", err)
	}

	outPath := filepath.Join(tmpDir, "out.json")
	cmd := exec.Command(binaryPath, "extract", "-i", csvPath, "-o", outPath, "-m", "phi4-mini:latest")
	output, err := cmd.CombinedOutput()

	outStr := string(output)
	if strings.Contains(outStr, "panic") {
		t.Fatalf("binary panicked on variable column CSV: %s", outStr)
	}

	if err != nil {
		t.Fatalf("failed to parse variable CSV cleanly: %v\nOutput: %s", err, outStr)
	}
}

// 4. Adversarial Context Hallucination Check (generic content under invoice schema)
func TestChaosAdversarialContext(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "parsecore_chaos_*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Ingest a recipe instead of an invoice
	recipeContent := "Chocolate Chip Cookies Recipe.\nIngredients: 2 cups flour, 1 cup chocolate chips, 1 cup sugar, 2 eggs. Mix ingredients and bake at 350 degrees Fahrenheit for 10 minutes."
	txtPath := filepath.Join(tmpDir, "recipe.txt")
	if err := os.WriteFile(txtPath, []byte(recipeContent), 0644); err != nil {
		t.Fatalf("failed to write recipe text: %v", err)
	}

	outPath := filepath.Join(tmpDir, "out.json")

	// Trigger extraction using --schema invoice
	cmd := exec.Command(binaryPath, "extract", "-i", txtPath, "-o", outPath, "-m", "phi4-mini:latest", "--schema", "invoice")
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("execution failed: %v\nOutput: %s", err, string(output))
	}

	// Read and parse the output JSON
	data, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("failed to read output file: %v", err)
	}

	var results []map[string]interface{}
	if err := json.Unmarshal(data, &results); err != nil {
		t.Fatalf("failed to parse output JSON: %v", err)
	}

	if len(results) == 0 {
		t.Fatal("expected at least 1 parsed result block")
	}

	invoiceResult := results[0]
	
	// Assert that invoice fields are null or empty because the text contains a recipe,
	// verifying that temperature=0 prevents hallucinations.
	for _, key := range []string{"invoice_number", "total_amount", "vendor_name"} {
		val, ok := invoiceResult[key]
		if ok && val != nil && val != "" {
			t.Errorf("Hallucination detected: key %q should be null or empty, got: %q", key, val)
		}
	}
}

// 5. Concurrency API stress overload test
func TestChaosStressWorkers(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "parsecore_chaos_*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create a document with 100 small paragraphs to split into 100 chunks
	var builder strings.Builder
	for i := 1; i <= 100; i++ {
		builder.WriteString(fmt.Sprintf("This is paragraph number %d describing project updates. Ollama resolves this safely.\n\n", i))
	}

	txtPath := filepath.Join(tmpDir, "stress.txt")
	if err := os.WriteFile(txtPath, []byte(builder.String()), 0644); err != nil {
		t.Fatalf("failed to write stress txt file: %v", err)
	}

	outPath := filepath.Join(tmpDir, "out.json")

	// Run extraction with 50 workers to stress-test local Ollama
	cmd := exec.Command(binaryPath, "extract", "-i", txtPath, "-o", outPath, "-m", "phi4-mini:latest", "-w", "50", "--chunk-size", "100", "--overlap", "10")
	output, err := cmd.CombinedOutput()

	outStr := string(output)
	if strings.Contains(outStr, "panic") {
		t.Errorf("binary panicked during concurrency stress test: %s", outStr)
	}

	if err != nil {
		t.Logf("concurrency stress run returned some errors (Ollama limits): %v", err)
	} else {
		t.Log("concurrency stress run completed successfully with exit code 0")
	}
}
