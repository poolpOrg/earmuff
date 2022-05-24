package types

type Track struct {
	bpm       uint8
	signature *Signature
	bars      []*Bar
}

func NewTrack() *Track {
	return &Track{
		bars: make([]*Bar, 0),
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
