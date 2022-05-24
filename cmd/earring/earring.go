package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/poolpOrg/earring/parser"
	"github.com/poolpOrg/earring/types"
)

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

	for {
		select {
		case <-t.C:
			beat := <-c
			for _, duration := range beat.GetDurations() {
				fmt.Println(time.Now().Round(time.Millisecond), duration.GetDurationName(), duration.GetPlayable().Type(), duration.GetPlayable().Name())
			}
			i++
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
