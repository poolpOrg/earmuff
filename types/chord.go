package types

import (
	"github.com/poolpOrg/go-harmony/chords"
	"github.com/poolpOrg/go-harmony/notes"
)

type Chord struct {
	duration uint16
	chord    chords.Chord
	tick     uint32
	velocity uint8
}

func NewChord(chord chords.Chord) *Chord {
	return &Chord{
		chord:    chord,
		velocity: 120,
	}
}

func (chord *Chord) GetName() string {
	return chord.chord.Name()
}

func (chord *Chord) SetDuration(duration uint16) {
	chord.duration = duration
}

func (chord *Chord) SetVelocity(velocity uint8) {
	chord.velocity = velocity
}

func (chord *Chord) GetDuration() uint16 {
	return chord.duration
}

func (chord *Chord) GetVelocity() uint8 {
	return chord.velocity
}

func (chord *Chord) SetTick(tick uint32) {
	chord.tick = tick
}

func (chord *Chord) GetTick() uint32 {
	return chord.tick
}

func (chord *Chord) GetNotes() []notes.Note {
	ret := make([]notes.Note, 0)
	for _, note := range chord.chord.Notes() {
		n := Note{
			duration: chord.duration,
			//beat:     chord.beat,
			tick: chord.tick,
			note: note,
		}
		ret = append(ret, n.note)
	}

	return ret
}

func (chord *Chord) GetPitches() []uint8 {
	ret := make([]uint8, 0)
	for _, note := range chord.chord.Notes() {
		ret = append(ret, note.MIDI())
	}
	return ret
}

func (chord *Chord) String() string {
	return ""
}
