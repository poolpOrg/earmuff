package types

import "github.com/poolpOrg/go-harmony/tunings"

type Track struct {
	bpm          uint8
	signature    *Signature
	bars         []*Bar
	tuning       tunings.Tuning
	instrument   string
	isPercussive bool
}

func NewTrack() *Track {
	return &Track{
		bars:   make([]*Bar, 0),
		tuning: tunings.A440,
	}
}

func (track *Track) GetBPM() uint8 {
	return track.bpm
}

func (track *Track) SetBPM(bpm uint8) {
	track.bpm = bpm
}

func (track *Track) GetSignature() *Signature {
	return track.signature
}

func (track *Track) SetSignature(signature *Signature) {
	track.signature = signature
}

func (track *Track) AddBar(bar *Bar) {
	track.bars = append(track.bars, bar)
}

func (track *Track) GetBars() []*Bar {
	return track.bars
}

func (track *Track) SetInstrument(instrument string) {
	track.instrument = instrument
}

func (track *Track) GetInstrument() string {
	return track.instrument
}

func (track *Track) SetPercussive() {
	track.isPercussive = true
}

func (track *Track) IsPercussive() bool {
	return track.isPercussive
}
