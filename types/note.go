package types

import (
	"time"

	"github.com/faiface/beep"
	"github.com/faiface/beep/generators"
	"github.com/faiface/beep/speaker"
	"github.com/poolpOrg/go-harmony/notes"
)

type Note struct {
	duration     uint16
	beat         uint8
	timestamp    time.Duration
	durationTime time.Duration
	note         notes.Note
}

func NewNote(note notes.Note) *Note {
	return &Note{
		note: note,
	}
}

func (note *Note) GetType() string {
	return "Note"
}

func (note *Note) GetName() string {
	return note.note.Name()
}

func (note *Note) SetDuration(duration uint16) {
	note.duration = duration
}

func (note *Note) GetDuration() uint16 {
	return note.duration
}

func (note *Note) SetBeat(beat uint8) {
	note.beat = beat
}

func (note *Note) GetBeat() uint8 {
	return note.beat
}

func (note *Note) SetTimestamp(timestamp time.Duration) {
	note.timestamp = timestamp
}

func (note *Note) GetTimestamp() time.Duration {
	return note.timestamp
}

func (note *Note) SetDurationTime(timestamp time.Duration) {
	note.durationTime = timestamp
}

func (note *Note) GetDurationTime() time.Duration {
	return note.durationTime
}

func (note *Note) GetFrequency() float64 {
	return note.note.Frequency()
}

func (note *Note) GetNotes() []Note {
	return []Note{*note}
}

func (note *Note) Play() {
	sr := beep.SampleRate(41100)
	sine, err := generators.SineTone(sr, note.note.Frequency())
	if err != nil {
		panic(err)
	}

	done := make(chan bool)
	speaker.Play(beep.Seq(beep.Take(sr.N(note.durationTime), sine), beep.Callback(func() {
		done <- true
	})))
	<-done
}
