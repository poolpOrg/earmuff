package parser

import (
	"testing"

	"github.com/poolpOrg/earmuff/ast"
)

func parseForStmt(t *testing.T, body string) *ast.For {
	t.Helper()
	src := `project "p" { track "t" instrument "piano" { ` + body + ` } }`
	prog, diags := New(src, "<test>").Parse()
	if len(diags) != 0 {
		t.Fatalf("parse %q: %v", body, diags)
	}
	proj := prog.Items[0].(*ast.Project)
	for _, st := range proj.Tracks[0].Body {
		if f, ok := st.(*ast.For); ok {
			return f
		}
	}
	t.Fatalf("no For found in %q", body)
	return nil
}

func TestFor_BoundRange(t *testing.T) {
	f := parseForStmt(t, `for i in 1..3 { bar 1 { C } }`)
	if f.Var != "i" {
		t.Fatalf("Var = %q, want i", f.Var)
	}
	if _, ok := f.Iterable.(*ast.Range); !ok {
		t.Fatalf("iterable = %T, want Range", f.Iterable)
	}
}

func TestFor_BoundBareSequence(t *testing.T) {
	f := parseForStmt(t, `for i in 1 2 3 { bar 1 { C } }`)
	if f.Var != "i" {
		t.Fatalf("Var = %q, want i", f.Var)
	}
	list, ok := f.Iterable.(*ast.ListLit)
	if !ok || len(list.Elements) != 3 {
		t.Fatalf("iterable = %T (%d elems), want ListLit of 3", f.Iterable, len(list.Elements))
	}
}

func TestFor_EachUnbound(t *testing.T) {
	f := parseForStmt(t, `for each 1 2 3 { bar 1 { C } }`)
	if f.Var != "" {
		t.Fatalf("Var = %q, want empty (unbound)", f.Var)
	}
	if list, ok := f.Iterable.(*ast.ListLit); !ok || len(list.Elements) != 3 {
		t.Fatalf("iterable = %T, want ListLit of 3", f.Iterable)
	}
}

func TestFor_EachChords(t *testing.T) {
	f := parseForStmt(t, `for each Am7 Dm7 G7 { bar 1 { C } }`)
	if f.Var != "" {
		t.Fatalf("Var = %q, want empty", f.Var)
	}
	list := f.Iterable.(*ast.ListLit)
	if len(list.Elements) != 3 {
		t.Fatalf("got %d chord elements, want 3", len(list.Elements))
	}
}

func TestFor_BoundList(t *testing.T) {
	// existing bracketed list form still works
	f := parseForStmt(t, `for ch in [Am7, Dm7] { bar 1 { ch } }`)
	if f.Var != "ch" {
		t.Fatalf("Var = %q, want ch", f.Var)
	}
}

func TestFor_BoundBindingName(t *testing.T) {
	f := parseForStmt(t, `for ch in changes { bar 1 { C } }`)
	if _, ok := f.Iterable.(*ast.Ident); !ok {
		t.Fatalf("iterable = %T, want Ident (binding name)", f.Iterable)
	}
}

func TestRepeat_DesugarsToForEach(t *testing.T) {
	f := parseForStmt(t, `repeat 12 { bar 1 { C } }`)
	if f.Var != "" {
		t.Fatalf("repeat Var = %q, want empty (unbound)", f.Var)
	}
	r, ok := f.Iterable.(*ast.Range)
	if !ok {
		t.Fatalf("repeat iterable = %T, want Range", f.Iterable)
	}
	lo, ok := r.Lo.(*ast.NumberLit)
	if !ok || lo.Value != 1 {
		t.Fatalf("repeat range lo = %v, want 1", r.Lo)
	}
}

func TestRepeat_ExprCount(t *testing.T) {
	// count can be an expression (a binding), not only a literal.
	f := parseForStmt(t, `let n = 4; repeat n { bar 1 { C } }`)
	r := f.Iterable.(*ast.Range)
	if _, ok := r.Hi.(*ast.Ident); !ok {
		t.Fatalf("repeat range hi = %T, want Ident", r.Hi)
	}
}
