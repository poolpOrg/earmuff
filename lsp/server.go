package lsp

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"strings"
	"sync"

	"github.com/poolpOrg/earmuff/analyzer"
	"github.com/poolpOrg/earmuff/parser"
	"github.com/poolpOrg/earmuff/token"
)

// Server is an LSP server speaking JSON-RPC over an io.Reader/io.Writer pair
// (normally stdin/stdout).
type Server struct {
	r *bufio.Reader
	w io.Writer

	mu   sync.Mutex
	docs map[string]string // uri -> full text

	wmu sync.Mutex // serializes writes to w
}

// NewServer creates a Server reading from r and writing to w.
func NewServer(r io.Reader, w io.Writer) *Server {
	return &Server{
		r:    bufio.NewReader(r),
		w:    w,
		docs: map[string]string{},
	}
}

// Run processes messages until EOF or an explicit exit.
func (s *Server) Run() error {
	for {
		msg, err := s.readMessage()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
		var req requestMessage
		if err := json.Unmarshal(msg, &req); err != nil {
			continue // ignore malformed frames
		}
		if s.dispatch(&req) {
			return nil // exit requested
		}
	}
}

// readMessage reads one Content-Length-framed JSON-RPC payload.
func (s *Server) readMessage() ([]byte, error) {
	length := 0
	for {
		line, err := s.r.ReadString('\n')
		if err != nil {
			return nil, err
		}
		line = strings.TrimRight(line, "\r\n")
		if line == "" {
			break // end of headers
		}
		if strings.HasPrefix(line, "Content-Length:") {
			cl := strings.TrimPrefix(line, "Content-Length:")
			n, err := strconv.Atoi(strings.TrimSpace(cl))
			if err == nil {
				length = n
			}
		}
	}
	if length == 0 {
		return nil, fmt.Errorf("missing Content-Length")
	}
	buf := make([]byte, length)
	if _, err := io.ReadFull(s.r, buf); err != nil {
		return nil, err
	}
	return buf, nil
}

// write sends a framed JSON payload.
func (s *Server) write(v interface{}) {
	data, err := json.Marshal(v)
	if err != nil {
		return
	}
	s.wmu.Lock()
	defer s.wmu.Unlock()
	fmt.Fprintf(s.w, "Content-Length: %d\r\n\r\n", len(data))
	s.w.Write(data)
}

func (s *Server) respond(id *json.RawMessage, result interface{}) {
	s.write(responseMessage{JSONRPC: "2.0", ID: id, Result: result})
}

func (s *Server) notify(method string, params interface{}) {
	s.write(notificationMessage{JSONRPC: "2.0", Method: method, Params: params})
}

// dispatch routes one message. It returns true when the client asked to exit.
func (s *Server) dispatch(req *requestMessage) (exit bool) {
	switch req.Method {
	case "initialize":
		s.respond(req.ID, initializeResult{
			Capabilities: serverCapabilities{
				TextDocumentSync:       1, // full document sync
				CompletionProvider:     &completionOptions{TriggerCharacters: []string{" "}},
				HoverProvider:          true,
				DefinitionProvider:     true,
				DocumentSymbolProvider: true,
			},
			ServerInfo: serverInfo{Name: "earmuff-lsp", Version: "0.1.0"},
		})
	case "initialized":
		// no-op
	case "shutdown":
		s.respond(req.ID, nil)
	case "exit":
		return true

	case "textDocument/didOpen":
		var p didOpenParams
		if json.Unmarshal(req.Params, &p) == nil {
			s.setDoc(p.TextDocument.URI, p.TextDocument.Text)
			s.publishDiagnostics(p.TextDocument.URI)
		}
	case "textDocument/didChange":
		var p didChangeParams
		if json.Unmarshal(req.Params, &p) == nil && len(p.ContentChanges) > 0 {
			// full sync: last change holds the whole document
			s.setDoc(p.TextDocument.URI, p.ContentChanges[len(p.ContentChanges)-1].Text)
			s.publishDiagnostics(p.TextDocument.URI)
		}
	case "textDocument/didClose":
		var p didCloseParams
		if json.Unmarshal(req.Params, &p) == nil {
			s.delDoc(p.TextDocument.URI)
			// clear diagnostics for the closed document
			s.notify("textDocument/publishDiagnostics", publishDiagnosticsParams{
				URI: p.TextDocument.URI, Diagnostics: []Diagnostic{},
			})
		}

	case "textDocument/completion":
		var p textDocumentPositionParams
		_ = json.Unmarshal(req.Params, &p)
		s.respond(req.ID, s.completion(p))
	case "textDocument/hover":
		var p textDocumentPositionParams
		_ = json.Unmarshal(req.Params, &p)
		s.respond(req.ID, s.hover(p))
	case "textDocument/definition":
		var p textDocumentPositionParams
		_ = json.Unmarshal(req.Params, &p)
		s.respond(req.ID, s.definition(p))
	case "textDocument/documentSymbol":
		var p documentSymbolParams
		_ = json.Unmarshal(req.Params, &p)
		s.respond(req.ID, s.documentSymbols(p.TextDocument.URI))

	default:
		// Unknown request: reply with null so the client isn't left waiting.
		if req.ID != nil {
			s.respond(req.ID, nil)
		}
	}
	return false
}

// --- document store ---

func (s *Server) setDoc(uri, text string) {
	s.mu.Lock()
	s.docs[uri] = text
	s.mu.Unlock()
}

func (s *Server) delDoc(uri string) {
	s.mu.Lock()
	delete(s.docs, uri)
	s.mu.Unlock()
}

func (s *Server) doc(uri string) (string, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	t, ok := s.docs[uri]
	return t, ok
}

// --- diagnostics ---

// publishDiagnostics parses + analyzes the document and pushes diagnostics.
func (s *Server) publishDiagnostics(uri string) {
	text, ok := s.doc(uri)
	if !ok {
		return
	}
	diags := Diagnose(text, uri)
	s.notify("textDocument/publishDiagnostics", publishDiagnosticsParams{
		URI:         uri,
		Diagnostics: diags,
	})
}

// Diagnose runs the parser and analyzer over src and converts their findings to
// LSP diagnostics. Exported so it can be unit-tested without the wire layer.
func Diagnose(src, uri string) []Diagnostic {
	out := []Diagnostic{}

	p := parser.New(src, uri)
	prog, parseDiags := p.Parse()
	for _, d := range parseDiags {
		out = append(out, Diagnostic{
			Range:    rangeAt(d.Pos, src),
			Severity: SeverityError,
			Source:   "earmuff",
			Message:  d.Msg,
		})
	}

	// Only run the analyzer when parsing produced a tree; a partial tree after
	// parse errors can still be analyzed, but we avoid double-reporting the same
	// spot — the analyzer adds semantic findings on top.
	if prog != nil {
		for _, d := range analyzer.Analyze(prog) {
			sev := SeverityError
			if d.Severity == analyzer.Warning {
				sev = SeverityWarning
			}
			out = append(out, Diagnostic{
				Range:    rangeAt(d.Pos, src),
				Severity: sev,
				Source:   "earmuff",
				Message:  d.Msg,
			})
		}
	}
	return out
}

// rangeAt converts a 1-based token.Position into an LSP range covering the word
// (or single character) at that position.
func rangeAt(pos token.Position, src string) Range {
	line := pos.Line - 1
	col := pos.Column - 1
	if line < 0 {
		line = 0
	}
	if col < 0 {
		col = 0
	}
	start := Position{Line: line, Character: col}
	// extend to the end of the token (word-like run) for a useful squiggle
	end := Position{Line: line, Character: col + tokenWidth(src, pos)}
	return Range{Start: start, End: end}
}

// tokenWidth returns the length of the word-like run at pos, or 1.
func tokenWidth(src string, pos token.Position) int {
	lines := strings.Split(src, "\n")
	if pos.Line-1 < 0 || pos.Line-1 >= len(lines) {
		return 1
	}
	line := lines[pos.Line-1]
	col := pos.Column - 1
	if col < 0 || col >= len(line) {
		return 1
	}
	n := 0
	for col+n < len(line) {
		c := line[col+n]
		if c == ' ' || c == '\t' || c == '{' || c == '}' || c == '(' || c == ')' ||
			c == ';' || c == ',' || c == '|' {
			break
		}
		n++
	}
	if n == 0 {
		return 1
	}
	return n
}
