package lsp

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"

	"github.com/Young-us/ycode/internal/logger"
)

// Client represents an LSP client with full protocol support
type Client struct {
	// Server process management
	cmd    *exec.Cmd
	stdin  io.WriteCloser
	stdout io.ReadCloser
	stderr io.ReadCloser

	// JSON-RPC state
	requestID int
	mu        sync.Mutex
	pending   map[interface{}]chan json.RawMessage

	// Server capabilities
	capabilities ServerCapabilities

	// Diagnostics storage
	diagnosticsMu sync.RWMutex
	diagnostics   map[DocumentURI][]Diagnostic

	// Callbacks
	onDiagnostics func(uri DocumentURI, diagnostics []Diagnostic)

	// File watcher
	watcher *FileWatcher

	// Context for lifecycle
	ctx    context.Context
	cancel context.CancelFunc
}

// NewClient creates a new LSP client
func NewClient() *Client {
	ctx, cancel := context.WithCancel(context.Background())
	return &Client{
		pending:     make(map[interface{}]chan json.RawMessage),
		diagnostics: make(map[DocumentURI][]Diagnostic),
		ctx:         ctx,
		cancel:      cancel,
	}
}

// StartServer starts an LSP server process
func (c *Client) StartServer(command string, args ...string) error {
	logger.Info("lsp", "Starting LSP server: %s %v", command, args)

	c.cmd = exec.Command(command, args...)

	var err error
	c.stdin, err = c.cmd.StdinPipe()
	if err != nil {
		logger.Error("lsp", "Failed to create stdin pipe: %v", err)
		return fmt.Errorf("failed to create stdin pipe: %w", err)
	}

	c.stdout, err = c.cmd.StdoutPipe()
	if err != nil {
		logger.Error("lsp", "Failed to create stdout pipe: %v", err)
		return fmt.Errorf("failed to create stdout pipe: %w", err)
	}

	c.stderr, err = c.cmd.StderrPipe()
	if err != nil {
		logger.Error("lsp", "Failed to create stderr pipe: %v", err)
		return fmt.Errorf("failed to create stderr pipe: %w", err)
	}

	if err := c.cmd.Start(); err != nil {
		logger.Error("lsp", "Failed to start LSP server: %v", err)
		return fmt.Errorf("failed to start LSP server: %w", err)
	}

	logger.Info("lsp", "LSP server started successfully")

	// Start reading responses
	go c.readResponses()

	// Log stderr
	go c.logStderr()

	return nil
}

// Initialize sends the initialize request to the LSP server
func (c *Client) Initialize(ctx context.Context, rootURI DocumentURI) (*InitializeResult, error) {
	logger.Debug("lsp", "Initializing LSP server with rootURI: %s", rootURI)

	params := InitializeParams{
		ProcessID: os.Getpid(),
		RootURI:   rootURI,
		RootPath:  strings.TrimPrefix(string(rootURI), "file://"),
		Capabilities: ClientCapabilities{
			TextDocument: TextDocumentClientCapabilities{
				Synchronization: &TextDocumentSyncClientCapabilities{
					DynamicRegistration: false,
					WillSave:            true,
					WillSaveWaitUntil:   true,
					DidSave:             true,
				},
				Completion: &CompletionClientCapabilities{
					DynamicRegistration: false,
					CompletionItem: &CompletionItemClientCapabilities{
						SnippetSupport:          true,
						CommitCharactersSupport: true,
						DocumentationFormat: []MarkupKind{
							PlainText,
							Markdown,
						},
					},
				},
				Hover: &HoverClientCapabilities{
					DynamicRegistration: false,
					ContentFormat: []MarkupKind{
						PlainText,
						Markdown,
					},
				},
				Definition: &DefinitionClientCapabilities{
					DynamicRegistration: false,
				},
				References: &ReferenceClientCapabilities{
					DynamicRegistration: false,
				},
				Diagnostic: &DiagnosticClientCapabilities{
					DynamicRegistration: false,
				},
			},
			Workspace: WorkspaceClientCapabilities{
				DidChangeWatchedFiles: &DidChangeWatchedFilesClientCapabilities{
					DynamicRegistration: false,
				},
			},
		},
		ClientInfo: &ClientInfo{
			Name:    "ycode",
			Version: "0.1.0",
		},
		Trace: "off",
	}

	result, err := c.sendRequest(ctx, "initialize", params)
	if err != nil {
		logger.Error("lsp", "Initialize request failed: %v", err)
		return nil, fmt.Errorf("initialize request failed: %w", err)
	}

	var initResult InitializeResult
	if err := json.Unmarshal(result, &initResult); err != nil {
		logger.Error("lsp", "Failed to unmarshal initialize result: %v", err)
		return nil, fmt.Errorf("failed to unmarshal initialize result: %w", err)
	}

	c.capabilities = initResult.Capabilities

	// Send initialized notification
	if err := c.sendNotification("initialized", struct{}{}); err != nil {
		logger.Error("lsp", "Failed to send initialized notification: %v", err)
		return nil, fmt.Errorf("failed to send initialized notification: %w", err)
	}

	logger.Info("lsp", "LSP server initialized successfully")
	return &initResult, nil
}

// Shutdown sends the shutdown request to the LSP server
func (c *Client) Shutdown(ctx context.Context) error {
	_, err := c.sendRequest(ctx, "shutdown", nil)
	if err != nil {
		return fmt.Errorf("shutdown request failed: %w", err)
	}

	// Send exit notification
	if err := c.sendNotification("exit", nil); err != nil {
		return fmt.Errorf("failed to send exit notification: %w", err)
	}

	// Cancel context
	c.cancel()

	// Stop server process
	if c.cmd != nil && c.cmd.Process != nil {
		if err := c.cmd.Process.Kill(); err != nil {
			// Silently handle kill error
		}
	}

	return nil
}

// DidOpenTextDocument sends a textDocument/didOpen notification
func (c *Client) DidOpenTextDocument(ctx context.Context, uri DocumentURI, languageID string, version int, text string) error {
	params := DidOpenTextDocumentParams{
		TextDocument: TextDocumentItem{
			URI:        uri,
			LanguageID: languageID,
			Version:    version,
			Text:       text,
		},
	}

	return c.sendNotification("textDocument/didOpen", params)
}

// DidChangeTextDocument sends a textDocument/didChange notification
func (c *Client) DidChangeTextDocument(ctx context.Context, uri DocumentURI, version int, text string) error {
	params := DidChangeTextDocumentParams{
		TextDocument: VersionedTextDocumentIdentifier{
			TextDocumentIdentifier: TextDocumentIdentifier{URI: uri},
			Version:                version,
		},
		ContentChanges: []TextDocumentContentChangeEvent{
			{Text: text},
		},
	}

	return c.sendNotification("textDocument/didChange", params)
}

// DidSaveTextDocument sends a textDocument/didSave notification
func (c *Client) DidSaveTextDocument(ctx context.Context, uri DocumentURI, text *string) error {
	params := DidSaveTextDocumentParams{
		TextDocument: TextDocumentIdentifier{URI: uri},
		Text:         text,
	}

	return c.sendNotification("textDocument/didSave", params)
}

// DidCloseTextDocument sends a textDocument/didClose notification
func (c *Client) DidCloseTextDocument(ctx context.Context, uri DocumentURI) error {
	params := DidCloseTextDocumentParams{
		TextDocument: TextDocumentIdentifier{URI: uri},
	}

	return c.sendNotification("textDocument/didClose", params)
}

// Hover sends a textDocument/hover request
func (c *Client) Hover(ctx context.Context, uri DocumentURI, line, character int) (*Hover, error) {
	logger.Debug("lsp", "Hover request at %s:%d:%d", uri, line, character)

	params := HoverParams{
		TextDocumentPositionParams: TextDocumentPositionParams{
			TextDocument: TextDocumentIdentifier{URI: uri},
			Position:     Position{Line: line, Character: character},
		},
	}

	result, err := c.sendRequest(ctx, "textDocument/hover", params)
	if err != nil {
		return nil, fmt.Errorf("hover request failed: %w", err)
	}

	if result == nil {
		return nil, nil
	}

	var hover Hover
	if err := json.Unmarshal(result, &hover); err != nil {
		return nil, fmt.Errorf("failed to unmarshal hover result: %w", err)
	}

	return &hover, nil
}

// Definition sends a textDocument/definition request
func (c *Client) Definition(ctx context.Context, uri DocumentURI, line, character int) ([]Location, error) {
	logger.Debug("lsp", "Definition request at %s:%d:%d", uri, line, character)

	params := DefinitionParams{
		TextDocumentPositionParams: TextDocumentPositionParams{
			TextDocument: TextDocumentIdentifier{URI: uri},
			Position:     Position{Line: line, Character: character},
		},
	}

	result, err := c.sendRequest(ctx, "textDocument/definition", params)
	if err != nil {
		return nil, fmt.Errorf("definition request failed: %w", err)
	}

	if result == nil {
		return nil, nil
	}

	// Result can be Location or Location[]
	var locations []Location
	if err := json.Unmarshal(result, &locations); err != nil {
		// Try single location
		var location Location
		if err2 := json.Unmarshal(result, &location); err2 != nil {
			return nil, fmt.Errorf("failed to unmarshal definition result: %w", err)
		}
		locations = []Location{location}
	}

	return locations, nil
}

// References sends a textDocument/references request
func (c *Client) References(ctx context.Context, uri DocumentURI, line, character int, includeDeclaration bool) ([]Location, error) {
	logger.Debug("lsp", "References request at %s:%d:%d (includeDeclaration=%v)", uri, line, character, includeDeclaration)

	params := ReferenceParams{
		TextDocumentPositionParams: TextDocumentPositionParams{
			TextDocument: TextDocumentIdentifier{URI: uri},
			Position:     Position{Line: line, Character: character},
		},
		Context: ReferenceContext{
			IncludeDeclaration: includeDeclaration,
		},
	}

	result, err := c.sendRequest(ctx, "textDocument/references", params)
	if err != nil {
		return nil, fmt.Errorf("references request failed: %w", err)
	}

	if result == nil {
		return nil, nil
	}

	var locations []Location
	if err := json.Unmarshal(result, &locations); err != nil {
		return nil, fmt.Errorf("failed to unmarshal references result: %w", err)
	}

	return locations, nil
}

// Completion sends a textDocument/completion request
func (c *Client) Completion(ctx context.Context, uri DocumentURI, line, character int) (*CompletionList, error) {
	params := CompletionParams{
		TextDocumentPositionParams: TextDocumentPositionParams{
			TextDocument: TextDocumentIdentifier{URI: uri},
			Position:     Position{Line: line, Character: character},
		},
	}

	result, err := c.sendRequest(ctx, "textDocument/completion", params)
	if err != nil {
		return nil, fmt.Errorf("completion request failed: %w", err)
	}

	if result == nil {
		return nil, nil
	}

	var completionList CompletionList
	if err := json.Unmarshal(result, &completionList); err != nil {
		// Try CompletionItem[]
		var items []CompletionItem
		if err2 := json.Unmarshal(result, &items); err2 != nil {
			return nil, fmt.Errorf("failed to unmarshal completion result: %w", err)
		}
		completionList = CompletionList{
			Items: items,
		}
	}

	return &completionList, nil
}

// GetDiagnostics returns diagnostics for a document
func (c *Client) GetDiagnostics(uri DocumentURI) []Diagnostic {
	c.diagnosticsMu.RLock()
	defer c.diagnosticsMu.RUnlock()

	diagnostics, ok := c.diagnostics[uri]
	if !ok {
		return nil
	}

	// Return a copy
	result := make([]Diagnostic, len(diagnostics))
	copy(result, diagnostics)
	return result
}

// OnDiagnostics sets a callback for diagnostics notifications
func (c *Client) OnDiagnostics(callback func(uri DocumentURI, diagnostics []Diagnostic)) {
	c.onDiagnostics = callback
}

// StartFileWatcher starts watching files for changes
func (c *Client) StartFileWatcher(rootDir string) error {
	watcher, err := NewFileWatcher(rootDir, func(path string, op FileOp) {
		uri := DocumentURI("file://" + filepath.ToSlash(path))

		switch op {
		case FileOpCreate, FileOpModify:
			// Read file content
			content, err := os.ReadFile(path)
			if err != nil {
				return
			}

			// Determine language ID
			languageID := getLanguageID(path)

			// Send didOpen or didChange
			if op == FileOpCreate {
				_ = c.DidOpenTextDocument(c.ctx, uri, languageID, 1, string(content))
			} else {
				_ = c.DidChangeTextDocument(c.ctx, uri, 1, string(content))
			}

		case FileOpDelete:
			_ = c.DidCloseTextDocument(c.ctx, uri)
		}
	})

	if err != nil {
		return fmt.Errorf("failed to create file watcher: %w", err)
	}

	c.watcher = watcher
	return watcher.Start()
}

// StopFileWatcher stops the file watcher
func (c *Client) StopFileWatcher() {
	if c.watcher != nil {
		c.watcher.Stop()
	}
}

// sendRequest sends a JSON-RPC request and waits for response
func (c *Client) sendRequest(ctx context.Context, method string, params interface{}) (json.RawMessage, error) {
	c.mu.Lock()
	id := c.requestID
	c.requestID++
	ch := make(chan json.RawMessage, 1)
	c.pending[id] = ch
	c.mu.Unlock()

	defer func() {
		c.mu.Lock()
		delete(c.pending, id)
		c.mu.Unlock()
	}()

	// Build request
	request := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      float64(id), // Use float64 for JSON compatibility
		"method":  method,
	}
	if params != nil {
		request["params"] = params
	}

	// Send request
	if err := c.writeMessage(request); err != nil {
		return nil, fmt.Errorf("failed to write request: %w", err)
	}

	// Wait for response
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case response := <-ch:
		// Check for error
		var resp struct {
			Error *struct {
				Code    int    `json:"code"`
				Message string `json:"message"`
			} `json:"error"`
			Result json.RawMessage `json:"result"`
		}
		if err := json.Unmarshal(response, &resp); err != nil {
			return nil, fmt.Errorf("failed to unmarshal response: %w", err)
		}
		if resp.Error != nil {
			return nil, fmt.Errorf("LSP error %d: %s", resp.Error.Code, resp.Error.Message)
		}
		return resp.Result, nil
	}
}

// sendNotification sends a JSON-RPC notification
func (c *Client) sendNotification(method string, params interface{}) error {
	notification := map[string]interface{}{
		"jsonrpc": "2.0",
		"method":  method,
	}
	if params != nil {
		notification["params"] = params
	}

	return c.writeMessage(notification)
}

// writeMessage writes a JSON-RPC message to stdin
func (c *Client) writeMessage(msg interface{}) error {
	data, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("failed to marshal message: %w", err)
	}

	// Write Content-Length header
	header := fmt.Sprintf("Content-Length: %d\r\n\r\n", len(data))
	if _, err := c.stdin.Write([]byte(header)); err != nil {
		return fmt.Errorf("failed to write header: %w", err)
	}

	// Write message body
	if _, err := c.stdin.Write(data); err != nil {
		return fmt.Errorf("failed to write message: %w", err)
	}

	return nil
}

// readResponses reads JSON-RPC responses from stdout
func (c *Client) readResponses() {
	reader := bufio.NewReader(c.stdout)

	for {
		select {
		case <-c.ctx.Done():
			return
		default:
		}

		// Read Content-Length header
		var contentLength int
		for {
			line, err := reader.ReadString('\n')
			if err != nil {
				if err != io.EOF {
					// Silently handle header read error
				}
				return
			}

			line = strings.TrimSpace(line)
			if line == "" {
				break
			}

			if strings.HasPrefix(line, "Content-Length:") {
				fmt.Sscanf(line, "Content-Length: %d", &contentLength)
			}
		}

		if contentLength <= 0 {
			continue
		}

		// Read message body
		body := make([]byte, contentLength)
		if _, err := io.ReadFull(reader, body); err != nil {
			// Silently handle body read error (server may have closed)
			return
		}

		// Parse message
		var msg struct {
			JSONRPC string          `json:"jsonrpc"`
			ID      interface{}     `json:"id,omitempty"`
			Method  string          `json:"method,omitempty"`
			Params  json.RawMessage `json:"params,omitempty"`
			Result  json.RawMessage `json:"result,omitempty"`
			Error   *struct {
				Code    int    `json:"code"`
				Message string `json:"message"`
			} `json:"error,omitempty"`
		}

		if err := json.Unmarshal(body, &msg); err != nil {
			// Silently skip malformed messages
			continue
		}

		// Handle response or notification
		if msg.Method != "" {
			// This is a notification or request from server
			c.handleServerMessage(msg.Method, msg.Params)
		} else if msg.ID != nil {
			// This is a response to our request
			// Convert ID to int for lookup (JSON may have decoded as float64)
			var id int
			switch v := msg.ID.(type) {
			case float64:
				id = int(v)
			case int:
				id = v
			default:
				continue
			}

			c.mu.Lock()
			ch, ok := c.pending[id]
			c.mu.Unlock()

			if ok {
				ch <- body
			}
		}
	}
}

// handleServerMessage handles notifications/requests from the server
func (c *Client) handleServerMessage(method string, params json.RawMessage) {
	switch method {
	case "textDocument/publishDiagnostics":
		c.handleDiagnostics(params)
	case "window/showMessage":
		// Silently handle showMessage - don't log to stdout
		c.handleShowMessage(params)
	case "window/logMessage":
		// Silently handle logMessage - don't log to stdout
		c.handleLogMessage(params)
	default:
		// Silently ignore unhandled messages
	}
}

// handleDiagnostics handles diagnostics notifications
func (c *Client) handleDiagnostics(params json.RawMessage) {
	var diagParams PublishDiagnosticsParams
	if err := json.Unmarshal(params, &diagParams); err != nil {
		return
	}

	c.diagnosticsMu.Lock()
	c.diagnostics[diagParams.URI] = diagParams.Diagnostics
	c.diagnosticsMu.Unlock()

	if c.onDiagnostics != nil {
		c.onDiagnostics(diagParams.URI, diagParams.Diagnostics)
	}
}

// handleShowMessage handles window/showMessage notifications
func (c *Client) handleShowMessage(params json.RawMessage) {
	var msgParams ShowMessageParams
	if err := json.Unmarshal(params, &msgParams); err != nil {
		return
	}
	// Store message for potential display but don't log to stdout
	_ = msgParams
}

// handleLogMessage handles window/logMessage notifications
func (c *Client) handleLogMessage(params json.RawMessage) {
	var msgParams LogMessageParams
	if err := json.Unmarshal(params, &msgParams); err != nil {
		return
	}
	// Silently handle log messages - don't log to stdout
	_ = msgParams
}

// logStderr logs stderr output from the LSP server
func (c *Client) logStderr() {
	scanner := bufio.NewScanner(c.stderr)
	for scanner.Scan() {
		// Silently consume stderr - don't log to stdout
		_ = scanner.Text()
	}
}

// getLanguageID returns the language ID for a file
func getLanguageID(path string) string {
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".go":
		return "go"
	case ".js", ".jsx":
		return "javascript"
	case ".ts", ".tsx":
		return "typescript"
	case ".py":
		return "python"
	case ".rs":
		return "rust"
	case ".java":
		return "java"
	case ".c":
		return "c"
	case ".cpp", ".cc", ".cxx":
		return "cpp"
	case ".h", ".hpp":
		return "c"
	case ".rb":
		return "ruby"
	case ".php":
		return "php"
	case ".swift":
		return "swift"
	case ".kt", ".kts":
		return "kotlin"
	case ".scala":
		return "scala"
	case ".lua":
		return "lua"
	case ".vim":
		return "vim"
	case ".sh", ".bash":
		return "shellscript"
	case ".yaml", ".yml":
		return "yaml"
	case ".json":
		return "json"
	case ".xml":
		return "xml"
	case ".html", ".htm":
		return "html"
	case ".css":
		return "css"
	case ".scss":
		return "scss"
	case ".less":
		return "less"
	case ".md":
		return "markdown"
	case ".sql":
		return "sql"
	default:
		return "plaintext"
	}
}
