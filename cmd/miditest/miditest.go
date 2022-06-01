package main

import (
	"flag"
	"log"
	"os"

	"github.com/poolpOrg/earring/midi"
	"github.com/poolpOrg/earring/parser"
	// (Meta Messages)
	// you may also want to use these
	// github.com/gomidi/midi/midimessage/cc         (ControlChange Messages)
	// github.com/gomidi/midi/midimessage/sysex      (System Exclusive Messages)
)

func main() {
	flag.Parse()

	if flag.NArg() == 0 {
		log.Fatal("need a source file to process")
	}

	fp, err := os.Open(flag.Arg(0))
	if err != nil {
		log.Fatal(err)
	}
	defer fp.Close()

	parser := parser.NewParser(fp)
	project, err := parser.Parse()
	if err != nil {
		log.Fatal(err)
	}

	b := midi.ToMidi(project)
	fp2, err := os.OpenFile("file.mid", os.O_CREATE|os.O_RDWR|os.O_TRUNC, 0600)
	if err != nil {
		log.Fatal(err)
	}
	defer fp2.Close()

	fp2.Write(b)

	/*
		tpq := smf.MetricTicks(0) // set the time resolution in ticks per quarter note; 0 uses the defaults (i.e. 960)

		writeMIDI := func(wr smf.Writer) {

			// always set the delta before writing
			wr.SetDelta(tpq.Ticks8th())

			// starts MIDI key 65 on MIDI channel 3 with velocity 90 with delta of 480 to
			// the beginning of the track (note starts after a quaver pause)
			// MIDI channels 1-16 correspond to channel.Channel0 - channel.Channel15.
			err = wr.Write(Channel2.NoteOn(65, 90))

			if err != nil {
				return
			}

			wr.SetDelta(tpq.Ticks4th())

			// stops MIDI note 65 on MIDI channel 3 with delta of 960 to previous message
			// this results in a duration of 1 quarter note for midi note 65
			err = wr.Write(Channel2.NoteOff(65))

			if err != nil {
				return
			}

			// finishes the first track and writes it to the file
			err = wr.Write(meta.EndOfTrack)

			if err != nil {
				return
			}

			// the next write writes to the second track
			// after writing delta is always 0, so we start here at the beginning of the second track
			err = wr.Write(Channel2.NoteOn(65, 90))

			if err != nil {
				return
			}

			wr.SetDelta(tpq.Ticks4th())

			// stops MIDI note 65 on MIDI channel 3 with delta of 960 to previous message
			// this results in a duration of 1 quarter note for midi note 65
			err = wr.Write(Channel2.NoteOff(65))

			if err != nil {
				return
			}

			if err != nil {
				return
			}

			// finishes the second track and writes it to the file
			err = wr.Write(meta.EndOfTrack)
		}

		// the number passed to the NumTracks option must match the tracks written
		// if NumTracks is not passed, it defaults to single track (SMF0)
		// if numtracks > 1, SMF1 format is chosen.
		// if TimeFormat is not passed, smf.MetricTicks(960) will be chosen
		smfwriter.WriteFile("file.mid", writeMIDI, smfwriter.NumTracks(2), smfwriter.TimeFormat(tpq))

		// deal with err
	*/
}
