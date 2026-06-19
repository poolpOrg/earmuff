// Package midiimport turns a Standard MIDI File into earmuff (.ear) source —
// the inverse of the parse -> elaborate -> smfwriter pipeline.
//
// MIDI is a flat, absolute-tick event stream; earmuff source is structured
// (projects, tracks, bars, named notes/chords, durations). Reconstructing
// readable source is therefore heuristic and lossy. Two modes trade fidelity
// for readability:
//
//   - Readable (default): note onsets and gates are quantized to a step grid,
//     producing clean `bar N { ... }` measures. Timing may drift slightly.
//   - Faithful (Options.Faithful): every note is placed with `on beat <exact>`
//     so re-compiling the output reproduces the original timing.
//
// In both modes simultaneous notes are grouped and named as chords when a name
// fits (via go-harmony), falling back to an explicit (a, b, c) note group.
package midiimport

import (
	"bytes"
	"fmt"
	"math"
	"sort"
	"strings"

	"github.com/poolpOrg/earmuff/midi"
	"github.com/poolpOrg/go-harmony/chords"
	"github.com/poolpOrg/go-harmony/notes"
	gm "gitlab.com/gomidi/midi/v2/smf"
)

// PPQ is earmuff's ticks-per-quarter; the importer rescales the source file's
// resolution to this so the emitted durations line up with the language.
const PPQ = 960

// Options controls how the MIDI is rendered to source.
type Options struct {
	// Faithful emits exact `on beat` placements instead of a quantized grid.
	Faithful bool
	// Grid is the quantization grid in the readable mode, as a note value
	// (16 = sixteenth note). Ignored when Faithful. Defaults to 16.
	Grid int
	// Name is the project name; defaults to a name derived from the file.
	Name string
}

// Import reads Standard MIDI File bytes and returns earmuff source.
func Import(data []byte, opts Options) (string, error) {
	if opts.Grid == 0 {
		opts.Grid = 16
	}
	if opts.Name == "" {
		opts.Name = "imported"
	}

	s, err := gm.ReadFrom(bytes.NewReader(data))
	if err != nil {
		return "", fmt.Errorf("read MIDI: %w", err)
	}
	mt, ok := s.TimeFormat.(gm.MetricTicks)
	if !ok {
		return "", fmt.Errorf("unsupported MIDI time format (SMPTE time code)")
	}
	srcPPQ := uint32(mt.Resolution())
	if srcPPQ == 0 {
		srcPPQ = PPQ
	}

	hdr := readHeader(s)
	tracks := readTracks(s, srcPPQ)
	if len(tracks) == 0 {
		return "", fmt.Errorf("no playable tracks found")
	}

	return render(opts, hdr, tracks), nil
}

// ---------------------------------------------------------------------------
// Reading
// ---------------------------------------------------------------------------

type header struct {
	bpm       float64
	beats     int
	unit      int
	copyright string
	texts     []string
}

func readHeader(s *gm.SMF) header {
	h := header{bpm: 120, beats: 4, unit: 4}
	for _, tr := range s.Tracks {
		for _, ev := range tr {
			m := ev.Message
			var bpm float64
			var num, den uint8
			var txt string
			switch {
			case m.GetMetaTempo(&bpm):
				h.bpm = bpm
			case m.GetMetaMeter(&num, &den):
				if num > 0 && den > 0 {
					h.beats, h.unit = int(num), int(den)
				}
			case m.GetMetaCopyright(&txt):
				h.copyright = txt
			}
		}
	}
	return h
}

// timedNote is one sounded note with absolute onset and gate, both in earmuff
// ticks (PPQ).
type timedNote struct {
	onset uint32
	gate  uint32
	key   uint8
	vel   uint8
}

type track struct {
	name       string
	instrument string
	channel    uint8 // 0-based; 9 == GM percussion
	percussion bool
	notes      []timedNote
}

func readTracks(s *gm.SMF, srcPPQ uint32) []track {
	var out []track
	for _, raw := range s.Tracks {
		tr := track{}
		open := map[uint8][]uint32{} // key -> stack of onset ticks (earmuff scale)
		var abs uint32               // absolute ticks, source scale
		for _, ev := range raw {
			abs += ev.Delta
			et := rescale(abs, srcPPQ) // earmuff ticks
			m := ev.Message
			var ch, key, vel uint8
			var name string
			switch {
			case m.GetNoteOn(&ch, &key, &vel) && vel > 0:
				tr.channel = ch
				open[key] = append(open[key], et)
			case m.GetNoteOff(&ch, &key, &vel) || (m.GetNoteOn(&ch, &key, &vel) && vel == 0):
				st := open[key]
				if len(st) == 0 {
					break
				}
				onset := st[len(st)-1]
				open[key] = st[:len(st)-1]
				gate := et - onset
				if gate == 0 {
					gate = 1
				}
				tr.notes = append(tr.notes, timedNote{onset: onset, gate: gate, key: key, vel: vel})
			case m.GetMetaTrackName(&name), m.GetMetaInstrument(&name):
				if tr.name == "" {
					tr.name = name
				}
			}
		}
		if len(tr.notes) == 0 {
			continue
		}
		sort.SliceStable(tr.notes, func(i, j int) bool {
			if tr.notes[i].onset != tr.notes[j].onset {
				return tr.notes[i].onset < tr.notes[j].onset
			}
			return tr.notes[i].key < tr.notes[j].key
		})
		tr.percussion = tr.channel == 9
		tr.instrument = instrumentFor(s, tr)
		if tr.name == "" {
			tr.name = fmt.Sprintf("track %d", len(out)+1)
		}
		out = append(out, tr)
	}
	return out
}

// instrumentFor resolves a track's instrument from its first program change,
// else a sensible default ("synth drum" for percussion, "piano" otherwise).
func instrumentFor(s *gm.SMF, tr track) string {
	if tr.percussion {
		return "synth drum"
	}
	for _, raw := range s.Tracks {
		var abs uint32
		for _, ev := range raw {
			abs += ev.Delta
			var ch, pc uint8
			if ev.Message.GetProgramChange(&ch, &pc) && ch == tr.channel {
				if name, err := midi.PCToInstrument(pc + 1); err == nil {
					return name
				}
			}
		}
	}
	return "piano"
}

func rescale(t, srcPPQ uint32) uint32 {
	if srcPPQ == PPQ {
		return t
	}
	return uint32(uint64(t) * PPQ / uint64(srcPPQ))
}

// ---------------------------------------------------------------------------
// Rendering
// ---------------------------------------------------------------------------

func render(opts Options, h header, tracks []track) string {
	var b strings.Builder
	fmt.Fprintf(&b, "project %q {\n", opts.Name)
	fmt.Fprintf(&b, "    bpm %s; time %d %d;\n", trimFloat(h.bpm), h.beats, h.unit)
	if h.copyright != "" {
		fmt.Fprintf(&b, "    copyright %q;\n", h.copyright)
	}
	for ti := range tracks {
		b.WriteString("\n")
		renderTrack(&b, opts, h, &tracks[ti])
	}
	b.WriteString("}\n")
	return b.String()
}

func renderTrack(b *strings.Builder, opts Options, h header, tr *track) {
	chanSuffix := ""
	if tr.percussion {
		chanSuffix = " channel 10"
	}
	fmt.Fprintf(b, "    track %q instrument %q%s {\n", tr.name, tr.instrument, chanSuffix)

	if tr.percussion {
		renderPercussionKit(b, tr)
	}

	ticksPerBar := uint32(h.beats) * (PPQ * 4 / uint32(h.unit))
	if opts.Faithful {
		renderFaithful(b, h, tr, ticksPerBar)
	} else {
		renderGridded(b, opts, h, tr, ticksPerBar)
	}
	b.WriteString("    }\n")
}

// renderFaithful places every chord group at its exact beat with `on beat`,
// inside one bar per measure, so the output recompiles to the same timing.
func renderFaithful(b *strings.Builder, h header, tr *track, ticksPerBar uint32) {
	groups := groupChords(tr.notes)
	beatTicks := PPQ * 4 / uint32(h.unit)
	byBar := map[uint32][]chordGroup{}
	var bars []uint32
	for _, g := range groups {
		bar := g.onset / ticksPerBar
		if _, ok := byBar[bar]; !ok {
			bars = append(bars, bar)
		}
		byBar[bar] = append(byBar[bar], g)
	}
	sort.Slice(bars, func(i, j int) bool { return bars[i] < bars[j] })
	for _, bar := range bars {
		b.WriteString("        bar {\n")
		for _, g := range byBar[bar] {
			within := g.onset - bar*ticksPerBar
			beat := 1 + float64(within)/float64(beatTicks)
			gateVal := nearestNoteValue(g.gate)
			// `on beat <n> <playable>:<gate>` — a bar item, space-separated, no
			// `play` keyword and no terminating `;`. `on beat` does not accept a
			// parenthesized (a, b) group, so an unnamed simultaneity is emitted
			// as one `on beat` line per voice (they share the beat and sound
			// together); a named chord stays a single token.
			for _, tok := range faithfulTokens(tr, g) {
				fmt.Fprintf(b, "            on beat %s %s:%d\n", trimFloat(beat), tok, gateVal)
			}
		}
		b.WriteString("        }\n")
	}
}

// renderGridded quantizes onsets to a step grid and lays one `bar grid { ... }`
// per measure, filling rests where nothing sounds.
func renderGridded(b *strings.Builder, opts Options, h header, tr *track, ticksPerBar uint32) {
	step := PPQ * 4 / uint32(opts.Grid)
	if step == 0 {
		step = PPQ / 4
	}
	stepsPerBar := int(ticksPerBar / step)
	if stepsPerBar < 1 {
		stepsPerBar = 1
	}

	groups := groupChords(tr.notes)
	// Map each group to its nearest grid slot (absolute step index).
	slots := map[int]chordGroup{}
	maxStep := 0
	for _, g := range groups {
		idx := int(math.Round(float64(g.onset) / float64(step)))
		if _, taken := slots[idx]; !taken { // first wins on collision
			slots[idx] = g
		}
		if idx > maxStep {
			maxStep = idx
		}
	}

	totalBars := maxStep/stepsPerBar + 1
	for bar := 0; bar < totalBars; bar++ {
		fmt.Fprintf(b, "        bar %d {", opts.Grid)
		for s := 0; s < stepsPerBar; s++ {
			idx := bar*stepsPerBar + s
			if g, ok := slots[idx]; ok {
				tok := playable(tr, g)
				if gateSteps := int(math.Round(float64(g.gate) / float64(step))); gateSteps > 1 {
					// Sounding longer than one step: set an explicit gate.
					tok += fmt.Sprintf(":%d", nearestNoteValue(g.gate))
				}
				b.WriteString(" " + tok)
			} else {
				b.WriteString(" _")
			}
		}
		b.WriteString(" }\n")
	}
}

// renderPercussionKit emits a `kit { ... }` aliasing every percussion key the
// track uses to its GM name, so the bars can refer to short aliases.
func renderPercussionKit(b *strings.Builder, tr *track) {
	seen := map[uint8]string{}
	var keys []uint8
	for _, n := range tr.notes {
		if _, ok := seen[n.key]; ok {
			continue
		}
		name, err := midi.KeyToPercussion(n.key)
		if err != nil {
			continue
		}
		seen[n.key] = name
		keys = append(keys, n.key)
	}
	if len(keys) == 0 {
		return
	}
	sort.Slice(keys, func(i, j int) bool { return keys[i] < keys[j] })
	b.WriteString("        kit {")
	for _, k := range keys {
		fmt.Fprintf(b, " %s = %q;", percAlias(seen[k]), seen[k])
	}
	b.WriteString(" }\n")
}

// ---------------------------------------------------------------------------
// Chord grouping + naming
// ---------------------------------------------------------------------------

type chordGroup struct {
	onset uint32
	gate  uint32
	keys  []uint8
}

// groupChords collapses notes that share an onset into a single group whose
// gate is the longest member's gate.
func groupChords(ns []timedNote) []chordGroup {
	var groups []chordGroup
	i := 0
	for i < len(ns) {
		j := i
		g := chordGroup{onset: ns[i].onset, gate: ns[i].gate}
		for j < len(ns) && ns[j].onset == ns[i].onset {
			g.keys = append(g.keys, ns[j].key)
			if ns[j].gate > g.gate {
				g.gate = ns[j].gate
			}
			j++
		}
		groups = append(groups, g)
		i = j
	}
	return groups
}

// playable renders a chord group as an earmuff token: a percussion alias, a
// single note name, a named chord when one fits, or an explicit note group.
func playable(tr *track, g chordGroup) string {
	if tr.percussion {
		if len(g.keys) == 1 {
			if name, err := midi.KeyToPercussion(g.keys[0]); err == nil {
				return percAlias(name)
			}
		}
		var parts []string
		for _, k := range g.keys {
			if name, err := midi.KeyToPercussion(k); err == nil {
				parts = append(parts, percAlias(name))
			}
		}
		if len(parts) == 1 {
			return parts[0]
		}
		return "(" + strings.Join(parts, ", ") + ")"
	}

	if len(g.keys) == 1 {
		return noteName(g.keys[0])
	}
	if name := chordName(g.keys); name != "" {
		return name
	}
	var parts []string
	for _, k := range g.keys {
		parts = append(parts, noteName(k))
	}
	return "(" + strings.Join(parts, ", ") + ")"
}

// faithfulTokens returns the token(s) to place at one beat. A single note,
// percussion alias, or named chord is one token; an unnamed simultaneity is
// split into one token per voice (since `on beat` rejects a (a, b) group).
func faithfulTokens(tr *track, g chordGroup) []string {
	tok := playable(tr, g)
	if strings.HasPrefix(tok, "(") {
		var out []string
		for _, k := range g.keys {
			if tr.percussion {
				if name, err := midi.KeyToPercussion(k); err == nil {
					out = append(out, percAlias(name))
				}
			} else {
				out = append(out, noteName(k))
			}
		}
		return out
	}
	return []string{tok}
}

var pcNames = []string{"C", "C#", "D", "D#", "E", "F", "F#", "G", "G#", "A", "A#", "B"}

// noteName renders a MIDI key as an earmuff note with octave (C4 == 60).
func noteName(key uint8) string {
	pc := pcNames[key%12]
	oct := int(key)/12 - 1
	return fmt.Sprintf("%s%d", pc, oct)
}

// chordName tries to name a pitch set with go-harmony, returning "" unless the
// name round-trips back to the same pitch classes (so we never emit a bogus
// name that would re-parse to something else).
func chordName(keys []uint8) string {
	if len(keys) < 3 {
		return "" // dyads rarely have a clean single name; prefer a group
	}
	var ns []notes.Note
	for _, k := range keys {
		n, err := notes.Parse(noteName(k))
		if err != nil {
			return ""
		}
		ns = append(ns, *n)
	}
	ch := chords.FromNotes(ns)
	name := ch.Name()
	if name == "" {
		return ""
	}
	if !chordRoundTrips(name, keys) {
		return ""
	}
	return name
}

// chordRoundTrips reports whether parsing name yields the same set of pitch
// classes as keys, so a named chord is only trusted when it is unambiguous.
func chordRoundTrips(name string, keys []uint8) bool {
	parsed, err := chords.Parse(name)
	if err != nil {
		return false
	}
	want := map[uint8]bool{}
	for _, k := range keys {
		want[k%12] = true
	}
	got := map[uint8]bool{}
	for _, n := range parsed.Notes() {
		got[n.MIDI()%12] = true
	}
	if len(want) != len(got) {
		return false
	}
	for pc := range want {
		if !got[pc] {
			return false
		}
	}
	return true
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// nearestNoteValue maps a tick gate to the closest plain note value (1=whole,
// 2=half, 4=quarter, ...), used for :gate suffixes and `on beat` durations.
func nearestNoteValue(gate uint32) int {
	values := []int{1, 2, 4, 8, 16, 32, 64}
	best, bestDiff := 4, math.MaxFloat64
	for _, v := range values {
		t := float64(PPQ*4) / float64(v)
		if d := math.Abs(float64(gate) - t); d < bestDiff {
			bestDiff, best = d, v
		}
	}
	return best
}

// percAlias makes a short kit alias from a percussion name (initials of the
// first words), e.g. "closed hi-hat" -> "chh", "acoustic snare" -> "as".
func percAlias(name string) string {
	fields := strings.Fields(strings.ToLower(name))
	var a strings.Builder
	for _, f := range fields {
		a.WriteByte(f[0])
	}
	s := a.String()
	if s == "" {
		return "x"
	}
	return s
}

func trimFloat(f float64) string {
	if f == math.Trunc(f) {
		return fmt.Sprintf("%d", int(f))
	}
	return strings.TrimRight(strings.TrimRight(fmt.Sprintf("%.3f", f), "0"), ".")
}
