package lsp

import (
	"strings"
	"testing"
)

const goodSrc = `project "p" {
	bpm 120; time 4 4;
	track "lead" instrument "piano" {
		pattern riff(root) { bar quarter { root root root _ } }
		riff(C)
		let changes = [Am7, Dm7];
		for ch in changes { bar quarter { ch ch ch ch } }
	}
}`

func TestDiagnose_CleanProgram(t *testing.T) {
	diags := Diagnose(goodSrc, "file:///t.ear")
	if len(diags) != 0 {
		for _, d := range diags {
			t.Errorf("unexpected diagnostic: %v %q", d.Range.Start, d.Message)
		}
		t.Fatalf("clean program produced %d diagnostics", len(diags))
	}
}

func TestDiagnose_ParseError(t *testing.T) {
	diags := Diagnose(`project "p" { track "t" { bar quarter { C @ } } }`, "file:///t.ear")
	if len(diags) == 0 {
		t.Fatalf("expected a parse diagnostic")
	}
	if diags[0].Severity != SeverityError {
		t.Fatalf("parse diagnostic severity = %d, want Error(1)", diags[0].Severity)
	}
}

func TestDiagnose_AnalyzerWarningAndError(t *testing.T) {
	// unknown instrument is an analyzer error; undefined pattern call too.
	diags := Diagnose(`project "p" { track "t" instrument "wombat" { nope() } }`, "file:///t.ear")
	var sawErr bool
	for _, d := range diags {
		if d.Severity == SeverityError {
			sawErr = true
		}
	}
	if !sawErr {
		t.Fatalf("expected an analyzer error among %d diagnostics", len(diags))
	}
}

func newTestServer(uri, text string) *Server {
	s := NewServer(strings.NewReader(""), &strings.Builder{})
	s.setDoc(uri, text)
	return s
}

func TestCompletion_OffersVocabulary(t *testing.T) {
	s := newTestServer("file:///t.ear", goodSrc)
	items := s.completion(textDocumentPositionParams{
		TextDocument: textDocumentIdentifier{URI: "file:///t.ear"},
	})
	has := func(label string) bool {
		for _, it := range items {
			if it.Label == label {
				return true
			}
		}
		return false
	}
	for _, want := range []string{"project", "track", "quarter", "mf", "fifth", "riff"} {
		if !has(want) {
			t.Errorf("completion missing %q", want)
		}
	}
	// a known GM instrument and percussion should be present
	if !has("acoustic grand piano") {
		t.Errorf("completion missing GM instrument")
	}
}

func TestHover_Keyword(t *testing.T) {
	s := newTestServer("file:///t.ear", goodSrc)
	// hover over "track" on line 3 (0-based line 2)
	h := s.hover(textDocumentPositionParams{
		TextDocument: textDocumentIdentifier{URI: "file:///t.ear"},
		Position:     Position{Line: 2, Character: 2}, // on "track"
	})
	if h == nil || !strings.Contains(h.Contents.Value, "track") {
		t.Fatalf("hover on 'track' = %v", h)
	}
}

func TestHover_Note(t *testing.T) {
	s := newTestServer("file:///n.ear", `project "p" { track "t" instrument "piano" { bar quarter { C E G _ } } }`)
	// "C" is at the position after "{ " inside the bar; find it
	text, _ := s.doc("file:///n.ear")
	idx := strings.Index(text, "{ C ")
	col := idx + 2
	h := s.hover(textDocumentPositionParams{
		TextDocument: textDocumentIdentifier{URI: "file:///n.ear"},
		Position:     Position{Line: 0, Character: col},
	})
	if h == nil || !strings.Contains(h.Contents.Value, "note") {
		t.Fatalf("hover on note C = %v", h)
	}
}

func TestDefinition_Pattern(t *testing.T) {
	s := newTestServer("file:///t.ear", goodSrc)
	// "riff(C)" call is on line 5 (0-based 4). Point at the call name.
	text, _ := s.doc("file:///t.ear")
	lines := strings.Split(text, "\n")
	callLine := -1
	for i, ln := range lines {
		if strings.Contains(ln, "riff(C)") {
			callLine = i
			break
		}
	}
	if callLine < 0 {
		t.Fatalf("test setup: riff(C) not found")
	}
	col := strings.Index(lines[callLine], "riff") + 1
	locs := s.definition(textDocumentPositionParams{
		TextDocument: textDocumentIdentifier{URI: "file:///t.ear"},
		Position:     Position{Line: callLine, Character: col},
	})
	if len(locs) != 1 {
		t.Fatalf("definition of riff = %d locations, want 1", len(locs))
	}
	// the definition is on the `pattern riff(...)` line (line index 3)
	if locs[0].Range.Start.Line != 3 {
		t.Errorf("definition at line %d, want 3", locs[0].Range.Start.Line)
	}
}

func TestDocumentSymbols(t *testing.T) {
	s := newTestServer("file:///t.ear", goodSrc)
	syms := s.documentSymbols("file:///t.ear")
	if len(syms) != 1 {
		t.Fatalf("got %d top-level symbols, want 1 project", len(syms))
	}
	proj := syms[0]
	if proj.Name != "p" || proj.Kind != SymbolModule {
		t.Fatalf("top symbol = %q kind %d, want project 'p'", proj.Name, proj.Kind)
	}
	// project should contain the track
	var sawTrack bool
	for _, c := range proj.Children {
		if c.Name == "lead" && c.Kind == SymbolClass {
			sawTrack = true
			// track should contain the riff pattern
			for _, cc := range c.Children {
				if cc.Name == "riff" && cc.Kind == SymbolFunction {
					return
				}
			}
		}
	}
	if !sawTrack {
		t.Fatalf("project children missing track 'lead': %+v", proj.Children)
	}
	t.Fatalf("track 'lead' missing nested pattern 'riff'")
}
