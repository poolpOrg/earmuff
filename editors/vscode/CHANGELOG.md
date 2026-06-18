# Changelog

## Unreleased

- New command **earmuff: Show Sheet Preview** — opens a live sheet-music PDF
  preview beside the editor that re-renders (debounced) from the unsaved buffer
  as you type. Engraving uses LilyPond via `earmuff -pdf`; configure its
  location with the new `earmuff.lilypond.path` setting. The PDF is rendered in
  the webview with a bundled PDF.js.

## 0.1.0

Initial release.

- Syntax highlighting for `.ear` files (keywords, note/chord literals,
  durations, dynamics, strings, comments, operators).
- Language-server integration via `earmuff-lsp`: live diagnostics, completion,
  hover, go-to-definition, and a document-symbol outline.
- Commands: **earmuff: Compile to MIDI** and **earmuff: Play**, which invoke the
  `earmuff` CLI.
- The language server ships bundled for macOS (arm64/x64), Linux (x64/arm64),
  and Windows (x64); it falls back to a `PATH` binary or a configured path.
