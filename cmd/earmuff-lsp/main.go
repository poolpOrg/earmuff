// Command earmuff-lsp is a Language Server Protocol server for earmuff v2
// source files. It speaks LSP over stdio and is meant to be launched by an
// editor (e.g. the VS Code extension in editors/vscode). It reuses earmuff's
// parser and analyzer so diagnostics match the compiler exactly.
package main

import (
	"os"

	"github.com/poolpOrg/earmuff/lsp"
)

func main() {
	srv := lsp.NewServer(os.Stdin, os.Stdout)
	if err := srv.Run(); err != nil {
		os.Exit(1)
	}
}
