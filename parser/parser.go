// Package parser builds an ast.Program from earmuff v2 source.
//
// It is a hand-written recursive-descent parser for the statement grammar with
// a Pratt (precedence-climbing) parser for expressions (see expr.go). Errors are
// collected as structured diagnostics rather than aborting on the first problem:
// after a parse error the parser synchronizes to a statement boundary and keeps
// going, so a single run can report many issues.
package parser

import (
	"fmt"

	"github.com/poolpOrg/earmuff/ast"
	"github.com/poolpOrg/earmuff/lexer"
	"github.com/poolpOrg/earmuff/token"
)

// Diagnostic is a parse error with a source position.
type Diagnostic struct {
	Pos token.Position
	Msg string
}

func (d Diagnostic) String() string { return fmt.Sprintf("%s: %s", d.Pos, d.Msg) }

// Parser holds lexer state and accumulated diagnostics.
type Parser struct {
	lex  *lexer.Lexer
	cur  token.Token
	peek token.Token

	errors []Diagnostic
}

// New creates a Parser over src. filename is used only for diagnostics.
func New(src, filename string) *Parser {
	p := &Parser{lex: lexer.New(src, filename)}
	// prime cur and peek
	p.cur = p.lex.Next()
	p.peek = p.lex.Next()
	return p
}

// Errors returns the diagnostics collected so far.
func (p *Parser) Errors() []Diagnostic { return p.errors }

func (p *Parser) errorf(pos token.Position, format string, args ...interface{}) {
	p.errors = append(p.errors, Diagnostic{Pos: pos, Msg: fmt.Sprintf(format, args...)})
}

func (p *Parser) next() {
	p.cur = p.peek
	p.peek = p.lex.Next()
}

func (p *Parser) curIs(t token.Type) bool  { return p.cur.Type == t }
func (p *Parser) peekIs(t token.Type) bool { return p.peek.Type == t }

// expect consumes cur if it matches t, else records an error and returns false.
func (p *Parser) expect(t token.Type) bool {
	if p.cur.Type == t {
		p.next()
		return true
	}
	p.errorf(p.cur.Pos, "expected %s, found %q", t, p.cur.Literal)
	return false
}

// Parse parses a whole program. The returned diagnostics (also via Errors) are
// empty on success.
func (p *Parser) Parse() (*ast.Program, []Diagnostic) {
	prog := &ast.Program{Position: p.cur.Pos}
	for !p.curIs(token.EOF) {
		switch p.cur.Type {
		case token.PROJECT:
			if proj := p.parseProject(); proj != nil {
				prog.Items = append(prog.Items, proj)
			}
		case token.PATTERN:
			if pat := p.parsePatternDef(); pat != nil {
				prog.Items = append(prog.Items, pat)
			}
		default:
			p.errorf(p.cur.Pos, "expected 'project' or 'pattern', found %q", p.cur.Literal)
			p.syncTopLevel()
		}
	}
	return prog, p.errors
}

// syncTopLevel skips tokens until the next top-level keyword or EOF.
func (p *Parser) syncTopLevel() {
	for !p.curIs(token.EOF) && !p.curIs(token.PROJECT) && !p.curIs(token.PATTERN) {
		p.next()
	}
}

// syncStmt skips tokens until a likely statement boundary inside a block.
func (p *Parser) syncStmt() {
	depth := 0
	for !p.curIs(token.EOF) {
		switch p.cur.Type {
		case token.SEMICOLON:
			p.next()
			return
		case token.LBRACE:
			depth++
		case token.RBRACE:
			if depth == 0 {
				return
			}
			depth--
		case token.BAR, token.TRACK, token.FOR, token.IF, token.LET, token.ON,
			token.PATTERN, token.KIT:
			if depth == 0 {
				return
			}
		}
		p.next()
	}
}

func (p *Parser) parseProject() *ast.Project {
	proj := &ast.Project{Position: p.cur.Pos}
	p.next() // 'project'

	if p.curIs(token.STRING) || p.curIs(token.IDENT) {
		proj.Name = p.cur.Literal
		p.next()
	} else {
		p.errorf(p.cur.Pos, "expected project name, found %q", p.cur.Literal)
	}

	if !p.expect(token.LBRACE) {
		p.syncTopLevel()
		return proj
	}

	for !p.curIs(token.RBRACE) && !p.curIs(token.EOF) {
		switch p.cur.Type {
		case token.BPM, token.TIME, token.COPYRIGHT, token.TEXT:
			if s := p.parseSetting(); s != nil {
				proj.Settings = append(proj.Settings, *s)
			}
		case token.TRACK:
			if tr := p.parseTrack(); tr != nil {
				proj.Tracks = append(proj.Tracks, tr)
			}
		case token.PATTERN:
			if pat := p.parsePatternDef(); pat != nil {
				proj.Patterns = append(proj.Patterns, pat)
			}
		default:
			p.errorf(p.cur.Pos, "expected bpm/time/track/pattern or '}', found %q", p.cur.Literal)
			p.syncStmt()
		}
	}
	p.expect(token.RBRACE)
	return proj
}

func (p *Parser) parseSetting() *ast.Setting {
	s := &ast.Setting{Position: p.cur.Pos}
	switch p.cur.Type {
	case token.BPM:
		s.Kind = ast.SettingBPM
		p.next()
		n, ok := p.parseNumberToken()
		if !ok {
			p.syncStmt()
			return nil
		}
		s.Number = n
		p.expect(token.SEMICOLON)
	case token.TIME:
		s.Kind = ast.SettingTime
		p.next()
		b, ok1 := p.parseIntToken()
		u, ok2 := p.parseIntToken()
		if !ok1 || !ok2 {
			p.syncStmt()
			return nil
		}
		s.TimeBeats, s.TimeUnit = b, u
		p.expect(token.SEMICOLON)
	case token.COPYRIGHT:
		s.Kind = ast.SettingCopyright
		p.next()
		s.Text = p.parseStringLike()
		p.expect(token.SEMICOLON)
	case token.TEXT:
		s.Kind = ast.SettingText
		p.next()
		s.Text = p.parseStringLike()
		p.expect(token.SEMICOLON)
	}
	return s
}

func (p *Parser) parseStringLike() string {
	if p.curIs(token.STRING) || p.curIs(token.IDENT) {
		v := p.cur.Literal
		p.next()
		return v
	}
	p.errorf(p.cur.Pos, "expected string, found %q", p.cur.Literal)
	return ""
}

func (p *Parser) parseNumberToken() (float64, bool) {
	if p.curIs(token.NUMBER) || p.curIs(token.FLOAT) {
		f := parseFloat(p.cur.Literal)
		p.next()
		return f, true
	}
	p.errorf(p.cur.Pos, "expected number, found %q", p.cur.Literal)
	return 0, false
}

func (p *Parser) parseIntToken() (int, bool) {
	if p.curIs(token.NUMBER) {
		n := int(parseFloat(p.cur.Literal))
		p.next()
		return n, true
	}
	p.errorf(p.cur.Pos, "expected integer, found %q", p.cur.Literal)
	return 0, false
}

func (p *Parser) parseTrack() *ast.Track {
	tr := &ast.Track{Position: p.cur.Pos, Channel: 0}
	p.next() // 'track'

	if p.curIs(token.STRING) || p.curIs(token.IDENT) {
		tr.Name = p.cur.Literal
		p.next()
	} else {
		p.errorf(p.cur.Pos, "expected track name, found %q", p.cur.Literal)
	}

	// optional header clauses: instrument / channel / port / velocity, any order
	for {
		switch {
		case p.curIs(token.INSTRUMENT):
			p.next()
			tr.Instrument = p.parseStringLike()
		case p.curIs(token.CHANNEL):
			p.next()
			if n, ok := p.parseIntToken(); ok {
				tr.HasChannel = true
				tr.Channel = n
			}
		case p.curIs(token.PORT):
			p.next()
			tr.Port = p.parseStringLike()
		case p.curIsVelocity():
			tr.Velocity = p.parseVelocity()
		default:
			goto body
		}
	}
body:
	if !p.expect(token.LBRACE) {
		p.syncStmt()
		return tr
	}
	tr.Body = p.parseStmtBlockBody()
	p.expect(token.RBRACE)
	return tr
}

func (p *Parser) parsePatternDef() *ast.PatternDef {
	pat := &ast.PatternDef{Position: p.cur.Pos}
	p.next() // 'pattern'
	if p.curIs(token.IDENT) {
		pat.Name = p.cur.Literal
		p.next()
	} else {
		p.errorf(p.cur.Pos, "expected pattern name, found %q", p.cur.Literal)
	}
	// The parameter list is optional: `pattern I { ... }` and
	// `pattern walk(root, third) { ... }` are both valid.
	if p.curIs(token.LPAREN) {
		p.next()
		for !p.curIs(token.RPAREN) && !p.curIs(token.EOF) {
			if p.curIs(token.IDENT) {
				pat.Params = append(pat.Params, p.cur.Literal)
				p.next()
			} else {
				p.errorf(p.cur.Pos, "expected parameter name, found %q", p.cur.Literal)
				break
			}
			if p.curIs(token.COMMA) {
				p.next()
			}
		}
		p.expect(token.RPAREN)
	}
	if !p.expect(token.LBRACE) {
		p.syncStmt()
		return pat
	}
	pat.Body = p.parseStmtBlockBody()
	p.expect(token.RBRACE)
	return pat
}
