package parser

import (
	"fmt"
	"io"
	"strconv"

	"github.com/poolpOrg/earring/lexer"
)

type Parser struct {
	s   *lexer.Scanner
	buf struct {
		tok lexer.Token
		lit string
		n   int
	}
}

type Bpm struct {
	bpm uint8
}

type TimeSignature struct {
	beats    uint8
	duration uint8
}

type Project struct {
	bpm           *Bpm
	timeSignature *TimeSignature
}

type Track struct {
	bpm           *Bpm
	timeSignature *TimeSignature
}

type Bar struct {
	bpm           *Bpm
	timeSignature *TimeSignature
	beats         []Beat
}

type Beat struct {
	bpm           *Bpm
	timeSignature *TimeSignature
}

type Duration struct {
	duration uint8
}

type Chord struct {
}

type Note struct {
}

type Rest struct {
}

func NewParser(r io.Reader) *Parser {
	return &Parser{s: lexer.NewScanner(r)}
}

// scan returns the next token from the underlying scanner.
// If a token has been unscanned then read that instead.
func (p *Parser) scan() (tok lexer.Token, lit string) {
	// If we have a token on the buffer, then return it.
	if p.buf.n != 0 {
		p.buf.n = 0
		return p.buf.tok, p.buf.lit
	}

	// Otherwise read the next token from the scanner.
	tok, lit = p.s.Scan()

	// Save it to the buffer in case we unscan later.
	p.buf.tok, p.buf.lit = tok, lit

	return
}

// unscan pushes the previously read token back onto the buffer.
func (p *Parser) unscan() { p.buf.n = 1 }

// scanIgnoreWhitespace scans the next non-whitespace token.
func (p *Parser) scanIgnoreWhitespace() (tok lexer.Token, lit string) {
	tok, lit = p.scan()
	if tok == lexer.WHITESPACE {
		tok, lit = p.scan()
	}
	return
}

func (p *Parser) parseBpm() (*Bpm, error) {
	bpm := &Bpm{}

	if tok, lit := p.scanIgnoreWhitespace(); tok != lexer.NUMBER {
		return nil, fmt.Errorf("found %q, expected number", lit)
	}
	beats, err := strconv.ParseUint(p.buf.lit, 10, 8)
	if err != nil {
		return nil, err
	}

	bpm.bpm = uint8(beats)

	if tok, lit := p.scanIgnoreWhitespace(); tok != lexer.SEMICOLON {
		return nil, fmt.Errorf("found %q, expected ;", lit)
	}

	return bpm, nil
}

func (p *Parser) parseTimeSignature() (*TimeSignature, error) {
	timeSignature := &TimeSignature{}

	if tok, lit := p.scanIgnoreWhitespace(); tok != lexer.NUMBER {
		return nil, fmt.Errorf("found %q, expected number", lit)
	}
	beats, err := strconv.ParseUint(p.buf.lit, 10, 8)
	if err != nil {
		return nil, err
	}

	if tok, lit := p.scanIgnoreWhitespace(); tok != lexer.NUMBER {
		return nil, fmt.Errorf("found %q, expected number", lit)
	}
	duration, err := strconv.ParseUint(p.buf.lit, 10, 8)
	if err != nil {
		return nil, err
	}

	timeSignature.beats = uint8(beats)
	timeSignature.duration = uint8(duration)

	if tok, lit := p.scanIgnoreWhitespace(); tok != lexer.SEMICOLON {
		return nil, fmt.Errorf("found %q, expected ;", lit)
	}

	return timeSignature, nil
}

func (p *Parser) parseProject() (*Project, error) {
	project := &Project{}

	if tok, lit := p.scanIgnoreWhitespace(); tok != lexer.PROJECT {
		return nil, fmt.Errorf("found %q, expected PROJECT", lit)
	}

	if tok, lit := p.scanIgnoreWhitespace(); tok != lexer.BRACKET_OPEN {
		return nil, fmt.Errorf("found %q, expected {", lit)
	}

	for {
		tok, lit := p.scanIgnoreWhitespace()
		if tok == lexer.BRACKET_CLOSE {
			break
		}
		switch tok {
		case lexer.BPM:
			bpm, err := p.parseBpm()
			if err != nil {
				return nil, err
			}
			project.bpm = bpm
		case lexer.TIME:
			timeSignature, err := p.parseTimeSignature()
			if err != nil {
				return nil, err
			}
			project.timeSignature = timeSignature
		case lexer.TRACK:
			_, err := p.parseTrack(project)
			if err != nil {
				return nil, err
			}
		default:
			return nil, fmt.Errorf("found %q, expected TRACK or }", lit)
		}
	}

	//	if tok, lit := p.scanIgnoreWhitespace(); tok != lexer.BRACKET_CLOSE {
	//		return nil, fmt.Errorf("found %q, expected }", lit)
	//	}

	if tok, lit := p.scanIgnoreWhitespace(); tok != lexer.EOF {
		return nil, fmt.Errorf("found %q, expected EOF", lit)
	}

	return project, nil
}

func (p *Parser) parseTrack(project *Project) (*Track, error) {
	track := &Track{}
	track.timeSignature = project.timeSignature
	track.bpm = project.bpm

	if tok, lit := p.scanIgnoreWhitespace(); tok != lexer.BRACKET_OPEN {
		return nil, fmt.Errorf("found %q, expected {", lit)
	}

	for {
		tok, lit := p.scanIgnoreWhitespace()
		if tok == lexer.BRACKET_CLOSE {
			break
		}
		switch tok {
		case lexer.BPM:
			_, err := p.parseBpm()
			if err != nil {
				return nil, err
			}
		case lexer.TIME:
			_, err := p.parseTimeSignature()
			if err != nil {
				return nil, err
			}
		case lexer.BAR:
			_, err := p.parseBar(track)
			if err != nil {
				return nil, err
			}
		default:
			return nil, fmt.Errorf("found %q, expected BAR or }", lit)
		}
	}
	return track, nil
}

func (p *Parser) parseBar(track *Track) (*Bar, error) {
	bar := &Bar{}
	bar.bpm = track.bpm
	bar.timeSignature = track.timeSignature
	bar.beats = make([]Beat, 0)

	if tok, lit := p.scanIgnoreWhitespace(); tok != lexer.BRACKET_OPEN {
		return nil, fmt.Errorf("found %q, expected {", lit)
	}

	for {
		tok, lit := p.scanIgnoreWhitespace()
		if tok == lexer.BRACKET_CLOSE {
			break
		}
		switch tok {
		case lexer.BPM:
			_, err := p.parseBpm()
			if err != nil {
				return nil, err
			}
		case lexer.TIME:
			_, err := p.parseTimeSignature()
			if err != nil {
				return nil, err
			}
		case lexer.BEAT:
			beat, err := p.parseBeat(bar)
			if err != nil {
				return nil, err
			}
			bar.beats = append(bar.beats, *beat)
		default:
			return nil, fmt.Errorf("found %q, expected TIME, BEAT or }", lit)
		}
	}

	return bar, nil
}

func (p *Parser) parseBeat(bar *Bar) (*Beat, error) {
	beat := &Beat{}
	beat.timeSignature = bar.timeSignature

	if tok, lit := p.scanIgnoreWhitespace(); tok != lexer.BRACKET_OPEN {
		return nil, fmt.Errorf("found %q, expected {", lit)
	}

	for {
		tok, lit := p.scanIgnoreWhitespace()
		if tok == lexer.BRACKET_CLOSE {
			break
		}
		switch tok {
		case lexer.WHOLE:
			_, err := p.parseDuration(beat, 1)
			if err != nil {
				return nil, err
			}
		case lexer.HALF:
			_, err := p.parseDuration(beat, 2)
			if err != nil {
				return nil, err
			}
		case lexer.QUARTER:
			_, err := p.parseDuration(beat, 4)
			if err != nil {
				return nil, err
			}
		case lexer.NUMBER:
			value, err := strconv.ParseUint(lit, 10, 8)
			if err != nil {
				return nil, err
			}

			if value == 8 {
				if tok, lit := p.scanIgnoreWhitespace(); tok != lexer.TH {
					return nil, fmt.Errorf("found %q, expected note name", lit)
				}
			} else if value == 16 {
				if tok, lit := p.scanIgnoreWhitespace(); tok != lexer.TH {
					return nil, fmt.Errorf("found %q, expected note name", lit)
				}
			} else if value == 32 {
				if tok, lit := p.scanIgnoreWhitespace(); tok != lexer.ND {
					return nil, fmt.Errorf("found %q, expected note name", lit)
				}
			} else if value == 64 {
				if tok, lit := p.scanIgnoreWhitespace(); tok != lexer.TH {
					return nil, fmt.Errorf("found %q, expected note name", lit)
				}
			} else if value == 128 {
				if tok, lit := p.scanIgnoreWhitespace(); tok != lexer.TH {
					return nil, fmt.Errorf("found %q, expected note name", lit)
				}
			} else if value == 256 {
				if tok, lit := p.scanIgnoreWhitespace(); tok != lexer.TH {
					return nil, fmt.Errorf("found %q, expected note name", lit)
				}
			} else {
				return nil, fmt.Errorf("found %q, expected value", lit)
			}
			_, err = p.parseDuration(beat, uint8(value))
			if err != nil {
				return nil, err
			}

		default:
			return nil, fmt.Errorf("found %q, expected }", lit)
		}
	}

	return beat, nil
}

func (p *Parser) parseDuration(beat *Beat, duration uint8) (*Duration, error) {
	d := &Duration{}
	d.duration = duration

	for {
		tok, lit := p.scanIgnoreWhitespace()
		if tok == lexer.SEMICOLON {
			break
		}
		switch tok {
		case lexer.REST:
			_, err := p.parseRest()
			if err != nil {
				return nil, err
			}
		case lexer.CHORD:
			_, err := p.parseChord()
			if err != nil {
				return nil, err
			}
		case lexer.NOTE:
			_, err := p.parseNote()
			if err != nil {
				return nil, err
			}
		default:
			return nil, fmt.Errorf("found %q, expected ;", lit)
		}
	}
	return d, nil
}

func (p *Parser) parseChord() (*Chord, error) {
	chord := &Chord{}

	if tok, lit := p.scanIgnoreWhitespace(); tok != lexer.IDENTIFIER {
		return nil, fmt.Errorf("found %q, expected chord name", lit)
	}

	return chord, nil
}

func (p *Parser) parseNote() (*Note, error) {
	note := &Note{}

	if tok, lit := p.scanIgnoreWhitespace(); tok != lexer.IDENTIFIER {
		return nil, fmt.Errorf("found %q, expected note name", lit)
	}

	return note, nil
}

func (p *Parser) parseRest() (*Rest, error) {
	rest := &Rest{}

	return rest, nil
}

func (p *Parser) Parse() (*Project, error) {
	return p.parseProject()
}
