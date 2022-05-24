package types

type Bar struct {
	bpm       uint8
	signature Signature
	beats     []Beat
}

func NewBar() *Bar {
	return &Bar{
		beats: make([]Beat, 0),
	}
}

func (bar *Bar) GetBPM() uint8 {
	return bar.bpm
}

func (bar *Bar) SetBPM(bpm uint8) {
	bar.bpm = bpm
}

func (bar *Bar) GetSignature() Signature {
	return bar.signature
}

func (bar *Bar) SetSignature(signature Signature) {
	bar.signature = signature
}

func (bar *Bar) AddBeat(beat Beat) {
	bar.beats = append(bar.beats, beat)
}
