package types

type Project struct {
	name      string
	bpm       float64
	signature *Signature
	tracks    []*Track
	copyright string
	texts     []string
}

func NewProject() *Project {
	return &Project{
		tracks: make([]*Track, 0),
		texts:  make([]string, 0),
	}
}

func (project *Project) GetName() string {
	return project.name
}

func (project *Project) SetName(name string) {
	project.name = name
}

func (project *Project) GetBPM() float64 {
	return project.bpm
}

func (project *Project) SetBPM(bpm float64) {
	project.bpm = bpm
}

func (project *Project) GetSignature() *Signature {
	return project.signature
}

func (project *Project) SetSignature(signature *Signature) {
	project.signature = signature
}

func (project *Project) AddTrack(track *Track) {
	project.tracks = append(project.tracks, track)
}

func (project *Project) AddText(text string) {
	project.texts = append(project.texts, text)
}

func (project *Project) SetCopyright(text string) {
	project.copyright = text
}

func (project *Project) GetTracks() []*Track {
	return project.tracks
}

func (project *Project) GetTexts() []string {
	return project.texts
}

func (project *Project) GetCopyright() string {
	return project.copyright
}
