package lilypond

import (
	"strings"
	"testing"

	"github.com/poolpOrg/earmuff/elaborator"
	"github.com/poolpOrg/earmuff/parser"
)

func render(t *testing.T, src string) string {
	t.Helper()
	prog, diags := parser.New(src, "<test>").Parse()
	if len(diags) != 0 {
		t.Fatalf("parse: %v", diags)
	}
	songs, errs := elaborator.Elaborate(prog)
	if len(errs) != 0 {
		t.Fatalf("elaborate: %v", errs)
	}
	return Render(songs[0])
}

func TestRender_Skeleton(t *testing.T) {
	ly := render(t, `project "demo" { bpm 120; time 4 4;
		track "lead" instrument "piano" { bar quarter { C E G _ } }
	}`)
	for _, want := range []string{
		"\\version", "\\header", "title = \"demo\"",
		"\\score {", "\\new Staff {", "\\clef", "\\time 4/4",
		"\\tempo 4 = 120", "instrumentName = \"lead\"", "\\layout",
	} {
		if !strings.Contains(ly, want) {
			t.Errorf("rendered .ly missing %q", want)
		}
	}
}

func TestRender_OneStaffPerTrack(t *testing.T) {
	ly := render(t, `project "p" { time 4 4;
		track "a" instrument "piano" { bar quarter { C E G _ } }
		track "b" instrument "bass"  { bar quarter { C2 _ _ _ } }
	}`)
	if n := strings.Count(ly, "\\new Staff"); n != 2 {
		t.Fatalf("got %d staves, want 2", n)
	}
}

func TestRender_BassClefForLowTrack(t *testing.T) {
	ly := render(t, `project "p" { time 4 4;
		track "bass" instrument "bass" { bar quarter { C2 E2 G2 _ } }
	}`)
	if !strings.Contains(ly, "\\clef bass") {
		t.Fatalf("low track should use bass clef:\n%s", ly)
	}
}

func TestRender_ChordBecomesAngleBrackets(t *testing.T) {
	ly := render(t, `project "p" { time 4 4;
		track "g" instrument "guitar" { bar 1 { Cmaj7 } }
	}`)
	if !strings.Contains(ly, "<") || !strings.Contains(ly, ">") {
		t.Fatalf("a chord should render as <...>:\n%s", ly)
	}
}

func TestQuantize(t *testing.T) {
	// a quarter note = ppq ticks -> "4"
	if got := quantize(ppq); len(got) != 1 || got[0] != "4" {
		t.Fatalf("quantize(quarter) = %v, want [4]", got)
	}
	// a whole note
	if got := quantize(ppq * 4); len(got) != 1 || got[0] != "1" {
		t.Fatalf("quantize(whole) = %v, want [1]", got)
	}
	// dotted half = 3 quarters -> "2."
	if got := quantize(ppq * 3); len(got) != 1 || got[0] != "2." {
		t.Fatalf("quantize(3 quarters) = %v, want [2.]", got)
	}
}

func TestPitch(t *testing.T) {
	cases := map[uint8]string{60: "c'", 62: "d'", 48: "c", 72: "c''", 61: "cis'"}
	for key, want := range cases {
		if got := pitch(key); got != want {
			t.Errorf("pitch(%d) = %q, want %q", key, got, want)
		}
	}
}
