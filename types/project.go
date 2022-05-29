package types

import (
	"fmt"
	"sync"
	"time"

	"github.com/faiface/beep"
	"github.com/faiface/beep/generators"
	"github.com/faiface/beep/speaker"
)

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
	wg := sync.WaitGroup{}
	for _, track := range project.GetTracks() {
		for _, bar := range track.GetBars() {
			for _, playable := range bar.GetPlayables() {
				wg.Add(1)
				go func(b *Bar, p Playable) {
					timer := time.NewTimer(b.GetTimestamp() + p.GetTimestamp())
					<-timer.C

					fmt.Println(time.Now(), "Playing", p.GetType(), p.GetName(), "for", p.GetDurationTime())
					p.Play()
					wg.Done()
				}(bar, playable)
			}
		}
	}
	wg.Wait()

}

//////
type Anote struct {
	freq float64
}

func (st *Anote) Play(duration time.Duration) {
	sr := beep.SampleRate(41100)
	sine, err := generators.SineTone(sr, st.freq)
	if err != nil {
		panic(err)
	}

	done := make(chan bool)
	speaker.Play(beep.Seq(beep.Take(sr.N(duration), sine), beep.Callback(func() {
		done <- true
	})))
	<-done
}
