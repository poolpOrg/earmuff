package types

import (
	"github.com/poolpOrg/go-harmony/chords"
)

type Chord struct {
	duration uint16
	chord    chords.Chord
	tick     uint32
}

func NewChord(chord chords.Chord) *Chord {
	return &Chord{
		chord: chord,
	}
}

func (chord *Chord) GetName() string {
	return chord.chord.Name()
}

func (chord *Chord) SetDuration(duration uint16) {
	chord.duration = duration
}

func (chord *Chord) GetDuration() uint16 {
	return chord.duration
}

func (chord *Chord) SetTick(tick uint32) {
	chord.tick = tick
}

func (chord *Chord) GetTick() uint32 {
	return chord.tick
}

func (chord *Chord) GetNotes() []Note {
	ret := make([]Note, 0)
	for _, note := range chord.chord.Notes() {
		n := Note{
			duration: chord.duration,
			//beat:     chord.beat,
			tick: chord.tick,
			note: note,
		}
		ret = append(ret, n)
	}

	return ret
}
