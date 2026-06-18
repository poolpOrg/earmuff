// Package analyzer is the static-analysis pass for earmuff v2.
//
// It walks an *ast.Program produced by the parser and returns a slice of
// Diagnostics describing structural and light-harmony problems. The pass is
// purely diagnostic: it never mutates the tree and never panics on a partial or
// nil tree (the parser can produce those after a recovered parse error).
//
// Checks implemented (see website/content/docs/language-reference.md):
//
// Structural (Error):
//  1. undefined pattern call
//  2. pattern arg-count mismatch
//  3. undefined binding (let / loop variable)
//  4. unknown instrument (track instrument / program change)
//  5. unresolved playable (note / chord / kit alias / binding)
//  6. channel out of range (1..16)
//  7. bar overflow / missing grid
//  8. velocity number out of range (0..127)
//
// Light harmony (Warning):
//  9. resolved note out of MIDI range (0..127)
//  10. chord-shaped spelling rejected by go-harmony
//  11. absolute beat out of range (1..beats)
package analyzer

import (
	"fmt"

	"github.com/poolpOrg/earmuff/ast"
	"github.com/poolpOrg/earmuff/midi"
	"github.com/poolpOrg/earmuff/token"
	"github.com/poolpOrg/go-harmony/chords"
	"github.com/poolpOrg/go-harmony/notes"
)

// Severity classifies a Diagnostic.
type Severity int

const (
	// Error marks a structural problem that would make the source fail to
	// elaborate.
	Error Severity = iota
	// Warning marks a probable mistake that still elaborates.
	Warning
)

func (s Severity) String() string {
	switch s {
	case Error:
		return "error"
	case Warning:
		return "warning"
	default:
		return "unknown"
	}
}

// Diagnostic is a single finding with a source position.
type Diagnostic struct {
	Pos      token.Position
	Severity Severity
	Msg      string
}

func (d Diagnostic) String() string {
	return fmt.Sprintf("%s: %s: %s", d.Pos, d.Severity, d.Msg)
}

// Analyze walks prog and returns the diagnostics found. It is safe to call with
// a nil program or partial subtrees; in that case it returns no diagnostics for
// the missing parts.
func Analyze(prog *ast.Program) []Diagnostic {
	a := &analysis{}
	if prog == nil {
		return nil
	}
	// Collect top-level (program-scope) pattern definitions first; these are
	// shared across every project and track.
	root := newScope(nil)
	for _, it := range prog.Items {
		if pd, ok := it.(*ast.PatternDef); ok && pd != nil {
			root.patterns[pd.Name] = pd
		}
	}
	for _, it := range prog.Items {
		switch n := it.(type) {
		case *ast.Project:
			a.analyzeProject(n, root)
		case *ast.PatternDef:
			// Analyze the pattern body in a scope where its params are bound.
			a.analyzePatternDef(n, root)
		}
	}
	return a.diags
}

// analysis accumulates diagnostics during the walk.
type analysis struct {
	diags []Diagnostic
}

func (a *analysis) errorf(pos token.Position, format string, args ...interface{}) {
	a.diags = append(a.diags, Diagnostic{Pos: pos, Severity: Error, Msg: fmt.Sprintf(format, args...)})
}

func (a *analysis) warnf(pos token.Position, format string, args ...interface{}) {
	a.diags = append(a.diags, Diagnostic{Pos: pos, Severity: Warning, Msg: fmt.Sprintf(format, args...)})
}

// ---------------------------------------------------------------------------
// Lexical scope
// ---------------------------------------------------------------------------

// scope is a lexical name environment. It resolves value bindings (let / loop
// vars), pattern definitions, and kit aliases, chaining to its parent.
type scope struct {
	parent   *scope
	bindings map[string]bool // let names and loop variables
	patterns map[string]*ast.PatternDef
	kits     map[string]string // alias -> percussion/note value
	beats    int               // active time-signature numerator (default 4)
}

func newScope(parent *scope) *scope {
	beats := 4
	if parent != nil {
		beats = parent.beats
	}
	return &scope{
		parent:   parent,
		bindings: map[string]bool{},
		patterns: map[string]*ast.PatternDef{},
		kits:     map[string]string{},
		beats:    beats,
	}
}

func (s *scope) hasBinding(name string) bool {
	for sc := s; sc != nil; sc = sc.parent {
		if sc.bindings[name] {
			return true
		}
	}
	return false
}

func (s *scope) lookupPattern(name string) (*ast.PatternDef, bool) {
	for sc := s; sc != nil; sc = sc.parent {
		if pd, ok := sc.patterns[name]; ok {
			return pd, true
		}
	}
	return nil, false
}

func (s *scope) lookupKit(name string) (string, bool) {
	for sc := s; sc != nil; sc = sc.parent {
		if v, ok := sc.kits[name]; ok {
			return v, true
		}
	}
	return "", false
}

// ---------------------------------------------------------------------------
// Project / track / pattern
// ---------------------------------------------------------------------------

func (a *analysis) analyzeProject(proj *ast.Project, parent *scope) {
	if proj == nil {
		return
	}
	sc := newScope(parent)
	// Apply project-level time signature, if any.
	for _, set := range proj.Settings {
		if set.Kind == ast.SettingTime && set.TimeBeats > 0 {
			sc.beats = set.TimeBeats
		}
	}
	// Project-level patterns are visible to every track.
	for _, pd := range proj.Patterns {
		if pd != nil {
			sc.patterns[pd.Name] = pd
		}
	}
	// Analyze the project patterns themselves.
	for _, pd := range proj.Patterns {
		a.analyzePatternDef(pd, sc)
	}
	for _, tr := range proj.Tracks {
		a.analyzeTrack(tr, sc)
	}
}

func (a *analysis) analyzePatternDef(pd *ast.PatternDef, parent *scope) {
	if pd == nil {
		return
	}
	sc := newScope(parent)
	for _, param := range pd.Params {
		sc.bindings[param] = true
	}
	a.analyzeBody(pd.Body, sc)
}

func (a *analysis) analyzeTrack(tr *ast.Track, parent *scope) {
	if tr == nil {
		return
	}
	// Check #4: unknown instrument.
	if tr.Instrument != "" {
		if _, err := midi.InstrumentToPC(tr.Instrument); err != nil {
			a.errorf(tr.Position, "unknown instrument %q", tr.Instrument)
		}
	}
	// Check #6: track channel range.
	if tr.HasChannel && (tr.Channel < 1 || tr.Channel > 16) {
		a.errorf(tr.Position, "channel %d out of range (must be 1..16)", tr.Channel)
	}
	// Check #8: track-default velocity.
	a.checkVelocity(tr.Velocity)

	sc := newScope(parent)
	a.analyzeBody(tr.Body, sc)
}

// ---------------------------------------------------------------------------
// Statement bodies
// ---------------------------------------------------------------------------

// analyzeBody walks a sequence of statements in scope sc. It pre-scans the body
// for kit aliases so that aliases declared anywhere in the block are visible to
// the whole block (kits are pure name bindings, not ordered).
func (a *analysis) analyzeBody(body []ast.Stmt, sc *scope) {
	for _, st := range body {
		switch n := st.(type) {
		case *ast.Kit:
			for _, al := range n.Aliases {
				sc.kits[al.Name] = al.Value
			}
		case *ast.PatternDef:
			// body-local pattern definitions are visible to the whole block,
			// including calls that precede the definition.
			if n != nil {
				sc.patterns[n.Name] = n
			}
		}
	}
	for _, st := range body {
		a.analyzeStmt(st, sc)
	}
}

func (a *analysis) analyzeStmt(st ast.Stmt, sc *scope) {
	switch n := st.(type) {
	case nil:
		return
	case *ast.Bar:
		a.analyzeBar(n, sc)
	case *ast.For:
		a.analyzeFor(n, sc)
	case *ast.If:
		a.analyzeIf(n, sc)
	case *ast.Let:
		// The value is analyzed in the *current* scope (before the binding is
		// visible to itself), then the name becomes visible to later siblings.
		a.analyzeExpr(n.Value, sc)
		sc.bindings[n.Name] = true
	case *ast.Kit:
		// Already collected in analyzeBody; validate each aliased percussion.
		for _, al := range n.Aliases {
			if _, err := midi.PercussionKeyMap(al.Value); err != nil {
				a.errorf(al.Position, "kit alias %q maps to unknown percussion %q", al.Name, al.Value)
			}
		}
	case *ast.PatternCall:
		a.analyzePatternCall(n.Position, n.Name, n.Args, sc)
	case *ast.SettingStmt:
		if n.Setting.Kind == ast.SettingTime && n.Setting.TimeBeats > 0 {
			sc.beats = n.Setting.TimeBeats
		}
	case *ast.CC:
		// The controller position accepts a named-CC keyword (e.g. `cutoff`),
		// which the parser leaves as a bare Ident; it is not a let/loop binding,
		// so we don't resolve it. The value is a normal expression.
		a.analyzeExpr(n.Value, sc)
	case *ast.Bend:
		a.analyzeExpr(n.Value, sc)
	case *ast.Pressure:
		a.analyzeExpr(n.Value, sc)
	case *ast.Program_:
		// Check #4: unknown instrument on a program (patch) change by name.
		if n.HasName {
			if _, err := midi.InstrumentToPC(n.Name); err != nil {
				a.errorf(n.Position, "unknown instrument %q", n.Name)
			}
		}
	case *ast.Meta, *ast.Sysex:
		// nothing to check
	case *ast.PatternDef:
		// already registered by analyzeBody's pre-scan; analyze its body.
		a.analyzePatternDef(n, sc)
	case *ast.Absolute:
		a.analyzeExpr(n.Beat, sc)
		a.analyzeStmt(n.Event, sc)
	case *ast.Step:
		// a top-level step (e.g. inside an `on beat` event) — validate playable.
		a.analyzeStep(n, sc)
	}
}

func (a *analysis) analyzeFor(n *ast.For, parent *scope) {
	if n == nil {
		return
	}
	a.analyzeExpr(n.Iterable, parent)
	sc := newScope(parent)
	if n.Var != "" {
		sc.bindings[n.Var] = true
	}
	a.analyzeBody(n.Body, sc)
}

func (a *analysis) analyzeIf(n *ast.If, parent *scope) {
	if n == nil {
		return
	}
	a.analyzeExpr(n.Cond, parent)
	a.analyzeBody(n.Then, newScope(parent))
	if n.ElseIf != nil {
		a.analyzeIf(n.ElseIf, parent)
	} else if n.Else != nil {
		a.analyzeBody(n.Else, newScope(parent))
	}
}

func (a *analysis) analyzePatternCall(pos token.Position, name string, args []ast.Expr, sc *scope) {
	for _, arg := range args {
		a.analyzeExpr(arg, sc)
	}
	pd, ok := sc.lookupPattern(name)
	if !ok {
		// Check #1: undefined pattern.
		a.errorf(pos, "call to undefined pattern %q", name)
		return
	}
	// Check #2: arg-count mismatch.
	if len(args) != len(pd.Params) {
		a.errorf(pos, "pattern %q called with %d argument(s), expected %d", name, len(args), len(pd.Params))
	}
}

// ---------------------------------------------------------------------------
// Bars and steps (timing model — check #7)
// ---------------------------------------------------------------------------

func (a *analysis) analyzeBar(bar *ast.Bar, parent *scope) {
	if bar == nil {
		return
	}
	a.checkVelocity(bar.Velocity)

	beats := parent.beats
	if beats <= 0 {
		beats = 4
	}

	// Track the active grid: starts at the bar grid, rebindable via GridSwitch.
	barGrid := 0
	if bar.HasGrid {
		barGrid = bar.Grid
	}
	curGrid := barGrid

	// advance is measured in whole-note fractions: a step on a grid of value g
	// advances 1/g of a whole note. A full bar is beats/unit; for the default
	// 4/4 (and the parser's note-value grids) the bar length is beats*(1/4)
	// whole notes when the unit is a quarter. We measure the capacity in grid
	// steps relative to a whole note: capacity = beats/unit whole notes. We work
	// in units of whole notes as a float.
	const unit = 4.0                // default denominator (quarter-note beat)
	barLen := float64(beats) / unit // whole notes per bar

	var advance float64 // whole notes consumed so far
	missingGridReported := false

	for _, item := range bar.Items {
		switch it := item.(type) {
		case *ast.GridSwitch:
			curGrid = it.Grid
		case *ast.BarSep:
			// terminates a grid region; does not advance.
			curGrid = barGrid
		case *ast.Step:
			a.analyzeStep(it, parent)
			rep := it.Repeat
			if rep < 1 {
				rep = 1
			}
			g := curGrid
			if g == 0 {
				if !missingGridReported {
					a.errorf(bar.Position, "no grid: bar needs a default duration or per-step duration")
					missingGridReported = true
				}
				continue
			}
			advance += float64(rep) / float64(g)
		case *ast.Absolute:
			a.analyzeAbsolute(it, beats, parent)
			// 'on beat' does not advance the cursor.
		case *ast.For:
			a.analyzeFor(it, parent)
		case *ast.If:
			a.analyzeIf(it, parent)
		case *ast.CC:
			// See analyzeStmt: the controller may be a named-CC keyword.
			a.analyzeExpr(it.Value, parent)
		case *ast.Bend:
			a.analyzeExpr(it.Value, parent)
		case *ast.Pressure:
			a.analyzeExpr(it.Value, parent)
		case *ast.Program_:
			if it.HasName {
				if _, err := midi.InstrumentToPC(it.Name); err != nil {
					a.errorf(it.Position, "unknown instrument %q", it.Name)
				}
			}
		case *ast.Meta, *ast.Sysex:
			// nothing to check
		}
	}

	// Check #7: bar overflow. Use a small epsilon to tolerate float noise.
	const eps = 1e-9
	if advance > barLen+eps {
		// Express the overflow in grid steps using the bar grid when known,
		// else the finest grid actually seen.
		steps := overflowSteps(advance, barLen, barGrid)
		a.errorf(bar.Position, "bar overflows: %d steps exceed the bar", steps)
	}
}

// overflowSteps reports how many grid steps the bar overflows by. It uses the
// bar grid as the step size when known; otherwise it falls back to expressing
// the surplus in 16th notes for a stable, readable count.
func overflowSteps(advance, barLen float64, barGrid int) int {
	g := barGrid
	if g == 0 {
		g = 16
	}
	surplus := (advance - barLen) * float64(g)
	steps := int(surplus + 0.5)
	if steps < 1 {
		steps = 1
	}
	return steps
}

func (a *analysis) analyzeAbsolute(n *ast.Absolute, beats int, sc *scope) {
	if n == nil {
		return
	}
	a.analyzeExpr(n.Beat, sc)
	// Check #11: beat out of range when it is a plain number literal.
	if lit, ok := n.Beat.(*ast.NumberLit); ok && lit != nil {
		if lit.Value < 1 || lit.Value > float64(beats) {
			a.warnf(n.Position, "beat %g out of range (must be 1..%d)", lit.Value, beats)
		}
	}
	if n.Event != nil {
		a.analyzeStmt(n.Event, sc)
	}
}

func (a *analysis) analyzeStep(st *ast.Step, sc *scope) {
	if st == nil {
		return
	}
	a.checkVelocity(st.Velocity)
	a.analyzePlayable(st.Play, sc)
}

func (a *analysis) analyzePlayable(p ast.Playable, sc *scope) {
	switch n := p.(type) {
	case nil, *ast.Rest, *ast.Tie:
		return
	case *ast.Group:
		for _, v := range n.Voices {
			a.analyzePlayable(v, sc)
		}
	case *ast.NoteRef:
		a.analyzeNoteRef(n, sc)
	}
}

func (a *analysis) analyzeNoteRef(n *ast.NoteRef, sc *scope) {
	if n == nil {
		return
	}
	// Check #6: per-note channel override range. Channel == -1 means "none".
	if n.Channel != -1 && (n.Channel < 1 || n.Channel > 16) {
		a.errorf(n.Position, "channel %d out of range (must be 1..16)", n.Channel)
	}

	text := n.Text
	if text == "" {
		return
	}

	// Resolution order: kit alias, then binding, then note, then chord.
	if val, ok := sc.lookupKit(text); ok {
		// A kit alias resolves to a percussion name; validate it.
		if _, err := midi.PercussionKeyMap(val); err != nil {
			a.errorf(n.Position, "kit alias %q maps to unknown percussion %q", text, val)
		}
		return
	}
	if sc.hasBinding(text) {
		return
	}
	if note, err := notes.Parse(text); err == nil {
		// Check #9: resolved note out of MIDI range.
		m := note.MIDI()
		if m > 127 {
			a.warnf(n.Position, "note %q resolves to MIDI %d, out of range (0..127)", text, m)
		}
		return
	}
	if _, err := chords.Parse(text); err == nil {
		return
	}

	// Unresolved. Distinguish a chord-shaped spelling (warning, #10) from a
	// fully unknown playable (error, #5).
	if len(text) > 1 {
		a.warnf(n.Position, "unrecognized chord/note spelling %q", text)
		return
	}
	a.errorf(n.Position, "unknown note/chord/percussion %q", text)
}

// ---------------------------------------------------------------------------
// Velocity (check #8)
// ---------------------------------------------------------------------------

func (a *analysis) checkVelocity(v *ast.Velocity) {
	if v == nil {
		return
	}
	if v.HasNumber && (v.Number < 0 || v.Number > 127) {
		a.errorf(v.Position, "velocity %d out of range (must be 0..127)", v.Number)
	}
}

// ---------------------------------------------------------------------------
// Expressions (checks #1, #2, #3)
// ---------------------------------------------------------------------------

func (a *analysis) analyzeExpr(e ast.Expr, sc *scope) {
	switch n := e.(type) {
	case nil:
		return
	case *ast.Ident:
		// Check #3: a bare identifier in expression position must be a known
		// binding. MusicLit / IntervalLit / DynamicLit are classified by the
		// parser and are NOT idents.
		if !sc.hasBinding(n.Name) {
			a.errorf(n.Position, "undefined binding %q", n.Name)
		}
	case *ast.Call:
		// A pattern call in expression position: same checks as a statement call.
		a.analyzePatternCall(n.Position, n.Name, n.Args, sc)
	case *ast.Unary:
		a.analyzeExpr(n.Operand, sc)
	case *ast.Binary:
		a.analyzeExpr(n.Left, sc)
		a.analyzeExpr(n.Right, sc)
	case *ast.Range:
		a.analyzeExpr(n.Lo, sc)
		a.analyzeExpr(n.Hi, sc)
	case *ast.ListLit:
		for _, el := range n.Elements {
			a.analyzeExpr(el, sc)
		}
	case *ast.NumberLit, *ast.BoolLit, *ast.MusicLit, *ast.IntervalLit, *ast.DynamicLit:
		// literals — nothing to resolve
	}
}
