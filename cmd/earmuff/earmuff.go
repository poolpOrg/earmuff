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

	"github.com/poolpOrg/earmuff/compiler"
	"github.com/poolpOrg/earmuff/parser"
	"github.com/poolpOrg/go-synctimer"
	"github.com/youpy/go-coremidi"
	"gitlab.com/gomidi/midi/v2/smf"
)

type tickevent struct {
	trackEvent smf.TrackEvent
	packet     coremidi.Packet
}

func main() {
	var opt_file string
	var opt_verbose bool
	var opt_quiet bool

	flag.StringVar(&opt_file, "out", "", "output file (.mid)")
	flag.BoolVar(&opt_quiet, "quiet", false, "do not play")
	flag.BoolVar(&opt_verbose, "verbose", false, "extensive logging")
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

	b := compiler.Compile(project)

	if opt_file != "" {
		fp, err := os.Create(opt_file)
		if err != nil {
			log.Fatal(err)
		}
		fp.Write(b)
		fp.Close()
	}

	ticks := make(map[int64][]tickevent)
	ticksList := make([]int64, 0)
	smf.ReadTracksFrom(bytes.NewReader(b)).Do(
		func(te smf.TrackEvent) {
			offset := te.AbsMicroSeconds / 1000
			if _, exists := ticks[offset]; !exists {
				ticks[offset] = make([]tickevent, 0)
				ticksList = append(ticksList, offset)
			}
			ticks[offset] = append(ticks[offset], tickevent{te, coremidi.NewPacket(te.Message.Bytes(), 0)})
			if opt_verbose {
				fmt.Printf("plan: [%v] @%vms %s\n", te.TrackNo, te.AbsMicroSeconds/1000, te.Message.String())
			}
		},
	)

	if opt_quiet {
		os.Exit(0)
	}

	client, err := coremidi.NewClient("earmuff")
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
	if dest == nil {
		log.Fatal("could not find synthesiser")
	}

	wg := sync.WaitGroup{}
	t := synctimer.NewTimer()
	for _, tick := range ticksList {
		events := ticks[tick]
		wg.Add(1)
		go func(_events []tickevent, c chan bool) {
			<-c
			for _, event := range _events {
				if opt_verbose {
					fmt.Printf("synth: [%v] @%vms %s\n", event.trackEvent.TrackNo, event.trackEvent.AbsMicroSeconds/1000, event.trackEvent.Message.String())
				}
				err := event.packet.Send(&outPorts[event.trackEvent.TrackNo], dest)
				if err != nil {
					fmt.Println(err)
				}
			}
			wg.Done()
		}(events, t.NewSubTimer(time.Duration(int(tick)*int(time.Millisecond))))
	}
	t.Start()
	wg.Wait()

}
