// Package value is the elaboration-time value model for earmuff v2.
//
// Expressions in earmuff are evaluated during elaboration (never at runtime) to
// musical values: numbers, booleans, notes, chords, intervals, and lists. This
// package defines that tagged-union Value type, a lexical Env scope chain, and
// Eval, which reduces an ast.Expr to a Value following the semantics in
// website/content/docs/language-reference.md §3.
package value

import (
	"fmt"

	"github.com/poolpOrg/earmuff/ast"
	"github.com/poolpOrg/earmuff/token"
	"github.com/poolpOrg/go-harmony/chords"
	"github.com/poolpOrg/go-harmony/intervals"
	"github.com/poolpOrg/go-harmony/notes"
)

// Kind tags the Value variants.
type Kind int

const (
	KindNumber Kind = iota
	KindBool
	KindNote
	KindChord
	KindInterval
	KindList
)

// Value is a tagged union of every elaboration-time musical value.
//
// Exactly one variant is meaningful, selected by Kind:
//   - KindNumber   -> Num
//   - KindBool     -> Bool
//   - KindNote     -> Note (the go-harmony note, for transposition) and its MIDI
//   - KindChord    -> Chord (pitches as MIDI keys) and Text (original spelling)
//   - KindInterval -> Interval and IntervalName
//   - KindList     -> List (uniform element kind, not enforced here)
type Value struct {
	Kind Kind

	Num  float64
	Bool bool

	// Note: the go-harmony note keeps full pitch identity so note+interval
	// transposition stays accurate; MIDI caches its key.
	Note *notes.Note
	MIDI uint8

	// Chord: the resolved MIDI keys plus original text for diagnostics/equality.
	Chord []uint8
	Text  string

	Interval     intervals.Interval
	IntervalName string

	List []Value
}

// Constructors ---------------------------------------------------------------

func Number(n float64) Value { return Value{Kind: KindNumber, Num: n} }
func Boolean(b bool) Value   { return Value{Kind: KindBool, Bool: b} }

// NotePitch wraps a go-harmony note, caching its MIDI key.
func NotePitch(n *notes.Note) Value {
	return Value{Kind: KindNote, Note: n, MIDI: n.MIDI()}
}

// ChordVal wraps a resolved chord (its pitches) plus its original text.
func ChordVal(keys []uint8, text string) Value {
	return Value{Kind: KindChord, Chord: keys, Text: text}
}

// IntervalVal wraps an interval keyword.
func IntervalVal(iv intervals.Interval, name string) Value {
	return Value{Kind: KindInterval, Interval: iv, IntervalName: name}
}

// List wraps a slice of values.
func ListVal(elems []Value) Value { return Value{Kind: KindList, List: elems} }

// Keys returns the MIDI keys this value sounds as (a note -> one key, a chord ->
// its tones). It reports false for values that are not playable as pitches.
func (v Value) Keys() ([]uint8, bool) {
	switch v.Kind {
	case KindNote:
		return []uint8{v.MIDI}, true
	case KindChord:
		return v.Chord, true
	case KindNumber:
		k := int(v.Num)
		if k < 0 || k > 127 {
			return nil, false
		}
		return []uint8{uint8(k)}, true
	}
	return nil, false
}

func (k Kind) String() string {
	switch k {
	case KindNumber:
		return "number"
	case KindBool:
		return "bool"
	case KindNote:
		return "note"
	case KindChord:
		return "chord"
	case KindInterval:
		return "interval"
	case KindList:
		return "list"
	}
	return "unknown"
}

// ---------------------------------------------------------------------------
// Lexical scope
// ---------------------------------------------------------------------------

// Env is a lexical scope chain: a map of bindings plus an optional parent. A new
// child shadows outer bindings; lookups walk outward. Bindings are immutable in
// the language (let), but Env itself does not enforce that.
type Env struct {
	parent *Env
	vars   map[string]Value
}

// NewEnv returns a fresh scope chained to parent (nil for a root scope).
func NewEnv(parent *Env) *Env {
	return &Env{parent: parent, vars: map[string]Value{}}
}

// Set binds name to v in this scope.
func (e *Env) Set(name string, v Value) { e.vars[name] = v }

// Lookup resolves name through the scope chain.
func (e *Env) Lookup(name string) (Value, bool) {
	for s := e; s != nil; s = s.parent {
		if v, ok := s.vars[name]; ok {
			return v, true
		}
	}
	return Value{}, false
}

// ---------------------------------------------------------------------------
// Keyword tables
// ---------------------------------------------------------------------------

// IntervalByName maps the v2 interval keywords to go-harmony intervals.
var IntervalByName = map[string]intervals.Interval{
	"min2": intervals.MinorSecond, "maj2": intervals.MajorSecond,
	"aug2": intervals.AugmentedSecond, "dim2": intervals.DiminishedSecond,
	"min3": intervals.MinorThird, "maj3": intervals.MajorThird,
	"aug3": intervals.AugmentedThird, "dim3": intervals.DiminishedThird,
	"fourth": intervals.PerfectFourth, "aug4": intervals.AugmentedFourth,
	"dim4":  intervals.DiminishedFourth,
	"fifth": intervals.PerfectFifth, "aug5": intervals.AugmentedFifth,
	"dim5": intervals.DiminishedFifth,
	"min6": intervals.MinorSixth, "maj6": intervals.MajorSixth,
	"min7": intervals.MinorSeventh, "maj7": intervals.MajorSeventh,
	"octave": intervals.Octave, "ninth": intervals.MajorNinth,
	"eleventh": intervals.PerfectEleventh, "thirteenth": intervals.MajorThirteenth,
}

// DynamicVelocity maps a dynamics keyword to its 0..127 velocity (docs §3b).
var DynamicVelocity = map[string]int{
	"ppp": 16, "pp": 32, "p": 48, "mp": 64,
	"mf": 80, "f": 96, "ff": 112, "fff": 127,
}

// ---------------------------------------------------------------------------
// Evaluation
// ---------------------------------------------------------------------------

// Eval reduces an expression to a Value over the lexical scope env. Pattern
// calls in expression position are an error: patterns are statements, not
// values (docs §3).
func Eval(ex ast.Expr, env *Env) (Value, error) {
	switch n := ex.(type) {
	case *ast.NumberLit:
		return Number(n.Value), nil
	case *ast.BoolLit:
		return Boolean(n.Value), nil
	case *ast.IntervalLit:
		iv, ok := IntervalByName[n.Name]
		if !ok {
			return Value{}, posErr(n.Position, "unknown interval %q", n.Name)
		}
		return IntervalVal(iv, n.Name), nil
	case *ast.DynamicLit:
		dv, ok := DynamicVelocity[n.Name]
		if !ok {
			return Value{}, posErr(n.Position, "unknown dynamic %q", n.Name)
		}
		return Number(float64(dv)), nil
	case *ast.MusicLit:
		return parseMusic(n.Text, n.Position)
	case *ast.Ident:
		return evalIdent(n.Name, n.Position, env)
	case *ast.ListLit:
		out := make([]Value, 0, len(n.Elements))
		for _, el := range n.Elements {
			v, err := Eval(el, env)
			if err != nil {
				return Value{}, err
			}
			out = append(out, v)
		}
		return ListVal(out), nil
	case *ast.Range:
		return evalRange(n, env)
	case *ast.Unary:
		return evalUnary(n, env)
	case *ast.Binary:
		return evalBinary(n, env)
	case *ast.Call:
		return Value{}, posErr(n.Position, "pattern call %q is not valid as a value (patterns are statements)", n.Name)
	default:
		return Value{}, posErr(ex.Pos(), "cannot evaluate expression %T", ex)
	}
}

// EvalNumber evaluates an expression that must be a number.
func EvalNumber(ex ast.Expr, env *Env) (float64, error) {
	v, err := Eval(ex, env)
	if err != nil {
		return 0, err
	}
	if v.Kind != KindNumber {
		return 0, posErr(ex.Pos(), "expected a number, got %s", v.Kind)
	}
	return v.Num, nil
}

// Iterate expands a Range / list literal / list-valued expr into a slice of
// element values for a `for` loop.
func Iterate(ex ast.Expr, env *Env) ([]Value, error) {
	switch n := ex.(type) {
	case *ast.Range:
		v, err := evalRange(n, env)
		if err != nil {
			return nil, err
		}
		return v.List, nil
	case *ast.ListLit:
		out := make([]Value, 0, len(n.Elements))
		for _, el := range n.Elements {
			v, err := Eval(el, env)
			if err != nil {
				return nil, err
			}
			out = append(out, v)
		}
		return out, nil
	default:
		v, err := Eval(ex, env)
		if err != nil {
			return nil, err
		}
		if v.Kind != KindList {
			return nil, posErr(ex.Pos(), "for iterable is not a list (got %s)", v.Kind)
		}
		return v.List, nil
	}
}

func parseMusic(text string, pos token.Position) (Value, error) {
	if n, err := notes.Parse(text); err == nil {
		return NotePitch(n), nil
	}
	if c, err := chords.Parse(text); err == nil {
		return ChordVal(chordKeys(c), text), nil
	}
	return Value{}, posErr(pos, "%q is not a valid note or chord", text)
}

func evalIdent(name string, pos token.Position, env *Env) (Value, error) {
	if v, ok := env.Lookup(name); ok {
		return v, nil
	}
	if iv, ok := IntervalByName[name]; ok {
		return IntervalVal(iv, name), nil
	}
	if dv, ok := DynamicVelocity[name]; ok {
		return Number(float64(dv)), nil
	}
	if n, err := notes.Parse(name); err == nil {
		return NotePitch(n), nil
	}
	if c, err := chords.Parse(name); err == nil {
		return ChordVal(chordKeys(c), name), nil
	}
	return Value{}, posErr(pos, "undefined identifier %q", name)
}

func evalRange(n *ast.Range, env *Env) (Value, error) {
	lo, err := Eval(n.Lo, env)
	if err != nil {
		return Value{}, err
	}
	hi, err := Eval(n.Hi, env)
	if err != nil {
		return Value{}, err
	}
	if lo.Kind != KindNumber || hi.Kind != KindNumber {
		return Value{}, posErr(n.Position, "range endpoints must be numbers")
	}
	var out []Value
	for i := int(lo.Num); i <= int(hi.Num); i++ {
		out = append(out, Number(float64(i)))
	}
	return ListVal(out), nil
}

func evalUnary(n *ast.Unary, env *Env) (Value, error) {
	v, err := Eval(n.Operand, env)
	if err != nil {
		return Value{}, err
	}
	switch n.Op {
	case token.PLUS:
		// explicit positive sign (e.g. `bend +2`) — identity on a number
		if v.Kind != KindNumber {
			return Value{}, posErr(n.Position, "unary + requires a number")
		}
		return Number(v.Num), nil
	case token.MINUS:
		if v.Kind != KindNumber {
			return Value{}, posErr(n.Position, "unary - requires a number")
		}
		return Number(-v.Num), nil
	case token.NOT:
		if v.Kind != KindBool {
			return Value{}, posErr(n.Position, "unary ! requires a bool")
		}
		return Boolean(!v.Bool), nil
	}
	return Value{}, posErr(n.Position, "unsupported unary operator")
}

func evalBinary(n *ast.Binary, env *Env) (Value, error) {
	// Short-circuit boolean operators evaluate the right side lazily.
	switch n.Op {
	case token.AND, token.OR:
		l, err := Eval(n.Left, env)
		if err != nil {
			return Value{}, err
		}
		if l.Kind != KindBool {
			return Value{}, posErr(n.Position, "logical operator requires bool operands")
		}
		if n.Op == token.AND && !l.Bool {
			return Boolean(false), nil
		}
		if n.Op == token.OR && l.Bool {
			return Boolean(true), nil
		}
		r, err := Eval(n.Right, env)
		if err != nil {
			return Value{}, err
		}
		if r.Kind != KindBool {
			return Value{}, posErr(n.Position, "logical operator requires bool operands")
		}
		return Boolean(r.Bool), nil
	}

	l, err := Eval(n.Left, env)
	if err != nil {
		return Value{}, err
	}
	r, err := Eval(n.Right, env)
	if err != nil {
		return Value{}, err
	}

	switch n.Op {
	case token.PLUS:
		return evalAdd(n, l, r)
	case token.MINUS:
		if l.Kind == KindNumber && r.Kind == KindNumber {
			return Number(l.Num - r.Num), nil
		}
		return Value{}, posErr(n.Position, "- requires numbers")
	case token.STAR:
		if l.Kind == KindNumber && r.Kind == KindNumber {
			return Number(l.Num * r.Num), nil
		}
		return Value{}, posErr(n.Position, "* requires numbers")
	case token.SLASH:
		if l.Kind == KindNumber && r.Kind == KindNumber {
			if r.Num == 0 {
				return Value{}, posErr(n.Position, "division by zero")
			}
			return Number(l.Num / r.Num), nil
		}
		return Value{}, posErr(n.Position, "/ requires numbers")
	case token.EQ, token.NEQ, token.LT, token.LTE, token.GT, token.GTE:
		return evalCompare(n, l, r)
	}
	return Value{}, posErr(n.Position, "unsupported binary operator")
}

// evalAdd is numeric addition, or note+interval transposition.
func evalAdd(n *ast.Binary, l, r Value) (Value, error) {
	if l.Kind == KindNumber && r.Kind == KindNumber {
		return Number(l.Num + r.Num), nil
	}
	if l.Kind == KindNote && r.Kind == KindInterval {
		t := l.Note.Interval(r.Interval)
		if t == nil {
			return Value{}, posErr(n.Position, "transposition out of range")
		}
		return NotePitch(t), nil
	}
	// interval + note (commutative for the user's convenience)
	if l.Kind == KindInterval && r.Kind == KindNote {
		t := r.Note.Interval(l.Interval)
		if t == nil {
			return Value{}, posErr(n.Position, "transposition out of range")
		}
		return NotePitch(t), nil
	}
	return Value{}, posErr(n.Position, "+ requires numbers or note+interval")
}

func evalCompare(n *ast.Binary, l, r Value) (Value, error) {
	if l.Kind == KindNumber && r.Kind == KindNumber {
		var b bool
		switch n.Op {
		case token.EQ:
			b = l.Num == r.Num
		case token.NEQ:
			b = l.Num != r.Num
		case token.LT:
			b = l.Num < r.Num
		case token.LTE:
			b = l.Num <= r.Num
		case token.GT:
			b = l.Num > r.Num
		case token.GTE:
			b = l.Num >= r.Num
		}
		return Boolean(b), nil
	}
	if n.Op == token.EQ || n.Op == token.NEQ {
		eq := equal(l, r)
		if n.Op == token.NEQ {
			eq = !eq
		}
		return Boolean(eq), nil
	}
	return Value{}, posErr(n.Position, "comparison operands are not ordered")
}

func equal(a, b Value) bool {
	if a.Kind != b.Kind {
		return false
	}
	switch a.Kind {
	case KindNumber:
		return a.Num == b.Num
	case KindBool:
		return a.Bool == b.Bool
	case KindNote:
		return a.MIDI == b.MIDI
	case KindChord:
		return a.Text == b.Text
	case KindInterval:
		return a.Interval == b.Interval
	}
	return false
}

func chordKeys(c *chords.Chord) []uint8 {
	ns := c.Notes()
	out := make([]uint8, 0, len(ns))
	for i := range ns {
		out = append(out, ns[i].MIDI())
	}
	return out
}

func posErr(pos token.Position, format string, args ...interface{}) error {
	return fmt.Errorf("%s: %s", pos, fmt.Sprintf(format, args...))
}
