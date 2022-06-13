package types

import (
	"time"

	"github.com/poolpOrg/go-harmony/chords"
)

type Playable interface {
	GetType() string
	GetName() string
	SetDuration(uint16)
	GetDuration() uint16
	SetDurationTime(time.Duration)
	GetDurationTime() time.Duration
	SetBeat(uint8)
	GetBeat() uint8
	SetTimestamp(time.Duration)
	GetTimestamp() time.Duration
	GetFrequency() float64
	GetNotes() []Note

	SetTick(uint32)
	GetTick() uint32
}

type Chord struct {
	name         string
	duration     uint16
	beat         uint8
	timestamp    time.Duration
	durationTime time.Duration
	chord        chords.Chord
	tick         uint32
}

func NewChord(chord chords.Chord) *Chord {
	return &Chord{
		chord: chord,
	}
}

func (chord *Chord) GetType() string {
	return "Chord"
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

func (chord *Chord) SetBeat(beat uint8) {
	chord.beat = beat
}

func (chord *Chord) GetBeat() uint8 {
	return chord.beat
}

func (chord *Chord) SetTimestamp(timestamp time.Duration) {
	chord.timestamp = timestamp
}

func (chord *Chord) GetTimestamp() time.Duration {
	return chord.timestamp
}

func (chord *Chord) SetDurationTime(timestamp time.Duration) {
	chord.durationTime = timestamp
}

func (chord *Chord) GetDurationTime() time.Duration {
	return chord.durationTime
}

func (chord *Chord) GetFrequency() float64 {
	return 0.0
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
			duration:     chord.duration,
			beat:         chord.beat,
			timestamp:    chord.timestamp,
			durationTime: chord.durationTime,
			tick:         chord.tick,
			note:         note,
		}
		ret = append(ret, n)
	}

	return ret
}
