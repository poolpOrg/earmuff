package parser

import (
	"testing"

	"github.com/poolpOrg/earmuff/ast"
)

func parseTrackBody(t *testing.T, body string) []ast.Stmt {
	t.Helper()
	src := `project "p" { track "t" instrument "piano" { ` + body + ` } }`
	prog, diags := New(src, "<test>").Parse()
	if len(diags) != 0 {
		t.Fatalf("parse %q: %v", body, diags)
	}
	return prog.Items[0].(*ast.Project).Tracks[0].Body
}

func TestSection_DefinesPattern(t *testing.T) {
	body := parseTrackBody(t, `section head { bar quarter { C E G _ } }`)
	pd, ok := body[0].(*ast.PatternDef)
	if !ok {
		t.Fatalf("section -> %T, want PatternDef", body[0])
	}
	if pd.Name != "head" || len(pd.Params) != 0 {
		t.Fatalf("section pattern = %q/%d params, want head/0", pd.Name, len(pd.Params))
	}
}

func TestBareName_IsZeroArgCall(t *testing.T) {
	body := parseTrackBody(t, `section head { bar 1 { C } } head head`)
	calls := 0
	for _, st := range body {
		if pc, ok := st.(*ast.PatternCall); ok {
			calls++
			if pc.Name != "head" || len(pc.Args) != 0 {
				t.Fatalf("bare call = %q/%d args, want head/0", pc.Name, len(pc.Args))
			}
		}
	}
	if calls != 2 {
		t.Fatalf("got %d bare calls, want 2", calls)
	}
}
