package extractor

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestWebExtractor(t *testing.T) {
	// Start local mock HTTP server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprintln(w, `
			<!DOCTYPE html>
			<html>
			<head>
				<title>Test Page</title>
				<style>
					body { font-family: sans-serif; }
				</style>
				<script>
					console.log("Ignore me!");
				</script>
			</head>
			<body>
				<nav>
					<a href="/home">Home</a> | <a href="/about">About</a>
				</nav>
				<h1>Welcome to ParseCore Test</h1>
				<p>This is a paragraph containing <strong>meaningful text</strong> to extract.</p>
				<footer>
					<p>© 2026 ParseCore Corp</p>
				</footer>
			</body>
			</html>
		`)
	}))
	defer server.Close()

	ext := &WebExtractor{}
	text, err := ext.ExtractText(server.URL)
	if err != nil {
		t.Fatalf("ExtractText failed: %v", err)
	}

	trimmed := strings.TrimSpace(text)
	
	// Verify script contents are ignored
	if strings.Contains(trimmed, "Ignore me") {
		t.Errorf("expected script content to be skipped, got: %q", trimmed)
	}

	// Verify style content/selectors are ignored
	if strings.Contains(trimmed, "font-family") {
		t.Errorf("expected style content to be skipped, got: %q", trimmed)
	}

	// Verify nav content is ignored
	if strings.Contains(trimmed, "Home") || strings.Contains(trimmed, "About") {
		t.Errorf("expected nav content to be skipped, got: %q", trimmed)
	}

	// Verify footer content is ignored
	if strings.Contains(trimmed, "ParseCore Corp") {
		t.Errorf("expected footer content to be skipped, got: %q", trimmed)
	}

	// Verify body main texts are present
	if !strings.Contains(trimmed, "Welcome to ParseCore Test") {
		t.Errorf("expected header text, got: %q", trimmed)
	}
	if !strings.Contains(trimmed, "meaningful text") {
		t.Errorf("expected paragraph text, got: %q", trimmed)
	}
}

func TestWebExtractorTimeout(t *testing.T) {
	// Triggering a client timeout is hard with local httptest unless we insert delay,
	// but we can test invalid URLs to verify it handles errors gracefully.
	ext := &WebExtractor{}
	_, err := ext.ExtractText("http://localhost:99999") // invalid port
	if err == nil {
		t.Error("expected error for invalid URL/port, got nil")
	}
}
