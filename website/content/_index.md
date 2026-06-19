---
title: earmuff
type: docs
---

# earmuff

> Write music as code — version it, diff it, review it, generate it — then play it or print the score.

**earmuff** is a small language for describing music in plain text. You write a
*project* of *tracks* and *bars*; earmuff parses and analyzes it, then turns it
into a Standard MIDI File, live playback through a synth, or engraved sheet
music. Because the source is just text, every developer tool you already use —
git, diff, code review, scripts that generate music — works on it for free.

```text
project "12 bars blues" {
    bpm 120; time 4 4;

    track "lead piano" instrument "piano" {
        pattern I  { bar quarter { C E G _ } }
        pattern IV { bar quarter { F A C _ } }
        pattern V  { bar quarter { G B D _ } }

        I() IV()
        repeat 2 { I() }
        repeat 2 { IV() }
        repeat 2 { I() }
        V() IV() I() V()
    }

    track "drums" instrument "synth drum" channel 10 {
        kit { hh = "closed hi-hat"; sn = "acoustic snare"; cy = "crash cymbal 1"; }
        pattern groove { bar quarter { (cy, sn) hh hh hh } }
        repeat 12 { groove() }
    }
}
```

Get started with the [Introduction]({{< relref "/docs/introduction" >}}).
