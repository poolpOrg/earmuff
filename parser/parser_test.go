package parser

import (
	"testing"

	"github.com/poolpOrg/earmuff/ast"
)

func parseOK(t *testing.T, src string) *ast.Program {
	t.Helper()
	p := New(src, "<test>")
	prog, errs := p.Parse()
	if len(errs) != 0 {
		for _, e := range errs {
			t.Errorf("diagnostic: %s", e)
		}
		t.Fatalf("parse of %q produced %d diagnostics", src, len(errs))
	}
	return prog
}

func parseErr(t *testing.T, src string) []Diagnostic {
	t.Helper()
	p := New(src, "<test>")
	_, errs := p.Parse()
	if len(errs) == 0 {
		t.Fatalf("expected diagnostics for %q, got none", src)
	}
	return errs
}

func TestParse_MinimalProject(t *testing.T) {
	parseOK(t, `project "p" { bpm 120; time 4 4; }`)
}

func TestParse_StepGridBar(t *testing.T) {
	parseOK(t, `project "p" {
		time 4 4;
		track "lead" instrument "piano" {
			bar quarter { C E G _ }
		}
	}`)
}

func TestParse_GatesTiesRepeats(t *testing.T) {
	parseOK(t, `project "p" {
		track "lead" instrument "piano" {
			bar 16 { _*8 C#:8 D:8 A:8 G#:8 G:8 _ F#:8 _ }
			bar 16 { F:2 _*13 E:8 _ }
			bar 8  { F:2 _ _ Eb:4 _ _ _ D:8 }
			bar 1  { D }
		}
	}`)
}

func TestParse_GridSwitchAndBarSep(t *testing.T) {
	parseOK(t, `project "p" {
		track "t" instrument "piano" {
			bar quarter { C 16: D E F G | A }
		}
	}`)
}

func TestParse_ControlFlow(t *testing.T) {
	parseOK(t, `project "p" {
		track "t" instrument "guitar" {
			for i in 1..2 {
				if i == 2 { bar whole { C7 } } else { bar whole { C7 } }
			}
		}
	}`)
}

func TestParse_PatternsAndLists(t *testing.T) {
	parseOK(t, `project "p" {
		pattern comp(changes) {
			for ch in changes { bar quarter { ch ch ch ch } }
		}
		track "piano" instrument "piano" {
			let aTune = [Am6, Dm6, E7, Am6];
			comp(aTune)
			comp([Dm6, Am6, E7, Am6])
		}
	}`)
}

func TestParse_Velocity(t *testing.T) {
	parseOK(t, `project "p" {
		track "lead" instrument "violin" v mp {
			bar quarter v f { C D E:v ff F }
		}
	}`)
}

func TestParse_RawMidiAndKit(t *testing.T) {
	parseOK(t, `project "p" {
		track "syn" instrument "lead 1 (square)" channel 3 {
			program "lead 2 (sawtooth)";
			bar quarter { C cc cutoff = 64 E cc cutoff = 100 }
			on beat 1 bend +2;
			on beat 2 pressure 90;
			sysex F0 7E 7F 09 01 F7;
		}
		track "drums" instrument "drum kit" channel 10 {
			kit { hh = "closed hi-hat"; oh = "open hi-hat"; sn = "acoustic snare"; cy = "crash cymbal 1"; }
			bar 8 { (oh,sn,cy) hh hh hh _*4 }
		}
	}`)
}

func TestParse_OnBeatEscape(t *testing.T) {
	parseOK(t, `project "p" {
		track "t" instrument "piano" {
			bar { on beat 1 Gmaj7:2 on beat 3 Am7:4 on beat 4 Bm7b5:4 }
			bar {}
		}
	}`)
}

func TestParse_Transposition(t *testing.T) {
	parseOK(t, `project "p" {
		track "bass" instrument "bass" {
			for root in [C2, F2, G2] {
				bar quarter { root (root + fifth) (root + octave) _ }
			}
		}
	}`)
}

func TestParse_PrattPrecedence(t *testing.T) {
	// 1 + 2 * 3 should bind * tighter than +
	p := New(`project "p" { track "t" { for i in 1 .. (1 + 2 * 3) { bar 1 { C } } } }`, "<test>")
	_, errs := p.Parse()
	if len(errs) != 0 {
		t.Fatalf("precedence parse failed: %v", errs)
	}
}

func TestParse_ErrorsRecover(t *testing.T) {
	// two separate errors should both be reported (recovery works)
	errs := parseErr(t, `project "p" {
		track "t" instrument "piano" {
			bar quarter { C @ }
			let = 5;
		}
	}`)
	if len(errs) < 2 {
		t.Fatalf("expected >=2 diagnostics with recovery, got %d: %v", len(errs), errs)
	}
}

func TestParse_DiagnosticPosition(t *testing.T) {
	errs := parseErr(t, "project \"p\" {\n  bogus\n}")
	if errs[0].Pos.Line != 2 {
		t.Fatalf("diagnostic at line %d, want 2 (%s)", errs[0].Pos.Line, errs[0])
	}
}
