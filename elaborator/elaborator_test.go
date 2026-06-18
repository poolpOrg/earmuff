package elaborator

import (
	"os"
	"path/filepath"
	"sort"
	"testing"

	"github.com/poolpOrg/earmuff/parser"
)

func elaborateFile(t *testing.T, name string) []Song {
	t.Helper()
	src, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatalf("read %s: %v", name, err)
	}
	p := parser.New(string(src), name)
	prog, diags := p.Parse()
	if len(diags) != 0 {
		for _, d := range diags {
			t.Errorf("parse diagnostic: %s", d)
		}
		t.Fatalf("%s did not parse cleanly", name)
	}
	songs, errs := Elaborate(prog)
	if len(errs) != 0 {
		for _, e := range errs {
			t.Errorf("elaborate error: %v", e)
		}
		t.Fatalf("%s did not elaborate cleanly", name)
	}
	return songs
}

// noteOns returns the (tick,key) of every NoteOn in the first track, sorted by
// tick then key.
func noteOns(song Song) [][2]int {
	var out [][2]int
	for _, ev := range song.Events {
		if ev.Msg.Kind == MsgNoteOn {
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

// TestNuagesBar1Ticks is the timing correctness anchor (docs §3a): an 8th-note
// run on a 16th grid starting on beat 3 must land on consecutive 16th-note
// positions, tick-exact.
func TestNuagesBar1Ticks(t *testing.T) {
	songs := elaborateFile(t, "nuages.ear")
	if len(songs) != 1 {
		t.Fatalf("nuages: got %d songs, want 1", len(songs))
	}
	ons := noteOns(songs[0])

	// midi key numbers (octave 4 default): C#4=61, D4=62, A4=69, G#4=68, G4=67, F#4=66
	wantBar1 := []struct {
		tick int
		key  uint8
	}{
		{1920, 61}, // C# on beat 3
		{2160, 62}, // D  on beat 3.25
		{2400, 69}, // A  on beat 3.5
		{2640, 68}, // G# on beat 3.75
		{2880, 67}, // G  on beat 4
		{3360, 66}, // F# on beat 4.5
	}
	for i, w := range wantBar1 {
		if i >= len(ons) {
			t.Fatalf("nuages: only %d NoteOns, expected at least %d", len(ons), len(wantBar1))
		}
		if ons[i][0] != w.tick {
			t.Errorf("NoteOn %d at tick %d, want %d (key %d)", i, ons[i][0], w.tick, ons[i][1])
		}
		if ons[i][1] != int(w.key) {
			t.Errorf("NoteOn %d key %d, want %d", i, ons[i][1], w.key)
		}
	}
}

// TestNuagesGates verifies that a :8 gate on a 16th grid produces an 8th-note
// sounding length (NoteOff one 8th = 480 ticks after NoteOn).
func TestNuagesGates(t *testing.T) {
	songs := elaborateFile(t, "nuages.ear")
	var onTick, offTick int = -1, -1
	for _, ev := range songs[0].Events {
		if ev.Msg.Kind == MsgNoteOn && ev.Tick == 1920 { // the C#
			onTick = int(ev.Tick)
		}
		if ev.Msg.Kind == MsgNoteOff && ev.Msg.Key == 61 && onTick >= 0 && offTick < 0 {
			offTick = int(ev.Tick)
		}
	}
	if onTick != 1920 {
		t.Fatalf("did not find C# NoteOn at 1920")
	}
	if offTick-onTick != 480 {
		t.Errorf("C# gate = %d ticks, want 480 (an 8th note)", offTick-onTick)
	}
}

func TestElaborate_AllExamples(t *testing.T) {
	for _, f := range []string{"nuages.ear", "blues.ear", "comp.ear", "bend.ear"} {
		songs := elaborateFile(t, f)
		if len(songs) == 0 {
			t.Errorf("%s: no songs", f)
			continue
		}
		total := 0
		for _, s := range songs {
			total += len(s.Events)
		}
		if total == 0 {
			t.Errorf("%s: elaborated to zero events", f)
		}
	}
}

func TestBendRPNAndValue(t *testing.T) {
	// `bend +2` should emit the RPN range setup CCs and a PitchBend event.
	songs := elaborateFile(t, "bend.ear")
	var sawBend, sawRPN bool
	for _, s := range songs {
		for _, ev := range s.Events {
			if ev.Msg.Kind == MsgPitchBend {
				sawBend = true
			}
			// RPN range setup uses CC 101/100/6/38
			if ev.Msg.Kind == MsgCC && (ev.Msg.Controller == 101 || ev.Msg.Controller == 100) {
				sawRPN = true
			}
		}
	}
	if !sawBend {
		t.Errorf("bend: no PitchBend event emitted")
	}
	if !sawRPN {
		t.Errorf("bend: no RPN pitch-bend-range setup emitted")
	}
}
