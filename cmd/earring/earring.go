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
)

func ticker(bpm uint, beats uint, duration uint, done <-chan bool) {
	step := time.Minute / time.Duration(bpm*(duration/beats))
	fmt.Println("will tick every", step)

	loTick := LoTick{}
	hiTick := HiTick{}

	now := time.Now()
	next := now.Add(step)

	i := uint(0)
	for {
		time.Sleep(time.Until(next))
		// do something
		if (i % beats) == 0 {
			go hiTick.Play(step / time.Duration(duration))
		} else {
			go loTick.Play(step / time.Duration(duration))
		}
		i++
		select { // check whether `done` was closed
		case <-done:
			return
		default:
			// pass
		}

		// next is step - delta between now and previous next (catchup lags)
		next = next.Add(step - time.Now().Sub(next))
	}
}

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
		sample := math.Sin((math.Pi * 2 / float64(44100)) * 220.0 * float64(i))
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
		sample := math.Sin((math.Pi * 2 / float64(44100)) * 440.0 * float64(i))
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

	/*
		fmt.Printf("Project signature %d/%d at %dbpm\n",
			project.GetSignature().GetBeats(),
			project.GetSignature().GetDuration(),
			project.GetBPM())

		for trackOffset, track := range project.GetTracks() {
			fmt.Println("  Track", trackOffset+1)
			barOffset := 1
			for _, bar := range track.GetBars() {
				fmt.Printf("    %s: Bar %v\n", bar.GetTimestamp(), barOffset)
				for _, playable := range bar.GetPlayables() {
					fmt.Printf("        %s+%s->%s %s: %s %d\n", bar.GetTimestamp(), playable.GetTimestamp(), playable.GetDurationTime(), playable.GetType(), playable.GetName(), playable.GetDuration())
				}
				barOffset += 1
			}
		}
	*/
	sr := beep.SampleRate(44100)
	speaker.Init(44100, sr.N(time.Second/10))
	project.Play()

	/*
		done := make(chan bool)
		go ticker(uint(project.GetBPM()), uint(project.GetSignature().GetBeats()), uint(project.GetSignature().GetDuration()), done)

		t := time.NewTimer(time.Second * 30)
		<-t.C
		done <- true
	*/

	/*
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
	*/

}
