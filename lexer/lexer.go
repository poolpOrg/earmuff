// Package lexer turns earmuff v2 source into a stream of tokens.
//
// The lexer is deliberately decoupled from music theory: it recognizes lexical
// *shapes* (words, numbers, strings, operators) and reserved keywords, but it
// does not decide whether a word is a note ("C#") or a chord ("Am7"). That
// classification depends on go-harmony and on grammatical context, so it is left
// to the parser. A word-shaped token is emitted as token.IDENT; its Literal
// holds the exact source text (including accidentals and any slash-chord tail).
package lexer

import (
	"unicode"
	"unicode/utf8"

	"github.com/poolpOrg/earmuff/token"
)

const eof = rune(0)

// Lexer scans a source buffer into tokens, tracking line/column for diagnostics.
type Lexer struct {
	src      string
	filename string

	offset     int // byte offset of the current rune (ch)
	readOffset int // byte offset of the next rune
	ch         rune

	line   int
	column int
}

// New creates a Lexer over src. filename is used only for diagnostics.
func New(src, filename string) *Lexer {
	l := &Lexer{src: src, filename: filename, line: 1, column: 0}
	l.readRune()
	return l
}

func (l *Lexer) readRune() {
	if l.readOffset >= len(l.src) {
		l.offset = len(l.src)
		l.ch = eof
		return
	}
	r, size := utf8.DecodeRuneInString(l.src[l.readOffset:])
	l.offset = l.readOffset
	l.readOffset += size
	if l.ch == '\n' {
		l.line++
		l.column = 1
	} else {
		l.column++
	}
	l.ch = r
}

func (l *Lexer) peek() rune {
	if l.readOffset >= len(l.src) {
		return eof
	}
	r, _ := utf8.DecodeRuneInString(l.src[l.readOffset:])
	return r
}

func (l *Lexer) pos() token.Position {
	return token.Position{Filename: l.filename, Line: l.line, Column: l.column}
}

func isWordStart(ch rune) bool {
	return unicode.IsLetter(ch) || ch == '_'
}

// isWordPart accepts the characters that may appear inside a word token. This is
// broad on purpose: it includes accidentals (#, ^) and digits so that "C#",
// "Am7", and "F#3" lex as a single IDENT. The slash-chord tail ("C7/E") is
// handled separately in scanWord.
func isWordPart(ch rune) bool {
	return unicode.IsLetter(ch) || unicode.IsDigit(ch) || ch == '_' || ch == '#' || ch == '^'
}

func isDigit(ch rune) bool { return ch >= '0' && ch <= '9' }

func isHexDigit(ch rune) bool {
	return isDigit(ch) || (ch >= 'a' && ch <= 'f') || (ch >= 'A' && ch <= 'F')
}

// Next returns the next token. Whitespace and comments are skipped.
func (l *Lexer) Next() token.Token {
	l.skipTrivia()

	pos := l.pos()
	ch := l.ch

	switch {
	case ch == eof:
		return token.Token{Type: token.EOF, Literal: "", Pos: pos}
	case ch == '"' || ch == '\'':
		return l.scanString()
	case isWordStart(ch):
		return l.scanWord()
	case isDigit(ch):
		return l.scanNumber()
	case ch == '.' && l.peek() == '.':
		l.readRune()
		l.readRune()
		return token.Token{Type: token.DOTDOT, Literal: "..", Pos: pos}
	case ch == '.' && isDigit(l.peek()):
		return l.scanNumber()
	}

	switch ch {
	case '{':
		return l.emit(token.LBRACE, pos)
	case '}':
		return l.emit(token.RBRACE, pos)
	case '[':
		return l.emit(token.LBRACKET, pos)
	case ']':
		return l.emit(token.RBRACKET, pos)
	case '(':
		return l.emit(token.LPAREN, pos)
	case ')':
		return l.emit(token.RPAREN, pos)
	case ';':
		return l.emit(token.SEMICOLON, pos)
	case ',':
		return l.emit(token.COMMA, pos)
	case ':':
		return l.emit(token.COLON, pos)
	case '|':
		if l.peek() == '|' {
			l.readRune()
			l.readRune()
			return token.Token{Type: token.OR, Literal: "||", Pos: pos}
		}
		return l.emit(token.BAR_SEP, pos)
	case '~':
		return l.emit(token.TILDE, pos)
	case '@':
		return l.emit(token.AT, pos)
	case '*':
		return l.emit(token.STAR, pos)
	case '/':
		return l.emit(token.SLASH, pos)
	case '+':
		return l.emit(token.PLUS, pos)
	case '-':
		return l.emit(token.MINUS, pos)
	case '=':
		if l.peek() == '=' {
			l.readRune()
			l.readRune()
			return token.Token{Type: token.EQ, Literal: "==", Pos: pos}
		}
		return l.emit(token.ASSIGN, pos)
	case '!':
		if l.peek() == '=' {
			l.readRune()
			l.readRune()
			return token.Token{Type: token.NEQ, Literal: "!=", Pos: pos}
		}
		return l.emit(token.NOT, pos)
	case '<':
		if l.peek() == '=' {
			l.readRune()
			l.readRune()
			return token.Token{Type: token.LTE, Literal: "<=", Pos: pos}
		}
		return l.emit(token.LT, pos)
	case '>':
		if l.peek() == '=' {
			l.readRune()
			l.readRune()
			return token.Token{Type: token.GTE, Literal: ">=", Pos: pos}
		}
		return l.emit(token.GT, pos)
	case '&':
		if l.peek() == '&' {
			l.readRune()
			l.readRune()
			return token.Token{Type: token.AND, Literal: "&&", Pos: pos}
		}
	}

	lit := string(ch)
	l.readRune()
	return token.Token{Type: token.ILLEGAL, Literal: lit, Pos: pos}
}

func (l *Lexer) emit(t token.Type, pos token.Position) token.Token {
	lit := string(l.ch)
	l.readRune()
	return token.Token{Type: t, Literal: lit, Pos: pos}
}

func (l *Lexer) skipTrivia() {
	for {
		switch {
		case l.ch == ' ' || l.ch == '\t' || l.ch == '\n' || l.ch == '\r':
			l.readRune()
		case l.ch == '/' && l.peek() == '/':
			for l.ch != '\n' && l.ch != eof {
				l.readRune()
			}
		case l.ch == '/' && l.peek() == '*':
			l.readRune() // /
			l.readRune() // *
			for !(l.ch == '*' && l.peek() == '/') && l.ch != eof {
				l.readRune()
			}
			if l.ch != eof {
				l.readRune() // *
				l.readRune() // /
			}
		default:
			return
		}
	}
}

func (l *Lexer) scanString() token.Token {
	pos := l.pos()
	quote := l.ch
	l.readRune() // opening quote
	var sb []rune
	for l.ch != quote && l.ch != eof {
		if l.ch == '\\' {
			l.readRune()
			if l.ch == eof {
				break
			}
		}
		sb = append(sb, l.ch)
		l.readRune()
	}
	if l.ch != quote {
		// unterminated string
		return token.Token{Type: token.ILLEGAL, Literal: string(sb), Pos: pos}
	}
	l.readRune() // closing quote
	return token.Token{Type: token.STRING, Literal: string(sb), Pos: pos}
}

func (l *Lexer) scanWord() token.Token {
	pos := l.pos()
	start := l.offset
	for isWordPart(l.ch) {
		l.readRune()
	}
	// allow a single slash-chord tail: C7/E, F7/1  (slash then word/digit)
	if l.ch == '/' && (isWordPart(l.peek()) || isDigit(l.peek())) {
		l.readRune() // '/'
		for isWordPart(l.ch) {
			l.readRune()
		}
	}
	lit := l.src[start:l.offset]

	if t := token.LookupKeyword(lit); t != token.IDENT {
		return token.Token{Type: t, Literal: lit, Pos: pos}
	}
	return token.Token{Type: token.IDENT, Literal: lit, Pos: pos}
}

func (l *Lexer) scanNumber() token.Token {
	pos := l.pos()
	start := l.offset
	isFloat := false
	for isDigit(l.ch) {
		l.readRune()
	}
	if l.ch == '.' && l.peek() != '.' { // a single '.', not a ".." range
		isFloat = true
		l.readRune() // '.'
		for isDigit(l.ch) {
			l.readRune()
		}
	}
	lit := l.src[start:l.offset]
	if isFloat {
		return token.Token{Type: token.FLOAT, Literal: lit, Pos: pos}
	}
	return token.Token{Type: token.NUMBER, Literal: lit, Pos: pos}
}

// ScanHexByte reads a two-hex-digit byte at the current position, used by the
// parser for `sysex` payloads. Returns ok=false if the next token is not a
// standalone hex byte (so the caller can fall back to Next).
func (l *Lexer) ScanHexByte() (token.Token, bool) {
	l.skipTrivia()
	pos := l.pos()
	if !isHexDigit(l.ch) || !isHexDigit(l.peek()) {
		return token.Token{}, false
	}
	r0 := l.ch
	start := l.offset
	l.readRune()
	l.readRune()
	// a hex byte is exactly two digits and not the prefix of a longer word
	if isWordPart(l.ch) && !(isHexDigit(r0)) {
		return token.Token{}, false
	}
	if isWordPart(l.ch) {
		return token.Token{}, false
	}
	return token.Token{Type: token.HEXBYTE, Literal: l.src[start:l.offset], Pos: pos}, true
}

// Tokens lexes the entire source into a slice (terminated by EOF).
func (l *Lexer) Tokens() []token.Token {
	var out []token.Token
	for {
		t := l.Next()
		out = append(out, t)
		if t.Type == token.EOF {
			return out
		}
	}
}
