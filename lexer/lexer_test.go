package lexer

import (
	"testing"

	"github.com/poolpOrg/earmuff/token"
)

func types(src string) []token.Type {
	l := New(src, "<test>")
	var out []token.Type
	for _, t := range l.Tokens() {
		out = append(out, t.Type)
	}
	return out
}

func TestLexer_Punctuation(t *testing.T) {
	got := types("{ } [ ] ( ) ; , : | @ * / + - .. = == != < <= > >= && || !")
	want := []token.Type{
		token.LBRACE, token.RBRACE, token.LBRACKET, token.RBRACKET,
		token.LPAREN, token.RPAREN, token.SEMICOLON, token.COMMA, token.COLON,
		token.BAR_SEP, token.AT, token.STAR, token.SLASH, token.PLUS, token.MINUS,
		token.DOTDOT, token.ASSIGN, token.EQ, token.NEQ, token.LT, token.LTE,
		token.GT, token.GTE, token.AND, token.OR, token.NOT, token.EOF,
	}
	if len(got) != len(want) {
		t.Fatalf("token count = %d, want %d (%v)", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("token %d = %s, want %s", i, got[i], want[i])
		}
	}
}

func TestLexer_Keywords(t *testing.T) {
	cases := map[string]token.Type{
		"project": token.PROJECT, "track": token.TRACK, "bar": token.BAR,
		"pattern": token.PATTERN, "for": token.FOR, "in": token.IN,
		"if": token.IF, "else": token.ELSE, "let": token.LET,
		"on": token.ON, "cc": token.CC, "bend": token.BEND,
		"sysex": token.SYSEX, "kit": token.KIT, "true": token.TRUE,
	}
	// Note: "beat" is intentionally NOT a keyword (recognized contextually after
	// "on"), so it must lex as IDENT.
	if New("beat", "<t>").Next().Type != token.IDENT {
		t.Fatalf("'beat' should lex as IDENT, not a keyword")
	}
	for src, want := range cases {
		l := New(src, "<test>")
		tok := l.Next()
		if tok.Type != want {
			t.Fatalf("%q lexed as %s, want %s", src, tok.Type, want)
		}
	}
}

func TestLexer_NumbersAndFloats(t *testing.T) {
	l := New("120 3.25 4 .5", "<test>")
	toks := l.Tokens()
	if toks[0].Type != token.NUMBER || toks[0].Literal != "120" {
		t.Fatalf("got %v", toks[0])
	}
	if toks[1].Type != token.FLOAT || toks[1].Literal != "3.25" {
		t.Fatalf("got %v", toks[1])
	}
	if toks[2].Type != token.NUMBER || toks[2].Literal != "4" {
		t.Fatalf("got %v", toks[2])
	}
	if toks[3].Type != token.FLOAT || toks[3].Literal != ".5" {
		t.Fatalf("got %v", toks[3])
	}
}

func TestLexer_RangeVsFloat(t *testing.T) {
	// "1..4" must lex NUMBER DOTDOT NUMBER, not floats
	got := types("1..4")
	want := []token.Type{token.NUMBER, token.DOTDOT, token.NUMBER, token.EOF}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("1..4 token %d = %s, want %s", i, got[i], want[i])
		}
	}
}

func TestLexer_WordsNotesChords(t *testing.T) {
	// note/chord literals lex as IDENT carrying exact text (incl. accidentals
	// and slash-chord tail); classification happens later.
	cases := map[string]string{
		"C": "C", "C#": "C#", "Eb": "Eb", "F#3": "F#3",
		"Am7": "Am7", "C7": "C7", "Gmaj7": "Gmaj7", "C7/E": "C7/E", "F7/1": "F7/1",
		"hh": "hh", "aTune": "aTune",
	}
	for src, lit := range cases {
		l := New(src, "<test>")
		tok := l.Next()
		if tok.Type != token.IDENT || tok.Literal != lit {
			t.Fatalf("%q lexed as %s %q, want IDENT %q", src, tok.Type, tok.Literal, lit)
		}
	}
}

func TestLexer_String(t *testing.T) {
	l := New(`"lead piano" 'single' "esc\"aped"`, "<test>")
	toks := l.Tokens()
	if toks[0].Type != token.STRING || toks[0].Literal != "lead piano" {
		t.Fatalf("got %v", toks[0])
	}
	if toks[1].Type != token.STRING || toks[1].Literal != "single" {
		t.Fatalf("got %v", toks[1])
	}
	if toks[2].Type != token.STRING || toks[2].Literal != `esc"aped` {
		t.Fatalf("got %v", toks[2])
	}
}

func TestLexer_Comments(t *testing.T) {
	got := types("bpm // line comment\n 120 /* block\ncomment */ ;")
	want := []token.Type{token.BPM, token.NUMBER, token.SEMICOLON, token.EOF}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("token %d = %s, want %s", i, got[i], want[i])
		}
	}
}

func TestLexer_UnterminatedString(t *testing.T) {
	l := New(`"oops`, "<test>")
	tok := l.Next()
	if tok.Type != token.ILLEGAL {
		t.Fatalf("unterminated string = %s, want ILLEGAL", tok.Type)
	}
}

func TestLexer_Positions(t *testing.T) {
	l := New("project\n  bpm", "<test>")
	p := l.Next()
	if p.Pos.Line != 1 || p.Pos.Column != 1 {
		t.Fatalf("project at %s, want 1:1", p.Pos)
	}
	b := l.Next()
	if b.Pos.Line != 2 || b.Pos.Column != 3 {
		t.Fatalf("bpm at %s, want 2:3", b.Pos)
	}
}

func TestLexer_StepGridTokens(t *testing.T) {
	// `(oh,sn,cy) hh _*4` style
	got := types("(oh,sn,cy) hh _*4")
	want := []token.Type{
		token.LPAREN, token.IDENT, token.COMMA, token.IDENT, token.COMMA,
		token.IDENT, token.RPAREN, token.IDENT, token.IDENT, token.STAR,
		token.NUMBER, token.EOF,
	}
	if len(got) != len(want) {
		t.Fatalf("count %d want %d: %v", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("token %d = %s want %s", i, got[i], want[i])
		}
	}
}
