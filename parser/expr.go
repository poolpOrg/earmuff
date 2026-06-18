package parser

import (
	"strconv"

	"github.com/poolpOrg/earmuff/ast"
	"github.com/poolpOrg/earmuff/token"
)

// Precedence levels for the Pratt expression parser, lowest to highest.
const (
	LOWEST     = iota
	OR         // ||
	AND        // &&
	COMPARISON // == != < <= > >=
	RANGE      // ..
	SUM        // + -
	PRODUCT    // * /
	PREFIX     // -x !x
	CALL       // f(...)
)

// infixPrec maps an infix operator token to its binding power.
var infixPrec = map[token.Type]int{
	token.OR:     OR,
	token.AND:    AND,
	token.EQ:     COMPARISON,
	token.NEQ:    COMPARISON,
	token.LT:     COMPARISON,
	token.LTE:    COMPARISON,
	token.GT:     COMPARISON,
	token.GTE:    COMPARISON,
	token.DOTDOT: RANGE,
	token.PLUS:   SUM,
	token.MINUS:  SUM,
	token.STAR:   PRODUCT,
	token.SLASH:  PRODUCT,
	token.LPAREN: CALL,
}

// intervalNames are the interval keywords recognized in expression position.
var intervalNames = map[string]bool{
	"min2": true, "maj2": true, "aug2": true, "dim2": true,
	"min3": true, "maj3": true, "aug3": true, "dim3": true,
	"fourth": true, "aug4": true, "dim4": true,
	"fifth": true, "aug5": true, "dim5": true,
	"min6": true, "maj6": true, "min7": true, "maj7": true,
	"octave": true, "ninth": true, "eleventh": true, "thirteenth": true,
}

// dynamicNames are the dynamics keywords recognized as velocity values.
var dynamicNames = map[string]bool{
	"ppp": true, "pp": true, "p": true, "mp": true,
	"mf": true, "f": true, "ff": true, "fff": true,
}

// parseExpr is the Pratt entry point: parse an expression with binding power > prec.
func (p *Parser) parseExpr(prec int) ast.Expr {
	left := p.parsePrefix()
	if left == nil {
		return nil
	}
	for {
		op := p.cur.Type
		lp, ok := infixPrec[op]
		if !ok || lp <= prec {
			break
		}
		left = p.parseInfix(left, op, lp)
		if left == nil {
			return nil
		}
	}
	return left
}

func (p *Parser) parsePrefix() ast.Expr {
	switch p.cur.Type {
	case token.NUMBER:
		v := parseFloat(p.cur.Literal)
		n := &ast.NumberLit{Position: p.cur.Pos, Value: v, IsFloat: false}
		p.next()
		return n
	case token.FLOAT:
		v := parseFloat(p.cur.Literal)
		n := &ast.NumberLit{Position: p.cur.Pos, Value: v, IsFloat: true}
		p.next()
		return n
	case token.TRUE, token.FALSE:
		n := &ast.BoolLit{Position: p.cur.Pos, Value: p.cur.Type == token.TRUE}
		p.next()
		return n
	case token.STRING:
		// strings are not first-class values in expressions; treat as ident-like
		p.errorf(p.cur.Pos, "string literal not valid in expression")
		p.next()
		return nil
	case token.IDENT:
		return p.parseIdentExpr()
	case token.MINUS, token.NOT:
		op := p.cur.Type
		pos := p.cur.Pos
		p.next()
		operand := p.parseExpr(PREFIX)
		return &ast.Unary{Position: pos, Op: op, Operand: operand}
	case token.PLUS:
		// explicit positive sign (e.g. bend +2)
		pos := p.cur.Pos
		p.next()
		operand := p.parseExpr(PREFIX)
		return &ast.Unary{Position: pos, Op: token.PLUS, Operand: operand}
	case token.LPAREN:
		p.next()
		e := p.parseExpr(LOWEST)
		p.expect(token.RPAREN)
		return e
	case token.LBRACKET:
		return p.parseListLit()
	default:
		p.errorf(p.cur.Pos, "unexpected %q in expression", p.cur.Literal)
		return nil
	}
}

// parseIdentExpr disambiguates an identifier in expression position into a
// pattern call, an interval/dynamic keyword, a note/chord literal, or a plain
// binding reference. The analyzer makes the final note-vs-chord call; here we
// only tag obvious keyword classes.
func (p *Parser) parseIdentExpr() ast.Expr {
	lit := p.cur.Literal
	pos := p.cur.Pos

	if p.peekIs(token.LPAREN) {
		// pattern call as a value
		p.next() // ident
		call := &ast.Call{Position: pos, Name: lit}
		p.expect(token.LPAREN)
		for !p.curIs(token.RPAREN) && !p.curIs(token.EOF) {
			call.Args = append(call.Args, p.parseExpr(LOWEST))
			if p.curIs(token.COMMA) {
				p.next()
			} else {
				break
			}
		}
		p.expect(token.RPAREN)
		return call
	}

	p.next()
	switch {
	case intervalNames[lit]:
		return &ast.IntervalLit{Position: pos, Name: lit}
	case dynamicNames[lit]:
		return &ast.DynamicLit{Position: pos, Name: lit}
	case looksLikePitch(lit):
		return &ast.MusicLit{Position: pos, Text: lit}
	default:
		return &ast.Ident{Position: pos, Name: lit}
	}
}

func (p *Parser) parseInfix(left ast.Expr, op token.Type, lp int) ast.Expr {
	pos := p.cur.Pos

	if op == token.DOTDOT {
		p.next()
		hi := p.parseExpr(lp) // right-assoc-ish; ranges don't chain
		return &ast.Range{Position: left.Pos(), Lo: left, Hi: hi}
	}

	p.next()
	right := p.parseExpr(lp)
	return &ast.Binary{Position: pos, Op: op, Left: left, Right: right}
}

func (p *Parser) parseListLit() ast.Expr {
	n := &ast.ListLit{Position: p.cur.Pos}
	p.next() // '['
	for !p.curIs(token.RBRACKET) && !p.curIs(token.EOF) {
		n.Elements = append(n.Elements, p.parseExpr(LOWEST))
		if p.curIs(token.COMMA) {
			p.next()
		} else {
			break
		}
	}
	p.expect(token.RBRACKET)
	return n
}

// curIsVelocity reports whether the current token begins a velocity clause.
// The sigil `v` lexes as a plain IDENT, so it is recognized by literal.
func (p *Parser) curIsVelocity() bool {
	return p.curIs(token.IDENT) && p.cur.Literal == "v"
}

// parseVelocity parses `v <number|dynamic>`. The sigil `v` lexes as IDENT.
func (p *Parser) parseVelocity() *ast.Velocity {
	if !p.curIsVelocity() {
		p.errorf(p.cur.Pos, "expected velocity 'v', found %q", p.cur.Literal)
		return nil
	}
	n := &ast.Velocity{Position: p.cur.Pos}
	p.next() // 'v'
	switch {
	case p.curIs(token.NUMBER):
		n.HasNumber = true
		n.Number = int(parseFloat(p.cur.Literal))
		p.next()
	case p.curIs(token.IDENT) && dynamicNames[p.cur.Literal]:
		n.Dynamic = p.cur.Literal
		p.next()
	default:
		p.errorf(p.cur.Pos, "expected velocity number or dynamic, found %q", p.cur.Literal)
	}
	return n
}

// --- small helpers --------------------------------------------------------

func parseFloat(s string) float64 {
	f, _ := strconv.ParseFloat(s, 64)
	return f
}

// parseHexByteStr parses a 1-2 char hex string into a byte.
func parseHexByteStr(s string) (byte, bool) {
	if len(s) == 0 || len(s) > 2 {
		return 0, false
	}
	v, err := strconv.ParseUint(s, 16, 16)
	if err != nil || v > 0xFF {
		return 0, false
	}
	return byte(v), true
}

// looksLikePitch is a cheap heuristic: starts with a note letter A-G (upper) and
// the rest is accidentals/digits/quality letters. The analyzer validates it via
// go-harmony; this only tags the AST so MusicLit vs Ident is distinguishable.
func looksLikePitch(s string) bool {
	if len(s) == 0 {
		return false
	}
	c := s[0]
	return c >= 'A' && c <= 'G'
}
