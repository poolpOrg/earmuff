package types

import (
	"fmt"
	"math"
)

type Project struct {
	bpm       float64
	signature *Signature
	tracks    []*Track
}

func NewProject() *Project {
	return &Project{
		tracks: make([]*Track, 0),
	}
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

func (project *Project) GetTracks() []*Track {
	return project.tracks
}

func (project *Project) String() string {
	ticksPerBeat := uint32(960)

	buf := "project {\n"
	buf += fmt.Sprintf("\tbpm %.02f;\n", project.GetBPM())
	buf += fmt.Sprintf("\ttime %d %d;\n", project.GetSignature().GetBeats(), project.GetSignature().GetDuration())

	for _, track := range project.tracks {
		buf += fmt.Sprintf("\n\tinstrument \"%s\" {\n", track.GetInstrument())
		if track.GetBPM() != project.GetBPM() {
			buf += fmt.Sprintf("\t\tbpm %.02f;\n", track.GetBPM())
		}
		if track.GetSignature() != project.GetSignature() {
			buf += fmt.Sprintf("\t\ttime %d %d;\n", track.GetSignature().GetBeats(), track.GetSignature().GetDuration())
		}

		for _, bar := range track.bars {
			buf += "\n\t\tbar {\n"
			if bar.GetBPM() != track.GetBPM() {
				buf += fmt.Sprintf("\t\t\tbpm %.02f;", bar.GetBPM())
			}
			if bar.GetSignature() != track.GetSignature() {
				buf += fmt.Sprintf("\t\t\ttime %d %d;", bar.GetSignature().GetBeats(), bar.GetSignature().GetDuration())
			}

			for _, playable := range bar.playables {
				begin := uint32(playable.GetTick()) / ticksPerBeat
				delta := uint32(playable.GetTick()) % ticksPerBeat
				_, frac := math.Modf(float64(delta) / float64(ticksPerBeat) * 100)

				beat := begin % uint32(bar.GetSignature().GetBeats())
				durationName := ""
				switch playable.GetDuration() {
				case 1:
					durationName = "whole"
				case 2:
					durationName = "half"
				case 4:
					durationName = "quarter"
				case 8:
					durationName = "8th"
				case 16:
					durationName = "16th"
				case 32:
					durationName = "32nd"
				case 64:
					durationName = "64th"
				case 128:
					durationName = "128th"
				case 256:
					durationName = "256th"

				}

				_, ok := playable.(*Note)
				if ok {
					note := playable.(*Note)

					if frac > 0. {
						buf += fmt.Sprintf("\t\t\t%s note %s on %f", durationName, note.GetName(), float64(beat+1)+frac)
					} else {
						buf += fmt.Sprintf("\t\t\t%s note %s on %d", durationName, note.GetName(), beat+1)
					}

					//fmt.Println(float64(note.duration) / float64(ticksPerBeat))
				}

				_, ok = playable.(*Chord)
				if ok {
					chord := playable.(*Chord)

					if frac > 0. {
						buf += fmt.Sprintf("\t\t\t%s note %s on %f", durationName, chord.GetName(), float64(beat+1)+frac)
					} else {
						buf += fmt.Sprintf("\t\t\t%s note %s on %d", durationName, chord.GetName(), beat+1)
					}

					//fmt.Println(float64(note.duration) / float64(ticksPerBeat))
				}

				buf += ";\n"
				//	buf := fmt.Sprintf("%d note %s on %d", note.duration, note.GetName(), note.GetTick())
				//	buf += ";"
			}

			buf += "\t\t}\n"
		}

		buf += "\t}\n"
	}

	buf += "}\n"
	return buf
}
