---
title: Notes and chords
weight: 2
---

# Notes and chords

Notes and chords are written by name, directly in musical position — the
`note`/`chord` keywords are optional.

## Notes

A note is a letter `A`–`G` with optional accidentals (`#` or `b`). On its own it
sounds at the default octave (4, middle C's octave):

```text
C  C#  Eb  Cb
```

To set the octave, add a caret `^` and the octave number. The octave **only**
ever appears after the caret:

```text
C^4   // middle C (the same as bare C)
C^5   // C an octave up
F#^3  Eb^2  G^7
```

`C^` with no number is the same as `C` — the default octave.

## Chords

A chord is a root followed by a quality. The quality can be a word (`maj`, `m`,
`dim`, `sus`, `add`, …) or a bare number (`5`, `7`, `9`, …):

```text
Am7  Gmaj7  C7  Dm7b5  C5  C7/E
```

Slash chords (`C7/E`) specify the bass note.

## Notes vs. chords — no ambiguity

The caret is what separates the two, so there is never any guessing:

```text
C      // NOTE  — C at the default octave
C^5    // NOTE  — C in octave 5
C5     // CHORD — C power chord (the 5 is a quality)
C7     // CHORD — C dominant 7th
Cmaj7  // CHORD
```

Rule of thumb: **a bare letter is a note; add a quality for a chord; add `^` for
a note's octave.** A digit straight after the letter (no caret) is always a
chord quality, so the note "C in octave 7" is written `C^7`, never `C7`.

## Interval transposition

Adding an interval to a note transposes it:

```text
C + fifth     // G
C + maj3      // E
C + octave    // C one octave up
```

Intervals include `min2`, `maj2`, `min3`, `maj3`, `fourth`, `fifth`, `min7`,
`maj7`, `octave`, and more. The operand types decide the operation: `note +
interval` transposes, while `+`/`-` on numbers is ordinary arithmetic. This
composes naturally with loops — see
[Patterns and control flow]({{< relref "/docs/language/patterns-and-control-flow" >}}):

```text
for root in [C2, F2, G2] {
    bar quarter { root (root + fifth) (root + octave) _ }
}
```

Simultaneous notes are grouped with parentheses: `(C, E, G)` sounds as a triad
in one slot.
