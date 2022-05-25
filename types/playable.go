package types

import "time"

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
}

type Chord struct {
	name         string
	duration     uint16
	beat         uint8
	timestamp    time.Duration
	durationTime time.Duration
}

type Note struct {
	name         string
	duration     uint16
	beat         uint8
	timestamp    time.Duration
	durationTime time.Duration
}

type Rest struct {
	duration     uint16
	beat         uint8
	timestamp    time.Duration
	durationTime time.Duration
}

func NewChord(name string) *Chord {
	return &Chord{
		name: name,
	}
}

func (chord *Chord) GetType() string {
	return "Chord"
}

func (chord *Chord) GetName() string {
	return chord.name
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

func NewNote(name string) *Note {
	return &Note{
		name: name,
	}
}

func (note *Note) GetType() string {
	return "Note"
}

func (note *Note) GetName() string {
	return note.name
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
