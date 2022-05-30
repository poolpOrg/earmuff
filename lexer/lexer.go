package lexer

import (
	"bufio"
	"bytes"
	"io"
	"strings"
)

type Token int

const (
	ILLEGAL Token = iota
	EOF
	WHITESPACE

	IDENTIFIER
	NUMBER
	FLOAT
	SEMICOLON
	BRACKET_OPEN
	BRACKET_CLOSE
	PLUS
	MINUS
	SLASH

	BPM
	TIME
	PROJECT
	TRACK
	BAR
	BEAT

	WHOLE
	HALF
	QUARTER
	TH
	ND

	ON

	REST
	NOTE
	INTERVAL
	CHORD
)

func isWhitespace(ch rune) bool {
	return ch == ' ' || ch == '\t' || ch == '\n'
}

func isLetter(ch rune) bool {
	return (ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z')
}

func isDigit(ch rune) bool {
	return (ch >= '0' && ch <= '9')
}

var eof = rune(0)

type Scanner struct {
	r *bufio.Reader
}

func NewScanner(r io.Reader) *Scanner {
	return &Scanner{r: bufio.NewReader(r)}
}

func (s *Scanner) read() rune {
	ch, _, err := s.r.ReadRune()
	if err != nil {
		return eof
	}
	return ch
}

func (s *Scanner) unread() { _ = s.r.UnreadRune() }

func (s *Scanner) Scan() (tok Token, lit string) {
	ch := s.read()

	// If we see whitespace then consume all contiguous whitespace.
	// If we see a letter then consume as an ident or reserved word.
	if isWhitespace(ch) {
		s.unread()
		return s.scanWhitespace()
	} else if isDigit(ch) || ch == '.' {
		s.unread()
		return s.scanNumber()
	} else if isLetter(ch) {
		s.unread()
		return s.scanIdent()
	}

	// Otherwise read the individual character.
	switch ch {
	case eof:
		return EOF, ""
	case '/':
		return SLASH, string(ch)
	case '{':
		return BRACKET_OPEN, string(ch)
	case '}':
		return BRACKET_CLOSE, string(ch)
	case ';':
		return SEMICOLON, string(ch)

	}

	return ILLEGAL, string(ch)
}

// scanWhitespace consumes the current rune and all contiguous whitespace.
func (s *Scanner) scanWhitespace() (tok Token, lit string) {
	// Create a buffer and read the current character into it.
	var buf bytes.Buffer
	buf.WriteRune(s.read())

	// Read every subsequent whitespace character into the buffer.
	// Non-whitespace characters and EOF will cause the loop to exit.
	for {
		if ch := s.read(); ch == eof {
			break
		} else if !isWhitespace(ch) {
			s.unread()
			break
		} else {
			buf.WriteRune(ch)
		}
	}

	return WHITESPACE, buf.String()
}

func (s *Scanner) scanIdent() (tok Token, lit string) {
	// Create a buffer and read the current character into it.
	var buf bytes.Buffer
	buf.WriteRune(s.read())

	// Read every subsequent ident character into the buffer.
	// Non-ident characters and EOF will cause the loop to exit.
	for {
		if ch := s.read(); ch == eof {
			break
		} else if !isLetter(ch) && !isDigit(ch) && ch != '_' && ch != '#' && ch != '^' {
			s.unread()
			break
		} else {
			_, _ = buf.WriteRune(ch)
		}
	}

	// If the string matches a keyword then return that keyword.
	switch strings.ToUpper(buf.String()) {
	case "BPM":
		return BPM, buf.String()
	case "TIME":
		return TIME, buf.String()

	case "PROJECT":
		return PROJECT, buf.String()
	case "TRACK":
		return TRACK, buf.String()
	case "BAR":
		return BAR, buf.String()
	case "BEAT":
		return BEAT, buf.String()

	case "WHOLE":
		return WHOLE, buf.String()
	case "HALF":
		return HALF, buf.String()
	case "QUARTER":
		return QUARTER, buf.String()
	case "TH":
		return TH, buf.String()
	case "ND":
		return ND, buf.String()

	case "REST":
		return REST, buf.String()
	case "NOTE":
		return NOTE, buf.String()
	case "INTERVAL":
		return INTERVAL, buf.String()
	case "CHORD":
		return CHORD, buf.String()

	case "ON":
		return ON, buf.String()

	}

	// Otherwise return as a regular identifier.
	return IDENTIFIER, buf.String()
}

func (s *Scanner) scanNumber() (tok Token, lit string) {
	// Create a buffer and read the current character into it.
	var buf bytes.Buffer
	buf.WriteRune(s.read())

	// Read every subsequent ident character into the buffer.
	// Non-ident characters and EOF will cause the loop to exit.
	isFloat := false
	for {
		if ch := s.read(); ch == eof {
			break
		} else if !isDigit(ch) && ch != '.' {
			s.unread()
			break
		} else {
			if ch == '.' {
				isFloat = true
			}
			_, _ = buf.WriteRune(ch)
		}
	}

	// Otherwise return as a regular identifier.
	if isFloat {
		return FLOAT, buf.String()
	}
	return NUMBER, buf.String()
}
