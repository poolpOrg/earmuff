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
