// Command earmuff-wasm compiles the earmuff toolchain to WebAssembly for the
// in-browser playground. It exposes a single global function,
// earmuffCompile(source) string, that runs the same pipeline as the CLI
// (parse -> analyze -> elaborate -> MIDI / LilyPond) and returns a JSON string.
//
// The JSON shape is the contract with the playground front end:
//
//	{
//	  "ok":          bool,                       // false if parse/elaborate failed
//	  "diagnostics": [ {line, column, severity, message}, ... ],
//	  "project":     "name",                     // first project, "" if none
//	  "trackCount":  int,
//	  "eventCount":  int,
//	  "bpm":         float,
//	  "timeBeats":   int,
//	  "timeUnit":    int,
//	  "durationTicks": int,                      // last event tick
//	  "ppq":         960,
//	  "tracks":      [ {name, instrument, channel, program}, ... ],
//	  "events":      [ {t, track, kind, ch, key, vel, ctrl, val, prog, bend, text}, ... ],
//	  "midiBase64":  "....",                      // Standard MIDI File bytes
//	  "lilypond":    "...."                       // LilyPond source for the score
//	}
//
// Build:  GOOS=js GOARCH=wasm go build -o earmuff.wasm ./cmd/earmuff-wasm
package main

import (
	"encoding/base64"
	"encoding/json"
	"syscall/js"

	"github.com/poolpOrg/earmuff/analyzer"
	"github.com/poolpOrg/earmuff/elaborator"
	"github.com/poolpOrg/earmuff/lilypond"
	"github.com/poolpOrg/earmuff/midiimport"
	"github.com/poolpOrg/earmuff/musicxml"
	"github.com/poolpOrg/earmuff/parser"
	"github.com/poolpOrg/earmuff/smfwriter"
)

// diag is one parse or analysis finding, flattened for the front end.
type diag struct {
	Line     int    `json:"line"`
	Column   int    `json:"column"`
	Severity string `json:"severity"` // "error" | "warning"
	Message  string `json:"message"`
}

// event is one MIDI event, named tersely to keep the JSON small (a piece can
// have thousands of events).
type event struct {
	T     uint32 `json:"t"`     // absolute tick
	Track int    `json:"track"` // track index
	Kind  int    `json:"kind"`  // elaborator.MsgKind
	Ch    uint8  `json:"ch"`
	Key   uint8  `json:"key,omitempty"`
	Vel   uint8  `json:"vel,omitempty"`
	Ctrl  uint8  `json:"ctrl,omitempty"`
	Val   uint8  `json:"val,omitempty"`
	Prog  uint8  `json:"prog,omitempty"`
	Bend  int16  `json:"bend,omitempty"`
	Text  string `json:"text,omitempty"`
}

type trackInfo struct {
	Name       string `json:"name"`
	Instrument string `json:"instrument"`
	Channel    uint8  `json:"channel"`
	Program    uint8  `json:"program"`
}

type result struct {
	OK            bool        `json:"ok"`
	Diagnostics   []diag      `json:"diagnostics"`
	Project       string      `json:"project"`
	TrackCount    int         `json:"trackCount"`
	EventCount    int         `json:"eventCount"`
	BPM           float64     `json:"bpm"`
	TimeBeats     int         `json:"timeBeats"`
	TimeUnit      int         `json:"timeUnit"`
	DurationTicks uint32      `json:"durationTicks"`
	PPQ           int         `json:"ppq"`
	Tracks        []trackInfo `json:"tracks"`
	Events        []event     `json:"events"`
	MIDIBase64    string      `json:"midiBase64"`
	LilyPond      string      `json:"lilypond"`
	MusicXML      string      `json:"musicxml"`
}

// compile runs the full pipeline and returns the result as JSON. It never
// panics on bad input: parse and elaborate errors come back as diagnostics with
// ok=false.
func compile(source string) string {
	res := result{PPQ: elaborator.PPQ, Diagnostics: []diag{}}

	prog, pdiags := parser.New(source, "<playground>").Parse()
	seen := map[diag]bool{}
	add := func(d diag) {
		if !seen[d] {
			seen[d] = true
			res.Diagnostics = append(res.Diagnostics, d)
		}
	}
	for _, d := range pdiags {
		add(diag{
			Line: d.Pos.Line, Column: d.Pos.Column,
			Severity: "error", Message: d.Msg,
		})
	}

	// Static analysis runs even with parse errors (it tolerates partial trees),
	// surfacing warnings alongside the hard errors.
	hasAnalyzerError := false
	for _, d := range analyzer.Analyze(prog) {
		sev := "warning"
		if d.Severity == analyzer.Error {
			sev = "error"
			hasAnalyzerError = true
		}
		add(diag{
			Line: d.Pos.Line, Column: d.Pos.Column,
			Severity: sev, Message: d.Msg,
		})
	}

	// If parsing produced errors, the tree is unreliable; stop before elaborating.
	if len(pdiags) > 0 || hasAnalyzerError {
		return marshal(res)
	}

	songs, eerrs := elaborator.Elaborate(prog)
	for _, e := range eerrs {
		res.Diagnostics = append(res.Diagnostics, diag{Severity: "error", Message: e.Error()})
	}
	if len(eerrs) > 0 || len(songs) == 0 {
		return marshal(res)
	}

	// The playground renders the first project, matching the CLI.
	song := songs[0]
	res.OK = true
	res.Project = song.Name
	res.TrackCount = len(song.Tracks)
	res.EventCount = len(song.Events)
	res.BPM = song.BPM
	res.TimeBeats = song.TimeBeats
	res.TimeUnit = song.TimeUnit

	for _, t := range song.Tracks {
		res.Tracks = append(res.Tracks, trackInfo{
			Name: t.Name, Instrument: t.Instrument,
			Channel: t.Channel, Program: t.Program,
		})
	}

	res.Events = make([]event, 0, len(song.Events))
	for _, ev := range song.Events {
		if ev.Tick > res.DurationTicks {
			res.DurationTicks = ev.Tick
		}
		m := ev.Msg
		res.Events = append(res.Events, event{
			T: ev.Tick, Track: ev.Track, Kind: int(m.Kind),
			Ch: m.Channel, Key: m.Key, Vel: m.Velocity,
			Ctrl: m.Controller, Val: m.Value, Prog: m.Program,
			Bend: m.Bend, Text: m.Text,
		})
	}

	res.MIDIBase64 = base64.StdEncoding.EncodeToString(smfwriter.Write(song))
	res.LilyPond = lilypond.Render(song)
	res.MusicXML = musicxml.Render(song)

	return marshal(res)
}

func marshal(res result) string {
	b, err := json.Marshal(res)
	if err != nil {
		// Should not happen for these types; surface it rather than crash.
		return `{"ok":false,"diagnostics":[{"line":0,"column":0,"severity":"error","message":"internal: ` + err.Error() + `"}]}`
	}
	return string(b)
}

func main() {
	js.Global().Set("earmuffCompile", js.FuncOf(func(_ js.Value, args []js.Value) any {
		if len(args) < 1 || args[0].Type() != js.TypeString {
			return `{"ok":false,"diagnostics":[{"line":0,"column":0,"severity":"error","message":"earmuffCompile expects a source string"}]}`
		}
		return compile(args[0].String())
	}))

	// earmuffImport(base64Midi, faithful) -> JSON {ok, source, error}.
	js.Global().Set("earmuffImport", js.FuncOf(func(_ js.Value, args []js.Value) any {
		if len(args) < 1 || args[0].Type() != js.TypeString {
			return `{"ok":false,"error":"earmuffImport expects a base64 MIDI string"}`
		}
		faithful := len(args) >= 2 && args[1].Truthy()
		return importMIDI(args[0].String(), faithful)
	}))

	// Signal readiness to the page, then block forever so the exported function
	// stays alive (a wasm main that returns tears down the instance).
	js.Global().Set("earmuffReady", js.ValueOf(true))
	select {}
}

// importMIDI decodes base64 SMF bytes and returns earmuff source as JSON.
func importMIDI(b64 string, faithful bool) string {
	data, err := base64.StdEncoding.DecodeString(b64)
	if err != nil {
		return `{"ok":false,"error":"invalid base64 MIDI data"}`
	}
	src, err := midiimport.Import(data, midiimport.Options{Faithful: faithful, Name: "imported"})
	if err != nil {
		b, _ := json.Marshal(struct {
			OK    bool   `json:"ok"`
			Error string `json:"error"`
		}{false, err.Error()})
		return string(b)
	}
	b, _ := json.Marshal(struct {
		OK     bool   `json:"ok"`
		Source string `json:"source"`
	}{true, src})
	return string(b)
}
