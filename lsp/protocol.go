// Package lsp implements a Language Server Protocol server for the earmuff v2
// language. It reuses earmuff's own parser and analyzer so editor diagnostics
// stay perfectly in sync with the compiler — no grammar or analysis is
// duplicated here.
//
// The wire protocol is LSP over stdio: Content-Length-framed JSON-RPC 2.0. Only
// the subset earmuff needs is implemented (lifecycle, document sync,
// diagnostics, completion, hover, definition, document symbols).
package lsp

import (
	"encoding/json"
)

// ---------------------------------------------------------------------------
// JSON-RPC 2.0 envelopes
// ---------------------------------------------------------------------------

// requestMessage is an incoming JSON-RPC request or notification. Notifications
// have no ID.
type requestMessage struct {
	JSONRPC string           `json:"jsonrpc"`
	ID      *json.RawMessage `json:"id,omitempty"`
	Method  string           `json:"method"`
	Params  json.RawMessage  `json:"params,omitempty"`
}

// responseMessage is an outgoing JSON-RPC response.
type responseMessage struct {
	JSONRPC string           `json:"jsonrpc"`
	ID      *json.RawMessage `json:"id,omitempty"`
	Result  interface{}      `json:"result,omitempty"`
	Error   *responseError   `json:"error,omitempty"`
}

// notificationMessage is an outgoing JSON-RPC notification (server -> client).
type notificationMessage struct {
	JSONRPC string      `json:"jsonrpc"`
	Method  string      `json:"method"`
	Params  interface{} `json:"params,omitempty"`
}

type responseError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// ---------------------------------------------------------------------------
// LSP types (the subset we use)
// ---------------------------------------------------------------------------

// Position is a zero-based line/character in a document (LSP convention).
type Position struct {
	Line      int `json:"line"`
	Character int `json:"character"`
}

// Range is a span between two positions.
type Range struct {
	Start Position `json:"start"`
	End   Position `json:"end"`
}

// Location is a range within a document URI.
type Location struct {
	URI   string `json:"uri"`
	Range Range  `json:"range"`
}

// DiagnosticSeverity per the LSP spec.
type DiagnosticSeverity int

const (
	SeverityError       DiagnosticSeverity = 1
	SeverityWarning     DiagnosticSeverity = 2
	SeverityInformation DiagnosticSeverity = 3
	SeverityHint        DiagnosticSeverity = 4
)

// Diagnostic is one problem reported on a document.
type Diagnostic struct {
	Range    Range              `json:"range"`
	Severity DiagnosticSeverity `json:"severity"`
	Source   string             `json:"source"`
	Message  string             `json:"message"`
}

type publishDiagnosticsParams struct {
	URI         string       `json:"uri"`
	Diagnostics []Diagnostic `json:"diagnostics"`
}

// --- lifecycle ---

type initializeParams struct {
	RootURI string `json:"rootUri"`
}

type initializeResult struct {
	Capabilities serverCapabilities `json:"capabilities"`
	ServerInfo   serverInfo         `json:"serverInfo"`
}

type serverInfo struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

type serverCapabilities struct {
	TextDocumentSync       int                `json:"textDocumentSync"` // 1 = full
	CompletionProvider     *completionOptions `json:"completionProvider,omitempty"`
	HoverProvider          bool               `json:"hoverProvider"`
	DefinitionProvider     bool               `json:"definitionProvider"`
	DocumentSymbolProvider bool               `json:"documentSymbolProvider"`
}

type completionOptions struct {
	TriggerCharacters []string `json:"triggerCharacters,omitempty"`
}

// --- document sync ---

type textDocumentItem struct {
	URI        string `json:"uri"`
	LanguageID string `json:"languageId"`
	Version    int    `json:"version"`
	Text       string `json:"text"`
}

type didOpenParams struct {
	TextDocument textDocumentItem `json:"textDocument"`
}

type versionedTextDocumentIdentifier struct {
	URI     string `json:"uri"`
	Version int    `json:"version"`
}

type textDocumentContentChangeEvent struct {
	Text string `json:"text"` // full document (we use full sync)
}

type didChangeParams struct {
	TextDocument   versionedTextDocumentIdentifier  `json:"textDocument"`
	ContentChanges []textDocumentContentChangeEvent `json:"contentChanges"`
}

type textDocumentIdentifier struct {
	URI string `json:"uri"`
}

type didCloseParams struct {
	TextDocument textDocumentIdentifier `json:"textDocument"`
}

// --- request params shared by hover/definition/completion ---

type textDocumentPositionParams struct {
	TextDocument textDocumentIdentifier `json:"textDocument"`
	Position     Position               `json:"position"`
}

// --- completion ---

type CompletionItemKind int

const (
	KindKeyword  CompletionItemKind = 14
	KindValue    CompletionItemKind = 12
	KindFunction CompletionItemKind = 3
	KindVariable CompletionItemKind = 6
	KindConstant CompletionItemKind = 21
	KindUnit     CompletionItemKind = 11
)

type CompletionItem struct {
	Label  string             `json:"label"`
	Kind   CompletionItemKind `json:"kind"`
	Detail string             `json:"detail,omitempty"`
	Doc    string             `json:"documentation,omitempty"`
}

// --- hover ---

type Hover struct {
	Contents MarkupContent `json:"contents"`
}

type MarkupContent struct {
	Kind  string `json:"kind"` // "markdown" | "plaintext"
	Value string `json:"value"`
}

// --- document symbols ---

type SymbolKind int

const (
	SymbolModule   SymbolKind = 2
	SymbolClass    SymbolKind = 5
	SymbolFunction SymbolKind = 12
	SymbolVariable SymbolKind = 13
)

type DocumentSymbol struct {
	Name           string           `json:"name"`
	Detail         string           `json:"detail,omitempty"`
	Kind           SymbolKind       `json:"kind"`
	Range          Range            `json:"range"`
	SelectionRange Range            `json:"selectionRange"`
	Children       []DocumentSymbol `json:"children,omitempty"`
}

type documentSymbolParams struct {
	TextDocument textDocumentIdentifier `json:"textDocument"`
}
