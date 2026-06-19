---
title: Command line
weight: 20
---

# Command line

```text
earmuff [flags] source.ear
```

| Flag | Description |
| --- | --- |
| `-out file.mid` | write a Standard MIDI File |
| `-pdf file.pdf` | engrave sheet music as PDF (needs lilypond) |
| `-svg file.svg` | engrave sheet music as SVG (needs lilypond) |
| `-ly file.ly` | write the intermediate LilyPond source |
| `-player <cmd>` | player command template, `{}` = the MIDI file |
| `-lilypond <path>` | path to the lilypond binary (for `-pdf`/`-svg`) |
| `-quiet` | suppress the summary and skip playback |
| `-verbose` | dump the elaborated event stream |
| `-import` | read a `.mid` and emit `.ear` source (the reverse direction) |
| `-faithful` | with `-import`: exact `on beat` timing instead of a quantized grid |
| `-grid N` | with `-import`: quantization grid as a note value (default 16) |

With no `-out`/`-pdf`/`-svg` and without `-quiet`, earmuff **plays** the piece.

```sh
earmuff song.ear                 # play it (auto-detects an available synth)
earmuff -out song.mid song.ear   # export a Standard MIDI File
earmuff -pdf song.pdf song.ear   # engrave sheet music
```

See [Sheet music]({{< relref "/docs/sheet-music" >}}) for the notation flags.

## Importing MIDI

earmuff also goes the other way: `-import` turns a Standard MIDI File into
editable earmuff source on stdout (or to `-out`).

```sh
earmuff -import song.mid > song.ear            # readable, quantized to a grid
earmuff -import -faithful song.mid > song.ear  # exact timing via `on beat`
```

Import is heuristic — MIDI is a flat event stream, so the result is a faithful
reconstruction, not the original source. Simultaneous notes are named as chords
when a name fits (otherwise emitted as a note group), and percussion tracks
become a `kit`. The readable mode quantizes onsets for clean bars; `-faithful`
places every note at its exact beat so re-compiling reproduces the timing.

(The same import is built into the [playground]({{< relref "/playground" >}}) —
drop a `.mid` or `.ear` file onto the page.)

## Playback

earmuff resolves a player in this order, so playback works out of the box on
most setups:

1. the `-player` flag or the `EARMUFF_PLAYER` env var — a command template where
   `{}` is replaced by the MIDI file;
2. a platform-native player (`timidity`/`wildmidi` on Linux, the file
   association on Windows);
3. `fluidsynth`, when a SoundFont is found. Set `EARMUFF_SOUNDFONT` to choose
   which `.sf2` it uses.

macOS has no built-in headless MIDI player. Install `fluidsynth`
(`brew install fluid-synth`, which brings a SoundFont) or set `EARMUFF_PLAYER`,
for example:

```sh
export EARMUFF_PLAYER='fluidsynth -a coreaudio /path/to/soundfont.sf2 {}'
```
