---
title: Bars and the step grid
weight: 1
---

# Bars and the step grid

A `bar` lays events on a grid. The grid's step is the bar's duration, and the
cursor advances exactly **one step per token**.

```text
bar quarter { C E G _ }    // four quarter-note slots
bar 8 { C C C C C C C C }  // an eighth-note grid: eight slots
```

A duration can be a word (`whole`, `half`, `quarter`) or a number (`1`, `2`,
`4`, `8`, `16`, `32`, `64`, `128`), with the suffix `.` for dotted (×1.5) and
`t` for triplet (×2/3).

## Advance vs. gate

Two ideas are kept separate:

- **Advance** — how far the cursor moves to the next token. This is *always*
  one grid step. Nothing about a note's duration changes the advance.
- **Gate** — how long a note actually *sounds*. By default the gate is one grid
  step (so a bare `C` on a quarter grid is a quarter note).

A `:dur` suffix sets the gate to an absolute note value, independent of the
advance. This is what enables legato and staccato:

```text
bar quarter { C:2 _ E _ }   // C rings a half note while the cursor advances a quarter
bar quarter { C:16 _ _ _ }  // staccato: C sounds a 16th, then silence
```

## Rests and ties

- `_` is a **rest**: one empty slot.
- `~` is a **tie**: it extends the *previous* note's gate by one grid step and
  advances the cursor one step.

A held note has two equivalent spellings — pick by readability, never combine
them:

```text
F:2                  // gate = an absolute half note
F ~ ~ ~ ~ ~ ~ ~      // gate = 8 sixteenths = a half note (on a 16th grid)
```

## Region grid switch (`N:`)

Inside a bar, `N:` rebinds the grid step for the tokens that follow, until the
next switch, a `|`, or the end of the bar. The cursor stays in ticks, so mixing
grids never drifts:

```text
bar quarter { C  16: D E F G  | A }   // C and A are quarters; D E F G are 16ths
```

`|` is an optional visual separator (and a region-switch terminator); it does
not advance the cursor on its own.

## Step repeat (`*k`)

A trailing `*k` on any step-token repeats it `k` times — handy for runs of
rests, holds, or ostinati:

```text
_*8          // eight rests
~*7          // hold the previous note seven more steps
(C,E,G)*4    // the triad four times
```

## Bar fill

The advances should sum to exactly one bar. Less, and the rest of the bar is
silent; more, and the bar **overflows**, which the elaborator reports as an
error (this catches miscounted grids).

## `on beat` escape hatch

When the grid does not fit, `on beat <expr>` places an event at an absolute
beat regardless of the cursor, and does not move the cursor. You can mix it
freely with step tokens in one bar:

```text
bar { on beat 1 Gmaj7:2  on beat 3 Am7:4  on beat 4 Bm7b5:4 }
```

For the formal timing rules, see the
[Language reference]({{< relref "/docs/language-reference" >}}).
