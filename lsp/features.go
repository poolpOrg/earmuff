package lsp

import (
	"fmt"
	"strings"

	"github.com/poolpOrg/earmuff/ast"
	"github.com/poolpOrg/earmuff/midi"
	"github.com/poolpOrg/earmuff/parser"
	"github.com/poolpOrg/earmuff/token"
)

// keywords offered by completion, with a one-line doc each.
var keywordDocs = map[string]string{
	"project":    "Top-level container: `project \"name\" { ... }`.",
	"track":      "A part on one channel: `track \"name\" instrument \"...\" { ... }`.",
	"pattern":    "Reusable body: `pattern name(params) { ... }`, called as `name(args)`.",
	"bar":        "A measure: `bar quarter { C E G _ }`. The duration sets the step grid.",
	"instrument": "Track header clause selecting a General MIDI instrument.",
	"channel":    "Track header clause setting the MIDI channel (1..16; 10 = drums).",
	"port":       "Track header clause selecting an output port.",
	"kit":        "Percussion aliases: `kit { hh = \"closed hi-hat\"; }`.",
	"bpm":        "Tempo in beats per minute: `bpm 120;`.",
	"time":       "Time signature: `time 4 4;`.",
	"copyright":  "Project copyright meta text.",
	"text":       "A text meta event.",
	"lyric":      "A lyric meta event.",
	"marker":     "A marker meta event.",
	"cue":        "A cue-point meta event.",
	"for":        "Bounded loop: `for i in 1..4 { ... }` (bound) or `for each 1..4 { ... }` (unbound). Iterates a range, bare sequence (`C E G`), or list.",
	"each":       "Marks an unbound loop: `for each 1..12 { ... }` iterates with no variable.",
	"in":         "Separates the loop variable from its range/list/sequence in a `for`.",
	"if":         "Elaboration-time conditional: `if cond { ... } else { ... }`.",
	"else":       "Alternative branch of an `if`.",
	"let":        "Immutable binding: `let changes = [Am7, D7, Gmaj7];`.",
	"on":         "Absolute placement: `on beat 2.5 play ...` (escape hatch).",
	"beat":       "Used after `on` to place an event at an absolute beat.",
	"cc":         "Control change: `cc 74 = 64`.",
	"bend":       "Pitch bend in semitones: `bend +2` (auto RPN range). Also `bend raw N`.",
	"pressure":   "Channel aftertouch: `pressure 90`.",
	"program":    "Program (patch) change: `program \"violin\";`.",
	"sysex":      "Raw system-exclusive bytes: `sysex F0 7E 7F 09 01 F7;`.",
}

var durationWords = []string{"whole", "half", "quarter", "eighth", "sixteenth", "thirtysecond", "sixtyfourth"}
var dynamicWords = []string{"ppp", "pp", "p", "mp", "mf", "f", "ff", "fff"}
var intervalWords = []string{"min2", "maj2", "min3", "maj3", "fourth", "fifth", "min6", "maj6", "min7", "maj7", "octave"}

// completion offers keywords, durations, dynamics, intervals, GM instruments,
// and percussion names. It is context-light by design: it returns the full
// vocabulary and lets the editor filter by the typed prefix.
func (s *Server) completion(p textDocumentPositionParams) []CompletionItem {
	var items []CompletionItem

	for kw, doc := range keywordDocs {
		items = append(items, CompletionItem{Label: kw, Kind: KindKeyword, Doc: doc})
	}
	for _, d := range durationWords {
		items = append(items, CompletionItem{Label: d, Kind: KindUnit, Detail: "duration"})
	}
	for _, d := range dynamicWords {
		items = append(items, CompletionItem{Label: d, Kind: KindConstant, Detail: "dynamic"})
	}
	for _, iv := range intervalWords {
		items = append(items, CompletionItem{Label: iv, Kind: KindConstant, Detail: "interval"})
	}
	for _, name := range midi.GetInstruments() {
		items = append(items, CompletionItem{Label: name, Kind: KindValue, Detail: "GM instrument"})
	}
	for _, name := range midi.GetPercussions() {
		items = append(items, CompletionItem{Label: name, Kind: KindValue, Detail: "percussion"})
	}

	// Also offer patterns and lets visible anywhere in the document (cheap and
	// usually correct for this small language).
	if text, ok := s.doc(p.TextDocument.URI); ok {
		prog, _ := parser.New(text, p.TextDocument.URI).Parse()
		for _, sym := range collectDefs(prog) {
			kind := KindFunction
			if sym.kind == defLet {
				kind = KindVariable
			}
			items = append(items, CompletionItem{Label: sym.name, Kind: kind, Detail: sym.detail})
		}
	}
	return items
}

// hover describes the word under the cursor: a keyword's doc, an instrument's
// program number, a percussion key, or a note/chord's MIDI/quality.
func (s *Server) hover(p textDocumentPositionParams) *Hover {
	text, ok := s.doc(p.TextDocument.URI)
	if !ok {
		return nil
	}
	word := wordAt(text, p.Position)
	if word == "" {
		return nil
	}

	if doc, ok := keywordDocs[word]; ok {
		return md(fmt.Sprintf("**%s** — %s", word, doc))
	}
	// instrument (exact, case-insensitive handled by midi)
	if pc, err := midi.InstrumentToPC(word); err == nil {
		return md(fmt.Sprintf("**instrument** `%s` — GM program %d", word, pc))
	}
	if key, err := midi.PercussionKeyMap(word); err == nil {
		return md(fmt.Sprintf("**percussion** `%s` — MIDI key %d", word, key))
	}
	for _, d := range durationWords {
		if word == d {
			return md(fmt.Sprintf("**duration** `%s`", word))
		}
	}
	// a note or chord
	if info := describePitch(word); info != "" {
		return md(info)
	}
	// a pattern/let definition in this document
	if text, ok := s.doc(p.TextDocument.URI); ok {
		prog, _ := parser.New(text, p.TextDocument.URI).Parse()
		for _, sym := range collectDefs(prog) {
			if sym.name == word {
				return md(fmt.Sprintf("**%s** %s", sym.kindLabel(), sym.detail))
			}
		}
	}
	return nil
}

// definition jumps to the pattern/let definition named by the word under the
// cursor.
func (s *Server) definition(p textDocumentPositionParams) []Location {
	text, ok := s.doc(p.TextDocument.URI)
	if !ok {
		return nil
	}
	word := wordAt(text, p.Position)
	if word == "" {
		return nil
	}
	prog, _ := parser.New(text, p.TextDocument.URI).Parse()
	for _, sym := range collectDefs(prog) {
		if sym.name == word {
			return []Location{{
				URI:   p.TextDocument.URI,
				Range: rangeAt(sym.pos, text),
			}}
		}
	}
	return nil
}

// documentSymbols builds the outline: projects -> (tracks, patterns); tracks ->
// nested patterns.
func (s *Server) documentSymbols(uri string) []DocumentSymbol {
	text, ok := s.doc(uri)
	if !ok {
		return nil
	}
	prog, _ := parser.New(text, uri).Parse()
	if prog == nil {
		return nil
	}
	var syms []DocumentSymbol
	for _, it := range prog.Items {
		switch n := it.(type) {
		case *ast.Project:
			proj := DocumentSymbol{
				Name:           n.Name,
				Detail:         "project",
				Kind:           SymbolModule,
				Range:          rangeAt(n.Position, text),
				SelectionRange: rangeAt(n.Position, text),
			}
			for _, pd := range n.Patterns {
				proj.Children = append(proj.Children, patternSymbol(pd, text))
			}
			for _, tr := range n.Tracks {
				proj.Children = append(proj.Children, trackSymbol(tr, text))
			}
			syms = append(syms, proj)
		case *ast.PatternDef:
			syms = append(syms, patternSymbol(n, text))
		}
	}
	return syms
}

func trackSymbol(tr *ast.Track, text string) DocumentSymbol {
	sym := DocumentSymbol{
		Name:           tr.Name,
		Detail:         "track",
		Kind:           SymbolClass,
		Range:          rangeAt(tr.Position, text),
		SelectionRange: rangeAt(tr.Position, text),
	}
	for _, st := range tr.Body {
		if pd, ok := st.(*ast.PatternDef); ok {
			sym.Children = append(sym.Children, patternSymbol(pd, text))
		}
	}
	return sym
}

func patternSymbol(pd *ast.PatternDef, text string) DocumentSymbol {
	detail := "pattern"
	if len(pd.Params) > 0 {
		detail = "pattern(" + strings.Join(pd.Params, ", ") + ")"
	}
	return DocumentSymbol{
		Name:           pd.Name,
		Detail:         detail,
		Kind:           SymbolFunction,
		Range:          rangeAt(pd.Position, text),
		SelectionRange: rangeAt(pd.Position, text),
	}
}

// --- definition/symbol collection helpers ---

type defKind int

const (
	defPattern defKind = iota
	defLet
)

type defSym struct {
	name   string
	kind   defKind
	pos    token.Position
	detail string
}

func (d defSym) kindLabel() string {
	if d.kind == defLet {
		return "binding"
	}
	return "pattern"
}

// collectDefs walks the whole program for pattern and let definitions.
func collectDefs(prog *ast.Program) []defSym {
	if prog == nil {
		return nil
	}
	var out []defSym
	var walkStmts func(stmts []ast.Stmt)
	addPattern := func(pd *ast.PatternDef) {
		detail := "()"
		if len(pd.Params) > 0 {
			detail = "(" + strings.Join(pd.Params, ", ") + ")"
		}
		out = append(out, defSym{name: pd.Name, kind: defPattern, pos: pd.Position, detail: detail})
	}
	walkStmts = func(stmts []ast.Stmt) {
		for _, st := range stmts {
			switch n := st.(type) {
			case *ast.PatternDef:
				addPattern(n)
				walkStmts(n.Body)
			case *ast.Let:
				out = append(out, defSym{name: n.Name, kind: defLet, pos: n.Position, detail: "= ..."})
			case *ast.For:
				walkStmts(n.Body)
			case *ast.If:
				walkStmts(n.Then)
				walkStmts(n.Else)
				if n.ElseIf != nil {
					walkStmts([]ast.Stmt{n.ElseIf})
				}
			}
		}
	}
	for _, it := range prog.Items {
		switch n := it.(type) {
		case *ast.Project:
			for _, pd := range n.Patterns {
				addPattern(pd)
				walkStmts(pd.Body)
			}
			for _, tr := range n.Tracks {
				walkStmts(tr.Body)
			}
		case *ast.PatternDef:
			addPattern(n)
			walkStmts(n.Body)
		}
	}
	return out
}

// --- small helpers ---

func md(s string) *Hover {
	return &Hover{Contents: MarkupContent{Kind: "markdown", Value: s}}
}

// wordAt returns the word-like token at an LSP position (0-based).
func wordAt(text string, pos Position) string {
	lines := strings.Split(text, "\n")
	if pos.Line < 0 || pos.Line >= len(lines) {
		return ""
	}
	line := lines[pos.Line]
	if pos.Character < 0 || pos.Character > len(line) {
		return ""
	}
	isWord := func(b byte) bool {
		return b == '_' || b == '#' || b == '/' ||
			(b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z') || (b >= '0' && b <= '9')
	}
	start := pos.Character
	for start > 0 && isWord(line[start-1]) {
		start--
	}
	end := pos.Character
	for end < len(line) && isWord(line[end]) {
		end++
	}
	if start >= end {
		return ""
	}
	return line[start:end]
}

// describePitch returns a hover string if word parses as a note or chord.
func describePitch(word string) string {
	if n, err := notesParse(word); err == nil {
		return fmt.Sprintf("**note** `%s` — MIDI %d", word, n)
	}
	if pitches, name, err := chordParse(word); err == nil {
		return fmt.Sprintf("**chord** `%s` (%s) — MIDI %v", word, name, pitches)
	}
	return ""
}
