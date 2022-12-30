package types

import (
	"github.com/poolpOrg/go-harmony/tunings"
)

type Track struct {
	name         string
	bpm          float64
	signature    *Signature
	bars         []*Bar
	copyrights   []string
	texts        []string
	tuning       tunings.Tuning
	instrument   string
	isPercussive bool
}

func NewTrack() *Track {
	return &Track{
		bars:       make([]*Bar, 0),
		copyrights: make([]string, 0),
		texts:      make([]string, 0),
		tuning:     tunings.A440,
	}
}

func (track *Track) GetName() string {
	return track.name
}

func (track *Track) SetName(name string) {
	track.name = name
}

func (track *Track) GetBPM() float64 {
	return track.bpm
}

func (track *Track) SetBPM(bpm float64) {
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

func (track *Track) AddCopyright(text string) {
	track.copyrights = append(track.copyrights, text)
}

func (track *Track) GetCopyrights() []string {
	return track.copyrights
}

func (track *Track) AddText(text string) {
	track.texts = append(track.texts, text)
}

func (track *Track) GetTexts() []string {
	return track.texts
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
