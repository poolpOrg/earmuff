package types

type Track struct {
	bpm       uint8
	signature Signature
}

func NewTrack() *Track {
	return &Track{}
}

func (track *Track) GetBPM() uint8 {
	return track.bpm
}

func (track *Track) SetBPM(bpm uint8) {
	track.bpm = bpm
}

func (track *Track) GetSignature() Signature {
	return track.signature
}

func (track *Track) SetSignature(signature Signature) {
	track.signature = signature
}
