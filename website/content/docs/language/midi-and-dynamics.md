---
title: MIDI and dynamics
weight: 4
---

# MIDI and dynamics

earmuff exposes the full MIDI set as first-class statements, alongside notes and
chords.

## Raw events

```text
program "violin";          // program change (by name or number)
cc 74 = 64;                // control change
bend +2;                   // pitch bend, in semitones
pressure 90;               // channel aftertouch
sysex F0 7E 7F 09 01 F7;   // raw system-exclusive bytes
```

These can sit in a step slot or be placed with `on beat`.

### Bend

`bend` is expressed in **semitones**. `bend +2` bends two semitones up; the
elaborator emits the RPN pitch-bend-range message once per track so the result
is correct regardless of the synth's default range, then the 14-bit value for
the requested offset. Escape hatches:

```text
bend raw 8192    // a direct 14-bit value
bend range 12    // set the range to ±12 semitones explicitly
```

## Per-event channel

A track binds one `channel`; events inherit it. A per-event `@channel` override
exists for full-MIDI cases:

```text
C@3   cc 74 = 100@3
```

## Velocity and dynamics

Velocity is one concept — a 0–127 value — settable in three places. The nearest
wins:

```text
per-note suffix  >  bar/block default  >  track default  >  built-in (64)
```

```text
track "lead" instrument "violin" v mp {     // track default = mp
    bar quarter v f { C D E:v ff F }         // bar default = f; E accented to ff
}
```

Write a bare number for an exact velocity (`v100`), or a named **dynamic** for a
musical one (`v mf`). Dynamics are named velocity values, usable anywhere a
velocity is:

| dynamic | velocity |   | dynamic | velocity |
|---------|----------|---|---------|----------|
| `ppp`   | 16       |   | `mf`    | 80       |
| `pp`    | 32       |   | `f`     | 96       |
| `p`     | 48       |   | `ff`    | 112      |
| `mp`    | 64       |   | `fff`   | 127      |

A bare number bypasses this table and sets the velocity exactly.

For the complete grammar and semantics, see the
[Language reference]({{< relref "/docs/language-reference" >}}).
