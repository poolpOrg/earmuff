---
title: Install
weight: 2
---

# Install

earmuff is written in [Go](https://go.dev) and requires Go 1.18+.

## Binaries

There are two binaries. Install the compiler/player:

```sh
go install github.com/poolpOrg/earmuff/cmd/earmuff@latest
```

And, for editor support, the language server:

```sh
go install github.com/poolpOrg/earmuff/cmd/earmuff-lsp@latest
```

Both land in `$(go env GOPATH)/bin`; make sure that directory is on your
`PATH`.

## Optional dependencies

These are only needed for the corresponding features:

- **Playback** — a MIDI synth. earmuff auto-detects one (`timidity` on Linux,
  `fluidsynth` with a SoundFont, etc.). See
  [Command line → Playback]({{< relref "/docs/command-line" >}}) for the
  resolution order and the macOS note.
- **Sheet music** — [LilyPond](https://lilypond.org) on your `PATH`, used by
  the `-pdf`/`-svg`/`-ly` flags. See
  [Sheet music]({{< relref "/docs/sheet-music" >}}).

## Verify it works

Compile and play one of the bundled examples:

```sh
earmuff examples/12bars-blues.ear
```

With no output flag and without `-quiet`, earmuff plays the piece. To check the
build without needing a synth, write a MIDI file instead:

```sh
earmuff -out blues.mid examples/12bars-blues.ear
```

Next: take the [Language]({{< relref "/docs/language" >}}) tour.
