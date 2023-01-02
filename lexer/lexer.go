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
	COMMENT

	IDENTIFIER
	NUMBER
	FLOAT
	STRING
	SEMICOLON
	BRACKET_OPEN
	BRACKET_CLOSE

	BPM
	COPYRIGHT
	TIME
	PROJECT
	INSTRUMENT
	TRACK
	BAR
	BEAT
	TEXT
	LYRIC
	MARKER
	CUE

	PLAY

	WHOLE
	HALF
	QUARTER
	TH
	ND

	ON

	CYMBAL
	SNARE
	OPEN_HI_HAT

	NOTE
	INTERVAL
	CHORD
	PERCUSSION

	VELOCITY
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

	if isWhitespace(ch) {
		s.unread()
		return s.scanWhitespace()
	} else if isDigit(ch) || ch == '.' {
		s.unread()
		return s.scanNumber()
	} else if isLetter(ch) {
		s.unread()
		return s.scanIdent()
	} else if ch == '"' || ch == '\'' {
		s.unread()
		return s.scanString()
	} else if ch == '/' {
		ch = s.read()
		if ch == '/' {
			s.unread()
			return s.scanSingleLineComment()
		} else if ch == '*' {
			s.unread()
			return s.scanMultiLineComment()
		} else {
			s.unread()
		}
	}

	// Otherwise read the individual character.
	switch ch {
	case eof:
		return EOF, ""
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

func (s *Scanner) scanString() (tok Token, lit string) {
	// Create a buffer and read the current character into it.
	var buf bytes.Buffer
	r := s.read()
	escaped := false

	// Read every subsequent whitespace character into the buffer.
	// Non-whitespace characters and EOF will cause the loop to exit.
	for {
		if ch := s.read(); ch == eof {
			break
		} else if ch == '\\' {
			escaped = true
		} else {
			if ch == r {
				if !escaped {
					break
				}
			}
			buf.WriteRune(ch)
			escaped = false
		}
	}
	return STRING, buf.String()
}

func (s *Scanner) scanMultiLineComment() (tok Token, lit string) {
	// Create a buffer and read the current character into it.
	var buf bytes.Buffer
	buf.WriteRune(s.read())

	// Read every subsequent whitespace character into the buffer.
	// Non-whitespace characters and EOF will cause the loop to exit.
	for {
		if ch := s.read(); ch == eof {
			break
		} else if ch == '*' {
			ch = s.read()
			if ch == '/' {
				break
			}
			s.unread()
		} else {
			buf.WriteRune(ch)
		}
	}
	return COMMENT, buf.String()
}

func (s *Scanner) scanSingleLineComment() (tok Token, lit string) {
	// Create a buffer and read the current character into it.
	var buf bytes.Buffer
	buf.WriteRune(s.read())

	// Read every subsequent whitespace character into the buffer.
	// Non-whitespace characters and EOF will cause the loop to exit.
	for {
		if ch := s.read(); ch == eof {
			break
		} else if ch == '\n' {
			break
		} else {
			buf.WriteRune(ch)
		}
	}
	return COMMENT, buf.String()
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

	case "TEXT":
		return TEXT, buf.String()
	case "COPYRIGHT":
		return COPYRIGHT, buf.String()
	case "LYRIC":
		return LYRIC, buf.String()
	case "MARKER":
		return MARKER, buf.String()
	case "CUE":
		return CUE, buf.String()

		//case "NAME":
	//	return NAME, buf.String()
	case "INSTRUMENT":
		return INSTRUMENT, buf.String()

	case "PROJECT":
		return PROJECT, buf.String()
	case "TRACK":
		return TRACK, buf.String()
	case "BAR":
		return BAR, buf.String()
	case "BEAT":
		return BEAT, buf.String()

	case "PLAY":
		return PLAY, buf.String()

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

	case "NOTE":
		return NOTE, buf.String()
	case "INTERVAL":
		return INTERVAL, buf.String()
	case "CHORD":
		return CHORD, buf.String()
	case "PERCUSSION":
		return PERCUSSION, buf.String()

	case "VELOCITY":
		return VELOCITY, buf.String()

	case "CYMBAL":
		return CYMBAL, buf.String()
	case "SNARE":
		return SNARE, buf.String()
	case "OPEN_HI_HAT":
		return OPEN_HI_HAT, buf.String()

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
