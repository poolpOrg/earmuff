---
title: Editor support
weight: 40
---

# Editor support

## Language server

`earmuff-lsp` is a language server that works in any LSP-capable editor. It
provides:

- live **diagnostics** (from earmuff's own parser and analyzer, so they match
  the compiler exactly);
- **completion**;
- **hover**;
- **go-to-definition**;
- a document-symbol **outline**.

Install it with:

```sh
go install github.com/poolpOrg/earmuff/cmd/earmuff-lsp@latest
```

## VS Code extension

The VS Code extension bundles the language server and adds:

- **syntax highlighting** for `.ear` files (keywords, note/chord literals,
  durations, dynamics, strings, comments, operators);
- a **live sheet-music preview** that re-renders automatically (debounced) as
  you type, engraving the unsaved buffer with LilyPond;
- commands (from the Command Palette or the editor's title-bar buttons):
  - **earmuff: Compile to MIDI** — writes `<file>.mid` next to the source;
  - **earmuff: Play** — streams the piece to a MIDI port;
  - **earmuff: Show Sheet Preview** — opens the live sheet-music preview beside
    the editor.

The language server is bundled for macOS (arm64/x64), Linux (x64/arm64), and
Windows (x64), so editor features need no extra install on those platforms. The
compile/play commands invoke the `earmuff` CLI, which you
[install]({{< relref "/docs/install" >}}) separately. The sheet preview needs
[LilyPond]({{< relref "/docs/sheet-music" >}}).

### Settings

| Setting | Default | Description |
| --- | --- | --- |
| `earmuff.languageServer.enable` | `true` | Enable the language server. |
| `earmuff.languageServer.path` | `earmuff-lsp` | Override the server binary. |
| `earmuff.cli.path` | `earmuff` | Path to the `earmuff` CLI used by the commands. |
| `earmuff.player` | `""` | Player command for **Play** (`{}` = the MIDI file). Empty = auto-detect. |
| `earmuff.lilypond.path` | `lilypond` | Path to the LilyPond binary used by the preview. |

For setup details, see
[`editors/vscode/README.md`](https://github.com/poolpOrg/earmuff/blob/main/editors/vscode/README.md).
