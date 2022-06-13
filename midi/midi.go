package midi

import (
	"bytes"
	"fmt"
	"sort"
	"strings"

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

		tr.Add(0, smf.MetaInstrument(track.GetInstrument()))
		pc, _ := InstrumentToPC(track.GetInstrument())
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

//https://www.midi.org/specifications-old/item/gm-level-1-sound-set
func InstrumentToPC(instrument string) (uint8, error) {
	switch strings.ToLower(instrument) {
	case "piano":
		return 1, nil
	case "acoustic grand piano":
		return 1, nil
	case "bright acoustic piano":
		return 2, nil
	case "electric grand piano":
		return 3, nil
	case "honky-tonk piano":
		return 4, nil
	case "electric piano 1":
		return 5, nil
	case "electric piano 2":
		return 6, nil
	case "hapsichord":
		return 7, nil
	case "clavi":
		return 8, nil

	case "chromatic percussion":
		return 9, nil
	case "celesta":
		return 9, nil
	case "glockenspiel":
		return 10, nil
	case "music box":
		return 11, nil
	case "vibraphone":
		return 12, nil
	case "marimba":
		return 13, nil
	case "xylophone":
		return 14, nil
	case "tubular bells":
		return 15, nil
	case "dulcimer":
		return 16, nil

	case "organ":
		return 17, nil
	case "drawbar organ":
		return 17, nil
	case "percussive organ":
		return 18, nil
	case "rock organ":
		return 19, nil
	case "church organ":
		return 20, nil
	case "reed organ":
		return 21, nil
	case "accordion":
		return 22, nil
	case "harmonica":
		return 23, nil
	case "tango accordion":
		return 24, nil

	case "guitar":
		return 25, nil
	case "acoustic guitar (nylon)":
		return 25, nil
	case "acoustic guitar (steel)":
		return 26, nil
	case "electric guitar (jazz)":
		return 27, nil
	case "electric guitar (clean)":
		return 28, nil
	case "electric guitar (muted)":
		return 29, nil
	case "overdriven guitar":
		return 30, nil
	case "distortion guitar":
		return 31, nil
	case "guitar harmonics":
		return 32, nil

	case "bass":
		return 33, nil
	case "acoustic bass":
		return 33, nil
	case "electric bass (finger)":
		return 34, nil
	case "electric bass (pick)":
		return 35, nil
	case "fretless bass":
		return 36, nil
	case "slap bass 1":
		return 37, nil
	case "slap bass 2":
		return 38, nil
	case "synth bass 1":
		return 39, nil
	case "synth bass 2":
		return 40, nil

	case "strings":
		return 41, nil
	case "violin":
		return 41, nil
	case "viola":
		return 42, nil
	case "cello":
		return 43, nil
	case "contrabass":
		return 44, nil
	case "tremolo strings":
		return 45, nil
	case "pizzicato strings":
		return 46, nil
	case "orchestral harp":
		return 47, nil
	case "timpani":
		return 48, nil

	case "ensemble":
		return 49, nil
	case "string ensemble 1":
		return 49, nil
	case "string ensemble 2":
		return 50, nil
	case "synthstrings 1":
		return 51, nil
	case "synthstrings 2":
		return 52, nil
	case "choir aahs":
		return 53, nil
	case "voice oohs":
		return 54, nil
	case "synth voice":
		return 55, nil
	case "orchestra hit":
		return 56, nil

	case "brass":
		return 57, nil
	case "trumpet":
		return 57, nil
	case "trombone":
		return 58, nil
	case "tuba":
		return 59, nil
	case "muted trumpet":
		return 60, nil
	case "french horn":
		return 61, nil
	case "brass section":
		return 62, nil
	case "synthbrass 1":
		return 63, nil
	case "synthbrass 2":
		return 64, nil

	case "reed":
		return 65, nil
	case "soprano sax":
		return 65, nil
	case "alto sax":
		return 66, nil
	case "tenor sax":
		return 67, nil
	case "baritone sax":
		return 68, nil
	case "oboe":
		return 69, nil
	case "english horn":
		return 70, nil
	case "bassoon":
		return 71, nil
	case "clarinet":
		return 72, nil

	case "pipe":
		return 73, nil
	case "piccolo":
		return 73, nil
	case "flute":
		return 74, nil
	case "recorder":
		return 75, nil
	case "pan flute":
		return 76, nil
	case "blown bottle":
		return 77, nil
	case "shakuhachi":
		return 78, nil
	case "whistle":
		return 79, nil
	case "ocarina":
		return 80, nil

	case "synth lead":
		return 81, nil
	case "lead 1 (square)":
		return 81, nil
	case "lead 2 (sawtooth)":
		return 82, nil
	case "lead 3 (calliope)":
		return 83, nil
	case "lead 4 (chiff)":
		return 84, nil
	case "lead 5 (charang)":
		return 85, nil
	case "lead 6 (voice)":
		return 86, nil
	case "lead 7 (fifths)":
		return 87, nil
	case "lead 8 (bass + lead)":
		return 87, nil

	case "sync pad":
		return 89, nil
	case "pad 1 (new age)":
		return 89, nil
	case "pad 2 (warm)":
		return 90, nil
	case "pad 3 (polysynth)":
		return 91, nil
	case "pad 4 (choir)":
		return 92, nil
	case "pad 5 (bowed)":
		return 93, nil
	case "pad 6 (metallic)":
		return 94, nil
	case "pad 7 (halo)":
		return 95, nil
	case "pad 8 (sweep)":
		return 96, nil

	case "synth effects":
		return 97, nil
	case "FX 1 (rain)":
		return 97, nil
	case "FX 2 (soundtrack)":
		return 98, nil
	case "FX 3 (crystal)":
		return 99, nil
	case "FX 4 (atmosphere)":
		return 100, nil
	case "FX 5 (brightness)":
		return 101, nil
	case "FX 6 (goblins)":
		return 102, nil
	case "FX 7 (echoes)":
		return 103, nil
	case "FX 8 (sci-fi)":
		return 104, nil

	case "ethnic":
		return 105, nil
	case "sitar":
		return 105, nil
	case "banjo":
		return 105, nil
	case "shamisen":
		return 105, nil
	case "koto":
		return 105, nil
	case "kalimba":
		return 105, nil
	case "bag pipe":
		return 105, nil
	case "fiddle":
		return 105, nil
	case "shanai":
		return 105, nil

	case "percussive":
		return 113, nil
	case "tinkle bell":
		return 113, nil
	case "agogo":
		return 114, nil
	case "steel drums":
		return 115, nil
	case "woodblock":
		return 116, nil
	case "taiko drum":
		return 117, nil
	case "melodic tom":
		return 118, nil
	case "synth drum":
		return 119, nil
	case "reverse cymbal":
		return 120, nil

	case "sound effects":
		return 121, nil
	case "guitar fret noise":
		return 121, nil
	case "breath noise":
		return 122, nil
	case "seashore":
		return 123, nil
	case "bird tweet":
		return 124, nil
	case "telephone ring":
		return 125, nil
	case "helicopter":
		return 126, nil
	case "applause":
		return 127, nil
	case "gunshot":
		return 127, nil
	default:
		return 0, fmt.Errorf("unknown instrument %s", instrument)
	}
}
