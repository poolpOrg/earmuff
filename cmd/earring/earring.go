package main

import (
	"bytes"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/poolpOrg/earring/midi"
	"github.com/poolpOrg/earring/parser"
	"github.com/poolpOrg/go-synctimer"
	"github.com/youpy/go-coremidi"
	"gitlab.com/gomidi/midi/v2/smf"
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

	b := midi.ToMidi(project)

	client, err := coremidi.NewClient("earring")
	if err != nil {
		fmt.Println(err)
		return
	}

	outPorts := make([]coremidi.OutputPort, len(project.GetTracks()))
	for i := 0; i < len(project.GetTracks()); i++ {
		port, err := coremidi.NewOutputPort(client, "output")
		if err != nil {
			fmt.Println(err)
			return
		}
		outPorts[i] = port
	}

	destinations, err := coremidi.AllDestinations()
	if err != nil {
		fmt.Println(err)
		return
	}
	var dest *coremidi.Destination
	for _, destination := range destinations {
		if strings.HasPrefix(destination.Name(), "FluidSynth") {
			dest = &destination
			break
		}
	}

	wg := sync.WaitGroup{}
	t := synctimer.NewTimer()

	smf.ReadTracksFrom(bytes.NewReader(b)).Do(
		func(te smf.TrackEvent) {
			if te.Message.IsMeta() {
				fmt.Printf("[%v] @%vms %s\n", te.TrackNo, te.AbsMicroSeconds/1000, te.Message.String())
			} else {
				//fmt.Printf("[%v] @%vms %s\n", te.TrackNo, te.AbsMicroSeconds/1000, te.Message.String())
				wg.Add(1)
				go func(_ev smf.TrackEvent, c chan bool) {
					p := coremidi.NewPacket(_ev.Message.Bytes(), 0)
					<-c
					//					fmt.Println("PLAYING", _ev)
					err := p.Send(&outPorts[_ev.TrackNo], dest)
					if err != nil {
						fmt.Println(err)
					}
					wg.Done()
				}(te, t.NewSubTimer(time.Duration(int(te.AbsMicroSeconds/1000)*int(time.Millisecond))))

			}
		},
	)
	t.Start()
	wg.Wait()

}
