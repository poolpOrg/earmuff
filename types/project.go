package types

type Project struct {
	bpm       uint8
	signature Signature
}

func NewProject() *Project {
	return &Project{}
}

func (project *Project) GetBPM() uint8 {
	return project.bpm
}

func (project *Project) SetBPM(bpm uint8) {
	project.bpm = bpm
}

func (project *Project) GetSignature() Signature {
	return project.signature
}

func (project *Project) SetSignature(signature Signature) {
	project.signature = signature
}
