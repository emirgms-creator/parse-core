package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "parsecore",
	Short: "ParseCore is an ultra-fast, air-gapped data ingestion and semantic structuring pipeline",
	Long: `ParseCore is a lightweight, privacy-first data ingestion pipeline.
It extracts unstructured text from documents (starting with PDFs), segments the text into semantic chunks,
and queries a local LLM via Ollama to structure the data into high-quality JSON schemas.
Designed for 100% air-gapped, zero-telemetry enterprise execution.`,
}

// Execute triggers the parsing of the CLI commands and flags.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error executing command: %v\n", err)
		os.Exit(1)
	}
}
