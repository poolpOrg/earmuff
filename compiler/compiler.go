package compiler

import (
	"bytes"
	"sort"

	lmidi "github.com/poolpOrg/earmuff/midi"
	"github.com/poolpOrg/earmuff/types"
	"gitlab.com/gomidi/midi/v2"
	"gitlab.com/gomidi/midi/v2/smf"
)

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

		for _, copyright := range project.GetCopyrights() {
			tr.Add(0, smf.MetaCopyright(copyright))
		}
		for _, text := range project.GetTexts() {
			tr.Add(0, smf.MetaText(text))
		}

		for _, copyright := range track.GetCopyrights() {
			tr.Add(0, smf.MetaCopyright(copyright))
		}
		for _, text := range track.GetTexts() {
			tr.Add(0, smf.MetaText(text))
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
			for _, text := range bar.GetTexts() {
				tr.Add(0, smf.MetaText(text))
			}

			for _, tickable := range bar.GetTickables() {
				_, isPlayable := tickable.(types.Playable)
				if isPlayable {
					playable := tickable.(types.Playable)
					for _, n := range playable.GetNotes() {
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
						}

						duration := unit
						switch playable.GetDuration() {
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
						}

						tick := playable.GetTick()
						//fmt.Println("TICK", tick, "DURATION", duration)
						if _, exists := events[tick]; !exists {
							events[tick] = make([]midi.Message, 0)
						}
						if _, exists := events[tick+duration]; !exists {
							events[tick+duration] = make([]midi.Message, 0)
						}

						noteOn := midi.NoteOn(uint8(channel), n.MIDI(), playable.GetVelocity())
						noteOff := midi.NoteOff(uint8(channel), n.MIDI())

						events[tick] = append(events[tick], noteOn)
						events[tick+duration] = append(events[tick+duration], noteOff)
					}
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
			events := events[key]
			sort.Slice(events, func(i, j int) bool {
				return events[i].Type().String() == "NoteOff"
			})
			for offset, message := range events {
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
