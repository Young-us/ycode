package tools

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"
)

// WebFetchTool fetches content from a URL
type WebFetchTool struct {
	Client *http.Client
}

// NewWebFetchTool creates a new WebFetchTool
func NewWebFetchTool() *WebFetchTool {
	return &WebFetchTool{
		Client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

func (t *WebFetchTool) Name() string {
	return "web_fetch"
}

func (t *WebFetchTool) Description() string {
	return "Fetch content from a URL. Returns the page content converted to markdown."
}

func (t *WebFetchTool) Category() ToolCategory {
	return CategoryBasic
}

func (t *WebFetchTool) Parameters() []Parameter {
	return []Parameter{
		{
			Name:        "url",
			Type:        "string",
			Description: "The URL to fetch content from",
			Required:    true,
		},
	}
}

func (t *WebFetchTool) Execute(ctx context.Context, args map[string]interface{}) (*ToolResult, error) {
	targetURL, ok := args["url"].(string)
	if !ok {
		return &ToolResult{
			Content: "Error: 'url' parameter is required and must be a string",
			IsError: true,
		}, nil
	}

	// Validate URL
	if !strings.HasPrefix(targetURL, "http://") && !strings.HasPrefix(targetURL, "https://") {
		return &ToolResult{
			Content: "Error: URL must start with http:// or https://",
			IsError: true,
		}, nil
	}

	// Fetch content
	content, err := t.fetchURL(ctx, targetURL)
	if err != nil {
		return &ToolResult{
			Content: fmt.Sprintf("Fetch error: %v", err),
			IsError: true,
		}, nil
	}

	return &ToolResult{
		Content: content,
		IsError: false,
	}, nil
}

func (t *WebFetchTool) fetchURL(ctx context.Context, targetURL string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", targetURL, nil)
	if err != nil {
		return "", err
	}

	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,text/plain")

	resp, err := t.Client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("HTTP %d: %s", resp.StatusCode, resp.Status)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	contentType := resp.Header.Get("Content-Type")

	// Handle different content types
	if strings.Contains(contentType, "text/html") {
		return t.htmlToMarkdown(string(body), targetURL), nil
	}

	// Return plain text for other types
	return string(body), nil
}

// htmlToMarkdown converts HTML to simple markdown
func (t *WebFetchTool) htmlToMarkdown(html, baseURL string) string {
	// Extract title
	title := ""
	if titleMatch := regexp.MustCompile(`<title[^>]*>([^<]+)</title>`).FindStringSubmatch(html); len(titleMatch) > 1 {
		title = strings.TrimSpace(titleMatch[1])
	}

	// Remove script and style tags
	html = regexp.MustCompile(`<script[^>]*>[\s\S]*?</script>`).ReplaceAllString(html, "")
	html = regexp.MustCompile(`<style[^>]*>[\s\S]*?</style>`).ReplaceAllString(html, "")
	html = regexp.MustCompile(`<!--[\s\S]*?-->`).ReplaceAllString(html, "")

	// Remove head, nav, footer, aside
	html = regexp.MustCompile(`<head[^>]*>[\s\S]*?</head>`).ReplaceAllString(html, "")
	html = regexp.MustCompile(`<nav[^>]*>[\s\S]*?</nav>`).ReplaceAllString(html, "")
	html = regexp.MustCompile(`<footer[^>]*>[\s\S]*?</footer>`).ReplaceAllString(html, "")
	html = regexp.MustCompile(`<aside[^>]*>[\s\S]*}</aside>`).ReplaceAllString(html, "")

	// Convert headings
	html = regexp.MustCompile(`<h1[^>]*>([^<]+)</h1>`).ReplaceAllString(html, "\n# $1\n")
	html = regexp.MustCompile(`<h2[^>]*>([^<]+)</h2>`).ReplaceAllString(html, "\n## $1\n")
	html = regexp.MustCompile(`<h3[^>]*>([^<]+)</h3>`).ReplaceAllString(html, "\n### $1\n")
	html = regexp.MustCompile(`<h4[^>]*>([^<]+)</h4>`).ReplaceAllString(html, "\n#### $1\n")

	// Convert links
	html = regexp.MustCompile(`<a[^>]*href="([^"]+)"[^>]*>([^<]+)</a>`).ReplaceAllString(html, "[$2]($1)")

	// Convert code blocks
	html = regexp.MustCompile(`<pre[^>]*><code[^>]*>([^<]+)</code></pre>`).ReplaceAllString(html, "\n```\n$1\n```\n")
	html = regexp.MustCompile(`<code[^>]*>([^<]+)</code>`).ReplaceAllString(html, "`$1`")

	// Convert lists
	html = regexp.MustCompile(`<li[^>]*>([^<]+)</li>`).ReplaceAllString(html, "- $1\n")

	// Convert paragraphs and breaks
	html = regexp.MustCompile(`<br\s*/?>`).ReplaceAllString(html, "\n")
	html = regexp.MustCompile(`<p[^>]*>([^<]*)</p>`).ReplaceAllString(html, "\n$1\n")

	// Remove remaining HTML tags
	html = regexp.MustCompile(`<[^>]+>`).ReplaceAllString(html, "")

	// Decode HTML entities
	html = strings.ReplaceAll(html, "&amp;", "&")
	html = strings.ReplaceAll(html, "&lt;", "<")
	html = strings.ReplaceAll(html, "&gt;", ">")
	html = strings.ReplaceAll(html, "&quot;", "\"")
	html = strings.ReplaceAll(html, "&#39;", "'")
	html = strings.ReplaceAll(html, "&nbsp;", " ")

	// Clean up whitespace
	html = regexp.MustCompile(`\n{3,}`).ReplaceAllString(html, "\n\n")
	html = regexp.MustCompile(`[ \t]+`).ReplaceAllString(html, " ")

	// Build output
	var output strings.Builder
	if title != "" {
		output.WriteString(fmt.Sprintf("# %s\n\n", title))
		output.WriteString(fmt.Sprintf("Source: %s\n\n---\n\n", baseURL))
	}
	output.WriteString(strings.TrimSpace(html))

	return output.String()
}