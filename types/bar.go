package types

type Bar struct {
	bpm       float64
	signature *Signature

	tickables []Tickable

	textsOn map[float64][]string
}

func NewBar() *Bar {
	return &Bar{
		tickables: make([]Tickable, 0),
		textsOn:   make(map[float64][]string),
	}
}

//func (bar *Bar) GetOffset() uint32 {
//	return bar.offset
//}

func (bar *Bar) GetBPM() float64 {
	return bar.bpm
}

func (bar *Bar) SetBPM(bpm float64) {
	bar.bpm = bpm
}

func (bar *Bar) GetSignature() *Signature {
	return bar.signature
}

func (bar *Bar) SetSignature(signature *Signature) {
	bar.signature = signature
}

func (bar *Bar) AddTickable(tickable Tickable) {
	bar.tickables = append(bar.tickables, tickable)
}

func (bar *Bar) GetTickables() []Tickable {
	return bar.tickables
}
