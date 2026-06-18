// Command earmuff is the v2 driver. It parses an .ear source file, runs the
// analyzer (if present), elaborates each project to an absolute-tick event
// stream, and writes a Standard MIDI File.
//
// Usage:
//
//	earmuff [flags] source.ear
//
// Flags:
//
//	-out file.mid   write the elaborated SMF to file.mid
//	-quiet          suppress the summary and skip playback
//	-verbose        dump the elaborated event stream
//	-player <tmpl>  player command template ("{}" = MIDI file)
//	-ly file.ly     write LilyPond sheet-music source
//	-pdf file.pdf   render a sheet-music PDF (requires lilypond)
//	-lilypond path  path to the lilypond binary (for -pdf)
//
// When -out is unset and not -quiet, earmuff plays the result through an
// available synth (see the player package): a -player/EARMUFF_PLAYER override,
// the platform-native player, or fluidsynth with a SoundFont. With -ly or -pdf,
// earmuff emits sheet music instead of MIDI.
package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/poolpOrg/earmuff/analyzer"
	"github.com/poolpOrg/earmuff/ast"
	"github.com/poolpOrg/earmuff/elaborator"
	"github.com/poolpOrg/earmuff/lilypond"
	"github.com/poolpOrg/earmuff/parser"
	"github.com/poolpOrg/earmuff/player"
	"github.com/poolpOrg/earmuff/smfwriter"
)

func main() {
	var (
		optOut      string
		optQuiet    bool
		optVerbose  bool
		optPlayer   string
		optLy       string
		optPDF      string
		optLilypond string
	)
	flag.StringVar(&optOut, "out", "", "output file (.mid)")
	flag.BoolVar(&optQuiet, "quiet", false, "suppress summary and playback")
	flag.BoolVar(&optVerbose, "verbose", false, "dump the elaborated event stream")
	flag.StringVar(&optPlayer, "player", "", "player command template, e.g. \"timidity {}\" ({} = MIDI file)")
	flag.StringVar(&optLy, "ly", "", "write LilyPond source to this file (sheet music)")
	flag.StringVar(&optPDF, "pdf", "", "render a sheet-music PDF to this file (requires lilypond)")
	flag.StringVar(&optLilypond, "lilypond", "lilypond", "path to the lilypond binary (for -pdf)")
	flag.Parse()

	if flag.NArg() == 0 {
		fmt.Fprintln(os.Stderr, "usage: earmuff [flags] source.ear")
		os.Exit(2)
	}

	file := flag.Arg(0)
	src, err := os.ReadFile(file)
	if err != nil {
		fmt.Fprintf(os.Stderr, "earmuff: %v\n", err)
		os.Exit(1)
	}

	// Parse: report diagnostics, abort on any.
	prog, pdiags := parser.New(string(src), file).Parse()
	for _, d := range pdiags {
		fmt.Fprintln(os.Stderr, d)
	}
	if len(pdiags) > 0 {
		os.Exit(1)
	}

	// Analyze: errors abort, warnings continue.
	if errs := analyze(prog); errs {
		os.Exit(1)
	}

	// Elaborate every project to a Song.
	songs, eerrs := elaborator.Elaborate(prog)
	for _, e := range eerrs {
		fmt.Fprintf(os.Stderr, "elaborate: %v\n", e)
	}
	if len(eerrs) > 0 {
		os.Exit(1)
	}
	if len(songs) == 0 {
		fmt.Fprintln(os.Stderr, "earmuff: no projects to elaborate")
		os.Exit(1)
	}

	if optVerbose {
		for _, song := range songs {
			fmt.Printf("# project %q: %d tracks, %d events\n", song.Name, len(song.Tracks), len(song.Events))
			for _, ev := range song.Events {
				fmt.Printf("[track %d] @%-7d %+v\n", ev.Track, ev.Tick, ev.Msg)
			}
		}
	}

	// Write the first project's score (the common single-project case).
	song := songs[0]

	// Sheet music: -ly writes LilyPond source; -pdf renders a PDF via lilypond.
	// Either short-circuits the MIDI/playback path.
	if optLy != "" || optPDF != "" {
		ly := lilypond.Render(song)
		if optLy != "" {
			if err := os.WriteFile(optLy, []byte(ly), 0o644); err != nil {
				fmt.Fprintf(os.Stderr, "earmuff: %v\n", err)
				os.Exit(1)
			}
		}
		if optPDF != "" {
			if err := renderPDF(ly, optPDF, optLilypond); err != nil {
				fmt.Fprintf(os.Stderr, "earmuff: %v\n", err)
				os.Exit(1)
			}
		}
		if !optQuiet {
			fmt.Printf("%s: %q -> sheet music\n", file, song.Name)
		}
		return
	}

	out := smfwriter.Write(song)

	if optOut != "" {
		if err := os.WriteFile(optOut, out, 0o644); err != nil {
			fmt.Fprintf(os.Stderr, "earmuff: %v\n", err)
			os.Exit(1)
		}
	}

	if !optQuiet {
		fmt.Printf("%s: %q: %d tracks, %d events, %d bytes",
			file, song.Name, len(song.Tracks), len(song.Events), len(out))
		if optOut != "" {
			fmt.Printf(" -> %s", optOut)
		}
		fmt.Println()

		// If we did not write to disk, play through an available synth.
		if optOut == "" {
			if err := player.Play(out, optPlayer); err != nil {
				fmt.Fprintf(os.Stderr, "earmuff: %v\n", err)
			}
		}
	}
}

// analyze runs the analyzer, printing diagnostics. It returns true if any
// error-severity diagnostic was found.
func analyze(prog *ast.Program) bool {
	diags := analyzer.Analyze(prog)
	hasErrors := false
	for _, d := range diags {
		fmt.Fprintln(os.Stderr, d)
		if d.Severity == analyzer.Error {
			hasErrors = true
		}
	}
	return hasErrors
}

// renderPDF writes LilyPond source to a temp dir, runs lilypond to engrave a
// PDF there, and moves it to outPath. lilypondBin is the engraver to invoke.
func renderPDF(ly, outPath, lilypondBin string) error {
	bin, err := exec.LookPath(lilypondBin)
	if err != nil {
		return fmt.Errorf("lilypond not found (%q): install it or pass -lilypond", lilypondBin)
	}
	dir, err := os.MkdirTemp("", "earmuff-ly-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(dir)

	lyPath := filepath.Join(dir, "score.ly")
	if err := os.WriteFile(lyPath, []byte(ly), 0o644); err != nil {
		return err
	}
	// lilypond writes <basename>.pdf into the output dir.
	cmd := exec.Command(bin, "--pdf", "-o", filepath.Join(dir, "score"), lyPath)
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("lilypond failed: %w", err)
	}
	data, err := os.ReadFile(filepath.Join(dir, "score.pdf"))
	if err != nil {
		return fmt.Errorf("lilypond produced no PDF: %w", err)
	}
	return os.WriteFile(outPath, data, 0o644)
}
