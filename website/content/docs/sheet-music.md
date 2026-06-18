---
title: Sheet music
weight: 30
---

# Sheet music

earmuff can engrave a score from the same source it plays. It emits
[LilyPond](https://lilypond.org) and lets LilyPond do the engraving.

```sh
earmuff -pdf song.pdf song.ear   # engrave a PDF
earmuff -svg song.svg song.ear   # engrave an SVG
earmuff -ly  song.ly  song.ear   # write the intermediate LilyPond source
```

## LilyPond requirement

`-pdf` and `-svg` require [LilyPond](https://lilypond.org) on your `PATH`:

```sh
brew install lilypond   # macOS; use your package manager elsewhere
```

If LilyPond is installed somewhere unusual, point `-lilypond` at the binary.
`-ly` writes only the LilyPond source and needs no LilyPond installed, which is
useful for inspecting or hand-tweaking the output.

## A note on accuracy

earmuff's internal model is a stream of **performance events** — notes with
absolute start ticks and gates, velocities, control changes, and so on — not a
notation document. Turning that back into engraved notation is therefore
**grid-quantized and approximate**: durations are snapped to the nearest
notatable value, each track becomes one staff, and performance nuances that have
no clean notational equivalent are simplified.

The result is a faithful, readable lead-sheet-style score, not a publisher-grade
engraving. For exact timing, the MIDI output is authoritative; the score is a
human-readable view of it.

The VS Code extension uses this same engraving path for its live preview — see
[Editor support]({{< relref "/docs/editor-support" >}}).
