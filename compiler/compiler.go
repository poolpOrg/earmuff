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

			copyright := project.GetCopyright()
			if copyright != "" {
				tr.Add(0, smf.MetaCopyright(copyright))
			}
		}

		trackName := track.GetName()
		if trackName != "" {
			tr.Add(0, smf.MetaTrackSequenceName(trackName))
		}

		for _, text := range project.GetTexts() {
			tr.Add(0, smf.MetaText(text))
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
		for barOffset, bar := range track.GetBars() {
			for _, tickable := range bar.GetTickables() {

				t, isText := tickable.(*types.Text)
				if isText {
					//fmt.Println("text.GetTick()", t.GetTick())
					tr.Add(t.GetTick(), smf.MetaText(t.GetValue()))
				}

				l, isLyrics := tickable.(*types.Lyric)
				if isLyrics {
					//fmt.Println("lyric.GetTick()", l.GetTick())
					tr.Add(l.GetTick(), smf.MetaLyric(l.GetValue()))
				}

				m, isMarker := tickable.(*types.Marker)
				if isMarker {
					//fmt.Println("marker.GetTick()", m.GetTick())
					tr.Add(m.GetTick(), smf.MetaMarker(m.GetValue()))
				}

				c, isCue := tickable.(*types.Cue)
				if isCue {
					//fmt.Println("cue.GetTick()", c.GetTick())
					tr.Add(c.GetTick(), smf.MetaCuepoint(c.GetValue()))
				}

				_, isPlayable := tickable.(types.Playable)
				if isPlayable {
					playable := tickable.(types.Playable)
					for _, pitch := range playable.GetPitches() {
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

						ticksPerBeat := uint32(960)
						ticksPerBar := uint32(bar.GetSignature().GetBeats()) * ticksPerBeat

						tick := playable.GetTick()
						tick += uint32(barOffset) * ticksPerBar

						//fmt.Println("TICK", tick, "DURATION", duration)
						if _, exists := events[tick]; !exists {
							events[tick] = make([]midi.Message, 0)
						}
						if _, exists := events[tick+duration]; !exists {
							events[tick+duration] = make([]midi.Message, 0)
						}

						noteOn := midi.NoteOn(uint8(channel), pitch, playable.GetVelocity())
						noteOff := midi.NoteOff(uint8(channel), pitch)

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
		lastDelta := uint32(0)
		for _, key := range keys {
			events := events[key]
			sort.Slice(events, func(i, j int) bool {
				return events[i].Type().String() == "NoteOff"
			})
			for offset, message := range events {
				if offset == 0 {
					tr.Add((key - lastKey), message)
					lastDelta = (key - lastKey)

				} else {
					tr.Add(0, message)
					lastDelta = 0
				}
			}
			lastKey = key
		}

		tr.Close(lastDelta)
		s.Add(tr)
	}

	s.WriteTo(&bf)
	return bf.Bytes()
}
