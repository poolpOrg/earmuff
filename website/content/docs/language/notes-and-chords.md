---
title: Notes and chords
weight: 2
---

# Notes and chords

Notes and chords are written by name, directly in musical position — the
`note`/`chord` keywords are optional.

## Notes

A note is a letter `A`–`G`, an optional accidental (`#` or `b`), and an optional
octave number:

```text
C  C#  Eb  F#3  Cb
```

Without an octave, a default register is used; an explicit number (`C5`, `F#3`,
`E2`) pins the octave.

## Chords

Chords are written by name:

```text
Am7  Gmaj7  C7  Dm7b5  C7/E
```

Slash chords (`C7/E`) specify the bass note.

## The note-vs-chord rule

A token like `C7` is ambiguous: it could be the chord *C dominant 7th* or the
note *C* in octave 7. **The chord reading wins.** If you mean the note, append a
trailing `^` to force the note interpretation:

```text
C7     // the chord C7
C7^    // the note C in octave 7
```

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
