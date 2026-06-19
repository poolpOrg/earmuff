package elaborator

import (
	"testing"

	"github.com/poolpOrg/earmuff/parser"
)

func disambigKeys(t *testing.T, src string) []uint8 {
	t.Helper()
	prog, diags := parser.New(src, "<test>").Parse()
	if len(diags) != 0 {
		t.Fatalf("parse diagnostics: %v", diags)
	}
	songs, errs := Elaborate(prog)
	if len(errs) != 0 {
		t.Fatalf("elaborate errors: %v", errs)
	}
	var keys []uint8
	for _, ev := range songs[0].Events {
		if ev.Msg.Kind == MsgNoteOn && ev.Msg.Velocity > 0 {
			keys = append(keys, ev.Msg.Key)
		}
	}
	return keys
}

func TestDisambig_NoteVsChord(t *testing.T) {
	tr := func(body string) string {
		return `project "t"{time 4 4;track "g" instrument "guitar"{` + body + `}}`
	}
	// C7 is ambiguous (note C8ve7 vs C dominant-7 chord); the chord wins.
	if got := disambigKeys(t, tr(`bar 1 { C7 }`)); len(got) != 4 {
		t.Fatalf("C7 -> %v, want a 4-note chord", got)
	}
	// A trailing ^ forces the note reading (MIDI 96 = C7 pitch).
	if got := disambigKeys(t, tr(`bar 1 { C7^ }`)); len(got) != 1 || got[0] != 96 {
		t.Fatalf("C7^ -> %v, want [96]", got)
	}
	// Low octave digits (0-4) are never chord names, so C4 is a note.
	if got := disambigKeys(t, tr(`bar 1 { C4 }`)); len(got) != 1 || got[0] != 60 {
		t.Fatalf("C4 -> %v, want [60]", got)
	}
	// A bare letter is a single note, not a major triad.
	if got := disambigKeys(t, tr(`bar quarter { C _ _ _ }`)); len(got) != 1 {
		t.Fatalf("bare C -> %v, want 1 note", got)
	}
	// Explicit qualities are unaffected.
	if got := disambigKeys(t, tr(`bar 1 { Am7 }`)); len(got) != 4 {
		t.Fatalf("Am7 -> %v, want 4-note chord", got)
	}
}

func TestFor_BareSequenceAndEach(t *testing.T) {
	tr := func(body string) string {
		return `project "t"{time 4 4;track "g" instrument "piano"{` + body + `}}`
	}
	// `for i in C E G` binds i across three pitches.
	if got := disambigKeys(t, tr(`for i in C E G { bar quarter { i _ _ _ } }`)); len(got) != 3 {
		t.Fatalf("for i in C E G -> %v, want 3 notes (C,E,G)", got)
	}
	// `for each 1 2 3` iterates three times with no binding.
	if got := disambigKeys(t, tr(`for each 1 2 3 { bar 1 { C } }`)); len(got) != 3 {
		t.Fatalf("for each 1 2 3 -> %v, want 3 iterations of C", got)
	}
	// ranges still work.
	if got := disambigKeys(t, tr(`for each 1..3 { bar 1 { C } }`)); len(got) != 3 {
		t.Fatalf("for each 1..3 -> %v, want 3", got)
	}
}

func swingOnsets(t *testing.T, src string) []uint32 {
	t.Helper()
	prog, diags := parser.New(src, "<test>").Parse()
	if len(diags) != 0 {
		t.Fatalf("parse diagnostics: %v", diags)
	}
	songs, errs := Elaborate(prog)
	if len(errs) != 0 {
		t.Fatalf("elaborate errors: %v", errs)
	}
	var ticks []uint32
	for _, ev := range songs[0].Events {
		if ev.Msg.Kind == MsgNoteOn && ev.Msg.Velocity > 0 {
			ticks = append(ticks, ev.Tick)
		}
	}
	return ticks
}

func TestSwing_DelaysOffBeats(t *testing.T) {
	const head = `project "p"{time 4 4;track "t" instrument "piano"{`
	straight := swingOnsets(t, head+` bar 8 { C C C C } }}`)
	swung := swingOnsets(t, head+` swing 67; bar 8 { C C C C } }}`)

	// On-beats (indices 0,2) are unchanged; off-beats (1,3) are delayed.
	if straight[0] != swung[0] || straight[2] != swung[2] {
		t.Fatalf("on-beats moved: straight=%v swung=%v", straight, swung)
	}
	if swung[1] <= straight[1] || swung[3] <= straight[3] {
		t.Fatalf("off-beats not delayed: straight=%v swung=%v", straight, swung)
	}
	// Delay should be (2*0.67-1)*480 ≈ 163 ticks.
	if d := swung[1] - straight[1]; d < 150 || d > 175 {
		t.Fatalf("swing delay = %d ticks, want ~163", d)
	}
}

func TestSwing_StraightIsNoOp(t *testing.T) {
	const head = `project "p"{time 4 4;track "t" instrument "piano"{`
	a := swingOnsets(t, head+` bar 8 { C C C C } }}`)
	b := swingOnsets(t, head+` swing 50; bar 8 { C C C C } }}`)
	for i := range a {
		if a[i] != b[i] {
			t.Fatalf("swing 50 changed onsets: %v vs %v", a, b)
		}
	}
}
