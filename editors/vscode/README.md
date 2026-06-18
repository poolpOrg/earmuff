# earmuff for VS Code

Language support for the [earmuff](https://github.com/poolpOrg/earmuff) music
DSL — write music as `.ear` source and get editor feedback, completion, and
one-click compile/play.

## Features

- **Syntax highlighting** for `.ear` files (keywords, note/chord literals,
  durations, dynamics, strings, comments, operators).
- **Language server** (`earmuff-lsp`): live diagnostics, completion, hover,
  go-to-definition, and a document-symbol outline. Diagnostics come from
  earmuff's own parser and analyzer, so they match the compiler exactly.
- **Commands** (Command Palette, or the buttons on a `.ear` editor's title bar):
  - **earmuff: Compile to MIDI** — writes `<file>.mid` next to the source.
  - **earmuff: Play** — streams the piece to a MIDI port (e.g. FluidSynth).
  - **earmuff: Show Sheet Preview** — opens a live sheet-music PDF preview
    beside the editor. It re-renders automatically (debounced) as you type,
    using the unsaved buffer, so you see the engraved score update as you write.

## Requirements

The language server is **bundled** for macOS (arm64/x64), Linux (x64/arm64), and
Windows (x64) — no extra install needed for editor features on those platforms.

The compile/play **commands** invoke the `earmuff` CLI, which you install
separately:

```sh
go install github.com/poolpOrg/earmuff/cmd/earmuff@latest
```

On an unbundled platform, also install the server:

```sh
go install github.com/poolpOrg/earmuff/cmd/earmuff-lsp@latest
```

The **Show Sheet Preview** command engraves the score with
[LilyPond](https://lilypond.org/), which you must install separately:

```sh
brew install lilypond        # macOS; use your package manager elsewhere
```

LilyPond must be on your `PATH`, or point `earmuff.lilypond.path` at the
binary. If it is missing, the preview panel shows the renderer's error instead
of the sheet music.

## Settings

| Setting | Type | Default | Description |
| --- | --- | --- | --- |
| `earmuff.languageServer.enable` | boolean | `true` | Enable the language server. |
| `earmuff.languageServer.path` | string | `earmuff-lsp` | Override the server binary. Takes precedence over the bundled one; a bare name resolves from `PATH`. |
| `earmuff.cli.path` | string | `earmuff` | Path to the `earmuff` CLI used by the compile/play commands. |
| `earmuff.player` | string | `""` | Player command for **Play**, e.g. `timidity {}` or `fluidsynth -a coreaudio /path/to.sf2 {}` (`{}` = the MIDI file). Empty = auto-detect. |
| `earmuff.lilypond.path` | string | `lilypond` | Path to the LilyPond binary used by **Show Sheet Preview**. A bare name resolves from `PATH`; a non-default value is passed to the CLI as `-lilypond`. |

The server is resolved in this order: an explicit `languageServer.path` →
the bundled binary for your platform → `earmuff-lsp` on `PATH`.

### Playback

**Play** asks the `earmuff` CLI to render and play through an available synth.
With `earmuff.player` empty, the CLI auto-detects one: a platform-native player
(`timidity` on Linux), then `fluidsynth` if a SoundFont is found. macOS has no
built-in headless MIDI player, so install one — `brew install fluid-synth` (a
SoundFont comes with it) — or set `earmuff.player`. Point
`EARMUFF_SOUNDFONT` at a `.sf2` to choose the SoundFont fluidsynth uses.

## Development

```sh
npm install
npm run build-server   # cross-compile earmuff-lsp into ./server/<platform>/
npm run compile        # tsc -> ./out
npm run package        # build server + compile + produce a .vsix
```

Press `F5` in VS Code to launch an Extension Development Host with the extension
loaded; open a `.ear` file to try it. `npm run watch` recompiles on save.
