package extractor

import (
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"golang.org/x/net/html"
)

// WebExtractor fetches a web page via HTTP and extracts clean human-readable text contents.
type WebExtractor struct{}

// ExtractText fetches the HTML from the given URL and strips script/style tags and markup.
func (w *WebExtractor) ExtractText(urlPath string) (string, error) {
	client := &http.Client{
		Timeout: 15 * time.Second,
	}

	req, err := http.NewRequest(http.MethodGet, urlPath, nil)
	if err != nil {
		return "", fmt.Errorf("failed to build HTTP request for %q: %w", urlPath, err)
	}

	// Use realistic user-agent to prevent bot-blocking
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")

	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("HTTP request failed for %q: %w", urlPath, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("failed to fetch URL %q: server returned status %s", urlPath, resp.Status)
	}

	return extractTextFromHTML(resp.Body)
}

// extractTextFromHTML parses HTML and recursively retrieves all text nodes, ignoring scripting and layout noise.
func extractTextFromHTML(r io.Reader) (string, error) {
	doc, err := html.Parse(r)
	if err != nil {
		return "", fmt.Errorf("failed to parse HTML: %w", err)
	}

	var sb strings.Builder
	var traverse func(*html.Node)
	traverse = func(n *html.Node) {
		// Ignore script, style, nav, and footer tags and skip processing their children
		if n.Type == html.ElementNode {
			name := strings.ToLower(n.Data)
			if name == "script" || name == "style" || name == "nav" || name == "footer" {
				return
			}
		}

		if n.Type == html.TextNode {
			txt := strings.TrimSpace(n.Data)
			if txt != "" {
				sb.WriteString(txt)
				sb.WriteString(" ")
			}
		}

		for c := n.FirstChild; c != nil; c = c.NextSibling {
			traverse(c)
		}
	}

	traverse(doc)
	return strings.TrimSpace(sb.String()), nil
}
