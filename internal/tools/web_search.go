package tools

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// WebSearchTool searches the web using a search engine API
type WebSearchTool struct {
	Client *http.Client
}

// NewWebSearchTool creates a new WebSearchTool
func NewWebSearchTool() *WebSearchTool {
	return &WebSearchTool{
		Client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

func (t *WebSearchTool) Name() string {
	return "web_search"
}

func (t *WebSearchTool) Description() string {
	return "Search the web for information. Returns a list of search results with titles, URLs, and snippets."
}

func (t *WebSearchTool) Category() ToolCategory {
	return CategoryBasic
}

func (t *WebSearchTool) Parameters() []Parameter {
	return []Parameter{
		{
			Name:        "query",
			Type:        "string",
			Description: "The search query",
			Required:    true,
		},
	}
}

func (t *WebSearchTool) Execute(ctx context.Context, args map[string]interface{}) (*ToolResult, error) {
	query, ok := args["query"].(string)
	if !ok {
		return &ToolResult{
			Content: "Error: 'query' parameter is required and must be a string",
			IsError: true,
		}, nil
	}

	// Use DuckDuckGo Instant Answer API (no API key required)
	results, err := t.searchDuckDuckGo(ctx, query)
	if err != nil {
		return &ToolResult{
			Content: fmt.Sprintf("Search error: %v", err),
			IsError: true,
		}, nil
	}

	if len(results) == 0 {
		return &ToolResult{
			Content: fmt.Sprintf("No results found for: %s", query),
			IsError: false,
		}, nil
	}

	var output strings.Builder
	output.WriteString(fmt.Sprintf("Search results for: %s\n\n", query))

	for i, result := range results {
		output.WriteString(fmt.Sprintf("%d. %s\n", i+1, result.Title))
		output.WriteString(fmt.Sprintf("   URL: %s\n", result.URL))
		if result.Snippet != "" {
			output.WriteString(fmt.Sprintf("   %s\n", result.Snippet))
		}
		output.WriteString("\n")
	}

	return &ToolResult{
		Content: output.String(),
		IsError: false,
	}, nil
}

type SearchResult struct {
	Title   string
	URL     string
	Snippet string
}

// searchDuckDuckGo uses DuckDuckGo HTML search
func (t *WebSearchTool) searchDuckDuckGo(ctx context.Context, query string) ([]SearchResult, error) {
	searchURL := fmt.Sprintf("https://html.duckduckgo.com/html/?q=%s", url.QueryEscape(query))

	req, err := http.NewRequestWithContext(ctx, "GET", searchURL, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")

	resp, err := t.Client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	// Parse HTML to extract results
	return t.parseDuckDuckGoResults(string(body)), nil
}

func (t *WebSearchTool) parseDuckDuckGoResults(html string) []SearchResult {
	var results []SearchResult

	// Simple HTML parsing - find result links
	// DuckDuckGo HTML format: <a class="result__a" href="...">Title</a>
	lines := strings.Split(html, "\n")

	for _, line := range lines {
		if strings.Contains(line, "class=\"result__a\"") {
			result := t.parseResultLink(line)
			if result.URL != "" {
				results = append(results, result)
			}
		}
		if len(results) >= 10 {
			break
		}
	}

	return results
}

func (t *WebSearchTool) parseResultLink(line string) SearchResult {
	result := SearchResult{}

	// Extract URL
	if idx := strings.Index(line, "href=\""); idx != -1 {
		start := idx + 6
		if end := strings.Index(line[start:], "\""); end != -1 {
			rawURL := line[start : start+end]
			// DuckDuckGo uses redirect URLs, extract actual URL
			if strings.Contains(rawURL, "uddg=") {
				if uddgIdx := strings.Index(rawURL, "uddg="); uddgIdx != -1 {
					encodedURL := rawURL[uddgIdx+5:]
					if decoded, err := url.QueryUnescape(encodedURL); err == nil {
						result.URL = decoded
					}
				}
			} else {
				result.URL = rawURL
			}
		}
	}

	// Extract title (between > and <)
	if start := strings.Index(line, ">"); start != -1 {
		rest := line[start+1:]
		if end := strings.Index(rest, "<"); end != -1 {
			result.Title = strings.TrimSpace(rest[:end])
		}
	}

	// Clean up title
	result.Title = strings.ReplaceAll(result.Title, "&amp;", "&")
	result.Title = strings.ReplaceAll(result.Title, "&lt;", "<")
	result.Title = strings.ReplaceAll(result.Title, "&gt;", ">")

	return result
}