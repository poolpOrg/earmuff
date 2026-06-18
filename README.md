# earmuff

> Write music as code — version it, diff it, review it, generate it — then play it or print the score.

**earmuff** is a small language for describing music in plain text. You write a
*project* of *tracks* and *bars*; earmuff parses and analyzes it, then turns it
into a Standard MIDI File, live playback through a synth, or engraved sheet
music. Because the source is just text, every developer tool you already use —
git, diff, code review, scripts that generate music — works on it for free.

```
project "12 bars blues" {
    bpm 120; time 4 4;

    track "lead piano" instrument "piano" {
        pattern I  { bar quarter { C E G _ } }
        pattern IV { bar quarter { F A C _ } }
        pattern V  { bar quarter { G B D _ } }

        I() IV()
        for _ in 1..2 { I() }
        for _ in 1..2 { IV() }
        for _ in 1..2 { I() }
        V() IV() I() V()
    }

    track "drums" instrument "synth drum" channel 10 {
        kit { hh = "closed hi-hat"; sn = "acoustic snare"; cy = "crash cymbal 1"; }
        pattern groove { bar quarter { (cy, sn) hh hh hh } }
        for _ in 1..12 { groove() }
    }
}
```

## Features

- **Readable, diffable syntax** — a step grid for rhythm, named notes and
  chords (`C#`, `Am7`, `Gmaj7`), and an `on beat` escape hatch for exact timing.
- **Programmable** — reusable `pattern`s with parameters, `for` loops over
  ranges and lists, `if`/`else`, and immutable `let` bindings, all resolved at
  compile time so output is deterministic.
- **Full MIDI** — control change, pitch bend, aftertouch, program change,
  sysex, and per-event channels, alongside notes and chords.
- **Three outputs from one source** — a Standard MIDI File, live playback, or
  engraved **sheet music** (PDF/SVG via LilyPond).
- **Editor support** — a language server (diagnostics, completion, hover,
  go-to-definition, outline) and a VS Code extension with a **live sheet-music
  preview** that updates as you type.

## Install

Requires [Go](https://go.dev) 1.18+.

```sh
# the compiler / player
go install github.com/poolpOrg/earmuff/cmd/earmuff@latest

# the language server (for editor support)
go install github.com/poolpOrg/earmuff/cmd/earmuff-lsp@latest
```

Optional, for the respective features:

- **Playback** — a MIDI synth. earmuff auto-detects one (`timidity` on Linux,
  `fluidsynth` with a SoundFont, etc.); see [Playback](#playback).
- **Sheet music** — [LilyPond](https://lilypond.org) on your `PATH`.

## Quickstart

```sh
# play it (auto-detects an available synth)
earmuff song.ear

# write a Standard MIDI File
earmuff -out song.mid song.ear

# engrave sheet music
earmuff -pdf song.pdf song.ear
earmuff -svg song.svg song.ear
```

Browse the [`examples/`](examples/) directory for complete pieces.

## The language

A quick tour; the full grammar and timing model are in
[`docs/language-v2.md`](docs/language-v2.md).

**Step grid.** A bar lays events on a grid whose step is the bar's duration; the
cursor advances one step per token. `_` is a rest, `~` ties the previous note,
and `:dur` sets a note's sounding length independently.

```
bar quarter { C E G _ }        // four quarter-note slots
bar 8 { C C C C C C C C }      // eighth-note grid
bar quarter { C:2 _ E _ }      // C rings a half note
```

**Notes and chords** are written by name and transposed with intervals:

```
C  C#  Eb  F#3  Cb            // notes (optional octave)
Am7  Gmaj7  C7  Dm7b5         // chords
C + fifth                    // transposition
```

**Patterns and control flow** run at compile time:

```
pattern walk(root, third) { bar quarter { root _ third _ } }

let changes = [Am7, Dm7, G7, Cmaj7];
for ch in changes { bar quarter { ch ch ch ch } }

for i in 1..4 {
    if i == 4 { bar whole { C7 } } else { bar whole { C } }
}
```

**Raw MIDI** events are first class:

```
program "violin";
cc 74 = 64;        // control change
bend +2;           // pitch bend, in semitones
pressure 90;       // channel aftertouch
sysex F0 7E 7F 09 01 F7;
```

**Dynamics and velocity** — `v100` or named dynamics (`v mf`), per note, bar, or
track:

```
track "lead" instrument "violin" v mp { bar quarter v f { C D E:v ff F } }
```

## Command line

```
earmuff [flags] source.ear

  -out  file.mid   write a Standard MIDI File
  -pdf  file.pdf   engrave sheet music as PDF (needs lilypond)
  -svg  file.svg   engrave sheet music as SVG (needs lilypond)
  -ly   file.ly    write the intermediate LilyPond source
  -player <cmd>    player command template, "{}" = the MIDI file
  -lilypond <path> path to the lilypond binary (for -pdf/-svg)
  -quiet           suppress the summary and skip playback
  -verbose         dump the elaborated event stream
```

With no `-out`/`-pdf`/`-svg` and not `-quiet`, earmuff plays the piece.

### Playback

earmuff resolves a player in this order, so playback works out of the box on
most setups:

1. the `-player` flag or `EARMUFF_PLAYER` env (a command template, `{}` = file);
2. a platform-native player (`timidity`/`wildmidi` on Linux, the file
   association on Windows);
3. `fluidsynth`, when a SoundFont is found (set `EARMUFF_SOUNDFONT` to choose
   one).

macOS has no built-in headless MIDI player; install `fluidsynth`
(`brew install fluid-synth`) or set `EARMUFF_PLAYER`.

## Editor support

The language server (`earmuff-lsp`) provides live diagnostics, completion,
hover, go-to-definition, and a document outline in any LSP-capable editor.

The **VS Code extension** ([`editors/vscode/`](editors/vscode/)) bundles the
language server and adds:

- syntax highlighting for `.ear`;
- a **live sheet-music preview** that re-renders as you type;
- commands: **Compile to MIDI**, **Play**, and **Show Sheet Preview**.

See [`editors/vscode/README.md`](editors/vscode/README.md) for setup. The sheet
preview and `-pdf`/`-svg` need LilyPond.

## Examples

| File | Shows |
| --- | --- |
| [`12bars-blues.ear`](examples/12bars-blues.ear) | patterns, loops, drums |
| [`minor-swing.ear`](examples/minor-swing.ear) | nested loops, comping |
| [`nuages.ear`](examples/nuages.ear) | syncopation on a fine grid |
| [`lofi.ear`](examples/lofi.ear) | lists, dynamics, off-beats |
| [`small-jazz.ear`](examples/small-jazz.ear) | a small swing arrangement |
| [`bach-prelude.ear`](examples/bach-prelude.ear) | a classical fragment |
| [`mozart-nachtmusik.ear`](examples/mozart-nachtmusik.ear) | a classical fragment |

## How it works

```
source.ear → lexer → parser → analyzer → elaborator → ┬→ Standard MIDI File
                                          (event stream) ├→ live playback
                                                         └→ LilyPond → PDF / SVG
```

The parser is a hand-written recursive-descent parser with a Pratt expression
grammar. The analyzer reports structural and light harmony problems. The
elaborator expands patterns, loops, and conditionals into a flat, absolute-tick
event stream, which the back ends turn into MIDI or sheet music.

| Path | Contents |
| --- | --- |
| `cmd/earmuff` | the compiler / player CLI |
| `cmd/earmuff-lsp` | the language server |
| `lexer`, `parser`, `ast` | front end |
| `analyzer` | static analysis |
| `value`, `elaborator` | compile-time evaluation → event stream |
| `smfwriter` | event stream → Standard MIDI File |
| `lilypond` | event stream → LilyPond source |
| `player` | MIDI playback |
| `midi` | General MIDI instrument / percussion maps |
| `lsp`, `editors/vscode` | editor tooling |
| `docs/language-v2.md` | language reference |

## Contributing

Issues and pull requests are welcome. The Go code is tested
(`go test ./...`) and formatted with `gofmt`; please keep both green.

## License

ISC. See [LICENSE](LICENSE).
