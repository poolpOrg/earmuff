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
