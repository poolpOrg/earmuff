package types

type Bar struct {
	offset    uint32
	bpm       uint8
	signature *Signature
	playables []Playable
}

func NewBar(offset uint32) *Bar {
	return &Bar{
		offset:    offset,
		playables: make([]Playable, 0),
	}
}

func (bar *Bar) GetOffset() uint32 {
	return bar.offset
}

func (bar *Bar) GetBPM() uint8 {
	return bar.bpm
}

func (bar *Bar) SetBPM(bpm uint8) {
	bar.bpm = bpm
}

func (bar *Bar) GetSignature() *Signature {
	return bar.signature
}

func (bar *Bar) SetSignature(signature *Signature) {
	bar.signature = signature
}

func (bar *Bar) AddPlayable(playable Playable) {
	bar.playables = append(bar.playables, playable)
}

func (bar *Bar) GetPlayables() []Playable {
	return bar.playables
}
