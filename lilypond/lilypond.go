// Package lilypond renders an elaborated Song into LilyPond source (.ly), which
// the lilypond engraver turns into a PDF or SVG score.
//
// earmuff's runtime model is performance events (absolute ticks + gates), not
// notation. Converting that to engraved notation is inherently lossy, so this
// emitter targets the common, grid-aligned cases: it quantizes note start times
// and durations to standard note values, lays one staff per track, and inserts
// rests for gaps. Durations that don't map cleanly are rounded to the nearest
// representable value rather than rendered as tuplets.
package lilypond

import (
	"fmt"
	"sort"
	"strings"

	"github.com/poolpOrg/earmuff/elaborator"
)

const ppq = elaborator.PPQ // 960 ticks per quarter note

// Render returns LilyPond source for song.
func Render(song elaborator.Song) string {
	beats, unit := song.TimeBeats, song.TimeUnit
	if beats == 0 {
		beats = 4
	}
	if unit == 0 {
		unit = 4
	}
	ticksPerBar := uint32(beats) * (ppq * 4 / uint32(unit))

	var b strings.Builder
	fmt.Fprintf(&b, "\\version \"2.24.0\"\n\n")
	title := song.Name
	if title == "" {
		title = "earmuff"
	}
	fmt.Fprintf(&b, "\\header {\n  title = %s\n", quote(title))
	if song.Copyright != "" {
		fmt.Fprintf(&b, "  copyright = %s\n", quote(song.Copyright))
	}
	fmt.Fprintf(&b, "  tagline = ##f\n}\n\n")

	fmt.Fprintf(&b, "\\score {\n  <<\n")
	for i, tr := range song.Tracks {
		notes := collectNotes(song, i)
		staff := renderStaff(tr.Name, notes, beats, unit, ticksPerBar, song.BPM, i == 0)
		b.WriteString(staff)
	}
	fmt.Fprintf(&b, "  >>\n  \\layout { }\n}\n")
	return b.String()
}

// note is one sounding note: when it starts, its pitch, and how long it lasts.
type note struct {
	tick uint32
	key  uint8
	dur  uint32
}

// collectNotes pairs NoteOn/NoteOff events for one track into notes.
func collectNotes(song elaborator.Song, track int) []note {
	type pending struct {
		tick uint32
		idx  int
	}
	open := map[uint8]pending{}
	var notes []note
	for _, ev := range song.Events {
		if ev.Track != track {
			continue
		}
		switch ev.Msg.Kind {
		case elaborator.MsgNoteOn:
			if ev.Msg.Velocity == 0 {
				continue
			}
			notes = append(notes, note{tick: ev.Tick, key: ev.Msg.Key})
			open[ev.Msg.Key] = pending{tick: ev.Tick, idx: len(notes) - 1}
		case elaborator.MsgNoteOff:
			if p, ok := open[ev.Msg.Key]; ok {
				notes[p.idx].dur = ev.Tick - p.tick
				delete(open, ev.Msg.Key)
			}
		}
	}
	// any still-open notes get a quarter-note default
	for _, p := range open {
		if notes[p.idx].dur == 0 {
			notes[p.idx].dur = ppq
		}
	}
	sort.SliceStable(notes, func(i, j int) bool {
		if notes[i].tick != notes[j].tick {
			return notes[i].tick < notes[j].tick
		}
		return notes[i].key < notes[j].key
	})
	return notes
}

// chord groups simultaneous notes sharing a start tick.
type chord struct {
	tick uint32
	dur  uint32
	keys []uint8
}

// groupChords merges notes that start on the same tick into chords (using the
// shortest member duration so the engraving stays simple).
func groupChords(notes []note) []chord {
	var chords []chord
	for _, n := range notes {
		if len(chords) > 0 && chords[len(chords)-1].tick == n.tick {
			c := &chords[len(chords)-1]
			c.keys = append(c.keys, n.key)
			if n.dur < c.dur {
				c.dur = n.dur
			}
			continue
		}
		chords = append(chords, chord{tick: n.tick, dur: n.dur, keys: []uint8{n.key}})
	}
	return chords
}

// renderStaff emits one \new Staff { ... } block.
func renderStaff(name string, notes []note, beats, unit int, ticksPerBar uint32, bpm float64, first bool) string {
	var b strings.Builder
	fmt.Fprintf(&b, "    \\new Staff {\n")
	if name != "" {
		fmt.Fprintf(&b, "      \\set Staff.instrumentName = %s\n", quote(name))
	}
	fmt.Fprintf(&b, "      \\clef %s\n", clefFor(notes))
	fmt.Fprintf(&b, "      \\time %d/%d\n", beats, unit)
	if first && bpm > 0 {
		fmt.Fprintf(&b, "      \\tempo 4 = %d\n", int(bpm+0.5))
	}

	chords := groupChords(notes)
	b.WriteString("      ")
	cursor := uint32(0) // absolute tick we've written up to
	for _, c := range chords {
		// rest to fill the gap before this chord
		if c.tick > cursor {
			writeDurations(&b, c.tick-cursor, "r")
			cursor = c.tick
		} else if c.tick < cursor {
			// overlap (legato/voicing we can't notate simply): skip to keep
			// the bar arithmetic honest.
			continue
		}
		dur := c.dur
		if dur == 0 {
			dur = ppq
		}
		writeChord(&b, c.keys, dur)
		cursor += dur
	}
	// pad the final bar with a rest so it's complete
	if rem := cursor % ticksPerBar; rem != 0 {
		writeDurations(&b, ticksPerBar-rem, "r")
	}
	b.WriteString("\n      \\bar \"|.\"\n    }\n")
	return b.String()
}

// writeChord writes a single note or a <...> chord with quantized duration(s).
func writeChord(b *strings.Builder, keys []uint8, dur uint32) {
	var body string
	if len(keys) == 1 {
		body = pitch(keys[0])
	} else {
		parts := make([]string, len(keys))
		for i, k := range keys {
			parts[i] = pitch(k)
		}
		body = "<" + strings.Join(parts, " ") + ">"
	}
	for i, d := range quantize(dur) {
		if i == 0 {
			fmt.Fprintf(b, "%s%s ", body, d)
		} else {
			// tie the continuation
			fmt.Fprintf(b, "~ %s%s ", body, d)
		}
	}
}

// writeDurations writes a run of one symbol (e.g. "r") covering dur ticks,
// split into representable note values.
func writeDurations(b *strings.Builder, dur uint32, sym string) {
	for _, d := range quantize(dur) {
		fmt.Fprintf(b, "%s%s ", sym, d)
	}
}

// quantize splits a tick duration into a sequence of LilyPond duration strings
// (e.g. "4", "8", "4.") summing to the nearest representable total.
func quantize(dur uint32) []string {
	// representable units in ticks, longest first: whole, half, quarter, 8th,
	// 16th, 32nd; with their dotted variants handled by the greedy split.
	type d struct {
		ticks uint32
		ly    string
	}
	units := []d{
		{ppq * 4, "1"}, {ppq * 3, "2."}, {ppq * 2, "2"}, {ppq * 3 / 2, "4."},
		{ppq, "4"}, {ppq * 3 / 4, "8."}, {ppq / 2, "8"}, {ppq / 4, "16"},
		{ppq / 8, "32"},
	}
	if dur == 0 {
		return []string{"4"}
	}
	var out []string
	remaining := dur
	// snap very small remainders up to a 32nd so we always emit something
	min := uint32(ppq / 8)
	for remaining >= min {
		placed := false
		for _, u := range units {
			if remaining >= u.ticks {
				out = append(out, u.ly)
				remaining -= u.ticks
				placed = true
				break
			}
		}
		if !placed {
			break
		}
	}
	if len(out) == 0 {
		out = []string{"16"} // shortest reasonable fallback
	}
	return out
}

// pitch maps a MIDI key to a LilyPond pitch (Dutch note names + octave marks).
// MIDI 60 = middle C = c'. Accidentals always use sharps for simplicity.
func pitch(key uint8) string {
	names := []string{"c", "cis", "d", "dis", "e", "f", "fis", "g", "gis", "a", "ais", "b"}
	name := names[key%12]
	octave := int(key)/12 - 1 // MIDI octave (60 -> 4)
	// LilyPond: c' is middle C (octave 4). Marks relative to octave 3 (c).
	marks := octave - 3
	var suffix string
	if marks > 0 {
		suffix = strings.Repeat("'", marks)
	} else if marks < 0 {
		suffix = strings.Repeat(",", -marks)
	}
	return name + suffix
}

// clefFor picks treble or bass from the average pitch of the notes.
func clefFor(notes []note) string {
	if len(notes) == 0 {
		return "treble"
	}
	var sum int
	for _, n := range notes {
		sum += int(n.key)
	}
	if sum/len(notes) < 56 { // below ~G#3 -> bass
		return "bass"
	}
	return "treble"
}

func quote(s string) string {
	return `"` + strings.ReplaceAll(s, `"`, `\"`) + `"`
}
