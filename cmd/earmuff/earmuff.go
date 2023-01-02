package main

import (
	"bytes"
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/poolpOrg/earmuff/compiler"
	"github.com/poolpOrg/earmuff/parser"
	"gitlab.com/gomidi/midi/v2"
	_ "gitlab.com/gomidi/midi/v2/drivers/rtmididrv"
	"gitlab.com/gomidi/midi/v2/smf"
)

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

	smf.ReadTracksFrom(bytes.NewReader(b)).Do(
		func(te smf.TrackEvent) {
			if opt_verbose {
				fmt.Printf("[%v] @%vms %s\n", te.TrackNo, te.AbsMicroSeconds/1000, te.Message.String())
			}
		},
	)

	if opt_quiet {
		os.Exit(0)
	}

	out, err := midi.FindOutPort("FluidSynth virtual port")
	if err != nil {
		log.Fatal(fmt.Errorf("can't find fluidsynth"))
	}
	smf.ReadTracksFrom(bytes.NewReader(b)).Play(out)
}
