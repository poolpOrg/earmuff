---
title: Introduction
weight: 1
---

# Introduction

**earmuff** is a small language for describing music in plain text. You write a
*project* made of *tracks* and *bars*; earmuff parses and analyzes the source,
then turns it into a Standard MIDI File, live playback through a synth, or
engraved sheet music.

## Why text

Music in earmuff is just source code, so the tools you already use apply to it
directly:

- **Diffable** — a change to a phrase is a line in a diff, not an opaque binary.
- **Reviewable** — pull requests, comments, and blame work on a song.
- **Scriptable** — generate progressions, transpose, or template arrangements
  with ordinary code, then compile the result.

Everything in earmuff is resolved at compile time, so a given source always
produces the same output — the music is deterministic.

## The pipeline

A source file flows through a fixed pipeline:

```text
source.ear → lexer → parser → analyzer → elaborator → ┬→ Standard MIDI File
                                          (event stream) ├→ live playback
                                                         └→ LilyPond → PDF / SVG
```

The parser is a hand-written recursive-descent parser with a Pratt expression
grammar. The analyzer reports structural and light harmony problems. The
**elaborator** expands patterns, loops, and conditionals into a flat,
absolute-tick event stream — the single source of truth that every back end
consumes. From that one stream earmuff can write a `.mid` file, play live, or
emit LilyPond for notation.

## Features

- **Readable, diffable syntax** — a step grid for rhythm, named notes and
  chords (`C#`, `Am7`, `Gmaj7`), and an `on beat` escape hatch for exact timing.
- **Programmable** — reusable `pattern`s and `section`s, `for` loops, `repeat`,
  a `swing` feel, `if`/`else`, and immutable `let` bindings, all resolved at
  compile time so output is deterministic.
- **Full MIDI** — control change, pitch bend, aftertouch, program change,
  sysex, and per-event channels, alongside notes and chords.
- **Three outputs from one source** — a Standard MIDI File, live playback, or
  engraved sheet music (PDF/SVG via LilyPond).
- **Imports MIDI, too** — `earmuff -import song.mid` turns a Standard MIDI File
  back into editable `.ear` source, so you can bring existing tunes in.
- **Editor support** — a language server (diagnostics, completion, hover,
  go-to-definition, outline) and a VS Code extension with a live sheet-music
  preview that updates as you type.

Next: [Install]({{< relref "/docs/install" >}}) earmuff, or jump to the
[Language]({{< relref "/docs/language" >}}) overview.
