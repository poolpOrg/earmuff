# earmuff

earmuff is a language for writing music as code. You describe a piece in plain
text — projects, tracks, bars, notes, chords — and earmuff compiles it to a
Standard MIDI File or streams it live to a synthesizer.

Because the source is text, you get every software-development tool for free:
version control, diffing, code review, and programmatic generation. earmuff is
not a replacement for a score editor; it is an intermediate, diffable format
that reads and produces MIDI.

> Status: the v2 language and toolchain (parser, analyzer, MIDI backend,
> language server, and VS Code extension) are in place. See
> [`docs/language-v2.md`](docs/language-v2.md) for the full language reference.

## Install

Requires Go 1.18+.

```sh
# the compiler / player
go install github.com/poolpOrg/earmuff/cmd/earmuff@latest

# the language server (for editors)
go install github.com/poolpOrg/earmuff/cmd/earmuff-lsp@latest
```

## What it looks like

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

    track "rythm guitar" instrument "guitar" {
        bar whole { C7 } bar whole { F7 }
        for _ in 1..2 { bar whole { C7 } }
    }

    track "drums" instrument "synth drum" channel 10 {
        kit { hh = "closed hi-hat"; sn = "acoustic snare"; cy = "crash cymbal 1"; }
        pattern groove { bar quarter { (cy, sn) hh hh hh } }
        for _ in 1..12 { groove() }
    }
}
```

More complete programs live in [`examples/`](examples/).

## Language essentials

- **Step grid.** A `bar quarter { C E G _ }` lays notes on a grid whose step is
  the bar's duration; the cursor advances one step per token. `_` is a rest,
  `~` ties (extends the previous note), and `:dur` sets a note's sounding length
  independently (`C:8`). Switch the grid mid-bar with `16:` and repeat a token
  with `*` (`_*4`).
- **Notes & chords** are recognized by name: `C`, `C#`, `Eb`, `F#3`, `Am7`,
  `Gmaj7`, `C7/E`. Transpose with intervals: `C + fifth`.
- **Patterns & control flow.** `pattern name(args) { ... }` defines reusable,
  parameterized bodies; `for x in 1..4 { ... }` / `for c in [Am7, D7] { ... }`,
  `if/else`, and immutable `let` bindings run at compile time, so output is
  deterministic and diffable.
- **Full MIDI.** Raw events are first class: `cc 74 = 64`, `bend +2`,
  `pressure 90`, `program "violin"`, `sysex F0 ... F7`, per-event `@channel`.
- **Velocity & dynamics.** `v100` or named dynamics (`v mf`), settable per note,
  per bar, or per track.
- **Escape hatch.** `on beat 2.5 ...` places an event at an absolute beat when
  the grid is inconvenient.

The full grammar and timing model are documented in
[`docs/language-v2.md`](docs/language-v2.md).

## Usage

```sh
earmuff song.ear                 # play to a MIDI port (e.g. FluidSynth)
earmuff -quiet -out song.mid song.ear   # compile to a Standard MIDI File
earmuff -verbose song.ear        # dump the elaborated event stream
```

earmuff parses, statically analyzes (reporting harmony and structural problems),
elaborates the program to a time-ordered event stream, and then writes MIDI or
plays it.

## Editor support

A language server (`earmuff-lsp`) and a VS Code extension provide diagnostics,
completion, hover, go-to-definition, a symbol outline, and compile/play
commands. See [`editors/vscode/`](editors/vscode/).

## Project layout

| Path | What |
| --- | --- |
| `cmd/earmuff` | the compiler / player CLI |
| `cmd/earmuff-lsp` | the language server |
| `lexer`, `parser`, `ast` | front end (Pratt expression parser) |
| `analyzer` | static analysis (structural + light harmony) |
| `value`, `elaborator` | compile-time evaluation → absolute-tick event stream |
| `smfwriter` | event stream → Standard MIDI File |
| `midi` | General MIDI instrument / percussion maps |
| `lsp`, `editors/vscode` | editor tooling |
| `docs/language-v2.md` | language reference |

## License

ISC. See [LICENSE](LICENSE).
