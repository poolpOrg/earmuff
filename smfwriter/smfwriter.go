// Package smfwriter turns an elaborated Song into Standard MIDI File bytes.
//
// It writes one smf.Track per elaborated track at PPQ 960 (MetricTicks), with
// per-track meta headers (tempo/time-signature/copyright on track 0, then
// sequence name, instrument, and an initial program change). Channel and meta
// events are converted from the Song's absolute ticks to SMF delta times after
// a deterministic sort (NoteOff before NoteOn at equal tick).
package smfwriter

import (
	"bytes"
	"sort"

	"github.com/poolpOrg/earmuff/ast"
	"github.com/poolpOrg/earmuff/elaborator"
	"gitlab.com/gomidi/midi/v2"
	"gitlab.com/gomidi/midi/v2/smf"
)

// Write serializes one Song to SMF bytes.
func Write(song elaborator.Song) []byte {
	s := smf.New()
	s.TimeFormat = smf.MetricTicks(elaborator.PPQ)

	byTrack := map[int][]elaborator.Event{}
	for _, ev := range song.Events {
		byTrack[ev.Track] = append(byTrack[ev.Track], ev)
	}

	for ti, info := range song.Tracks {
		var tr smf.Track

		if ti == 0 {
			beats, unit := song.TimeBeats, song.TimeUnit
			if beats == 0 {
				beats = 4
			}
			if unit == 0 {
				unit = 4
			}
			tr.Add(0, smf.MetaMeter(uint8(beats), uint8(unit)))
			tr.Add(0, smf.MetaTempo(song.BPM))
			if song.Copyright != "" {
				tr.Add(0, smf.MetaCopyright(song.Copyright))
			}
			for _, t := range song.Texts {
				tr.Add(0, smf.MetaText(t))
			}
		}

		if info.Name != "" {
			tr.Add(0, smf.MetaTrackSequenceName(info.Name))
		}
		if info.Instrument != "" {
			tr.Add(0, smf.MetaInstrument(info.Instrument))
		}
		if info.HasProgram {
			tr.Add(0, midi.ProgramChange(info.Channel, info.Program))
		}

		events := byTrack[ti]
		// Stable sort by tick (Song is already globally sorted with NoteOff
		// before NoteOn at equal tick; this keeps that order within the track).
		sort.SliceStable(events, func(i, j int) bool {
			return events[i].Tick < events[j].Tick
		})

		var lastTick uint32
		for _, ev := range events {
			delta := ev.Tick - lastTick
			tr.Add(delta, message(ev.Msg))
			lastTick = ev.Tick
		}
		tr.Close(0)
		s.Add(tr)
	}

	var bf bytes.Buffer
	s.WriteTo(&bf)
	return bf.Bytes()
}

// message converts a MIDIMsg to its SMF wire bytes.
func message(m elaborator.MIDIMsg) smf.Message {
	switch m.Kind {
	case elaborator.MsgNoteOn:
		return smf.Message(midi.NoteOn(m.Channel, m.Key, m.Velocity))
	case elaborator.MsgNoteOff:
		return smf.Message(midi.NoteOff(m.Channel, m.Key))
	case elaborator.MsgCC:
		return smf.Message(midi.ControlChange(m.Channel, m.Controller, m.Value))
	case elaborator.MsgPitchBend:
		return smf.Message(midi.Pitchbend(m.Channel, m.Bend))
	case elaborator.MsgPressure:
		return smf.Message(midi.AfterTouch(m.Channel, m.Value))
	case elaborator.MsgProgram:
		return smf.Message(midi.ProgramChange(m.Channel, m.Program))
	case elaborator.MsgSysex:
		return smf.Message(midi.SysEx(m.Bytes))
	case elaborator.MsgMeta:
		return metaMessage(m)
	}
	return smf.MetaText("")
}

func metaMessage(m elaborator.MIDIMsg) smf.Message {
	switch m.MetaKind {
	case ast.MetaText:
		return smf.MetaText(m.Text)
	case ast.MetaLyric:
		return smf.MetaLyric(m.Text)
	case ast.MetaMarker:
		return smf.MetaMarker(m.Text)
	case ast.MetaCue:
		return smf.MetaCuepoint(m.Text)
	}
	return smf.MetaText(m.Text)
}
