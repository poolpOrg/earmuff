package parser

import (
	"github.com/poolpOrg/earmuff/ast"
	"github.com/poolpOrg/earmuff/token"
)

// durationValues are the note values accepted as a bar grid or gate.
var durationValues = map[int]bool{1: true, 2: true, 4: true, 8: true, 16: true, 32: true, 64: true, 128: true}

// durationNames maps spelled-out note values (IDENT tokens) to their numeric
// note value, so `bar quarter` and `bar 4` are equivalent.
var durationNames = map[string]int{
	"whole": 1, "half": 2, "quarter": 4, "eighth": 8,
	"sixteenth": 16, "thirtysecond": 32, "sixtyfourth": 64,
}

// curDuration reports a duration note-value at the current token, accepting both
// a numeric note value (NUMBER) and a spelled-out name (IDENT). It does not
// consume the token.
func (p *Parser) curDuration() (int, bool) {
	if p.curIs(token.NUMBER) {
		v := int(parseFloat(p.cur.Literal))
		if durationValues[v] {
			return v, true
		}
		return 0, false
	}
	if p.curIs(token.IDENT) {
		if v, ok := durationNames[p.cur.Literal]; ok {
			return v, true
		}
	}
	return 0, false
}

func (p *Parser) parseBar() *ast.Bar {
	n := &ast.Bar{Position: p.cur.Pos}
	p.next() // 'bar'

	// optional grid duration: a numeric note value (4) or a name (quarter)
	if v, ok := p.curDuration(); ok {
		n.HasGrid = true
		n.Grid = v
		p.next()
	}
	// optional bar-level velocity default
	if p.curIsVelocity() {
		n.Velocity = p.parseVelocity()
	}

	if !p.expect(token.LBRACE) {
		p.syncStmt()
		return n
	}

	for !p.curIs(token.RBRACE) && !p.curIs(token.EOF) {
		item := p.parseBarItem()
		if item != nil {
			n.Items = append(n.Items, item)
		} else {
			// avoid infinite loop on an unrecognized token
			if !p.curIs(token.RBRACE) {
				p.next()
			}
		}
	}
	p.expect(token.RBRACE)
	return n
}

func (p *Parser) parseBarItem() ast.BarItem {
	switch p.cur.Type {
	case token.SEMICOLON:
		// tolerate a stray ';' between bar items (e.g. `C; E;`)
		pos := p.cur.Pos
		p.next()
		return &ast.BarSep{Position: pos}

	case token.BAR_SEP:
		pos := p.cur.Pos
		p.next()
		return &ast.BarSep{Position: pos}

	case token.ON:
		return p.parseAbsolute()

	case token.FOR:
		return p.parseFor()
	case token.IF:
		return p.parseIf()

	case token.CC, token.BEND, token.PRESSURE, token.PROGRAM, token.SYSEX:
		return p.parseEventStmt(false)
	case token.TEXT, token.LYRIC, token.MARKER, token.CUE:
		return p.parseMetaStmt()

	case token.NUMBER:
		// a grid switch  `16:`
		if p.peekIs(token.COLON) {
			v := int(parseFloat(p.cur.Literal))
			pos := p.cur.Pos
			p.next() // number
			p.next() // ':'
			if !durationValues[v] {
				p.errorf(pos, "invalid grid duration %d", v)
			}
			return &ast.GridSwitch{Position: pos, Grid: v}
		}
		p.errorf(p.cur.Pos, "unexpected number %q in bar", p.cur.Literal)
		return nil

	case token.IDENT:
		// a named grid switch  `quarter:`
		if p.peekIs(token.COLON) {
			if v, ok := durationNames[p.cur.Literal]; ok {
				pos := p.cur.Pos
				p.next() // name
				p.next() // ':'
				return &ast.GridSwitch{Position: pos, Grid: v}
			}
		}
		return p.parseStep()

	case token.LPAREN, token.TILDE:
		return p.parseStep()

	default:
		p.errorf(p.cur.Pos, "unexpected %q in bar", p.cur.Literal)
		return nil
	}
}

// parseStep parses a step-grid token: playable [":" gate] [velocity] ["*" k].
func (p *Parser) parseStep() *ast.Step {
	n := &ast.Step{Position: p.cur.Pos, Repeat: 1}
	n.Play = p.parsePlayable()
	if n.Play == nil {
		return nil
	}
	if p.curIs(token.COLON) {
		p.next()
		if v, ok := p.curDuration(); ok {
			n.HasGate = true
			n.Gate = v
			p.next()
		} else if p.curIsVelocity() {
			// `:v` shorthand seen in the design (E:v mf) — gate stays default,
			// velocity follows.
			n.Velocity = p.parseVelocity()
		} else {
			p.errorf(p.cur.Pos, "expected gate duration after ':', found %q", p.cur.Literal)
		}
	}
	if p.curIsVelocity() {
		n.Velocity = p.parseVelocity()
	}
	if p.curIs(token.STAR) {
		p.next()
		if p.curIs(token.NUMBER) {
			n.Repeat = int(parseFloat(p.cur.Literal))
			p.next()
		} else {
			p.errorf(p.cur.Pos, "expected repeat count after '*', found %q", p.cur.Literal)
		}
	}
	return n
}

// parsePlayable parses a note/chord/percussion ref, rest, tie, or group.
func (p *Parser) parsePlayable() ast.Playable {
	switch p.cur.Type {
	case token.LPAREN:
		// A parenthesized playable is either a simultaneous group of voices
		// `(oh, sn, cy)` (comma-separated) or a single computed pitch
		// `(root + fifth)` (one expression). Parse the first element as an
		// expression and branch on whether a comma follows.
		pos := p.cur.Pos
		p.next()
		first := p.parseExpr(LOWEST)
		if p.curIs(token.COMMA) {
			g := &ast.Group{Position: pos}
			g.Voices = append(g.Voices, exprToPlayable(first))
			for p.curIs(token.COMMA) {
				p.next()
				if p.curIs(token.RPAREN) {
					break
				}
				g.Voices = append(g.Voices, exprToPlayable(p.parseExpr(LOWEST)))
			}
			p.expect(token.RPAREN)
			return g
		}
		p.expect(token.RPAREN)
		return exprToPlayable(first)

	case token.TILDE:
		pos := p.cur.Pos
		p.next()
		return &ast.Tie{Position: pos}

	case token.IDENT:
		lit := p.cur.Literal
		pos := p.cur.Pos
		p.next()
		if lit == "_" {
			return &ast.Rest{Position: pos}
		}
		ref := &ast.NoteRef{Position: pos, Text: lit, Channel: -1}
		if p.curIs(token.AT) {
			p.next()
			if p.curIs(token.NUMBER) {
				ref.Channel = int(parseFloat(p.cur.Literal))
				p.next()
			} else {
				p.errorf(p.cur.Pos, "expected channel number after '@', found %q", p.cur.Literal)
			}
		}
		return ref

	default:
		p.errorf(p.cur.Pos, "expected a note, chord, percussion, '_' or '~', found %q", p.cur.Literal)
		return nil
	}
}

// exprToPlayable converts an expression used in playable position into the
// appropriate Playable: a bare note/chord/binding becomes a NoteRef; anything
// computed (transposition, etc.) becomes an ExprPlay.
func exprToPlayable(e ast.Expr) ast.Playable {
	switch v := e.(type) {
	case *ast.MusicLit:
		return &ast.NoteRef{Position: v.Position, Text: v.Text, Channel: -1}
	case *ast.Ident:
		return &ast.NoteRef{Position: v.Position, Text: v.Name, Channel: -1}
	case nil:
		return nil
	default:
		return &ast.ExprPlay{Position: e.Pos(), Value: e, Channel: -1}
	}
}

// parseAbsolute parses `on beat <expr> <event>`.
func (p *Parser) parseAbsolute() *ast.Absolute {
	n := &ast.Absolute{Position: p.cur.Pos}
	p.next() // 'on'
	if p.curIs(token.IDENT) && p.cur.Literal == "beat" {
		p.next()
	} else {
		p.errorf(p.cur.Pos, "expected 'beat' after 'on', found %q", p.cur.Literal)
		return nil
	}
	n.Beat = p.parseExpr(LOWEST)
	// the event: a playable-with-modifiers (Step) or a raw event statement
	switch p.cur.Type {
	case token.CC, token.BEND, token.PRESSURE, token.PROGRAM, token.SYSEX:
		n.Event = p.parseEventStmt(true)
	case token.TEXT, token.LYRIC, token.MARKER, token.CUE:
		n.Event = p.parseMetaStmt()
	case token.IDENT, token.LPAREN:
		n.Event = p.parseStep()
	default:
		p.errorf(p.cur.Pos, "expected an event after 'on beat', found %q", p.cur.Literal)
	}
	return n
}
