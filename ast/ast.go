// Package ast defines the abstract syntax tree for the earmuff v2 language.
//
// Every node carries a token.Position (via Pos()) so the analyzer and
// elaborator can attach precise diagnostics. The tree mirrors the grammar in
// website/content/docs/language-reference.md: a Program holds top-level items (projects, shared
// patterns); a Project holds settings and tracks; a Track holds bars, control
// flow, pattern calls, and raw events; a Bar holds step-grid items.
package ast

import "github.com/poolpOrg/earmuff/token"

// Node is implemented by every AST node.
type Node interface {
	Pos() token.Position
}

// ---------------------------------------------------------------------------
// Top level
// ---------------------------------------------------------------------------

// Program is the whole source file.
type Program struct {
	Position token.Position
	Items    []Item // projects and top-level pattern definitions
}

func (n *Program) Pos() token.Position { return n.Position }

// Item is a top-level construct (Project or PatternDef).
type Item interface{ Node }

// Project is a named collection of settings and tracks.
type Project struct {
	Position token.Position
	Name     string
	Settings []Setting // bpm/time/copyright/text encountered at project scope
	Patterns []*PatternDef
	Tracks   []*Track
}

func (n *Project) Pos() token.Position { return n.Position }

// Setting is a project- or track-level configuration statement.
type Setting struct {
	Position token.Position
	Kind     SettingKind
	// Number holds bpm; TimeBeats/TimeUnit hold a time signature; Text holds a
	// string for copyright/text.
	Number    float64
	TimeBeats int
	TimeUnit  int
	Text      string
}

func (n *Setting) Pos() token.Position { return n.Position }

// SettingKind distinguishes the Setting variants.
type SettingKind int

const (
	SettingBPM SettingKind = iota
	SettingTime
	SettingCopyright
	SettingText
)

// ---------------------------------------------------------------------------
// Tracks and statements
// ---------------------------------------------------------------------------

// Track is one instrument's part.
type Track struct {
	Position   token.Position
	Name       string
	Instrument string // "" if unset
	HasChannel bool
	Channel    int
	Port       string // "" if unset
	Velocity   *Velocity
	Body       []Stmt
}

func (n *Track) Pos() token.Position { return n.Position }

// PatternDef is a reusable, parameterized chunk of track body.
type PatternDef struct {
	Position token.Position
	Name     string
	Params   []string
	Body     []Stmt
}

func (n *PatternDef) Pos() token.Position { return n.Position }

// Stmt is anything that can appear in a track or pattern body.
type Stmt interface{ Node }

// Kit binds short aliases to long percussion / note names.
type Kit struct {
	Position token.Position
	Aliases  []KitAlias
}

func (n *Kit) Pos() token.Position { return n.Position }

// KitAlias is one `name = "value";` binding.
type KitAlias struct {
	Position token.Position
	Name     string
	Value    string
}

// Let is an immutable binding.
type Let struct {
	Position token.Position
	Name     string
	Value    Expr
}

func (n *Let) Pos() token.Position { return n.Position }

// For iterates a bound name over a range or list, elaborating Body each round.
type For struct {
	Position token.Position
	Var      string
	Iterable Expr // a Range, ListLit, or expr evaluating to a list
	Body     []Stmt
}

func (n *For) Pos() token.Position { return n.Position }

// If is structured, elaboration-time conditional flow.
type If struct {
	Position token.Position
	Cond     Expr
	Then     []Stmt
	Else     []Stmt // nil if absent
	ElseIf   *If    // set when `else if` chains; mutually exclusive with Else
}

func (n *If) Pos() token.Position { return n.Position }

// Swing sets the swing feel for the bars that follow it in a track body. The
// percentage is the share of each step-pair given to the on-beat: 50 is
// straight (no swing), 67 is triplet swing, up to a sane ceiling. It is a
// running modifier — it applies until the next `swing` in the same body.
type Swing struct {
	Position token.Position
	Percent  Expr // evaluates to a number in [50, 75]
}

func (n *Swing) Pos() token.Position { return n.Position }

// PatternCall invokes a defined pattern with arguments.
type PatternCall struct {
	Position token.Position
	Name     string
	Args     []Expr
}

func (n *PatternCall) Pos() token.Position { return n.Position }

// SettingStmt wraps a Setting used inside a track/bar body (bpm/time overrides).
type SettingStmt struct{ Setting }

// ---------------------------------------------------------------------------
// Bars and step-grid items
// ---------------------------------------------------------------------------

// Bar is a measure with an active step duration and a list of bar items.
type Bar struct {
	Position token.Position
	HasGrid  bool
	Grid     int // step duration as a note value (1,2,4,8,...); 0 if unset
	Velocity *Velocity
	Items    []BarItem
}

func (n *Bar) Pos() token.Position { return n.Position }

// BarItem is anything appearing inside a bar.
type BarItem interface{ Node }

// Step is a step-grid token: a playable with optional gate, velocity, repeat.
type Step struct {
	Position token.Position
	Play     Playable
	HasGate  bool
	Gate     int // sounding length as a note value; 0 = one grid step
	Velocity *Velocity
	Repeat   int // *k; 1 if absent
}

func (n *Step) Pos() token.Position { return n.Position }

// GridSwitch rebinds the step duration for following tokens (e.g. `16:`).
type GridSwitch struct {
	Position token.Position
	Grid     int
}

func (n *GridSwitch) Pos() token.Position { return n.Position }

// BarSep is the optional `|` separator (also terminates a grid region).
type BarSep struct{ Position token.Position }

func (n *BarSep) Pos() token.Position { return n.Position }

// Absolute places an event at an explicit beat, bypassing the step cursor.
type Absolute struct {
	Position token.Position
	Beat     Expr // evaluates to a number (e.g. 3.25)
	Event    Stmt // a Step-like playable or an event statement
}

func (n *Absolute) Pos() token.Position { return n.Position }

// ---------------------------------------------------------------------------
// Playables
// ---------------------------------------------------------------------------

// Playable is something that can sound: a note, chord, percussion alias, rest,
// tie, or a simultaneous group.
type Playable interface{ Node }

// NoteRef is a note/chord/percussion reference by its source text. The analyzer
// resolves whether it is a note, a chord, or a kit alias.
type NoteRef struct {
	Position token.Position
	Text     string // exact lexed text: "C#", "Am7", "hh", ...
	Channel  int    // @channel override; -1 if none
}

func (n *NoteRef) Pos() token.Position { return n.Position }

// Rest is a silent slot (`_`).
type Rest struct{ Position token.Position }

func (n *Rest) Pos() token.Position { return n.Position }

// Tie extends the previous note's gate by one step (`~`).
type Tie struct{ Position token.Position }

func (n *Tie) Pos() token.Position { return n.Position }

// Group is a set of simultaneous playables, e.g. `(oh,sn,cy)`.
type Group struct {
	Position token.Position
	Voices   []Playable
}

func (n *Group) Pos() token.Position { return n.Position }

// ExprPlay is a playable produced by an expression that evaluates to a note or
// chord, e.g. `(root + fifth)`. The elaborator evaluates Value to a pitch.
type ExprPlay struct {
	Position token.Position
	Value    Expr
	Channel  int // @channel override; -1 if none
}

func (n *ExprPlay) Pos() token.Position { return n.Position }

// ---------------------------------------------------------------------------
// Raw MIDI / meta event statements
// ---------------------------------------------------------------------------

// CC is a control-change event.
type CC struct {
	Position   token.Position
	Controller Expr // number or named-CC ident
	Value      Expr
}

func (n *CC) Pos() token.Position { return n.Position }

// BendMode distinguishes the bend variants.
type BendMode int

const (
	BendSemitones BendMode = iota // bend +2
	BendRaw                       // bend raw 8192
	BendRange                     // bend range 12
)

// Bend is a pitch-bend event.
type Bend struct {
	Position token.Position
	Mode     BendMode
	Value    Expr
}

func (n *Bend) Pos() token.Position { return n.Position }

// Pressure is channel aftertouch.
type Pressure struct {
	Position token.Position
	Value    Expr
}

func (n *Pressure) Pos() token.Position { return n.Position }

// Program is a program (patch) change.
type Program_ struct {
	Position token.Position
	Name     string // instrument name; mutually exclusive with Number
	HasName  bool
	Number   int
}

func (n *Program_) Pos() token.Position { return n.Position }

// Sysex is a raw system-exclusive payload.
type Sysex struct {
	Position token.Position
	Bytes    []byte
}

func (n *Sysex) Pos() token.Position { return n.Position }

// MetaKind distinguishes the text-like meta events.
type MetaKind int

const (
	MetaText MetaKind = iota
	MetaLyric
	MetaMarker
	MetaCue
)

// Meta is a text/lyric/marker/cue event.
type Meta struct {
	Position token.Position
	Kind     MetaKind
	Value    string
}

func (n *Meta) Pos() token.Position { return n.Position }

// ---------------------------------------------------------------------------
// Velocity / dynamics
// ---------------------------------------------------------------------------

// Velocity is `v <number|dynamic>`.
type Velocity struct {
	Position  token.Position
	HasNumber bool
	Number    int    // exact 0..127 when HasNumber
	Dynamic   string // "pp".."ff" otherwise
}

func (n *Velocity) Pos() token.Position { return n.Position }

// ---------------------------------------------------------------------------
// Expressions
// ---------------------------------------------------------------------------

// Expr is a value-producing expression, evaluated at elaboration time.
type Expr interface{ Node }

// NumberLit is an integer or float literal.
type NumberLit struct {
	Position token.Position
	Value    float64
	IsFloat  bool
}

func (n *NumberLit) Pos() token.Position { return n.Position }

// BoolLit is true/false.
type BoolLit struct {
	Position token.Position
	Value    bool
}

func (n *BoolLit) Pos() token.Position { return n.Position }

// Ident references a binding (let/loop var) or a note/chord/interval/dynamic
// keyword in expression position. The analyzer resolves which.
type Ident struct {
	Position token.Position
	Name     string
}

func (n *Ident) Pos() token.Position { return n.Position }

// MusicLit is a note or chord literal in expression position (e.g. `Am6` in a
// list). Kept distinct from Ident so the parser can tag pitch-shaped words.
type MusicLit struct {
	Position token.Position
	Text     string
}

func (n *MusicLit) Pos() token.Position { return n.Position }

// IntervalLit is an interval keyword (maj3, fifth, octave, ...).
type IntervalLit struct {
	Position token.Position
	Name     string
}

func (n *IntervalLit) Pos() token.Position { return n.Position }

// DynamicLit is a dynamics keyword used as a value (pp..ff).
type DynamicLit struct {
	Position token.Position
	Name     string
}

func (n *DynamicLit) Pos() token.Position { return n.Position }

// ListLit is `[a, b, c]`.
type ListLit struct {
	Position token.Position
	Elements []Expr
}

func (n *ListLit) Pos() token.Position { return n.Position }

// Range is `lo..hi` (inclusive integer range).
type Range struct {
	Position token.Position
	Lo       Expr
	Hi       Expr
}

func (n *Range) Pos() token.Position { return n.Position }

// Unary is `-x` or `!x`.
type Unary struct {
	Position token.Position
	Op       token.Type
	Operand  Expr
}

func (n *Unary) Pos() token.Position { return n.Position }

// Binary is an infix operation; Op is one of +,-,*,/,==,!=,<,<=,>,>=,&&,||.
type Binary struct {
	Position token.Position
	Op       token.Type
	Left     Expr
	Right    Expr
}

func (n *Binary) Pos() token.Position { return n.Position }

// Call is a pattern call used in expression position (rare; mostly statements).
type Call struct {
	Position token.Position
	Name     string
	Args     []Expr
}

func (n *Call) Pos() token.Position { return n.Position }
