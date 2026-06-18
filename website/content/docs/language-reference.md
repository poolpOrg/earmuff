---
title: Language reference
weight: 90
---

# earmuff language reference

This is the complete reference for the earmuff language: its surface syntax, the
execution model behind it, and the step-grid timing rules. Everything described
here is implemented.

## Goals (from design discussion)

1. Express the **entire MIDI set** (notes, CC, pitch bend, aftertouch, program
   change, sysex, per-event channel/port).
2. Stay **developer-friendly** and **less verbose** than v1.
3. Add **functions, patterns, loops**.
4. Be able to **interpret and emit MIDI live** as the source is processed, not
   only compile to an SMF file.

## Design decisions

- **Step-grid is the default note-entry surface.** A block sets a default step
  duration; events auto-advance one step. `on beat` remains as an explicit
  escape hatch for absolute placement.
- **Patterns and functions are pure.** Source elaborates deterministically into a
  flat, time-ordered event stream. User code never touches the clock or the MIDI
  port — only the scheduler does. Live playback and SMF export consume the same
  elaborated stream.
- **Full MIDI now.** Raw primitives (`cc`, `bend`, `pressure`, `program`,
  `sysex`, per-event `@channel`) are first-class, with named sugar over raw
  numbers.
- **Structured control flow, evaluated at elaboration time and pure.** Real
  `if`/`else`, `for x in <range|list>`, boolean operators, and immutable `let`
  bindings. They run during elaboration over compile-time-known values — they are
  **not** live/runtime branches — so the event stream is fully determined before
  any MIDI is emitted. Loops are bounded (range or list, no `while`), bindings
  are immutable: this guarantees termination and keeps music diffable and
  deterministic.
- **Domain language, not general-purpose.** Values are musical: number, boolean,
  note, chord, duration, interval, list, and pattern. No mutable state, no
  strings-as-data arithmetic, no I/O from user code, no recursion/`while` — no
  Turing-completeness creep.

---

## 1. Execution model

Two phases, one stream:

```
source ──parse──▶ AST ──elaborate──▶ Event Stream ──┬─▶ SMF writer  (offline)
                          (pure)     (abs. ticks)    └─▶ Scheduler   (live MIDI)
```

- **Elaborate**: expand patterns, loops, and function calls into a flat list of
  events, each stamped with an **absolute rational tick** (numerator/denominator
  of a whole note × PPQ), a channel, and a port. Pure and deterministic.
- **Schedule**: walk the stream in tick order against a clock. Offline → SMF
  bytes. Live → emit to a MIDI port as wall-clock catches each tick.

A single source of truth for PPQ (e.g. 960). Every event carries an absolute
tick from elaboration onward; the parser never computes final ticks (this fixes
the v1 parser/compiler tick-model split).

---

## 2. Lexical changes

- Note/chord literals are recognized in musical position; the `note`/`chord`
  keywords become optional.
- New punctuation: `|` (bar/step group separator, optional sugar), `:`
  (duration suffix), `_` (rest), `~` (tie/hold), `@` (channel/raw qualifier),
  `..` (range), `=` (assignment in cc/bend), `,` (chord-tone / arg separator),
  `(` `)` (grouping & calls).
- Comments stay `//` and `/* */`.
- Strings stay `"..."`.

### Durations

| token        | meaning                  |
|--------------|--------------------------|
| `1` / `whole`| whole note               |
| `2` / `half` | half                     |
| `4` / `quarter` | quarter               |
| `8`,`16`,`32`,`64`,`128` | nth notes    |
| suffix `.`   | dotted (×1.5)            |
| suffix `t`   | triplet (×2/3)           |

Duration appears as a **block default** (`bar quarter { … }`) or a **per-event
suffix** (`C:8`, `G:2.`, `D:8t`).

---

## 3. Grammar (EBNF-ish)

```ebnf
program      = { statement } ;
statement    = project | pattern_def | track | tempo | timesig | meta ;

project      = "project" string "{" { proj_item } "}" ;
proj_item    = tempo | timesig | copyright | text | track | pattern_def ;

tempo        = "bpm" number ";" ;
timesig      = "time" number number ";" ;
copyright    = "copyright" string ";" ;
text         = "text" string ";" ;

track        = "track" string [ "instrument" (string|number) ]
                            [ "channel" number ] [ "port" (number|string) ]
                            [ velocity ]
               "{" { track_item } "}" ;
track_item   = bar | flow | let | kit | pattern_call | event_stmt
             | tempo | timesig | meta ;

(* per-track aliases for long percussion / note names (pure name bindings) *)
kit          = "kit" "{" { ident "=" (string|note) ";" } "}" ;

pattern_def  = "pattern" ident "(" [ params ] ")" "{" { track_item } "}" ;
params       = ident { "," ident } ;
pattern_call = ident "(" [ args ] ")" ;
args         = expr { "," expr } ;

(* --- structured control flow: pure, elaboration-time, bounded --- *)
flow         = for | if ;
for          = "for" ident "in" iterable block ;
iterable     = range | list | expr ;          (* expr must evaluate to a list *)
range        = expr ".." expr ;               (* inclusive, integer endpoints *)
list         = "[" [ expr { "," expr } ] "]" ;
if           = "if" expr block [ "else" ( if | block ) ] ;
let          = "let" ident "=" expr ";" ;      (* immutable binding *)
block        = "{" { track_item } "}" ;

bar          = "bar" [ duration ] [ velocity ] "{" { bar_item } "}" ;
bar_item     = step | grid_switch | absolute | event_stmt | bar_flow | "|" ;
bar_flow     = for | if ;                       (* same flow, scoped to a bar *)

(* region grid switch: rebinds the step duration for following tokens
   until the next switch, a "|", or end of bar (see §3a) *)
grid_switch  = duration ":" ;                   (* e.g.  16:  *)

(* step-grid: cursor advances ONE grid step per step-token (see §3a).
   ":" duration sets the GATE (sounding length), not the advance.
   trailing "*" number repeats the step k times. *)
step         = step_atom [ "*" number ] ;
step_atom    = playable [ ":" duration ] [ velocity ] ;
playable     = note | chord | percussion | "_" | "~" | group ;
group        = "(" playable { "," playable } ")" ;   (* simultaneous *)

(* escape hatch: explicit placement *)
absolute     = "on" "beat" expr event_stmt ;

(* raw MIDI + meta, placeable in a step slot or via 'on beat' *)
event_stmt   = note_evt | cc | bend | pressure | program | sysex | meta ;
note_evt     = playable [ "@" channel ] ;
cc           = "cc" (number|cc_name) "=" expr ;
bend         = "bend" ( signed_expr | "raw" expr | "range" expr ) ;
                                                (* semitones (auto-RPN), or raw
                                                   14-bit, or set the range *)
pressure     = "pressure" expr ;                (* channel aftertouch *)
program      = "program" (string|number) ;
sysex        = "sysex" { hexbyte } ;
meta         = "text" string | "lyric" string | "marker" string | "cue" string ;

note         = NOTE_LITERAL ;                  (* C, C#, Eb, C5, F#3 ... *)
chord        = CHORD_LITERAL ;                 (* Am7, C7, Gmaj7, C7/E ... *)

(* velocity: a value usable as a per-note suffix, a bar/block default, or a
   track default. precedence per-note > block > track > built-in 64 (see §3b) *)
velocity     = "v" ( number | dynamic ) ;      (* v100  or  v mf *)
dynamic      = "ppp"|"pp"|"p"|"mp"|"mf"|"f"|"ff"|"fff" ;

(* --- expression language (musical values; precedence high→low) --- *)
expr         = or_expr ;
or_expr      = and_expr { "||" and_expr } ;
and_expr     = cmp_expr { "&&" cmp_expr } ;
cmp_expr     = add_expr [ ( "==" | "!=" | "<" | "<=" | ">" | ">=" ) add_expr ] ;
add_expr     = mul_expr { ( "+" | "-" ) mul_expr } ;   (* note+interval, ints *)
mul_expr     = unary    { ( "*" | "/" ) unary } ;
unary        = [ "!" | "-" ] primary ;
primary      = number | bool | note | chord | interval | dynamic | list
             | ident | pattern_call | "(" expr ")" ;
signed_expr  = [ "+" | "-" ] expr ;            (* explicit sign, e.g. bend +2 *)
bool         = "true" | "false" ;
interval     = "min2"|"maj2"|"min3"|"maj3"|"fourth"|"fifth"
             |"min7"|"maj7"|"octave"| ... ;
```

Notes on the expression language:
- `note + interval` yields a transposed note (`C + maj3` → `E`); `+`/`-` on
  numbers are ordinary arithmetic. The operand types decide the operation.
- `==`/`!=` compare any same-typed values (numbers, notes, chords, bools).
- **Lists are first-class values**: a `let` may bind a list, a `pattern` may take
  a list parameter and iterate it, and list literals may nest. The element type
  is uniform within a list (all notes, all chords, all numbers, …).
- Everything is evaluated during elaboration; an expression that can't be
  reduced to a value then (e.g. references an undefined binding) is an error,
  not a runtime decision.

### Pattern operators (composition)

```
a then b      sequence a, then b
a over b      layer a and b in parallel (same start)
a * 4         repeat a four times
```

## 3a. Step-grid semantics (the timing model)

This is the heart of v2 and the resolution of the porting blockers. All four
rules below are validated against the examples to tick precision.

**Position vs. gate are independent.** A bar maintains a *cursor* in absolute
ticks. Two separate concepts:

- **Advance** — how far the cursor moves to the next token. This is **always one
  grid step**, period. A step-token (note, chord, rest, tie, group) occupies
  exactly one slot. Nothing about a note's duration changes the advance.
- **Gate** — how long a note *sounds*. Default gate = one grid step (so a bare
  `C` on a quarter grid is a quarter note — the intuitive case). A `:dur` suffix
  sets the gate to an **absolute** note value, independent of the advance. This
  is what enables overlap/legato (`C:2` on a 16th grid rings 8 slots while the
  next token is one slot later) and staccato (`C:16` on a quarter grid).

**Ties.** `~` extends the **previous** note's gate by one grid step and advances
the cursor one step. So a held note has two equivalent spellings — pick by
readability, never combine them:

```
F:2                 // gate = absolute half note
F ~ ~ ~ ~ ~ ~ ~     // gate = 8 sixteenths = a half note (on a 16th grid)
```

**Region grid switch (`N:`).** Inside a bar, `N:` rebinds the grid step for the
tokens that follow, until the next switch, a `|`, or end of bar. The cursor stays
in ticks, so mixing grids never drifts. This makes mixed-subdivision bars
writable without dropping the whole bar to the finest grid:

```
bar quarter { C  16: D E F G  | A }   // C & A are quarters; D E F G are 16ths
```

**Step repeat (`*k`).** A trailing `*k` on any step-token repeats it k times —
sugar for rest/hold/ostinato runs:

```
_*8                 // eight rests
~*7                 // hold previous note seven more steps
(C,E,G)*4           // the triad four times
```

**Bar fill.** The sum of advances should equal exactly one bar
(`beats × whole/denominator`). Less → the rest of the bar is silent; more → the
bar **overflows**, which the elaborator reports as an error (catches miscounted
grids). `|` is an optional visual separator and a region-switch terminator; it
does not itself advance the cursor.

**`on beat` escape hatch** places an event at an absolute beat regardless of the
cursor, and does not move the cursor. Mix freely with step tokens in one bar.

## 3b. Velocity, dynamics, and bend semantics

**Velocity is one concept (a 0–127 value) settable in three places**, resolved
by precedence — nearest wins:

```
per-note suffix  >  bar/block default  >  track default  >  built-in (64)
```

```
track "lead" instrument "violin" v mp {     // track default = mp
    bar quarter v f { C D E:v ff F }        // bar default = f; E accented to ff
}
```

**Dynamics keywords are named velocity values**, usable anywhere a velocity is
(`v mf`, `bar quarter v p { … }`, track default). Proposed mapping (tunable):

| dyn | vel |   | dyn | vel |
|-----|-----|---|-----|-----|
| ppp | 16  |   | mf  | 80  |
| pp  | 32  |   | f   | 96  |
| p   | 48  |   | ff  | 112 |
| mp  | 64  |   | fff | 127 |

A bare number (`v100`) is an exact velocity and bypasses the table.

**Bend is in semitones with automatic range setup.** `bend +2` means two
semitones up; the elaborator emits the RPN pitch-bend-range message (RPN 0) once
per track so the result is correct regardless of the synth's default range, then
the 14-bit value for the requested semitone offset. Escapes: `bend raw 8192`
(direct 14-bit) and `bend range 12` (set range to ±12 explicitly).

---

## 4. Examples rewritten

### 4a. 12-bar blues (lead) — step grid

v1 (excerpt):
```
bar {
    on beat 1 play quarter note C;
    on beat 2 play quarter note E;
    on beat 3 play quarter note G;
}
```

v2:
```
project "12 bars blues" {
    bpm 120; time 4 4;

    track "lead piano" instrument "piano" {
        pattern I  { bar quarter { C E G _ } }
        pattern IV { bar quarter { F A C _ } }
        pattern V  { bar quarter { G B D _ } }

        I  IV
        for _ in 1..2 { I }
        for _ in 1..2 { IV }
        for _ in 1..2 { I }
        V  IV  I  V
    }

    track "rythm guitar" instrument "guitar" {
        bar whole { C7 } bar whole { F7 }
        for _ in 1..2 { bar whole { C7 } }
        for _ in 1..2 { bar whole { F7 } }
        for _ in 1..2 { bar whole { C7 } }
        bar whole { G7 } bar whole { F7 } bar whole { C7 } bar whole { G7 }
    }

    track "bass" instrument "bass" {
        pattern walk(root, third) { bar quarter { root _ third _ } }
        walk(C2, E2)  walk(F2, A2)
        for _ in 1..2 { walk(C2, E2) }
        for _ in 1..2 { walk(F2, G2) }
        for _ in 1..2 { walk(C2, E2) }
        walk(G2, B2)  walk(F2, G2)  walk(C2, E2)  walk(G2, B2)
    }

    track "drums" instrument "drum kit" channel 10 {
        kit {
            hh  = "closed hi-hat";
            oh  = "open hi-hat";
            sn  = "acoustic snare";
            cy  = "crash cymbal 1";
        }
        pattern beat {
            bar 8 { (oh,sn,cy) hh hh hh _ _ _ _ }
        }
        for _ in 1..12 { beat() }
    }
}
```

`for _ in 1..N` is the counted-repeat idiom (`_` discards the index). Bind the
index when you need it (see 4c).

### 4b. nuages — the full port (the case that drove §3a)

This is the file that exposed the gate-vs-advance blocker. With the §3a rules it
ports cleanly and was verified tick-for-tick against the original.

v1 lead, bar 1:
```
bar {
    on beat 3    play 8th note C#;
    on beat 3.25 play 8th note D;
    on beat 3.5  play 8th note A;
    on beat 3.75 play 8th note G#;
    on beat 4    play 8th note G;
    on beat 4.5  play 8th note F#;
}
```

v2 — full lead guitar track:
```
track "lead guitar" instrument "guitar" {
    // bar 1: 8th notes on a 16th grid (so 3.25/3.75 are real slots).
    // :8 sets each note's gate; the cursor still advances one 16th per token.
    bar 16 {
        _*8                       // beats 1-2 silent
        C#:8 D:8 A:8 G#:8         // beats 3, 3.25, 3.5, 3.75
        G:8  _   F#:8 _           // beat 4, 4.5
    }
    // bar 2: half note on 1, late 8th on 4.5 (slot 14 of 16); 13 rests between
    bar 16 { F:2  _*13  E:8 _ }
    // bar 3: half(1) + quarter(2.5) + 8th(4.5) all land on the 8th grid
    bar 8  { F:2  _ _ Eb:4  _ _ _ D:8 }
    // bar 4: whole note
    bar 1  { D }
}
```

v2 — piano track (even durations are the grid's sweet spot; last bar mixes in
`on beat` where it reads better):
```
track "rythm piano" instrument "piano" {
    bar {}                        // one bar of rest
    bar 1 { Eb9 }                 // whole chord
    bar 2 { Am7b5 D7b9 }          // two half chords
    bar { on beat 1 Gmaj7:2  on beat 3 Am7:4  on beat 4 Bm7b5:4 }
}
```

### 4c. loop index + if/else (from exp.ear)

The `exp.ear` scratch used postfix `if loop == 2` guards; with structured flow
this becomes a real `if/else` over the bound loop index:

```
track "rythm guitar" instrument "guitar" {
    for i in 1..2 {
        if i == 2 {
            bar whole { C7/G }      // last pass: voice the fifth in the bass
        } else {
            bar whole { C7/E }
        }
    }
}
```

### 4d. for over a list — iterate a chord progression

The reason the loop primitive is `for x in <list>` and not just a counted
repeat: progressions become data you can walk.

```
track "comp" instrument "piano" {
    let changes = [Am6, Dm6, E7, Am6];   // ii-V-i-ish

    for ch in changes {
        bar quarter { ch ch ch ch }      // 4-to-the-bar comping
    }
}
```

Combine with transposition for sequences:

```
track "riff" instrument "bass" {
    // play the same shape rooted on each scale degree
    for root in [C2, F2, G2] {
        bar quarter { root (root + fifth) (root + octave) _ }
    }
}
```

### 4e. Full-MIDI primitives

```
track "synth lead" instrument "lead 1 (square)" channel 3 {
    program "lead 2 (sawtooth)";     // mid-track patch change
    bar quarter {
        C   cc cutoff = 64   E   cc cutoff = 100
    }
    on beat 1 bend +2;               // pitch bend up 2 semitones (raw escape: bend raw 8192)
    on beat 2 pressure 90;
    sysex F0 7E 7F 09 01 F7;         // GM reset
}
```

### 4f. First-class lists + dynamics — a reusable comping pattern

Lists as values let a pattern take a progression as a parameter; dynamics make
the feel concise:

```
project "comp demo" {
    bpm 120; time 4 4;

    // a pattern that comps any progression, soft, four-to-the-bar
    pattern comp(changes) {
        for ch in changes {
            bar quarter v mp { ch ch ch ch:v mf }   // last hit slightly accented
        }
    }

    track "piano" instrument "piano" v p {          // track sits at p by default
        let aTune = [Am6, Dm6, E7, Am6];
        comp(aTune)
        comp([Dm6, Am6, E7, Am6])                   // list literal inline
    }
}
```

---

## 5. Migration / compatibility

- v1 syntax (`on beat N play <dur> note/chord X;`) can remain valid as a strict
  subset, so existing `.ear` files keep working while the step-grid surface is
  added on top. A `--strict-v1` flag (or a per-file pragma) can gate the old
  grammar if we later want to retire it.
- The four shipped examples can be ported incrementally; 4a–4f above show target
  forms.

## 6. Resolved design decisions

All of the original open questions are now decided:

- **Bend** — semitones with **automatic RPN range setup**; `bend raw N` (14-bit)
  and `bend range N` escapes. (§3b)
- **Velocity** — one 0–127 value settable per-note / per-block / per-track, plus
  named **dynamics** (`ppp`…`fff`); precedence per-note > block > track > 64. (§3b)
- **Lists** — **first-class values**: bindable with `let`, passable to patterns,
  iterable with `for`; uniform element type. (§3, §4f)
- **Channel vs. port** — a track binds one `channel` and one `port`; events
  inherit them. Per-event `@channel` override exists for the full-MIDI cases; no
  per-event port override for now (rare; can be added without grammar churn).
- **Scoping** — **lexical**. Patterns/`let` bindings are visible in their
  enclosing block and nested blocks; project-level patterns are shared across all
  tracks; inner bindings shadow outer. `let` is immutable.
- **`for` altitude** — a `for`/`if` adapts to context: in a track/pattern body it
  yields bars and pattern calls; inside a bar it yields steps (advancing the
  cursor). Nesting (track-level `for` of bars containing a bar-level `for` of
  steps) is allowed and reads cleanly.
- **Tie across a group** — `~` extends the gate of the **whole** preceding
  simultaneous group `(a,b,c)` by one step (every voice is held together). To
  tie only one voice, place that voice as its own step.

## 7. Remaining work before implementation

Design is complete enough to build; these are implementation-time details, not
open language questions:

- Exact dynamics→velocity numbers (table in §3b is a tunable starting point).
- Lexer/parser rewrite to the v2 grammar; reuse v1 as a recognized subset.
- The elaboration pass (AST → absolute-tick event stream) — the simulator in the
  design notes is the reference semantics for the step grid.
- The live scheduler (the second consumer of the event stream).

## 8. Port findings (stress-testing the syntax against the examples)

Porting all four example files to v2 surfaced the issues below. The step grid
handles the *common* case beautifully (minor-swing's 96 lines collapse to one
`pattern comp(ch)` + a few `for`s; blues comping is one line per bar) and nuages
exposed real gaps. **All blockers are now resolved** — see §3a for the formal
rules; the resolutions were validated against every nuages bar to tick precision.

### BLOCKER 1 (RESOLVED → §3a) — gate vs. cursor advance

`:dur` was overloaded: it tried to mean both "how long the note sounds" and
"where the next note goes." **Resolution:** split them. The cursor *always*
advances one grid step (S1); `:dur` sets only the **gate** (sounding length).
`~` extends the previous gate by one step. `F:2` and `F ~*7` are equivalent and
never combined. This is the central §3a rule.

### BLOCKER 2 (RESOLVED → §3a) — mixed subdivisions in one bar

nuages bar 1 mixes 8th and 16th offsets. **Resolution:** the in-bar region
switch `N:` rebinds the grid step for the following tokens (until the next
switch, `|`, or end of bar). Verified to introduce no timing drift across grid
changes within a bar.

### PAPER-CUT 1 (RESOLVED → §3a) — rest/hold-run verbosity

**Resolution:** step-level repeat `*k` — `_*8`, `~*7`, `(C,E,G)*4`.

### PAPER-CUT 2 (RESOLVED → §3a) — long percussion names

**Resolution:** per-track `kit { hh = "closed hi-hat"; ... }` alias block; then
`bar 8 { (oh,sn,cy) hh hh hh _*4 }`. Aliases are pure name bindings.

### CONFIRMED OK (no blocker)

- Empty bar `bar {}` = one bar of rest — clean.
- Whole note/chord per bar (`bar 1 { D }`, `bar 1 { Eb9 }`) — clean.
- Multiple even durations per bar (`bar 2 { Am7b5 D7b9 }`) — the grid's sweet spot.
- Nested counted repeats (minor-swing) via nested `for _ in 1..N` + a `pattern`.
- Mixing step-grid and `on beat` in one bar (nuages piano bar 4) — works as the
  designed escape hatch.
- `for`-over-list for chord progressions — natural and a clear win.
