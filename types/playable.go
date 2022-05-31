package types

import (
	"sync"
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
	Play()
}

type Chord struct {
	name         string
	duration     uint16
	beat         uint8
	timestamp    time.Duration
	durationTime time.Duration
	chord        chords.Chord
}

type Rest struct {
	duration     uint16
	beat         uint8
	timestamp    time.Duration
	durationTime time.Duration
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
func (chord *Chord) GetNotes() []Note {
	ret := make([]Note, 0)
	for _, note := range chord.chord.Notes() {
		n := Note{
			duration:     chord.duration,
			beat:         chord.beat,
			timestamp:    chord.timestamp,
			durationTime: chord.durationTime,
			note:         note,
		}
		ret = append(ret, n)
	}

	return ret
}

func (chord *Chord) Play() {
	wg := sync.WaitGroup{}
	for _, note := range chord.chord.Notes() {
		wg.Add(1)
		go func() {
			n := Note{
				duration:     chord.duration,
				beat:         chord.beat,
				timestamp:    chord.timestamp,
				durationTime: chord.durationTime,
				note:         note,
			}
			n.Play()
			wg.Done()
		}()
	}
	wg.Wait()
}

func NewRest() *Rest {
	return &Rest{}
}

func (rest *Rest) GetType() string {
	return "Rest"
}

func (rest *Rest) GetName() string {
	return ""
}

func (rest *Rest) SetDuration(duration uint16) {
	rest.duration = duration
}

func (rest *Rest) GetDuration() uint16 {
	return rest.duration
}

func (rest *Rest) SetBeat(beat uint8) {
	rest.beat = beat
}

func (rest *Rest) GetBeat() uint8 {
	return rest.beat
}

func (rest *Rest) SetTimestamp(timestamp time.Duration) {
	rest.timestamp = timestamp
}

func (rest *Rest) GetTimestamp() time.Duration {
	return rest.timestamp
}

func (rest *Rest) SetDurationTime(timestamp time.Duration) {
	rest.durationTime = timestamp
}

func (rest *Rest) GetDurationTime() time.Duration {
	return rest.durationTime
}

func (rest *Rest) GetFrequency() float64 {
	return 0.0
}

func (rest *Rest) Play() {

}

func (rest *Rest) GetNotes() []Note {
	return []Note{}
}
