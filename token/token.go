// Package token defines the lexical tokens of the earmuff v2 language and the
// source positions used by the lexer, parser, and diagnostics.
package token

import "fmt"

// Type enumerates the kinds of tokens produced by the lexer.
type Type int

const (
	ILLEGAL Type = iota
	EOF

	// literals
	IDENT  // foo, hh, aTune
	NUMBER // 120, 4
	FLOAT  // 3.25, 1.5
	STRING // "lead piano"
	NOTE   // C, C#, Eb, F#3, Cbb   (recognized as a pitch literal)
	CHORD  // Am7, C7, Gmaj7, C7/E  (recognized as a chord literal)
	HEXBYTE

	// structural keywords
	PROJECT
	TRACK
	BAR
	PATTERN
	KIT
	INSTRUMENT
	CHANNEL
	PORT

	// settings / meta keywords
	BPM
	TIME
	COPYRIGHT
	TEXT
	LYRIC
	MARKER
	CUE

	// control flow
	FOR
	IN
	IF
	ELSE
	LET
	REPEAT

	// arrangement
	SECTION
	SWING

	// placement
	ON
	BEAT

	// raw MIDI events
	CC
	BEND
	RAW
	RANGE
	PRESSURE
	PROGRAM
	SYSEX

	// pattern composition operators
	THEN
	OVER

	// literals: booleans
	TRUE
	FALSE

	// punctuation / operators
	LBRACE   // {
	RBRACE   // }
	LBRACKET // [
	RBRACKET // ]
	LPAREN   // (
	RPAREN   // )
	SEMICOLON
	COMMA
	COLON   // :
	BAR_SEP // |
	TILDE   // ~
	AT      // @
	STAR    // *
	SLASH   // /
	PLUS    // +
	MINUS   // -
	DOTDOT  // ..
	ASSIGN  // =
	EQ      // ==
	NEQ     // !=
	LT      // <
	LTE     // <=
	GT      // >
	GTE     // >=
	AND     // &&
	OR      // ||
	NOT     // !
	VELO    // v  (velocity sigil)
)

// Position is a 1-based line/column location in the source, plus the source
// name (file path or "<input>").
type Position struct {
	Filename string
	Line     int
	Column   int
}

func (p Position) String() string {
	if p.Filename != "" {
		return fmt.Sprintf("%s:%d:%d", p.Filename, p.Line, p.Column)
	}
	return fmt.Sprintf("%d:%d", p.Line, p.Column)
}

// Token is a lexed token: its kind, the exact source text, and where it began.
type Token struct {
	Type    Type
	Literal string
	Pos     Position
}

// keywords maps reserved words (lowercased) to their token type. Note and chord
// literals are NOT keywords — they are recognized structurally by the lexer.
var keywords = map[string]Type{
	"project":    PROJECT,
	"track":      TRACK,
	"bar":        BAR,
	"pattern":    PATTERN,
	"kit":        KIT,
	"instrument": INSTRUMENT,
	"channel":    CHANNEL,
	"port":       PORT,

	"bpm":       BPM,
	"time":      TIME,
	"copyright": COPYRIGHT,
	"text":      TEXT,
	"lyric":     LYRIC,
	"marker":    MARKER,
	"cue":       CUE,

	"for":    FOR,
	"in":     IN,
	"if":     IF,
	"else":   ELSE,
	"let":    LET,
	"repeat": REPEAT,

	"section": SECTION,
	"swing":   SWING,

	"on": ON,
	// NOTE: "beat" is intentionally NOT a reserved keyword so it can be used as
	// a pattern/binding name; `on beat` recognizes it contextually as an IDENT.

	"cc":       CC,
	"bend":     BEND,
	"raw":      RAW,
	"range":    RANGE,
	"pressure": PRESSURE,
	"program":  PROGRAM,
	"sysex":    SYSEX,

	"then": THEN,
	"over": OVER,

	"true":  TRUE,
	"false": FALSE,
}

// LookupKeyword returns the keyword token type for an identifier, or IDENT if it
// is not a reserved word.
func LookupKeyword(ident string) Type {
	if t, ok := keywords[ident]; ok {
		return t
	}
	return IDENT
}

// IsKeyword reports whether ident is a reserved word.
func IsKeyword(ident string) bool {
	_, ok := keywords[ident]
	return ok
}

var typeNames = map[Type]string{
	ILLEGAL: "ILLEGAL", EOF: "EOF",
	IDENT: "IDENT", NUMBER: "NUMBER", FLOAT: "FLOAT", STRING: "STRING",
	NOTE: "NOTE", CHORD: "CHORD", HEXBYTE: "HEXBYTE",
	PROJECT: "project", TRACK: "track", BAR: "bar", PATTERN: "pattern",
	KIT: "kit", INSTRUMENT: "instrument", CHANNEL: "channel", PORT: "port",
	BPM: "bpm", TIME: "time", COPYRIGHT: "copyright", TEXT: "text",
	LYRIC: "lyric", MARKER: "marker", CUE: "cue",
	FOR: "for", IN: "in", IF: "if", ELSE: "else", LET: "let", REPEAT: "repeat",
	SECTION: "section", SWING: "swing",
	ON: "on", BEAT: "beat",
	CC: "cc", BEND: "bend", RAW: "raw", RANGE: "range", PRESSURE: "pressure",
	PROGRAM: "program", SYSEX: "sysex", THEN: "then", OVER: "over",
	TRUE: "true", FALSE: "false",
	LBRACE: "{", RBRACE: "}", LBRACKET: "[", RBRACKET: "]",
	LPAREN: "(", RPAREN: ")", SEMICOLON: ";", COMMA: ",", COLON: ":",
	BAR_SEP: "|", TILDE: "~", AT: "@", STAR: "*", SLASH: "/", PLUS: "+", MINUS: "-",
	DOTDOT: "..", ASSIGN: "=", EQ: "==", NEQ: "!=", LT: "<", LTE: "<=",
	GT: ">", GTE: ">=", AND: "&&", OR: "||", NOT: "!", VELO: "v",
}

func (t Type) String() string {
	if s, ok := typeNames[t]; ok {
		return s
	}
	return fmt.Sprintf("Type(%d)", int(t))
}
