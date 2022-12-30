package main

import (
	"flag"
	"fmt"
	"log"
	"sort"

	"github.com/poolpOrg/earmuff/types"
	"github.com/poolpOrg/go-harmony/chords"
	"github.com/poolpOrg/go-harmony/notes"
	"gitlab.com/gomidi/midi/v2"
	"gitlab.com/gomidi/midi/v2/smf"
)

func main() {
	var opt_file string
	var opt_verbose bool
	var opt_quiet bool

	flag.StringVar(&opt_file, "out", "", "output file (.ear)")
	flag.BoolVar(&opt_quiet, "quiet", false, "do not play")
	flag.BoolVar(&opt_verbose, "verbose", false, "extensive logging")
	flag.Parse()

	if flag.NArg() == 0 {
		log.Fatal("need a MIDI file to process")
	}

	project := types.NewProject()
	tracks := make(map[int]*types.Track)

	issetProjectBPM := false
	issetProjectTimeSig := false

	_ = project
	channels := make(map[int]map[int64]map[string]uint8)
	smf.ReadTracks(flag.Arg(0)).Do(
		func(te smf.TrackEvent) {
			if _, exists := tracks[te.TrackNo]; !exists {
				tracks[te.TrackNo] = types.NewTrack()
				tracks[te.TrackNo].SetBPM(project.GetBPM())
				tracks[te.TrackNo].SetSignature(project.GetSignature())
				channels[te.TrackNo] = make(map[int64]map[string]uint8) // XXX to be deleted
				project.AddTrack(tracks[te.TrackNo])
			}

			if te.Message.IsMeta() {

				if te.Message.Type().Is(smf.MetaInstrumentMsg) {
					var instrument string
					te.Message.GetMetaInstrument(&instrument)

					tracks[te.TrackNo].SetInstrument(instrument)

				} else if te.Message.Type().Is(smf.MetaTempoMsg) {
					var bpm float64
					te.Message.GetMetaTempo(&bpm)
					if !issetProjectBPM {
						project.SetBPM(bpm)
						issetProjectBPM = true
					}
					tracks[te.TrackNo].SetBPM(bpm)
				} else if te.Message.Type().Is(smf.MetaTimeSigMsg) {
					var numerator uint8
					var denominator uint8
					var clocksPerClick uint8
					var demiSemiQuaverPerQuarter uint8
					te.Message.GetMetaTimeSig(&numerator, &denominator, &clocksPerClick, &demiSemiQuaverPerQuarter)
					signature := types.NewSignature(numerator, denominator)
					if !issetProjectTimeSig {
						project.SetSignature(signature)
						issetProjectTimeSig = true
					}
					tracks[te.TrackNo].SetSignature(signature)

				} else {
					//fmt.Printf("[%v] @%vms %s\n", te.TrackNo, te.AbsMicroSeconds/1000, te.Message.String())
				}

			} else {
				var ch, key, vel uint8
				switch te.Message.Type().String() {
				case "NoteOn":
					if _, exists := channels[te.TrackNo][te.AbsTicks]; !exists {
						channels[te.TrackNo][te.AbsTicks] = make(map[string]uint8)
					}
					te.Message.GetNoteStart(&ch, &key, &vel)
					channels[te.TrackNo][te.AbsTicks][midi.Note(key).String()] = key

				case "NoteOff":
					keys := make([]int64, 0)
					for key := range channels[te.TrackNo] {
						if key < te.AbsTicks {
							keys = append(keys, key)
						}
					}

					sort.SliceStable(keys, func(i, j int) bool {
						return keys[i] < keys[j]
					})

					ticksPerBeat := uint32(960)
					ticksPerBar := uint32(project.GetSignature().GetBeats()) * ticksPerBeat

					// need to compute bars

					// need to compute key in beats

					for _, key := range keys {
						begin := uint32(key) / ticksPerBeat
						//delta := uint32(key) % ticksPerBeat
						//_, frac := math.Modf(float64(delta) / float64(ticksPerBeat) * 100)
						barno := begin / uint32(project.GetSignature().GetBeats())

						for {
							if len(tracks[te.TrackNo].GetBars()) <= int(barno) {
								b := types.NewBar(uint32(len(tracks[te.TrackNo].GetBars()) + 1))
								b.SetBPM(tracks[te.TrackNo].GetBPM())
								b.SetSignature(tracks[te.TrackNo].GetSignature())
								tracks[te.TrackNo].AddBar(b)
							} else {
								break
							}
						}
						bar := tracks[te.TrackNo].GetBars()[barno]

						activeNotes := make([]string, 0)
						for note, _ := range channels[te.TrackNo][key] {
							activeNotes = append(activeNotes, note)
						}

						sort.SliceStable(activeNotes, func(i, j int) bool {
							return channels[te.TrackNo][key][activeNotes[i]] < channels[te.TrackNo][key][activeNotes[j]]
						})

						switch len(activeNotes) {
						case 1:
							note, _ := notes.Parse(activeNotes[0])

							n := types.NewNote(*note)
							n.SetTick(uint32(key))

							// duration := uint16(math.Pow(2, float64(uint16(ticksPerBeat)/(uint16(te.AbsTicks)-uint16(key)))))
							absDuration := float64(uint16(ticksPerBar) / (uint16(te.AbsTicks) - uint16(key)))

							n.SetDuration(uint16(absDuration))

							bar.AddPlayable(n)
							//n.SetVelocity()
						case 2:
							root, _ := notes.Parse(activeNotes[0])
							target, _ := notes.Parse(activeNotes[1])
							fmt.Println(root.Distance(*target).Name(), "interval:", root.OctaveName(), target.OctaveName())
						case 3:
							fallthrough
						case 4:
							chordNotes := make([]notes.Note, 0)
							for _, noteName := range activeNotes {
								n, _ := notes.Parse(noteName)
								chordNotes = append(chordNotes, *n)
							}

							c := chords.FromNotes(chordNotes)

							n := types.NewChord(c)
							n.SetTick(uint32(key))

							// duration := uint16(math.Pow(2, float64(uint16(ticksPerBeat)/(uint16(te.AbsTicks)-uint16(key)))))
							absDuration := float64(uint16(ticksPerBar) / (uint16(te.AbsTicks) - uint16(key)))

							n.SetDuration(uint16(absDuration))

							bar.AddPlayable(n)

							//c.SetRoot(*c.Root().Interval(intervals.PerfectFourth))

							//for _, interval := range c.Structure() {
							//fmt.Println(interval.Name())
							//}
							//fmt.Println(c.Relative().Name())
						}

						delete(channels[te.TrackNo], key)
					}

				default:
					//fmt.Printf("[%v] %s %s\n", te.TrackNo, te.Message.Type().String(), te.Message)
				}

			}
		},
	)

	fmt.Println(project.String())

}
