// Package musicxml renders an elaborated Song into MusicXML, a notation format
// that engravers (e.g. Verovio in the playground) turn into beautiful SVG.
//
// Like the lilypond emitter it targets the common, grid-aligned cases: it pairs
// NoteOn/NoteOff into notes, groups simultaneous notes into chords, fills gaps
// with rests, and lays one part per track. Durations that don't land on a clean
// note value are split (and tied) into representable pieces; anything that
// crosses a barline is split at the barline and tied across it. One <part> per
// track, treble or bass clef chosen from the register.
package musicxml

import (
	"fmt"
	"sort"
	"strings"

	"github.com/poolpOrg/earmuff/elaborator"
)

const ppq = elaborator.PPQ // 960 ticks per quarter note

// divisions per quarter note used in the MusicXML output. Using PPQ directly
// keeps durations exact.
const divisions = ppq

// Render returns MusicXML for song.
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
	b.WriteString(`<?xml version="1.0" encoding="UTF-8"?>` + "\n")
	b.WriteString(`<!DOCTYPE score-partwise PUBLIC "-//Recordare//DTD MusicXML 3.1 Partwise//EN" "http://www.musicxml.org/dtds/partwise.dtd">` + "\n")
	b.WriteString(`<score-partwise version="3.1">` + "\n")

	// Work title.
	if song.Name != "" {
		b.WriteString("  <work><work-title>" + esc(song.Name) + "</work-title></work>\n")
	}

	// Build the per-track parts first so we can write the part-list header.
	type part struct {
		id    string
		name  string
		clef  string
		notes []note
	}
	var parts []part
	for i, tr := range song.Tracks {
		notes := collectNotes(song, i)
		if len(notes) == 0 {
			continue
		}
		parts = append(parts, part{
			id:    fmt.Sprintf("P%d", len(parts)+1),
			name:  tr.Name,
			clef:  clefFor(notes),
			notes: notes,
		})
	}

	b.WriteString("  <part-list>\n")
	for _, p := range parts {
		b.WriteString("    <score-part id=\"" + p.id + "\">")
		b.WriteString("<part-name>" + esc(p.name) + "</part-name>")
		b.WriteString("</score-part>\n")
	}
	b.WriteString("  </part-list>\n")

	for _, p := range parts {
		b.WriteString("  <part id=\"" + p.id + "\">\n")
		writeMeasures(&b, p.notes, beats, unit, ticksPerBar, p.clef, song.BPM)
		b.WriteString("  </part>\n")
	}

	b.WriteString("</score-partwise>\n")
	return b.String()
}

// ---------------------------------------------------------------------------
// Note extraction (mirrors the lilypond emitter)
// ---------------------------------------------------------------------------

type note struct {
	tick uint32
	key  uint8
	dur  uint32
}

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

type chord struct {
	tick uint32
	dur  uint32
	keys []uint8
}

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

// ---------------------------------------------------------------------------
// Measure writing
// ---------------------------------------------------------------------------

// segment is one chord (or rest, keys==nil) occupying a contiguous span that
// does not cross a barline.
type segment struct {
	dur     uint32
	keys    []uint8 // nil = rest
	tieStop bool    // this segment ends a tie started by the previous one
	tieCont bool    // this segment is tied to the next (same chord, split)
}

func writeMeasures(b *strings.Builder, notes []note, beats, unit int, ticksPerBar uint32, clef string, bpm float64) {
	chords := groupChords(notes)

	// Walk the timeline, emitting chords and rest-fills, splitting anything that
	// crosses a barline into tied pieces, and bucket the pieces by measure.
	type meas struct{ segs []segment }
	var measures []meas
	ensure := func(m int) {
		for len(measures) <= m {
			measures = append(measures, meas{})
		}
	}

	var cursor uint32
	emit := func(start, dur uint32, keys []uint8) {
		// Split [start, start+dur) at barlines; tie chord pieces across.
		t := start
		end := start + dur
		first := true
		for t < end {
			bar := int(t / ticksPerBar)
			barEnd := uint32(bar+1) * ticksPerBar
			pieceEnd := end
			if barEnd < pieceEnd {
				pieceEnd = barEnd
			}
			seg := segment{dur: pieceEnd - t, keys: keys}
			if keys != nil {
				if !first {
					seg.tieStop = true
				}
				if pieceEnd < end {
					seg.tieCont = true
				}
			}
			ensure(bar)
			measures[bar].segs = append(measures[bar].segs, seg)
			t = pieceEnd
			first = false
		}
	}

	for _, c := range chords {
		if c.tick > cursor {
			emit(cursor, c.tick-cursor, nil) // rest fill
			cursor = c.tick
		} else if c.tick < cursor {
			continue // overlap we can't notate simply; keep bar math honest
		}
		dur := c.dur
		if dur == 0 {
			dur = ppq
		}
		emit(cursor, dur, c.keys)
		cursor += dur
	}
	// Pad the final measure with a rest so it's complete.
	if rem := cursor % ticksPerBar; rem != 0 {
		emit(cursor, ticksPerBar-rem, nil)
	}
	if len(measures) == 0 {
		ensure(0)
		measures[0].segs = append(measures[0].segs, segment{dur: ticksPerBar})
	}

	for mi, m := range measures {
		b.WriteString("    <measure number=\"" + fmt.Sprintf("%d", mi+1) + "\">\n")
		if mi == 0 {
			b.WriteString("      <attributes>\n")
			b.WriteString(fmt.Sprintf("        <divisions>%d</divisions>\n", divisions))
			b.WriteString("        <key><fifths>0</fifths></key>\n")
			b.WriteString(fmt.Sprintf("        <time><beats>%d</beats><beat-type>%d</beat-type></time>\n", beats, unit))
			if clef == "bass" {
				b.WriteString("        <clef><sign>F</sign><line>4</line></clef>\n")
			} else {
				b.WriteString("        <clef><sign>G</sign><line>2</line></clef>\n")
			}
			b.WriteString("      </attributes>\n")
			if bpm > 0 {
				b.WriteString(fmt.Sprintf("      <direction placement=\"above\"><direction-type><metronome><beat-unit>quarter</beat-unit><per-minute>%d</per-minute></metronome></direction-type></direction>\n", int(bpm+0.5)))
			}
		}
		for _, s := range m.segs {
			writeSegment(b, s)
		}
		b.WriteString("    </measure>\n")
	}
}

// writeSegment writes one chord or rest as MusicXML <note> element(s). A chord
// of N keys becomes one <note> plus N-1 <note><chord/> elements. Durations that
// aren't a single note value are split into tied pieces.
func writeSegment(b *strings.Builder, s segment) {
	pieces := quantize(s.dur)
	if s.keys == nil {
		// Rest: one <note><rest/> per piece (ties don't apply to rests).
		for _, p := range pieces {
			b.WriteString("      <note>")
			b.WriteString("<rest/>")
			b.WriteString(fmt.Sprintf("<duration>%d</duration>", p.ticks))
			b.WriteString("<type>" + p.typ + "</type>")
			if p.dots == 1 {
				b.WriteString("<dot/>")
			}
			b.WriteString("</note>\n")
		}
		return
	}

	for pi, p := range pieces {
		// Tie state across the multi-piece split AND across the barline.
		tieStart := pi < len(pieces)-1 || s.tieCont
		tieStop := pi > 0 || s.tieStop
		for ki, key := range s.keys {
			b.WriteString("      <note>")
			if ki > 0 {
				b.WriteString("<chord/>")
			}
			st, alter, oct := pitch(key)
			b.WriteString("<pitch><step>" + st + "</step>")
			if alter != 0 {
				b.WriteString(fmt.Sprintf("<alter>%d</alter>", alter))
			}
			b.WriteString(fmt.Sprintf("<octave>%d</octave></pitch>", oct))
			b.WriteString(fmt.Sprintf("<duration>%d</duration>", p.ticks))
			if tieStart {
				b.WriteString("<tie type=\"start\"/>")
			}
			if tieStop {
				b.WriteString("<tie type=\"stop\"/>")
			}
			b.WriteString("<type>" + p.typ + "</type>")
			if p.dots == 1 {
				b.WriteString("<dot/>")
			}
			if tieStop {
				b.WriteString("<notations><tied type=\"stop\"/></notations>")
			} else if tieStart {
				b.WriteString("<notations><tied type=\"start\"/></notations>")
			}
			b.WriteString("</note>\n")
		}
	}
}

// ---------------------------------------------------------------------------
// Duration quantization + pitch
// ---------------------------------------------------------------------------

// piece is one representable note value with its tick length, MusicXML type
// name, and dot count.
type piece struct {
	ticks uint32
	typ   string
	dots  int
}

// quantize splits a tick duration into representable note-value pieces, longest
// first, mirroring the lilypond emitter's greedy split.
func quantize(dur uint32) []piece {
	units := []piece{
		{ppq * 4, "whole", 0}, {ppq * 3, "half", 1}, {ppq * 2, "half", 0},
		{ppq * 3 / 2, "quarter", 1}, {ppq, "quarter", 0},
		{ppq * 3 / 4, "eighth", 1}, {ppq / 2, "eighth", 0},
		{ppq / 4, "16th", 0}, {ppq / 8, "32nd", 0},
	}
	if dur == 0 {
		return []piece{{ppq, "quarter", 0}}
	}
	var out []piece
	remaining := dur
	min := uint32(ppq / 8)
	for remaining >= min {
		placed := false
		for _, u := range units {
			if remaining >= u.ticks {
				out = append(out, u)
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
		out = []piece{{ppq / 4, "16th", 0}}
	}
	return out
}

// pitch maps a MIDI key to a MusicXML (step, alter, octave). Accidentals are
// spelled as sharps (alter=+1), matching the rest of the toolchain. MIDI 60 =
// middle C = octave 4.
func pitch(key uint8) (step string, alter int, octave int) {
	// step + alter for each pitch class (sharps).
	type pc struct {
		step  string
		alter int
	}
	table := []pc{
		{"C", 0}, {"C", 1}, {"D", 0}, {"D", 1}, {"E", 0}, {"F", 0},
		{"F", 1}, {"G", 0}, {"G", 1}, {"A", 0}, {"A", 1}, {"B", 0},
	}
	p := table[key%12]
	return p.step, p.alter, int(key)/12 - 1
}

func clefFor(notes []note) string {
	if len(notes) == 0 {
		return "treble"
	}
	var sum int
	for _, n := range notes {
		sum += int(n.key)
	}
	if sum/len(notes) < 56 {
		return "bass"
	}
	return "treble"
}

func esc(s string) string {
	r := strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;", `"`, "&quot;")
	return r.Replace(s)
}
