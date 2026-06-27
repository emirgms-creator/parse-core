<p align="center">
  <img src="logo.svg" alt="ParseCore Logo" width="120px" height="120px" />
</p>

<h1 align="center">ParseCore</h1>

**ParseCore** is an ultra-fast, privacy-first, 100% local data ingestion and semantic structuring pipeline. 

It is engineered to extract unstructured text from multiple document formats, break the text down into contextual semantic chunks, and feed them concurrently into a local LLM via Ollama to output high-quality, structured JSON. The system operates entirely air-gapped with zero external dependencies and telemetry, preserving sensitive corporate data and IP.

---

## Key Features

*   **Multi-Format Ingestion**: Unified Document Extractor Factory supporting:
    *   `.pdf` (using lightweight text extraction)
    *   `.docx` (pure-Go CGO-free Word XML processor)
    *   `.csv` (variable-column tolerant table parser mapping headers to values)
    *   `.txt` & `.md` (native standard library readers)
*   **Hybrid Fast-Pass Router (v0.2.0 New)**: Aggressively intercept structured rows (CSV). Clean or trivially dirty cells (containing `ERROR`, `UNKNOWN`, `N/A`, `NULL`, `NONE`, `-` or empty values) are resolved and mapped deterministically, bypassing the LLM completely (**Fast-Pass ⚡**). Unstructured or complex natural language text is automatically routed to LLM concurrent workers (**AI-Pass Fallback ✅**), enabling massive performance speedups (e.g., 10,000 CSV rows parsed in under 200ms).
*   **Auto-Schema Inference (v0.2.0 New)**: Default schema option (`auto`) automatically inspects structured inputs and generates JSON schema fields on-the-fly from the CSV column headers, generating target prompts dynamically. For unstructured files (PDF, DOCX, etc.), it fallback-routes to the `generic` schema.
*   **Semantic Chunking with Overlap**: Respects natural language boundaries. Segments text hierarchically:
    1.  *Primary*: Paragraph boundaries (`\n\n`)
    2.  *Secondary*: Sentence boundaries (`. `, `! `, `? `)
    3.  *Fallback*: Word boundaries (spaces, preventing mid-word breaks)
    4.  *Sliding Window*: Maintains context by aligning a sliding window overlap region with boundaries.
*   **Structured CLI**: Clean developer experience powered by Cobra with fail-fast validations, custom flag routing, and emoji-decorated terminal outputs.
*   **Dynamic & Custom JSON Schemas**:
    *   Predefined templates: `auto`, `generic`, `contract`, `invoice`.
    *   Custom Templates: Feed custom JSON schema files (e.g. `--schema ./my_template.json`). System compiler automatically builds target prompts on-the-fly.
*   **Deterministic & Resilient Ingestion**:
    *   Enforces `temperature = 0.0` to eradicate LLM hallucinations.
    *   Implements HTTP request retry routines with exponential backoff on network/rate-limiting errors.
    *   Concurrent worker pool execution utilizing Go channels and `sync.WaitGroup`.

---

## Project Structure

```text
parse-core/
├── cmd/
│   └── parsecore/
│       ├── main.go         # Lightweight app entrypoint
│       ├── root.go         # Root Cobra CLI definition
│       └── extract.go      # 'extract' subcommand and runner logic
├── internal/
│   ├── extractor/
│   │   ├── extractor.go           # Extractor interface, factory, and helpers
│   │   ├── semantic_chunker.go    # Semantic chunking sliding window code
│   │   ├── pdf.go                 # PDF extractor
│   │   ├── text.go                # Text and Markdown extractor
│   │   ├── csv.go                 # Variable-column CSV extractor
│   │   ├── docx.go                # CGO-free DOCX extractor
│   │   ├── router.go              # Heuristic cleaning & LLM routing engine
│   │   ├── router_test.go         # Router and fast-pass unit tests
│   │   └── extractor_test.go      # Boundary unit tests
│   ├── llm/
│   │   ├── client.go              # Ollama API client with exponential backoff retries
│   │   └── schemas.go             # Prompt templates and custom file compiler
│   └── models/
│       └── models.go              # Structured output Go model
├── tests/
│   └── chaos_test.go       # E2E integration and adversarial red-team tests
├── LICENSE                 # MIT License
├── go.mod                  # Go module definition
└── README.md               # Documentation
```

---

## Getting Started

### Prerequisites

1.  **Go**: Go 1.21+ installed on your local machine.
2.  **Ollama**: Install [Ollama](https://ollama.com/) locally.
3.  **Local Model**: Pull the default model (phi4-mini):
    ```bash
    ollama pull phi4-mini
    ```

### Compilation

Compile the CLI binary:
```bash
go build ./cmd/parsecore
```
*(On Windows, this generates `parsecore.exe`)*.

---

## Usage

Use the `extract` subcommand to process local documents.

```bash
./parsecore extract -i <input_file> -o <output_json_path> [flags]
```

### Command Flags

*   `-i, --input` (Required): Path to target document (`.pdf`, `.docx`, `.txt`, `.md`, `.csv`).
*   `-o, --output` (Required): Path to save the resulting structured JSON array.
*   `-s, --schema` (Default: `auto`): Extraction schema type (`auto`, `generic`, `contract`, `invoice`) or a local custom JSON schema file path (e.g. `--schema ./invoice_template.json`). If `auto`, the system auto-infers headers for structured CSV rows and falls back to `generic` for unstructured documents.
*   `-m, --model` (Default: `phi4-mini`): Local LLM model tag registered in Ollama.
*   `-w, --workers` (Default: `4`): Number of concurrent goroutines parsing text chunks.
*   `--chunk-size` (Default: `2000`): Maximum character limit per semantic chunk.
*   `--overlap` (Default: `200`): Overlap size between adjacent chunks.
*   `--url` (Default: `http://localhost:11434`): local Ollama API server address.

### Examples

**1. Parse an Invoice using Predefined Schema:**
```bash
./parsecore extract -i invoice.pdf -o invoice_data.json --schema invoice
```

**2. Ingest a Contract using Custom JSON Layout:**
Create a custom schema file `contract_terms.json`:
```json
{
  "contract_name": "string",
  "effective_date": "string",
  "signing_parties": ["string"]
}
```
Execute extraction:
```bash
./parsecore extract -i lease.docx -o terms.json --schema contract_terms.json -m phi4-mini:latest -w 2
```

**3. Zero-Config CSV Auto-Ingestion & Cleaning (v0.2.0):**
Ingest a structured CSV dataset without specifying a schema, triggering automatic header mapping and Fast-Pass cleaning:
```bash
./parsecore extract -i sales_data.csv -o cleaned_sales.json
```

---

## Testing

### Unit Tests
Run unit tests checking formatting extractors and semantic boundary chunking:
```bash
go test ./internal/... -v
```

### Chaos & Stress Integration Tests
Run E2E adversarial tests. The suite programmatically compiles the CLI binary and validates behavior under boundary strings (500,000 continuous chars), corrupted zip/XML docs, variable-column CSVs, adversarial contexts (null verification checks), and concurrency stress limits (50 concurrent worker threads):
```bash
go test ./tests -v
```

---

## License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.
