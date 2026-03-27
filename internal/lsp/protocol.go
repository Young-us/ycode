package lsp

import (
	"encoding/json"
)

// DocumentURI represents a document URI
type DocumentURI string

// Position represents a position in a text document
type Position struct {
	Line      int `json:"line"`
	Character int `json:"character"`
}

// Range represents a range in a text document
type Range struct {
	Start Position `json:"start"`
	End   Position `json:"end"`
}

// Location represents a location in a text document
type Location struct {
	URI   DocumentURI `json:"uri"`
	Range Range       `json:"range"`
}

// DiagnosticSeverity represents the severity of a diagnostic
type DiagnosticSeverity int

const (
	DiagnosticSeverityError       DiagnosticSeverity = 1
	DiagnosticSeverityWarning     DiagnosticSeverity = 2
	DiagnosticSeverityInformation DiagnosticSeverity = 3
	DiagnosticSeverityHint        DiagnosticSeverity = 4
)

// Diagnostic represents a diagnostic, such as a compiler error or warning
type Diagnostic struct {
	Range    Range              `json:"range"`
	Severity DiagnosticSeverity `json:"severity,omitempty"`
	Code     interface{}        `json:"code,omitempty"` // string or int
	Source   string             `json:"source,omitempty"`
	Message  string             `json:"message"`
}

// PublishDiagnosticsParams represents parameters for publishDiagnostics notification
type PublishDiagnosticsParams struct {
	URI         DocumentURI  `json:"uri"`
	Version     int          `json:"version,omitempty"`
	Diagnostics []Diagnostic `json:"diagnostics"`
}

// MarkupKind represents the kind of markup used
type MarkupKind string

const (
	PlainText MarkupKind = "plaintext"
	Markdown  MarkupKind = "markdown"
)

// MarkupContent represents markup content
type MarkupContent struct {
	Kind  MarkupKind `json:"kind"`
	Value string     `json:"value"`
}

// Hover represents the result of a hover request
type Hover struct {
	Contents MarkupContent `json:"contents"`
	Range    *Range        `json:"range,omitempty"`
}

// CompletionItemKind represents the kind of a completion item
type CompletionItemKind int

const (
	CompletionItemKindText          CompletionItemKind = 1
	CompletionItemKindMethod        CompletionItemKind = 2
	CompletionItemKindFunction      CompletionItemKind = 3
	CompletionItemKindConstructor   CompletionItemKind = 4
	CompletionItemKindField         CompletionItemKind = 5
	CompletionItemKindVariable      CompletionItemKind = 6
	CompletionItemKindClass         CompletionItemKind = 7
	CompletionItemKindInterface     CompletionItemKind = 8
	CompletionItemKindModule        CompletionItemKind = 9
	CompletionItemKindProperty      CompletionItemKind = 10
	CompletionItemKindUnit          CompletionItemKind = 11
	CompletionItemKindValue         CompletionItemKind = 12
	CompletionItemKindEnum          CompletionItemKind = 13
	CompletionItemKindKeyword       CompletionItemKind = 14
	CompletionItemKindSnippet       CompletionItemKind = 15
	CompletionItemKindColor         CompletionItemKind = 16
	CompletionItemKindFile          CompletionItemKind = 17
	CompletionItemKindReference     CompletionItemKind = 18
	CompletionItemKindFolder        CompletionItemKind = 19
	CompletionItemKindEnumMember    CompletionItemKind = 20
	CompletionItemKindConstant      CompletionItemKind = 21
	CompletionItemKindStruct        CompletionItemKind = 22
	CompletionItemKindEvent         CompletionItemKind = 23
	CompletionItemKindOperator      CompletionItemKind = 24
	CompletionItemKindTypeParameter CompletionItemKind = 25
)

// CompletionItem represents a completion item
type CompletionItem struct {
	Label               string             `json:"label"`
	Kind                CompletionItemKind `json:"kind,omitempty"`
	Detail              string             `json:"detail,omitempty"`
	Documentation       interface{}        `json:"documentation,omitempty"` // string or MarkupContent
	Deprecated          bool               `json:"deprecated,omitempty"`
	Preselect           bool               `json:"preselect,omitempty"`
	SortText            string             `json:"sortText,omitempty"`
	FilterText          string             `json:"filterText,omitempty"`
	InsertText          string             `json:"insertText,omitempty"`
	InsertTextFormat    int                `json:"insertTextFormat,omitempty"`
	TextEdit            *TextEdit          `json:"textEdit,omitempty"`
	AdditionalTextEdits []TextEdit         `json:"additionalTextEdits,omitempty"`
	CommitCharacters    []string           `json:"commitCharacters,omitempty"`
	Command             *Command           `json:"command,omitempty"`
	Data                interface{}        `json:"data,omitempty"`
}

// TextEdit represents a text edit
type TextEdit struct {
	Range   Range  `json:"range"`
	NewText string `json:"newText"`
}

// Command represents a command
type Command struct {
	Title     string        `json:"title"`
	Command   string        `json:"command"`
	Arguments []interface{} `json:"arguments,omitempty"`
}

// CompletionList represents a list of completion items
type CompletionList struct {
	IsIncomplete bool             `json:"isIncomplete"`
	Items        []CompletionItem `json:"items"`
}

// TextDocumentIdentifier identifies a text document
type TextDocumentIdentifier struct {
	URI DocumentURI `json:"uri"`
}

// VersionedTextDocumentIdentifier identifies a versioned text document
type VersionedTextDocumentIdentifier struct {
	TextDocumentIdentifier
	Version int `json:"version"`
}

// TextDocumentItem represents a text document item
type TextDocumentItem struct {
	URI        DocumentURI `json:"uri"`
	LanguageID string      `json:"languageId"`
	Version    int         `json:"version"`
	Text       string      `json:"text"`
}

// TextDocumentPositionParams represents parameters for text document position requests
type TextDocumentPositionParams struct {
	TextDocument TextDocumentIdentifier `json:"textDocument"`
	Position     Position               `json:"position"`
}

// HoverParams represents parameters for a hover request
type HoverParams struct {
	TextDocumentPositionParams
}

// DefinitionParams represents parameters for a definition request
type DefinitionParams struct {
	TextDocumentPositionParams
}

// ReferenceContext represents context for a references request
type ReferenceContext struct {
	IncludeDeclaration bool `json:"includeDeclaration"`
}

// ReferenceParams represents parameters for a references request
type ReferenceParams struct {
	TextDocumentPositionParams
	Context ReferenceContext `json:"context"`
}

// CompletionParams represents parameters for a completion request
type CompletionParams struct {
	TextDocumentPositionParams
	Context *CompletionContext `json:"context,omitempty"`
}

// CompletionContext represents the context of a completion request
type CompletionContext struct {
	TriggerKind      int    `json:"triggerKind"`
	TriggerCharacter string `json:"triggerCharacter,omitempty"`
}

// DidOpenTextDocumentParams represents parameters for didOpen notification
type DidOpenTextDocumentParams struct {
	TextDocument TextDocumentItem `json:"textDocument"`
}

// TextDocumentContentChangeEvent represents a text document content change
type TextDocumentContentChangeEvent struct {
	Text string `json:"text"`
}

// DidChangeTextDocumentParams represents parameters for didChange notification
type DidChangeTextDocumentParams struct {
	TextDocument   VersionedTextDocumentIdentifier  `json:"textDocument"`
	ContentChanges []TextDocumentContentChangeEvent `json:"contentChanges"`
}

// DidSaveTextDocumentParams represents parameters for didSave notification
type DidSaveTextDocumentParams struct {
	TextDocument TextDocumentIdentifier `json:"textDocument"`
	Text         *string                `json:"text,omitempty"`
}

// DidCloseTextDocumentParams represents parameters for didClose notification
type DidCloseTextDocumentParams struct {
	TextDocument TextDocumentIdentifier `json:"textDocument"`
}

// InitializeParams represents parameters for the initialize request
type InitializeParams struct {
	ProcessID    int                `json:"processId"`
	RootURI      DocumentURI        `json:"rootUri"`
	RootPath     string             `json:"rootPath,omitempty"`
	Capabilities ClientCapabilities `json:"capabilities"`
	ClientInfo   *ClientInfo        `json:"clientInfo,omitempty"`
	Trace        string             `json:"trace,omitempty"`
}

// ClientInfo represents client information
type ClientInfo struct {
	Name    string `json:"name"`
	Version string `json:"version,omitempty"`
}

// ClientCapabilities represents client capabilities
type ClientCapabilities struct {
	TextDocument TextDocumentClientCapabilities `json:"textDocument,omitempty"`
	Workspace    WorkspaceClientCapabilities    `json:"workspace,omitempty"`
}

// TextDocumentClientCapabilities represents text document client capabilities
type TextDocumentClientCapabilities struct {
	Synchronization *TextDocumentSyncClientCapabilities `json:"synchronization,omitempty"`
	Completion      *CompletionClientCapabilities       `json:"completion,omitempty"`
	Hover           *HoverClientCapabilities            `json:"hover,omitempty"`
	Definition      *DefinitionClientCapabilities       `json:"definition,omitempty"`
	References      *ReferenceClientCapabilities        `json:"references,omitempty"`
	Diagnostic      *DiagnosticClientCapabilities       `json:"diagnostic,omitempty"`
}

// TextDocumentSyncClientCapabilities represents text document sync client capabilities
type TextDocumentSyncClientCapabilities struct {
	DynamicRegistration bool `json:"dynamicRegistration,omitempty"`
	WillSave            bool `json:"willSave,omitempty"`
	WillSaveWaitUntil   bool `json:"willSaveWaitUntil,omitempty"`
	DidSave             bool `json:"didSave,omitempty"`
}

// CompletionClientCapabilities represents completion client capabilities
type CompletionClientCapabilities struct {
	DynamicRegistration bool                              `json:"dynamicRegistration,omitempty"`
	CompletionItem      *CompletionItemClientCapabilities `json:"completionItem,omitempty"`
}

// CompletionItemClientCapabilities represents completion item client capabilities
type CompletionItemClientCapabilities struct {
	SnippetSupport          bool         `json:"snippetSupport,omitempty"`
	CommitCharactersSupport bool         `json:"commitCharactersSupport,omitempty"`
	DocumentationFormat     []MarkupKind `json:"documentationFormat,omitempty"`
}

// HoverClientCapabilities represents hover client capabilities
type HoverClientCapabilities struct {
	DynamicRegistration bool         `json:"dynamicRegistration,omitempty"`
	ContentFormat       []MarkupKind `json:"contentFormat,omitempty"`
}

// DefinitionClientCapabilities represents definition client capabilities
type DefinitionClientCapabilities struct {
	DynamicRegistration bool `json:"dynamicRegistration,omitempty"`
}

// ReferenceClientCapabilities represents reference client capabilities
type ReferenceClientCapabilities struct {
	DynamicRegistration bool `json:"dynamicRegistration,omitempty"`
}

// DiagnosticClientCapabilities represents diagnostic client capabilities
type DiagnosticClientCapabilities struct {
	DynamicRegistration bool `json:"dynamicRegistration,omitempty"`
}

// WorkspaceClientCapabilities represents workspace client capabilities
type WorkspaceClientCapabilities struct {
	DidChangeWatchedFiles *DidChangeWatchedFilesClientCapabilities `json:"didChangeWatchedFiles,omitempty"`
}

// DidChangeWatchedFilesClientCapabilities represents did change watched files client capabilities
type DidChangeWatchedFilesClientCapabilities struct {
	DynamicRegistration bool `json:"dynamicRegistration,omitempty"`
}

// InitializeResult represents the result of an initialize request
type InitializeResult struct {
	Capabilities ServerCapabilities `json:"capabilities"`
}

// ServerCapabilities represents server capabilities
type ServerCapabilities struct {
	TextDocumentSync                 interface{}                      `json:"textDocumentSync,omitempty"` // int or TextDocumentSyncOptions
	CompletionProvider               *CompletionOptions               `json:"completionProvider,omitempty"`
	HoverProvider                    interface{}                      `json:"hoverProvider,omitempty"`      // bool or HoverOptions
	DefinitionProvider               interface{}                      `json:"definitionProvider,omitempty"` // bool or DefinitionOptions
	ReferencesProvider               interface{}                      `json:"referencesProvider,omitempty"` // bool or ReferenceOptions
	DocumentSymbolProvider           interface{}                      `json:"documentSymbolProvider,omitempty"`
	WorkspaceSymbolProvider          interface{}                      `json:"workspaceSymbolProvider,omitempty"`
	DocumentHighlightProvider        interface{}                      `json:"documentHighlightProvider,omitempty"`
	CodeActionProvider               interface{}                      `json:"codeActionProvider,omitempty"`
	CodeLensProvider                 *CodeLensOptions                 `json:"codeLensProvider,omitempty"`
	DocumentFormattingProvider       interface{}                      `json:"documentFormattingProvider,omitempty"`
	DocumentRangeFormattingProvider  interface{}                      `json:"documentRangeFormattingProvider,omitempty"`
	DocumentOnTypeFormattingProvider *DocumentOnTypeFormattingOptions `json:"documentOnTypeFormattingProvider,omitempty"`
	RenameProvider                   interface{}                      `json:"renameProvider,omitempty"`
	DocumentLinkProvider             *DocumentLinkOptions             `json:"documentLinkProvider,omitempty"`
	ExecuteCommandProvider           *ExecuteCommandOptions           `json:"executeCommandProvider,omitempty"`
	Experimental                     interface{}                      `json:"experimental,omitempty"`
}

// CompletionOptions represents completion options
type CompletionOptions struct {
	ResolveProvider   bool     `json:"resolveProvider,omitempty"`
	TriggerCharacters []string `json:"triggerCharacters,omitempty"`
}

// CodeLensOptions represents code lens options
type CodeLensOptions struct {
	ResolveProvider bool `json:"resolveProvider,omitempty"`
}

// DocumentOnTypeFormattingOptions represents document on type formatting options
type DocumentOnTypeFormattingOptions struct {
	FirstTriggerCharacter string   `json:"firstTriggerCharacter"`
	MoreTriggerCharacter  []string `json:"moreTriggerCharacter,omitempty"`
}

// DocumentLinkOptions represents document link options
type DocumentLinkOptions struct {
	ResolveProvider bool `json:"resolveProvider,omitempty"`
}

// ExecuteCommandOptions represents execute command options
type ExecuteCommandOptions struct {
	Commands []string `json:"commands,omitempty"`
}

// ShowMessageParams represents parameters for showMessage notification
type ShowMessageParams struct {
	Type    int    `json:"type"`
	Message string `json:"message"`
}

// LogMessageParams represents parameters for logMessage notification
type LogMessageParams struct {
	Type    int    `json:"type"`
	Message string `json:"message"`
}

// MessageType represents message types
type MessageType int

const (
	MessageTypeError   MessageType = 1
	MessageTypeWarning MessageType = 2
	MessageTypeInfo    MessageType = 3
	MessageTypeLog     MessageType = 4
)

// UnmarshalJSON unmarshals a CompletionItem's Documentation field
func (ci *CompletionItem) UnmarshalJSON(data []byte) error {
	type Alias CompletionItem
	aux := &struct {
		Documentation json.RawMessage `json:"documentation,omitempty"`
		*Alias
	}{
		Alias: (*Alias)(ci),
	}

	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}

	if aux.Documentation != nil {
		// Try to unmarshal as string first
		var docStr string
		if err := json.Unmarshal(aux.Documentation, &docStr); err == nil {
			ci.Documentation = docStr
		} else {
			// Try to unmarshal as MarkupContent
			var docContent MarkupContent
			if err := json.Unmarshal(aux.Documentation, &docContent); err == nil {
				ci.Documentation = docContent
			}
		}
	}

	return nil
}
