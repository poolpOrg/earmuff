package types

type Project struct {
	bpm       uint8
	signature *Signature
	tracks    []*Track
}

func NewProject() *Project {
	return &Project{
		tracks: make([]*Track, 0),
	}
}

func (project *Project) GetBPM() uint8 {
	return project.bpm
}

func (project *Project) SetBPM(bpm uint8) {
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

func (project *Project) GetTracks() []*Track {
	return project.tracks
}

func (project *Project) Play() {
	// first, start a goroutine to ensure that we always know what's up for next beat
	done := make(chan bool)
	go func() {
		for _, track := range project.GetTracks() {
			go func() {

			}()
		}
		done <- true
	}()

	// then start a ticker, playing playables for that beat
	<-done
}
