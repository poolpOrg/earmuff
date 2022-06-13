package compiler

import (
	"bytes"
	"sort"

	lmidi "github.com/poolpOrg/earring/midi"
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

func Compile(project *types.Project) []byte {
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

		tr.Add(0, smf.MetaInstrument(track.GetInstrument()))
		pc, _ := lmidi.InstrumentToPC(track.GetInstrument())
		var channel = trackNumber
		if pc >= 113 && pc <= 120 {
			channel = 10
		} else {
			if channel == 10 {
				channel = channel + 1
			}
		}
		tr.Add(0, midi.ProgramChange(uint8(channel), pc))

		events := make(map[uint32][]midi.Message)
		for _, bar := range track.GetBars() {
			for _, playable := range bar.GetPlayables() {
				for _, n := range playable.GetNotes() {
					fn := midiMap[n.GetName()]

					unit := clock.Ticks4th()
					switch bar.GetSignature().GetDuration() {
					case 1:
						unit = clock.Ticks4th() * 4
					case 2:
						unit = clock.Ticks4th() * 2
					case 4:
						unit = clock.Ticks4th()
					case 8:
						unit = clock.Ticks8th()
					case 16:
						unit = clock.Ticks16th()
					case 32:
						unit = clock.Ticks32th()
					case 64:
						unit = clock.Ticks64th()
					case 128:
						unit = clock.Ticks128th()
						/*
							case 256:
								unit = clock.Ticks256th()
						*/
					}

					duration := unit
					switch n.GetDuration() {
					case 1:
						duration *= 4
					case 2:
						duration *= 2
					case 4:
						duration = unit
					case 8:
						duration = unit / 2
					case 16:
						duration = unit / 4
					case 32:
						duration = unit / 8
					case 64:
						duration = unit / 16
					case 128:
						duration = unit / 32
						/*case 256*/
					}

					tick := n.GetTick()

					//fmt.Println("TICK", tick, "DURATION", duration)
					if _, exists := events[tick]; !exists {
						events[tick] = make([]midi.Message, 0)
					}
					if _, exists := events[tick+duration]; !exists {
						events[tick+duration] = make([]midi.Message, 0)
					}
					events[tick] = append(events[tick], midi.NoteOn(uint8(channel), fn(n.GetOctave()), 120))
					events[tick+duration] = append(events[tick+duration], midi.NoteOff(uint8(channel), fn(4)))
				}
			}
		}
		keys := make([]uint32, 0)
		for t := range events {
			keys = append(keys, t)
		}
		sort.Slice(keys, func(i, j int) bool { return keys[i] < keys[j] })

		lastKey := uint32(0)
		for _, key := range keys {
			//			fmt.Println(key-lastKey, events[key])
			for offset, message := range events[key] {
				if offset == 0 {
					tr.Add((key - lastKey), message)
				} else {
					tr.Add(0, message)
				}
			}
			lastKey = key
		}

		tr.Close(0)
		s.Add(tr)
	}

	s.WriteTo(&bf)
	return bf.Bytes()
}
