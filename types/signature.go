package types

type Signature struct {
	beats    uint8
	duration uint8
}

func NewSignature(beats uint8, duration uint8) *Signature {
	return &Signature{
		beats:    beats,
		duration: duration,
	}
}

func (signature *Signature) GetBeats() uint8 {
	return signature.beats
}

func (signature *Signature) GetDuration() uint8 {
	return signature.duration
}
