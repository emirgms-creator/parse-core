package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/spf13/cobra"

	"parse-core/internal/extractor"
	"parse-core/internal/llm"
)

var (
	inputPath   string
	outputPath  string
	modelName   string
	ollamaURL   string
	schemaName  string
	workerCount int
	chunkSize   int
	overlapSize int
)

var extractCmd = &cobra.Command{
	Use:   "extract",
	Short: "Extract and semantically structure data from a document",
	Long: `Extracts unstructured text from a local file (.pdf, .docx, .txt, .md, .csv), segments it into semantic chunks,
and structures the contents into specific JSON schemas concurrently using a local Ollama LLM.`,
	PreRunE: func(cmd *cobra.Command, args []string) error {
		// 1. Validate Schema (bypass for auto schema)
		if schemaName != "auto" {
			_, err := llm.ResolveSystemPrompt(schemaName)
			if err != nil {
				return fmt.Errorf("schema validation failed: %w", err)
			}
		}

		// 2. Validate File Format / Extension (bypass for URLs)
		if !strings.HasPrefix(inputPath, "http://") && !strings.HasPrefix(inputPath, "https://") {
			ext := filepath.Ext(inputPath)
			_, err := extractor.NewExtractor(ext)
			if err != nil {
				return fmt.Errorf("file validation failed: %w", err)
			}
		}
		return nil
	},
	RunE: runExtract,
}

func init() {
	extractCmd.Flags().StringVarP(&inputPath, "input", "i", "", "Path to the target document (Required)")
	extractCmd.Flags().StringVarP(&outputPath, "output", "o", "", "Path to save the resulting JSON array (Required)")
	extractCmd.Flags().StringVarP(&modelName, "model", "m", "phi4-mini", "The local Ollama model to use")
	extractCmd.Flags().StringVar(&ollamaURL, "url", "http://localhost:11434", "Ollama API server URL")
	extractCmd.Flags().StringVarP(&schemaName, "schema", "s", "auto", "The extraction schema to target (auto, generic, contract, invoice)")
	extractCmd.Flags().IntVarP(&workerCount, "workers", "w", 4, "Number of concurrent worker Go routines")
	extractCmd.Flags().IntVar(&chunkSize, "chunk-size", 2000, "Semantic chunk character limit")
	extractCmd.Flags().IntVar(&overlapSize, "overlap", 200, "Sliding window overlap size between chunks")

	// Mark required flags
	_ = extractCmd.MarkFlagRequired("input")
	_ = extractCmd.MarkFlagRequired("output")

	rootCmd.AddCommand(extractCmd)
}

func runExtract(cmd *cobra.Command, args []string) error {
	var systemPrompt string
	var err error
	if schemaName != "auto" {
		systemPrompt, err = llm.ResolveSystemPrompt(schemaName)
		if err != nil {
			return err // Should have been caught by PreRunE
		}
	}

	// 1. Clean terminal header
	fmt.Println("🚀 ParseCore Ingestion Pipeline Started")
	fmt.Printf("📂 Input:      %s\n", inputPath)
	fmt.Printf("💾 Output:     %s\n", outputPath)
	fmt.Printf("🤖 Model:      %s\n", modelName)
	fmt.Printf("📋 Schema:     %s\n", schemaName)
	fmt.Printf("⚙️  Workers:    %d\n", workerCount)
	fmt.Printf("📦 Chunking:   Max size %d, Overlap %d\n", chunkSize, overlapSize)
	fmt.Println("--------------------------------------------------")

	// 2. Extraction & Semantic Chunking/Row Parsing
	fmt.Print("⏳ Extracting text and segmenting chunks... ")
	startTime := time.Now()

	var docExtractor extractor.DocumentExtractor
	if strings.HasPrefix(inputPath, "http://") || strings.HasPrefix(inputPath, "https://") {
		docExtractor = &extractor.WebExtractor{}
	} else {
		ext := filepath.Ext(inputPath)
		docExtractor, err = extractor.NewExtractor(ext)
		if err != nil {
			fmt.Println("FAILED")
			return fmt.Errorf("failed to initialize extractor: %w", err)
		}
	}

	var chunks []string
	var rows []extractor.Row
	var isRowExtractor bool
	var rowExtractor extractor.RowExtractor

	if rowExt, ok := docExtractor.(extractor.RowExtractor); ok {
		isRowExtractor = true
		rowExtractor = rowExt
		rows, err = rowExtractor.ExtractRows(inputPath)
		if err != nil {
			fmt.Println("FAILED")
			return fmt.Errorf("extraction failed: %w", err)
		}
		fmt.Printf("Done! (Extracted %d structured rows)\n", len(rows))
	} else {
		if schemaName == "auto" {
			schemaName = "generic"
			systemPrompt, err = llm.ResolveSystemPrompt("generic")
			if err != nil {
				return fmt.Errorf("failed to resolve generic system prompt for auto schema: %w", err)
			}
		}
		rawText, err := docExtractor.ExtractText(inputPath)
		if err != nil {
			fmt.Println("FAILED")
			return fmt.Errorf("extraction failed: %w", err)
		}
		chunks = extractor.SemanticChunking(rawText, chunkSize, overlapSize)
		fmt.Printf("Done! (Segmented into %d chunks)\n", len(chunks))
		if len(chunks) == 0 {
			return fmt.Errorf("no text chunks found in document")
		}
	}

	// 3. Concurrency Worker Pool Setup
	client := llm.NewClient(ollamaURL, modelName)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
	defer cancel()

	var totalItems int
	if isRowExtractor {
		totalItems = len(rows)
	} else {
		totalItems = len(chunks)
	}

	// Resolve schema fields and system prompt for RowExtractor
	var schemaFields []extractor.SchemaField
	if isRowExtractor {
		if schemaName == "auto" {
			schemaFields = extractor.GetSchemaFieldsFromRows(rows)
			systemPrompt = extractor.GenerateSystemPromptForFields(schemaFields)
		} else {
			schemaFields, err = extractor.GetSchemaFields(schemaName)
			if err != nil {
				return fmt.Errorf("failed to resolve schema fields: %w", err)
			}
		}
	}

	jobs := make(chan struct {
		id   int
		text string
	}, totalItems)
	results := make(chan struct {
		chunkID    int
		doc        json.RawMessage
		isFastPass bool
		err        error
	}, totalItems)

	var wg sync.WaitGroup

	fmt.Printf("⏳ Initializing concurrency pool with %d workers... \n", workerCount)
	for w := 1; w <= workerCount; w++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			for job := range jobs {
				doc, err := client.ParseText(ctx, job.text, systemPrompt)
				results <- struct {
					chunkID    int
					doc        json.RawMessage
					isFastPass bool
					err        error
				}{
					chunkID:    job.id,
					doc:        doc,
					isFastPass: false,
					err:        err,
				}
			}
		}(w)
	}

	// 4. Ingest and route items (Hybrid Fast-Pass Router)
	if isRowExtractor {

		for idx, row := range rows {
			rowID := idx + 1

			// Try Fast-Pass Heuristic Cleaning
			if cleanedJSON, ok := extractor.CleanRow(row, schemaFields); ok {
				results <- struct {
					chunkID    int
					doc        json.RawMessage
					isFastPass bool
					err        error
				}{
					chunkID:    rowID,
					doc:        cleanedJSON,
					isFastPass: true,
					err:        nil,
				}
			} else {
				// Fallback to AI-Pass (LLM worker pool)
				// Reconstruct row text in original CSV extractor format
				var cols []string
				for _, col := range row.Columns {
					cols = append(cols, fmt.Sprintf("%s=%s", col.Name, col.Value))
				}
				rowText := fmt.Sprintf("Row %d: %s.\n", row.Index, strings.Join(cols, ", "))

				jobs <- struct {
					id   int
					text string
				}{
					id:   rowID,
					text: rowText,
				}
			}
		}
		close(jobs)
	} else {
		// Feed standard chunks into jobs channel
		for i, chunk := range chunks {
			jobs <- struct {
				id   int
				text string
			}{id: i + 1, text: chunk}
		}
		close(jobs)
	}

	// Monitor workers to close results channel
	go func() {
		wg.Wait()
		close(results)
	}()

	// Collect outputs concurrently
	parsedDocs := make([]json.RawMessage, totalItems)
	var errors []string

	for res := range results {
		if res.err != nil {
			errMsg := fmt.Sprintf("item #%d: %v", res.chunkID, res.err)
			errors = append(errors, errMsg)
			fmt.Printf("❌ Item #%d: Error - %v\n", res.chunkID, res.err)
		} else {
			parsedDocs[res.chunkID-1] = res.doc
			summary := getJSONSummarySnippet(res.doc, schemaName)
			if res.isFastPass {
				fmt.Printf("⚡ Item #%d (Fast-Pass): Success - %s\n", res.chunkID, summary)
			} else {
				fmt.Printf("✅ Item #%d (AI-Pass): Success - %s\n", res.chunkID, summary)
			}
		}
	}

	if len(errors) > 0 {
		fmt.Printf("\n⚠️  Completed with %d error(s):\n", len(errors))
		for _, errStr := range errors {
			fmt.Printf("   - %s\n", errStr)
		}
	}

	// 5. Save aggregated output JSON
	fmt.Print("⏳ Packaging and saving structured results... ")
	var successfulDocs []json.RawMessage
	for _, doc := range parsedDocs {
		if doc != nil {
			successfulDocs = append(successfulDocs, doc)
		}
	}

	outputBytes, err := json.MarshalIndent(successfulDocs, "", "  ")
	if err != nil {
		fmt.Println("FAILED")
		return fmt.Errorf("failed to marshal final JSON array: %w", err)
	}

	if err := os.WriteFile(outputPath, outputBytes, 0644); err != nil {
		fmt.Println("FAILED")
		return fmt.Errorf("failed to write output file: %w", err)
	}
	fmt.Println("Done!")

	duration := time.Since(startTime)
	fmt.Println("--------------------------------------------------")
	fmt.Printf("🎉 Pipeline completed in %v.\n", duration.Round(time.Millisecond))
	fmt.Printf("📁 Output saved to: %s\n", outputPath)

	return nil
}

// getJSONSummarySnippet dynamically retrieves descriptive fields from the schema JSON to show in worker progress output.
func getJSONSummarySnippet(raw json.RawMessage, schema string) string {
	var data map[string]interface{}
	if err := json.Unmarshal(raw, &data); err != nil {
		return "Parsed Raw JSON Document"
	}

	switch schema {
	case "invoice":
		vendor, _ := data["vendor_name"].(string)
		invNum, _ := data["invoice_number"].(string)
		if vendor != "" && invNum != "" {
			return fmt.Sprintf("Invoice %s from %s", invNum, vendor)
		} else if vendor != "" {
			return fmt.Sprintf("Invoice from %s", vendor)
		}
	case "contract":
		parties, _ := data["parties_involved"].([]interface{})
		if len(parties) >= 2 {
			p1, _ := parties[0].(string)
			p2, _ := parties[1].(string)
			if p1 != "" && p2 != "" {
				return fmt.Sprintf("Contract between %s and %s", p1, p2)
			}
		}
	case "generic":
		title, _ := data["title"].(string)
		if title != "" {
			return title
		}
	}

	return "Parsed Structured Data"
}
