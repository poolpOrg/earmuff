// Package elaborator turns an *ast.Program into one Song per project: a flat,
// time-ordered stream of MIDI events stamped with absolute ticks. It is the
// pure, deterministic phase described in website/content/docs/language-reference.md §1 — patterns,
// loops, and conditionals are expanded over compile-time-known values, so the
// event stream is fully determined before any MIDI is emitted.
//
// Timing (docs §3a) is implemented exactly:
//   - PPQ = 960 ticks per quarter note; a grid value N means one step is
//     (4/N) quarter-notes = 960*4/N ticks.
//   - A bar keeps a cursor in within-bar ticks. Each step token advances the
//     cursor by ONE grid step, regardless of its gate. Repeat *k advances k
//     steps and emits k copies.
//   - Gate (sounding length) defaults to one grid step; a :dur suffix sets it to
//     an absolute note value = 960*4/dur ticks.
//   - Tie (~) extends the previous note's gate by one grid step and advances the
//     cursor one step.
//   - GridSwitch rebinds the step within the bar; BarSep resets to the bar's
//     base grid and does not advance; Absolute (on beat B) places at (B-1)*960
//     and does not move the cursor.
//   - Bar k starts at k * beats * 960 ticks (beats from the time signature).
package elaborator

import (
	"fmt"
	"sort"
	"strings"

	"github.com/poolpOrg/earmuff/ast"
	lmidi "github.com/poolpOrg/earmuff/midi"
	"github.com/poolpOrg/earmuff/token"
	"github.com/poolpOrg/earmuff/value"
	"github.com/poolpOrg/go-harmony/chords"
	"github.com/poolpOrg/go-harmony/notes"
)

// PPQ is the single source of truth for ticks per quarter note (docs §1).
const PPQ = 960

// durTicks returns the tick length of a note value (whole=1, quarter=4, ...).
func durTicks(noteValue int) uint32 {
	if noteValue <= 0 {
		return PPQ
	}
	return uint32(PPQ * 4 / noteValue)
}

// ---------------------------------------------------------------------------
// MIDI message model
// ---------------------------------------------------------------------------

// MsgKind tags a MIDIMsg variant.
type MsgKind int

const (
	MsgNoteOn MsgKind = iota
	MsgNoteOff
	MsgCC
	MsgPitchBend
	MsgPressure
	MsgProgram
	MsgMeta
	MsgSysex
)

// MIDIMsg is the semantic content of an event, independent of any wire format;
// smfwriter converts it to SMF bytes.
//
// Fields are interpreted per Kind:
//   - NoteOn/NoteOff: Channel, Key, Velocity (Velocity unused for NoteOff)
//   - CC:             Channel, Controller, Value
//   - PitchBend:      Channel, Bend (-8192..8191, 0 = center)
//   - Pressure:       Channel, Value
//   - Program:        Channel, Program (0-based)
//   - Sysex:          Bytes (full payload incl. F0..F7)
//   - Meta:           MetaKind, Text (or Beats/Unit/BPM/Copyright for headers)
type MIDIMsg struct {
	Kind MsgKind

	Channel    uint8
	Key        uint8
	Velocity   uint8
	Controller uint8
	Value      uint8
	Program    uint8
	Bend       int16
	Bytes      []byte

	MetaKind ast.MetaKind
	Text     string
}

// Event is one MIDI message stamped with an absolute tick and a track index.
type Event struct {
	Tick  uint32
	Track int
	Msg   MIDIMsg

	// order disambiguates same-tick events from the same emission stream so
	// elaboration is deterministic.
	order int
}

// TrackInfo is the per-track metadata smfwriter needs for SMF headers.
type TrackInfo struct {
	Name       string
	Instrument string
	Channel    uint8
	Program    uint8 // 0-based GM program; valid only if HasProgram
	HasProgram bool
}

// Song is one project's elaboration: a flat event stream plus per-track and
// project metadata.
type Song struct {
	Name      string
	Events    []Event
	Tracks    []TrackInfo
	BPM       float64
	TimeBeats int
	TimeUnit  int
	Copyright string
	Texts     []string
}

// Elaborate turns a program into one Song per project. It is pure and
// deterministic: the same program yields identical Songs.
func Elaborate(prog *ast.Program) ([]Song, []error) {
	if prog == nil {
		return nil, nil
	}
	global := map[string]*ast.PatternDef{}
	for _, it := range prog.Items {
		if pd, ok := it.(*ast.PatternDef); ok && pd != nil {
			global[pd.Name] = pd
		}
	}
	var songs []Song
	var errs []error
	for _, it := range prog.Items {
		if proj, ok := it.(*ast.Project); ok {
			e := &elab{
				song:          &Song{Name: proj.Name, BPM: 120, TimeBeats: 4, TimeUnit: 4},
				globalPattern: global,
			}
			e.elabProject(proj)
			e.finalize()
			songs = append(songs, *e.song)
			errs = append(errs, e.errs...)
		}
	}
	return songs, errs
}

// ---------------------------------------------------------------------------
// Scope: bindings (delegated to value.Env), patterns, and kit aliases
// ---------------------------------------------------------------------------

type scope struct {
	parent   *scope
	env      *value.Env
	patterns map[string]*ast.PatternDef
	kit      map[string]string // alias -> percussion/note/chord value text
}

func newScope(parent *scope) *scope {
	var penv *value.Env
	if parent != nil {
		penv = parent.env
	}
	return &scope{
		parent:   parent,
		env:      value.NewEnv(penv),
		patterns: map[string]*ast.PatternDef{},
		kit:      map[string]string{},
	}
}

func (s *scope) lookupPattern(name string) (*ast.PatternDef, bool) {
	for sc := s; sc != nil; sc = sc.parent {
		if p, ok := sc.patterns[name]; ok {
			return p, true
		}
	}
	return nil, false
}

func (s *scope) lookupKit(name string) (string, bool) {
	for sc := s; sc != nil; sc = sc.parent {
		if v, ok := sc.kit[name]; ok {
			return v, true
		}
	}
	return "", false
}

// ---------------------------------------------------------------------------
// Elaboration state
// ---------------------------------------------------------------------------

type elab struct {
	song *Song
	errs []error

	globalPattern map[string]*ast.PatternDef

	curTrack    int
	trackChan   uint8
	timeBeats   int
	timeUnit    int
	trackVel    int  // track-level velocity default; -1 if unset
	bendRangeRP bool // RPN pitch-bend-range already emitted for this track?
	bendRange   uint8

	trackOffset uint32 // running tick offset where the next bar starts
	orderCtr    int

	swing float64 // current swing ratio (0.5 = straight); a running modifier
}

func (e *elab) errorf(pos token.Position, format string, args ...interface{}) {
	e.errs = append(e.errs, fmt.Errorf("%s: %s", pos, fmt.Sprintf(format, args...)))
}

func (e *elab) emit(tick uint32, msg MIDIMsg) {
	e.orderCtr++
	e.song.Events = append(e.song.Events, Event{
		Tick:  tick,
		Track: e.curTrack,
		Msg:   msg,
		order: e.orderCtr,
	})
}

// ---------------------------------------------------------------------------
// Projects and tracks
// ---------------------------------------------------------------------------

func (e *elab) elabProject(proj *ast.Project) {
	root := newScope(nil)
	for name, pd := range e.globalPattern {
		root.patterns[name] = pd
	}
	for _, pd := range proj.Patterns {
		root.patterns[pd.Name] = pd
	}
	for _, s := range proj.Settings {
		e.applyProjectSetting(s)
	}

	nextChan := uint8(0)
	for _, tr := range proj.Tracks {
		e.elabTrack(tr, root, &nextChan)
	}
}

func (e *elab) applyProjectSetting(s ast.Setting) {
	switch s.Kind {
	case ast.SettingBPM:
		e.song.BPM = s.Number
	case ast.SettingTime:
		e.song.TimeBeats = s.TimeBeats
		e.song.TimeUnit = s.TimeUnit
	case ast.SettingCopyright:
		e.song.Copyright = s.Text
	case ast.SettingText:
		e.song.Texts = append(e.song.Texts, s.Text)
	}
}

// allocChannel returns the next free channel, skipping 9 (percussion) and
// clamping to 0..15.
func allocChannel(next *uint8) uint8 {
	c := *next
	if c == 9 {
		c++
	}
	if c > 15 {
		c = 15
	}
	*next = c + 1
	return c
}

func (e *elab) elabTrack(tr *ast.Track, parent *scope, nextChan *uint8) {
	e.curTrack = len(e.song.Tracks)
	e.timeBeats = e.song.TimeBeats
	e.timeUnit = e.song.TimeUnit
	e.trackOffset = 0
	e.bendRangeRP = false
	e.bendRange = 2
	e.swing = 0.5 // straight until a `swing` statement says otherwise

	sc := newScope(parent)

	percussion := e.trackIsPercussion(tr, sc)

	var ch uint8
	switch {
	case tr.HasChannel:
		ch = clampChan(tr.Channel - 1) // source uses 1-based channels
	case percussion:
		ch = 9
	default:
		ch = allocChannel(nextChan)
	}
	e.trackChan = ch

	e.trackVel = -1
	if tr.Velocity != nil {
		e.trackVel = velocityValue(tr.Velocity)
	}

	info := TrackInfo{Name: tr.Name, Instrument: tr.Instrument, Channel: ch}
	if tr.Instrument != "" {
		if pc, err := lmidi.InstrumentToPC(tr.Instrument); err == nil {
			info.Program = pc - 1 // InstrumentToPC is 1-based; wire program is 0-based
			info.HasProgram = true
		}
	}
	e.song.Tracks = append(e.song.Tracks, info)

	e.elabBody(tr.Body, sc, e.trackVel)
}

// trackIsPercussion reports whether every NoteRef in the body resolves to a kit
// alias or percussion key-map name (and there is at least one).
func (e *elab) trackIsPercussion(tr *ast.Track, parent *scope) bool {
	probe := newScope(parent)
	collectKits(tr.Body, probe)
	any := false
	allPerc := true
	var walk func(items []ast.Stmt)
	walkBar := func(bar *ast.Bar) {
		for _, it := range bar.Items {
			st, ok := it.(*ast.Step)
			if !ok {
				continue
			}
			for _, r := range playableRefs(st.Play) {
				if _, ok := probe.lookupKit(r); ok {
					any = true
					continue
				}
				if _, err := lmidi.PercussionKeyMap(r); err == nil {
					any = true
					continue
				}
				allPerc = false
			}
		}
	}
	walk = func(items []ast.Stmt) {
		for _, st := range items {
			switch n := st.(type) {
			case *ast.Bar:
				walkBar(n)
			case *ast.For:
				walk(n.Body)
			case *ast.If:
				walk(n.Then)
				walk(n.Else)
				if n.ElseIf != nil {
					walk([]ast.Stmt{n.ElseIf})
				}
			}
		}
	}
	walk(tr.Body)
	return any && allPerc
}

func collectKits(items []ast.Stmt, sc *scope) {
	for _, st := range items {
		switch n := st.(type) {
		case *ast.Kit:
			for _, a := range n.Aliases {
				sc.kit[a.Name] = a.Value
			}
		case *ast.For:
			collectKits(n.Body, sc)
		case *ast.If:
			collectKits(n.Then, sc)
			collectKits(n.Else, sc)
		}
	}
}

func playableRefs(p ast.Playable) []string {
	switch n := p.(type) {
	case *ast.NoteRef:
		return []string{n.Text}
	case *ast.Group:
		var out []string
		for _, v := range n.Voices {
			out = append(out, playableRefs(v)...)
		}
		return out
	}
	return nil
}

// ---------------------------------------------------------------------------
// Statement bodies
// ---------------------------------------------------------------------------

func (e *elab) elabBody(body []ast.Stmt, sc *scope, vel int) {
	// Pre-register track/body-local pattern definitions so calls can appear
	// before the definition and so patterns are visible to the whole body.
	for _, st := range body {
		if pd, ok := st.(*ast.PatternDef); ok && pd != nil {
			sc.patterns[pd.Name] = pd
		}
	}
	for _, st := range body {
		e.elabStmt(st, sc, vel)
	}
}

func (e *elab) elabStmt(st ast.Stmt, sc *scope, vel int) {
	switch n := st.(type) {
	case *ast.Kit:
		for _, a := range n.Aliases {
			sc.kit[a.Name] = a.Value
		}
	case *ast.Let:
		v, err := value.Eval(n.Value, sc.env)
		if err != nil {
			e.errs = append(e.errs, err)
			return
		}
		sc.env.Set(n.Name, v)
	case *ast.Bar:
		e.elabBar(n, sc, vel)
	case *ast.For:
		e.elabFor(n, sc, vel)
	case *ast.If:
		e.elabIf(n, sc, vel)
	case *ast.PatternCall:
		e.elabPatternCall(n, sc, vel)
	case *ast.SettingStmt:
		e.applyTrackSetting(n.Setting)
	case *ast.Swing:
		pct, err := value.EvalNumber(n.Percent, sc.env)
		if err != nil {
			e.errs = append(e.errs, err)
			return
		}
		e.swing = pct / 100.0
	case *ast.Meta:
		e.emitMeta(e.trackOffset, n)
	case *ast.PatternDef:
		// already registered by elabBody's pre-pass; nothing to emit
	case *ast.Absolute:
		// `on beat N <event>` at track-body level: place within the current
		// track offset (treated as the current bar position).
		beat, err := value.EvalNumber(n.Beat, sc.env)
		if err != nil {
			e.errs = append(e.errs, err)
			return
		}
		at := e.trackOffset + uint32((beat-1)*float64(PPQ))
		switch ev := n.Event.(type) {
		case *ast.Step:
			gate := durTicks(4)
			if ev.HasGate {
				gate = durTicks(ev.Gate)
			}
			v := vel
			if ev.Velocity != nil {
				v = velocityValue(ev.Velocity)
			}
			if v < 0 {
				v = 64
			}
			e.playNote(ev.Play, sc, at, at+gate, uint8(v))
		default:
			e.elabEventStmt(n.Event, sc, at, e.trackChan, vel)
		}
	default:
		// Raw event statement at track level: placed at the current offset.
		e.elabEventStmt(st, sc, e.trackOffset, e.trackChan, vel)
	}
}

func (e *elab) applyTrackSetting(s ast.Setting) {
	switch s.Kind {
	case ast.SettingTime:
		e.timeBeats = s.TimeBeats
		e.timeUnit = s.TimeUnit
	case ast.SettingBPM:
		// A mid-song tempo change is not representable in the per-song header;
		// the first project-scope bpm wins. Ignore here for determinism.
	}
}

func (e *elab) elabPatternCall(call *ast.PatternCall, sc *scope, vel int) {
	pd, ok := sc.lookupPattern(call.Name)
	if !ok {
		e.errorf(call.Position, "undefined pattern %q", call.Name)
		return
	}
	if len(call.Args) != len(pd.Params) {
		e.errorf(call.Position, "pattern %q expects %d args, got %d", call.Name, len(pd.Params), len(call.Args))
		return
	}
	inner := newScope(sc)
	for i, p := range pd.Params {
		v, err := value.Eval(call.Args[i], sc.env)
		if err != nil {
			e.errs = append(e.errs, err)
			return
		}
		inner.env.Set(p, v)
	}
	e.elabBody(pd.Body, inner, vel)
}

func (e *elab) elabFor(n *ast.For, sc *scope, vel int) {
	items, err := value.Iterate(n.Iterable, sc.env)
	if err != nil {
		e.errs = append(e.errs, err)
		return
	}
	for _, item := range items {
		inner := newScope(sc)
		inner.env.Set(n.Var, item)
		e.elabBody(n.Body, inner, vel)
	}
}

func (e *elab) elabIf(n *ast.If, sc *scope, vel int) {
	cond, err := value.Eval(n.Cond, sc.env)
	if err != nil {
		e.errs = append(e.errs, err)
		return
	}
	if cond.Kind != value.KindBool {
		e.errorf(n.Position, "if condition is not boolean")
		return
	}
	if cond.Bool {
		e.elabBody(n.Then, newScope(sc), vel)
		return
	}
	if n.ElseIf != nil {
		e.elabIf(n.ElseIf, sc, vel)
		return
	}
	if n.Else != nil {
		e.elabBody(n.Else, newScope(sc), vel)
	}
}

// ---------------------------------------------------------------------------
// Bars and the step grid (docs §3a)
// ---------------------------------------------------------------------------

func (e *elab) elabBar(bar *ast.Bar, sc *scope, parentVel int) {
	baseGrid := 4 // default grid = quarter
	if bar.HasGrid {
		baseGrid = bar.Grid
	}
	barVel := parentVel
	if bar.Velocity != nil {
		barVel = velocityValue(bar.Velocity)
	}

	barLen := uint32(e.timeBeats) * durTicks(e.timeUnit)
	start := e.trackOffset

	bc := &barCtx{
		e:        e,
		sc:       sc,
		start:    start,
		curStep:  baseGrid,
		baseGrid: baseGrid,
		barVel:   barVel,
		swing:    e.swing,
	}
	bc.run(bar.Items)

	// Bar overflow check (docs §3a): summed advances must not exceed one bar.
	if bc.cursor > barLen {
		e.errorf(bar.Position, "bar overflows: %d ticks of steps exceed bar length %d", bc.cursor, barLen)
	}

	e.trackOffset = start + barLen
}

// barCtx carries the mutable cursor while walking one bar's items.
type barCtx struct {
	e        *elab
	sc       *scope
	start    uint32 // absolute tick where the bar begins
	cursor   uint32 // within-bar tick of the next step
	curStep  int    // current grid step note-value
	baseGrid int    // bar's base grid (BarSep / "|" resets to this)
	barVel   int
	swing    float64 // swing ratio (0.5 = straight) for this bar

	// lastNoteOffs holds the NoteOff events of the previous sounding step so a
	// tilde can extend their gate.
	lastNoteOffs []int // indices into song.Events
}

func (bc *barCtx) run(items []ast.BarItem) {
	for _, it := range items {
		switch n := it.(type) {
		case *ast.GridSwitch:
			bc.curStep = n.Grid
		case *ast.BarSep:
			bc.curStep = bc.baseGrid
		case *ast.Step:
			bc.step(n)
		case *ast.Absolute:
			bc.absolute(n)
		case *ast.Meta:
			bc.e.emitMeta(bc.start+bc.cursor, n)
		default:
			// Raw event statement in a bar slot: emit at cursor, advance one step.
			bc.e.elabEventStmt(it.(ast.Stmt), bc.sc, bc.start+bc.cursor, bc.e.trackChan, bc.barVel)
			bc.cursor += durTicks(bc.curStep)
			bc.lastNoteOffs = nil
		}
	}
}

func (bc *barCtx) step(st *ast.Step) {
	repeat := st.Repeat
	if repeat < 1 {
		repeat = 1
	}
	for i := 0; i < repeat; i++ {
		bc.oneStep(st)
	}
}

func (bc *barCtx) oneStep(st *ast.Step) {
	stepLen := durTicks(bc.curStep)

	switch st.Play.(type) {
	case *ast.Rest:
		bc.cursor += stepLen
		bc.lastNoteOffs = nil
		return
	case *ast.Tie:
		// Extend the previous step's gate by one grid step, then advance.
		for _, idx := range bc.lastNoteOffs {
			bc.e.song.Events[idx].Tick += stepLen
		}
		bc.cursor += stepLen
		return
	}

	gate := stepLen
	if st.HasGate {
		gate = durTicks(st.Gate)
	}

	// velocity precedence: per-step > bar default > track default > 64
	vel := bc.barVel
	if st.Velocity != nil {
		vel = velocityValue(st.Velocity)
	}
	if vel < 0 {
		vel = 64
	}

	onTick := bc.start + bc.cursor + bc.swingDelay(stepLen)
	offTick := onTick + gate

	bc.lastNoteOffs = bc.e.playNote(st.Play, bc.sc, onTick, offTick, uint8(vel))
	bc.cursor += stepLen
}

// swingDelay returns how far to push this step's onset for swing feel. With a
// swing ratio s, each pair of steps becomes long+short: the on-beat (even step
// index at the current grid) keeps its place, and the off-beat (odd index) is
// delayed by (2s-1)*stepLen so it lands later in the pair. s=0.5 is straight,
// so the delay is zero. Only whole, aligned grid steps swing; the offset never
// pushes a step past the on-beat that follows it.
func (bc *barCtx) swingDelay(stepLen uint32) uint32 {
	if bc.swing == 0.5 || stepLen == 0 {
		return 0
	}
	if (bc.cursor/stepLen)%2 == 0 {
		return 0 // on-beat
	}
	return uint32((2*bc.swing - 1) * float64(stepLen))
}

func (bc *barCtx) absolute(n *ast.Absolute) {
	beat, err := value.EvalNumber(n.Beat, bc.sc.env)
	if err != nil {
		bc.e.errs = append(bc.e.errs, err)
		return
	}
	// beat N -> (N-1)*PPQ within the bar; does not move the cursor.
	at := bc.start + uint32((beat-1)*float64(PPQ))

	switch ev := n.Event.(type) {
	case *ast.Step:
		gate := durTicks(4)
		if ev.HasGate {
			gate = durTicks(ev.Gate)
		}
		vel := bc.barVel
		if ev.Velocity != nil {
			vel = velocityValue(ev.Velocity)
		}
		if vel < 0 {
			vel = 64
		}
		bc.e.playNote(ev.Play, bc.sc, at, at+gate, uint8(vel))
	default:
		bc.e.elabEventStmt(n.Event, bc.sc, at, bc.e.trackChan, bc.barVel)
	}
}

// ---------------------------------------------------------------------------
// Playables -> note on/off events
// ---------------------------------------------------------------------------

// playNote resolves a playable to pitches and emits NoteOn at onTick / NoteOff
// at offTick. It returns the song.Events indices of the emitted NoteOffs so a
// following tie can extend their gate.
func (e *elab) playNote(p ast.Playable, sc *scope, onTick, offTick uint32, vel uint8) []int {
	var offs []int
	emitPitch := func(ch uint8, key uint8) {
		e.emit(onTick, MIDIMsg{Kind: MsgNoteOn, Channel: ch, Key: key, Velocity: vel})
		e.emit(offTick, MIDIMsg{Kind: MsgNoteOff, Channel: ch, Key: key})
		offs = append(offs, len(e.song.Events)-1)
	}

	switch n := p.(type) {
	case *ast.NoteRef:
		ch := e.trackChan
		if n.Channel >= 0 {
			ch = clampChan(n.Channel)
		}
		keys, ok := e.resolveNoteRef(n, sc)
		if !ok {
			return nil
		}
		for _, k := range keys {
			emitPitch(ch, k)
		}
	case *ast.ExprPlay:
		ch := e.trackChan
		if n.Channel >= 0 {
			ch = clampChan(n.Channel)
		}
		v, err := value.Eval(n.Value, sc.env)
		if err != nil {
			e.errs = append(e.errs, err)
			return nil
		}
		keys, ok := v.Keys()
		if !ok {
			e.errorf(n.Position, "expression is not playable as a note")
			return nil
		}
		for _, k := range keys {
			emitPitch(ch, k)
		}
	case *ast.Group:
		for _, voice := range n.Voices {
			offs = append(offs, e.playNote(voice, sc, onTick, offTick, vel)...)
		}
	case *ast.Rest, *ast.Tie:
		// handled by the caller
	default:
		e.errorf(p.Pos(), "unsupported playable %T", p)
	}
	return offs
}

// resolveNoteRef resolves a NoteRef.Text to MIDI keys, following the order:
// kit alias -> let/loop binding -> percussion -> note -> chord.
func (e *elab) resolveNoteRef(n *ast.NoteRef, sc *scope) ([]uint8, bool) {
	if val, ok := sc.lookupKit(n.Text); ok {
		if key, err := lmidi.PercussionKeyMap(val); err == nil {
			return []uint8{key}, true
		}
		if keys, ok := resolvePitch(val); ok {
			return keys, true
		}
		e.errorf(n.Position, "kit alias %q -> %q is not a known percussion/note/chord", n.Text, val)
		return nil, false
	}

	if v, ok := sc.env.Lookup(n.Text); ok {
		keys, ok := v.Keys()
		if !ok {
			e.errorf(n.Position, "binding %q is not playable as a note", n.Text)
			return nil, false
		}
		return keys, true
	}

	if key, err := lmidi.PercussionKeyMap(n.Text); err == nil {
		return []uint8{key}, true
	}
	if keys, ok := resolvePitch(n.Text); ok {
		return keys, true
	}
	e.errorf(n.Position, "cannot resolve %q to a note, chord, or percussion", n.Text)
	return nil, false
}

// resolvePitch turns a text token into MIDI keys, resolving the note-vs-chord
// ambiguity. Tokens like "C7" are valid as both a note (C in octave 7) and a
// chord (C dominant 7); the chord reading wins, matching musical intent. A
// "plain note" — a letter, accidentals, and an optional low octave digit (0-4),
// which is never a chord name — stays a note. A trailing "^" forces the note
// reading as an escape hatch for high-octave pitches (e.g. "C7^").
func resolvePitch(text string) ([]uint8, bool) {
	// Escape hatch: trailing '^' forces the note interpretation.
	if strings.HasSuffix(text, "^") {
		forced := strings.TrimSuffix(text, "^")
		if note, err := notes.Parse(forced); err == nil {
			return []uint8{note.MIDI()}, true
		}
		return nil, false
	}

	noteOK := false
	var noteKey uint8
	if note, err := notes.Parse(text); err == nil {
		noteOK = true
		noteKey = note.MIDI()
	}
	chordOK := false
	var chordVal *chords.Chord
	if ch, err := chords.Parse(text); err == nil {
		chordOK = true
		chordVal = ch
	}

	switch {
	case noteOK && chordOK:
		// Ambiguous (e.g. C, C5, C6, C7). Prefer the note only when it is a
		// "plain pitch": a bare letter+accidentals, or with a low octave digit
		// (0-4) that is never also a chord quality. Otherwise the chord wins.
		if isPlainNote(text) {
			return []uint8{noteKey}, true
		}
		return chordKeys(chordVal), true
	case chordOK:
		return chordKeys(chordVal), true
	case noteOK:
		return []uint8{noteKey}, true
	}
	return nil, false
}

// isPlainNote reports whether text is unambiguously a pitch: a note letter,
// optional accidentals (# or b), and at most a single octave digit 0-4. The
// bare-number chord qualities (5, 6, 7) and multi-digit forms (9, 11, 13) are
// excluded, so "C"/"Eb"/"C4" are notes but "C7"/"C13" fall through to a chord.
func isPlainNote(text string) bool {
	if text == "" {
		return false
	}
	i := 0
	if text[0] < 'A' || text[0] > 'G' {
		return false
	}
	i++
	for i < len(text) && (text[i] == '#' || text[i] == 'b') {
		i++
	}
	switch len(text) - i {
	case 0:
		return true // bare letter, e.g. C, Eb
	case 1:
		return text[i] >= '0' && text[i] <= '4' // low octave, never a chord
	default:
		return false
	}
}

func chordKeys(c *chords.Chord) []uint8 {
	ns := c.Notes()
	out := make([]uint8, 0, len(ns))
	for i := range ns {
		out = append(out, ns[i].MIDI())
	}
	return out
}

func clampChan(c int) uint8 {
	if c < 0 {
		c = 0
	}
	if c > 15 {
		c = 15
	}
	return uint8(c)
}

// ---------------------------------------------------------------------------
// Raw MIDI / meta event statements
// ---------------------------------------------------------------------------

func (e *elab) emitMeta(tick uint32, m *ast.Meta) {
	e.emit(tick, MIDIMsg{Kind: MsgMeta, MetaKind: m.Kind, Text: m.Value})
}

func (e *elab) elabEventStmt(st ast.Stmt, sc *scope, tick uint32, ch uint8, vel int) {
	switch n := st.(type) {
	case *ast.CC:
		ctrl, err := value.EvalNumber(n.Controller, sc.env)
		if err != nil {
			e.errs = append(e.errs, err)
			return
		}
		val, err := value.EvalNumber(n.Value, sc.env)
		if err != nil {
			e.errs = append(e.errs, err)
			return
		}
		e.emit(tick, MIDIMsg{Kind: MsgCC, Channel: ch, Controller: uint8(ctrl), Value: uint8(val)})
	case *ast.Bend:
		e.elabBend(n, sc, tick, ch)
	case *ast.Pressure:
		val, err := value.EvalNumber(n.Value, sc.env)
		if err != nil {
			e.errs = append(e.errs, err)
			return
		}
		e.emit(tick, MIDIMsg{Kind: MsgPressure, Channel: ch, Value: uint8(val)})
	case *ast.Program_:
		var pc uint8
		if n.HasName {
			p, err := lmidi.InstrumentToPC(n.Name)
			if err != nil {
				e.errorf(n.Position, "unknown instrument %q", n.Name)
				return
			}
			pc = p - 1
		} else {
			pc = uint8(n.Number)
		}
		e.emit(tick, MIDIMsg{Kind: MsgProgram, Channel: ch, Program: pc})
	case *ast.Sysex:
		e.emit(tick, MIDIMsg{Kind: MsgSysex, Bytes: append([]byte(nil), n.Bytes...)})
	case *ast.Meta:
		e.emitMeta(tick, n)
	case *ast.NoteRef, *ast.ExprPlay, *ast.Group:
		if vel < 0 {
			vel = 64
		}
		e.playNote(st.(ast.Playable), sc, tick, tick+durTicks(4), uint8(vel))
	default:
		e.errorf(st.Pos(), "unsupported event statement %T", st)
	}
}

func (e *elab) elabBend(n *ast.Bend, sc *scope, tick uint32, ch uint8) {
	switch n.Mode {
	case ast.BendRaw:
		raw, err := value.EvalNumber(n.Value, sc.env)
		if err != nil {
			e.errs = append(e.errs, err)
			return
		}
		// raw 14-bit value 0..16383 (8192 = center); Bend wants -8192..8191.
		e.emit(tick, MIDIMsg{Kind: MsgPitchBend, Channel: ch, Bend: int16(int(raw) - 8192)})
	case ast.BendRange:
		semis, err := value.EvalNumber(n.Value, sc.env)
		if err != nil {
			e.errs = append(e.errs, err)
			return
		}
		e.bendRange = uint8(semis)
		e.bendRangeRP = true
		e.emitBendRangeRPN(tick, ch, uint8(semis))
	case ast.BendSemitones:
		semis, err := value.EvalNumber(n.Value, sc.env)
		if err != nil {
			e.errs = append(e.errs, err)
			return
		}
		// Lazily emit the RPN pitch-bend-range once per track (default ±2).
		if !e.bendRangeRP {
			e.emitBendRangeRPN(tick, ch, e.bendRange)
			e.bendRangeRP = true
		}
		rng := float64(e.bendRange)
		if rng == 0 {
			rng = 2
		}
		val := int16(semis / rng * 8192.0)
		if val > 8191 {
			val = 8191
		}
		if val < -8192 {
			val = -8192
		}
		e.emit(tick, MIDIMsg{Kind: MsgPitchBend, Channel: ch, Bend: val})
	}
}

// emitBendRangeRPN sets pitch-bend sensitivity via RPN 0
// (CC101=0, CC100=0, CC6=semitones, CC38=0).
func (e *elab) emitBendRangeRPN(tick uint32, ch uint8, semitones uint8) {
	e.emit(tick, MIDIMsg{Kind: MsgCC, Channel: ch, Controller: 101, Value: 0})
	e.emit(tick, MIDIMsg{Kind: MsgCC, Channel: ch, Controller: 100, Value: 0})
	e.emit(tick, MIDIMsg{Kind: MsgCC, Channel: ch, Controller: 6, Value: semitones})
	e.emit(tick, MIDIMsg{Kind: MsgCC, Channel: ch, Controller: 38, Value: 0})
}

// ---------------------------------------------------------------------------
// Velocity
// ---------------------------------------------------------------------------

// velocityValue resolves a *ast.Velocity to a 0..127 value, or -1 for nil.
func velocityValue(v *ast.Velocity) int {
	if v == nil {
		return -1
	}
	if v.HasNumber {
		return v.Number
	}
	if val, ok := value.DynamicVelocity[v.Dynamic]; ok {
		return val
	}
	return 64
}

// ---------------------------------------------------------------------------
// Finalize: sort the event stream deterministically
// ---------------------------------------------------------------------------

func (e *elab) finalize() {
	evs := e.song.Events
	sort.SliceStable(evs, func(i, j int) bool {
		if evs[i].Tick != evs[j].Tick {
			return evs[i].Tick < evs[j].Tick
		}
		if evs[i].Track != evs[j].Track {
			return evs[i].Track < evs[j].Track
		}
		// NoteOff before NoteOn at equal tick.
		oi, oj := offRank(evs[i].Msg), offRank(evs[j].Msg)
		if oi != oj {
			return oi < oj
		}
		return evs[i].order < evs[j].order
	})
}

func offRank(m MIDIMsg) int {
	if m.Kind == MsgNoteOff {
		return 0
	}
	return 1
}
