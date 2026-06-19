package parser

import (
	"github.com/poolpOrg/earmuff/ast"
	"github.com/poolpOrg/earmuff/token"
)

// parseStmtBlockBody parses statements until a closing '}' (not consumed).
func (p *Parser) parseStmtBlockBody() []ast.Stmt {
	var stmts []ast.Stmt
	for !p.curIs(token.RBRACE) && !p.curIs(token.EOF) {
		before := p.cur.Pos
		s := p.parseStmt()
		if s != nil {
			stmts = append(stmts, s)
		} else {
			p.syncStmt()
		}
		// Guarantee forward progress: if neither parseStmt nor syncStmt advanced
		// (e.g. cur sits on a sync boundary that has no statement rule), consume
		// one token so the loop can never spin.
		if p.cur.Pos == before && !p.curIs(token.RBRACE) && !p.curIs(token.EOF) {
			p.next()
		}
	}
	return stmts
}

// parseBlock parses `{ ...stmts... }` and returns the body.
func (p *Parser) parseBlock() []ast.Stmt {
	if !p.expect(token.LBRACE) {
		return nil
	}
	body := p.parseStmtBlockBody()
	p.expect(token.RBRACE)
	return body
}

func (p *Parser) parseStmt() ast.Stmt {
	switch p.cur.Type {
	case token.BAR:
		return p.parseBar()
	case token.PATTERN:
		// track-local pattern definition
		return p.parsePatternDef()
	case token.SECTION:
		// a named arrangement block: sugar for a zero-arg pattern
		return p.parseSection()
	case token.FOR:
		return p.parseFor()
	case token.REPEAT:
		return p.parseRepeat()
	case token.SWING:
		return p.parseSwing()
	case token.IF:
		return p.parseIf()
	case token.LET:
		return p.parseLet()
	case token.KIT:
		return p.parseKit()
	case token.BPM, token.TIME, token.COPYRIGHT, token.TEXT:
		// project-style settings allowed as overrides; text/copyright also meta
		if p.cur.Type == token.TEXT && (p.peekIs(token.STRING)) {
			// `text "..."` at body level is a track text setting
		}
		s := p.parseSetting()
		if s == nil {
			return nil
		}
		return &ast.SettingStmt{Setting: *s}
	case token.LYRIC, token.MARKER, token.CUE:
		return p.parseMetaStmt()
	case token.ON:
		// `on beat N <event>` may appear directly in a track/pattern body
		// (not only inside a bar), e.g. a bare program/bend at a beat.
		return p.parseAbsolute()
	case token.CC, token.BEND, token.PRESSURE, token.PROGRAM, token.SYSEX:
		return p.parseEventStmt(true)
	case token.IDENT:
		// A pattern/section call. With arguments: `name(a, b)`. Without: a bare
		// `name` plays a zero-arg pattern or a section — the natural way to lay
		// out song structure (`head head solo head`).
		if p.peekIs(token.LPAREN) {
			return p.parsePatternCall()
		}
		n := &ast.PatternCall{Position: p.cur.Pos, Name: p.cur.Literal}
		p.next()
		return n
	default:
		p.errorf(p.cur.Pos, "unexpected %q in body", p.cur.Literal)
		return nil
	}
}

func (p *Parser) parseLet() *ast.Let {
	n := &ast.Let{Position: p.cur.Pos}
	p.next() // 'let'
	if p.curIs(token.IDENT) {
		n.Name = p.cur.Literal
		p.next()
	} else {
		p.errorf(p.cur.Pos, "expected binding name after 'let', found %q", p.cur.Literal)
		return nil
	}
	if !p.expect(token.ASSIGN) {
		return nil
	}
	n.Value = p.parseExpr(LOWEST)
	p.expect(token.SEMICOLON)
	return n
}

func (p *Parser) parseKit() *ast.Kit {
	n := &ast.Kit{Position: p.cur.Pos}
	p.next() // 'kit'
	if !p.expect(token.LBRACE) {
		return nil
	}
	for !p.curIs(token.RBRACE) && !p.curIs(token.EOF) {
		a := ast.KitAlias{Position: p.cur.Pos}
		if !p.curIs(token.IDENT) {
			p.errorf(p.cur.Pos, "expected alias name, found %q", p.cur.Literal)
			p.syncStmt()
			continue
		}
		a.Name = p.cur.Literal
		p.next()
		if !p.expect(token.ASSIGN) {
			continue
		}
		a.Value = p.parseStringLike()
		p.expect(token.SEMICOLON)
		n.Aliases = append(n.Aliases, a)
	}
	p.expect(token.RBRACE)
	return n
}

// parseFor handles two forms:
//
//	for i in <iterable> { ... }   // bound: i takes each value
//	for each <iterable> { ... }   // unbound: iterate, no variable
//
// The iterable is a bare space-separated sequence (1 2 3, C E G, Am7 Dm7), a
// range (1..4), a [..] list literal, or a binding/expression yielding a list.
func (p *Parser) parseFor() *ast.For {
	n := &ast.For{Position: p.cur.Pos}
	p.next() // 'for'

	if p.curIs(token.IDENT) && p.cur.Literal == "each" {
		p.next() // 'each' — unbound, Var stays ""
	} else if p.curIs(token.IDENT) {
		n.Var = p.cur.Literal
		p.next()
		if !p.expect(token.IN) {
			return nil
		}
	} else {
		p.errorf(p.cur.Pos, "expected a loop variable or 'each' after 'for', found %q", p.cur.Literal)
		return nil
	}

	n.Iterable = p.parseIterable()
	n.Body = p.parseBlock()
	return n
}

// parseIterable parses what a `for` loops over. It accepts a bare sequence of
// space-separated primaries (e.g. `1 2 3`, `C E G`) in addition to a single
// range / list / expression. A bare sequence becomes a ListLit.
func (p *Parser) parseIterable() ast.Expr {
	first := p.parseExpr(LOWEST)
	if first == nil {
		return nil
	}
	// If the next token begins another element (not the body '{' or EOF), this
	// is a bare sequence: gather the rest as primaries into a list.
	if !p.iterableSeqContinues() {
		return first
	}
	list := &ast.ListLit{Position: first.Pos(), Elements: []ast.Expr{first}}
	for p.iterableSeqContinues() {
		el := p.parsePrefix()
		if el == nil {
			break
		}
		list.Elements = append(list.Elements, el)
	}
	return list
}

// iterableSeqContinues reports whether the current token can start another
// element of a bare for-sequence (a value), as opposed to the loop body.
func (p *Parser) iterableSeqContinues() bool {
	switch p.cur.Type {
	case token.NUMBER, token.FLOAT, token.IDENT, token.MINUS, token.LPAREN:
		return true
	default:
		return false
	}
}

// parseRepeat parses `repeat N { ... }`, the counted-repeat sugar. It desugars
// to an unbound `for each 1..N`, reusing the loop machinery downstream.
func (p *Parser) parseRepeat() *ast.For {
	pos := p.cur.Pos
	p.next() // 'repeat'
	count := p.parseExpr(LOWEST)
	if count == nil {
		return nil
	}
	n := &ast.For{
		Position: pos,
		Iterable: &ast.Range{
			Position: pos,
			Lo:       &ast.NumberLit{Position: pos, Value: 1},
			Hi:       count,
		},
	}
	n.Body = p.parseBlock()
	return n
}

// parseSwing parses `swing <percent>;`, a running feel modifier for the bars
// that follow it in the body. `swing 50` (straight) turns it off.
func (p *Parser) parseSwing() *ast.Swing {
	n := &ast.Swing{Position: p.cur.Pos}
	p.next() // 'swing'
	n.Percent = p.parseExpr(LOWEST)
	p.expect(token.SEMICOLON)
	return n
}

// parseSection parses `section <name> { ... }`. A section is a named block of
// arrangement that you replay by name (`head`, `solo`, ...) — sugar for a
// zero-parameter pattern, so it shares all of the pattern machinery.
func (p *Parser) parseSection() *ast.PatternDef {
	pat := &ast.PatternDef{Position: p.cur.Pos}
	p.next() // 'section'
	if p.curIs(token.IDENT) {
		pat.Name = p.cur.Literal
		p.next()
	} else {
		p.errorf(p.cur.Pos, "expected section name, found %q", p.cur.Literal)
	}
	if !p.expect(token.LBRACE) {
		p.syncStmt()
		return pat
	}
	pat.Body = p.parseStmtBlockBody()
	p.expect(token.RBRACE)
	return pat
}

func (p *Parser) parseIf() *ast.If {
	n := &ast.If{Position: p.cur.Pos}
	p.next() // 'if'
	n.Cond = p.parseExpr(LOWEST)
	n.Then = p.parseBlock()
	if p.curIs(token.ELSE) {
		p.next()
		if p.curIs(token.IF) {
			n.ElseIf = p.parseIf()
		} else {
			n.Else = p.parseBlock()
		}
	}
	return n
}

func (p *Parser) parsePatternCall() *ast.PatternCall {
	n := &ast.PatternCall{Position: p.cur.Pos, Name: p.cur.Literal}
	p.next() // ident
	p.expect(token.LPAREN)
	for !p.curIs(token.RPAREN) && !p.curIs(token.EOF) {
		n.Args = append(n.Args, p.parseExpr(LOWEST))
		if p.curIs(token.COMMA) {
			p.next()
		} else {
			break
		}
	}
	p.expect(token.RPAREN)
	return n
}

func (p *Parser) parseMetaStmt() ast.Stmt {
	n := &ast.Meta{Position: p.cur.Pos}
	switch p.cur.Type {
	case token.LYRIC:
		n.Kind = ast.MetaLyric
	case token.MARKER:
		n.Kind = ast.MetaMarker
	case token.CUE:
		n.Kind = ast.MetaCue
	default:
		n.Kind = ast.MetaText
	}
	p.next()
	n.Value = p.parseStringLike()
	p.expect(token.SEMICOLON)
	return n
}

// endEvent ends a raw-MIDI event. In statement context (term=true) a ';' is
// required. As a bar item (term=false) raw events are space-separated step
// tokens, but a trailing ';' is tolerated for authors who write it.
func (p *Parser) endEvent(term bool) {
	if term {
		p.expect(token.SEMICOLON)
		return
	}
	if p.curIs(token.SEMICOLON) {
		p.next()
	}
}

// parseEventStmt parses a raw-MIDI event (cc/bend/pressure/program/sysex). When
// term is true it consumes a terminating ';' (track-statement context); when
// false it does not (inline bar-item context).
func (p *Parser) parseEventStmt(term bool) ast.Stmt {
	switch p.cur.Type {
	case token.CC:
		n := &ast.CC{Position: p.cur.Pos}
		p.next()
		n.Controller = p.parseExpr(LOWEST)
		if !p.expect(token.ASSIGN) {
			return nil
		}
		n.Value = p.parseExpr(LOWEST)
		p.endEvent(term)
		return n
	case token.BEND:
		n := &ast.Bend{Position: p.cur.Pos}
		p.next()
		switch p.cur.Type {
		case token.RAW:
			n.Mode = ast.BendRaw
			p.next()
			n.Value = p.parseExpr(LOWEST)
		case token.RANGE:
			n.Mode = ast.BendRange
			p.next()
			n.Value = p.parseExpr(LOWEST)
		default:
			n.Mode = ast.BendSemitones
			n.Value = p.parseExpr(LOWEST)
		}
		p.endEvent(term)
		return n
	case token.PRESSURE:
		n := &ast.Pressure{Position: p.cur.Pos}
		p.next()
		n.Value = p.parseExpr(LOWEST)
		p.endEvent(term)
		return n
	case token.PROGRAM:
		n := &ast.Program_{Position: p.cur.Pos}
		p.next()
		if p.curIs(token.STRING) || p.curIs(token.IDENT) {
			n.HasName = true
			n.Name = p.cur.Literal
			p.next()
		} else if p.curIs(token.NUMBER) {
			n.Number = int(parseFloat(p.cur.Literal))
			p.next()
		} else {
			p.errorf(p.cur.Pos, "expected instrument name or number, found %q", p.cur.Literal)
		}
		p.endEvent(term)
		return n
	case token.SYSEX:
		n := &ast.Sysex{Position: p.cur.Pos}
		p.next()
		// Hex bytes lex as NUMBER (e.g. "09") or IDENT (e.g. "F0", "7E").
		// Collect them from the normal token stream until ';'.
		for !p.curIs(token.SEMICOLON) && !p.curIs(token.EOF) {
			if p.curIs(token.NUMBER) || p.curIs(token.IDENT) {
				b, ok := parseHexByteStr(p.cur.Literal)
				if !ok {
					p.errorf(p.cur.Pos, "invalid sysex byte %q (expected two hex digits)", p.cur.Literal)
					p.next()
					continue
				}
				n.Bytes = append(n.Bytes, b)
				p.next()
			} else {
				p.errorf(p.cur.Pos, "unexpected %q in sysex payload", p.cur.Literal)
				break
			}
		}
		p.endEvent(term)
		return n
	}
	return nil
}
