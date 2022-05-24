package main

import (
	"flag"
	"fmt"
	"log"
	"math"
	"os"
	"time"

	"github.com/faiface/beep"
	"github.com/faiface/beep/speaker"
	"github.com/poolpOrg/earring/parser"
	"github.com/poolpOrg/earring/types"
)

type LoTick struct{}
type HiTick struct{}

func (st *LoTick) Play(duration time.Duration) {
	sr := beep.SampleRate(44100)
	done := make(chan bool)
	speaker.Play(beep.Seq(beep.Take(sr.N(duration), st), beep.Callback(func() {
		done <- true
	})))
	<-done
}

func (st LoTick) Stream(samples [][2]float64) (n int, ok bool) {
	for i := range samples {
		sample := math.Sin((math.Pi * 2 / float64(44100)) * 440.0 * float64(i))
		samples[i][0] = sample
		samples[i][1] = sample
	}
	return len(samples), true
}
func (st LoTick) Err() error {
	return nil
}

func (st *HiTick) Play(duration time.Duration) {
	sr := beep.SampleRate(44100)
	done := make(chan bool)
	speaker.Play(beep.Seq(beep.Take(sr.N(duration), st), beep.Callback(func() {
		done <- true
	})))
	<-done
}

func (st HiTick) Stream(samples [][2]float64) (n int, ok bool) {
	for i := range samples {
		sample := math.Sin((math.Pi * 2 / float64(44100)) * 220.0 * float64(i))
		samples[i][0] = sample
		samples[i][1] = sample
	}
	return len(samples), true
}
func (st HiTick) Err() error {
	return nil
}

func main() {
	flag.Parse()

	if flag.NArg() == 0 {
		log.Fatal("need a source file to process")
	}

	fp, err := os.Open(flag.Arg(0))
	if err != nil {
		log.Fatal(err)
	}
	defer fp.Close()

	parser := parser.NewParser(fp)
	project, err := parser.Parse()
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("Project signature %d/%d at %dbpm\n",
		project.GetSignature().GetBeats(),
		project.GetSignature().GetDuration(),
		project.GetBPM())

	c := make(chan *types.Beat, 100)

	for trackOffset, track := range project.GetTracks() {
		fmt.Println("\tTrack", trackOffset)
		barOffset := 0
		for _, bar := range track.GetBars() {
			fmt.Println("\t\tBar", barOffset)
			for offset, beat := range bar.GetBeats() {
				c <- beat
				fmt.Println("\t\t\tBeat", offset)
				for _, duration := range beat.GetDurations() {
					fmt.Println("\t\t\t\t",
						duration.GetDurationName(), duration.GetPlayable().Type(), duration.GetPlayable().Name())
				}
			}
			barOffset += 1
		}
	}

	d := time.Minute / time.Duration(project.GetBPM()*(project.GetSignature().GetDuration()/project.GetSignature().GetBeats()))
	t := time.NewTicker(d)
	i := uint8(0)

	sr := beep.SampleRate(44100)
	speaker.Init(44100, sr.N(time.Second/10))

	loTick := LoTick{}
	hiTick := HiTick{}

	done := false
	for {
		select {
		case <-t.C:
			if (i % project.GetSignature().GetBeats()) == 0 {
				go loTick.Play(time.Millisecond * 100)
			} else {
				go hiTick.Play(time.Millisecond * 100)
			}

			go func() {
				beat := <-c
				for _, duration := range beat.GetDurations() {
					fmt.Println(time.Now().Round(time.Millisecond), duration.GetDurationName(), duration.GetPlayable().Type(), duration.GetPlayable().Name())
				}
			}()
			i++
		}
		if done {
			break
		}
	}

	/*	for {
		select {
		case <-t.C:
			if i%project.GetSignature().GetBeats() == 0 {
				fmt.Printf("+")
			} else {
				fmt.Printf("-")
			}
			i++
		}
	}*/

}
