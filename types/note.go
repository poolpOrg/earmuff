package types

import (
	"github.com/poolpOrg/go-harmony/notes"
)

type Note struct {
	duration uint16
	note     notes.Note
	tick     uint32
}

func NewNote(note notes.Note) *Note {
	return &Note{
		note: note,
	}
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

func (note *Note) GetNotes() []Note {
	return []Note{*note}
}

func (note *Note) SetTick(tick uint32) {
	note.tick = tick
}

func (note *Note) GetTick() uint32 {
	return note.tick
}

func (note *Note) GetOctave() uint8 {
	return note.note.Octave()
}
