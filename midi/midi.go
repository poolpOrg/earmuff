package midi

import (
	"bytes"
	"fmt"

	"github.com/poolpOrg/earring/types"
	"gitlab.com/gomidi/midi/v2"
	"gitlab.com/gomidi/midi/v2/smf"
)

var midiMap = map[string]func(uint8) uint8{
	"Cb": midi.B,
	"C":  midi.C,
	"C#": midi.Db,

	"Db": midi.Db,
	"D":  midi.D,
	"D#": midi.Eb,

	"Eb": midi.Eb,
	"E":  midi.E,

	"Fbb": midi.Eb,
	"Fb":  midi.E,
	"F":   midi.F,
	"F#":  midi.Gb,

	"Gb": midi.Gb,
	"G":  midi.G,
	"G#": midi.Ab,

	"Ab": midi.Ab,
	"A":  midi.A,
	"A#": midi.Bb,

	"Bb": midi.Bb,
	"B":  midi.B,
}

func ToMidi(project *types.Project) []byte {
	var bf bytes.Buffer
	var clock = smf.MetricTicks(960)

	s := smf.New()
	s.TimeFormat = clock

	for trackNumber, track := range project.GetTracks() {
		var tr smf.Track

		if trackNumber == 0 {
			tr.Add(0, smf.MetaMeter(project.GetSignature().GetBeats(), project.GetSignature().GetDuration()))
			tr.Add(0, smf.MetaTempo(float64(project.GetBPM())))
		}

		tr.Add(0, smf.MetaInstrument("Piano"))
		//	tr.Add(0, midi.ProgramChange(0, gm.Instr_BrassSection.Value()))

		for _, bar := range track.GetBars() {
			for _, playable := range bar.GetPlayables() {
				for _, n := range playable.GetNotes() {
					fn := midiMap[n.GetName()]
					duration := clock.Ticks4th()
					switch n.GetDuration() {
					case 1:
						duration = clock.Ticks4th() * 4
					case 2:
						duration = clock.Ticks4th() * 2
					case 4:
						duration = clock.Ticks4th()
					case 8:
						duration = clock.Ticks8th()
					case 16:
						duration = clock.Ticks16th()
					}
					fmt.Println(duration)
					tr.Add(duration, midi.NoteOn(1, fn(4), 120))
				}
				for _, n := range playable.GetNotes() {
					fn := midiMap[n.GetName()]
					tr.Add(clock.Ticks8th(), midi.NoteOff(1, fn(4)))
				}
			}
		}
		tr.Close(0)
		s.Add(tr)
	}

	s.WriteTo(&bf)
	return bf.Bytes()
}
