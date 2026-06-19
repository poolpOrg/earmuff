package midiimport

import (
	"sort"
	"strings"
	"testing"

	"github.com/poolpOrg/earmuff/elaborator"
	"github.com/poolpOrg/earmuff/parser"
	"github.com/poolpOrg/earmuff/smfwriter"
)

// compile parses+elaborates source to a Song (first project), failing on error.
func compile(t *testing.T, src string) elaborator.Song {
	t.Helper()
	prog, diags := parser.New(src, "<test>").Parse()
	if len(diags) != 0 {
		t.Fatalf("parse: %v\nsource:\n%s", diags, src)
	}
	songs, errs := elaborator.Elaborate(prog)
	if len(errs) != 0 {
		t.Fatalf("elaborate: %v\nsource:\n%s", errs, src)
	}
	if len(songs) == 0 {
		t.Fatalf("no songs from:\n%s", src)
	}
	return songs[0]
}

// noteOnset returns sorted (tick,key) pairs for every NoteOn in a song.
func noteOnsets(song elaborator.Song) [][2]int {
	var out [][2]int
	for _, ev := range song.Events {
		if ev.Msg.Kind == elaborator.MsgNoteOn && ev.Msg.Velocity > 0 {
			out = append(out, [2]int{int(ev.Tick), int(ev.Msg.Key)})
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i][0] != out[j][0] {
			return out[i][0] < out[j][0]
		}
		return out[i][1] < out[j][1]
	})
	return out
}

func TestImport_FaithfulRoundTrip(t *testing.T) {
	src := `project "p" { bpm 120; time 4 4;
        track "lead" instrument "piano" {
            bar quarter { Cmaj7 G7 Am7 Dm7 }
            bar quarter { C^4 E^4 G^4 C^5 }
        }
    }`
	orig := compile(t, src)
	mid := smfwriter.Write(orig)

	out, err := Import(mid, Options{Faithful: true})
	if err != nil {
		t.Fatalf("import: %v", err)
	}
	t.Logf("imported source:\n%s", out)

	round := compile(t, out)
	want := noteOnsets(orig)
	got := noteOnsets(round)
	if len(got) != len(want) {
		t.Fatalf("note count: got %d, want %d\n%s", len(got), len(want), out)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("note %d: got (tick=%d,key=%d), want (tick=%d,key=%d)\n%s",
				i, got[i][0], got[i][1], want[i][0], want[i][1], out)
		}
	}
}

func TestImport_ReadableCompiles(t *testing.T) {
	src := `project "p" { bpm 100; time 4 4;
        track "lead" instrument "piano" { bar 8 { C^ E^ G^ C^ E^ G^ C^ E^ } }
    }`
	mid := smfwriter.Write(compile(t, src))
	out, err := Import(mid, Options{}) // readable
	if err != nil {
		t.Fatalf("import: %v", err)
	}
	t.Logf("readable source:\n%s", out)
	if !strings.Contains(out, "bar 16") && !strings.Contains(out, "bar 8") {
		t.Fatalf("expected a gridded bar in output:\n%s", out)
	}
	compile(t, out) // must recompile without error
}

func TestImport_ChordNames(t *testing.T) {
	src := `project "p" { time 4 4; track "t" instrument "piano" { bar 1 { Cmaj7 } } }`
	mid := smfwriter.Write(compile(t, src))
	out, _ := Import(mid, Options{Faithful: true})
	if !strings.Contains(out, "Cmaj7") {
		t.Fatalf("expected chord name Cmaj7 in:\n%s", out)
	}
}
