# Changelog

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
