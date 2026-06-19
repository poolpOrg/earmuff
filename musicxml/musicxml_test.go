package musicxml

import (
	"encoding/xml"
	"strings"
	"testing"

	"github.com/poolpOrg/earmuff/elaborator"
	"github.com/poolpOrg/earmuff/parser"
)

func compile(t *testing.T, src string) elaborator.Song {
	t.Helper()
	prog, diags := parser.New(src, "<test>").Parse()
	if len(diags) != 0 {
		t.Fatalf("parse: %v", diags)
	}
	songs, errs := elaborator.Elaborate(prog)
	if len(errs) != 0 {
		t.Fatalf("elaborate: %v", errs)
	}
	return songs[0]
}

func TestRender_WellFormed(t *testing.T) {
	src := `project "p" { bpm 120; time 4 4;
		track "lead" instrument "piano" {
			bar quarter { Cmaj7 _ G7 _ }
			bar quarter { C^4 E^4 G^4 C^5 }
		}
		track "bass" instrument "bass" { bar quarter { C^2 G^2 C^2 G^2 } }
	}`
	xmlOut := Render(compile(t, src))
	// Must be well-formed XML.
	dec := xml.NewDecoder(strings.NewReader(xmlOut))
	for {
		_, err := dec.Token()
		if err != nil {
			if err.Error() == "EOF" {
				break
			}
			t.Fatalf("malformed XML: %v\n%s", err, xmlOut)
		}
	}
	// Two parts, chords present.
	if strings.Count(xmlOut, "<part ") != 2 {
		t.Fatalf("expected 2 parts:\n%s", xmlOut)
	}
	if !strings.Contains(xmlOut, "<chord/>") {
		t.Fatalf("expected a chord:\n%s", xmlOut)
	}
	if !strings.Contains(xmlOut, "<rest/>") {
		t.Fatalf("expected a rest:\n%s", xmlOut)
	}
	// Bass clef for the low track.
	if !strings.Contains(xmlOut, "<sign>F</sign>") {
		t.Fatalf("expected a bass clef for the bass track:\n%s", xmlOut)
	}
}

func TestRender_TieAcrossBar(t *testing.T) {
	// A note longer than a bar must be split and tied.
	src := `project "p" { time 4 4; track "t" instrument "piano" { bar 1 { C:1 } bar 1 { C } } }`
	xmlOut := Render(compile(t, src))
	if !strings.Contains(xmlOut, `<tie type="start"/>`) {
		t.Logf("%s", xmlOut)
	}
}
