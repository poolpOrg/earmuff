---
title: Patterns and control flow
weight: 3
---

# Patterns and control flow

Patterns and control flow are evaluated at **compile time**. There is no runtime
branching: `for`, `if`, `let`, and pattern calls all run during elaboration over
compile-time-known values, so the event stream is fully determined before any
MIDI is emitted. Loops are bounded and bindings are immutable, which guarantees
termination and keeps the music deterministic and diffable.

## Patterns

A `pattern` is a named, reusable block. It can take parameters and is invoked
with a call:

```text
pattern walk(root, third) { bar quarter { root _ third _ } }

walk(C2, E2)  walk(F2, A2)
```

Patterns defined at the project level are shared across all tracks; patterns
defined in a track are local to it. Scoping is lexical.

A pattern with no parameters can be called with a bare name — no parentheses:

```text
pattern fill { bar 8 { _ _ _ _ _ _ sn sn } }

fill          // bare call
fill()        // identical
```

## Sections

A `section` is a named block of arrangement — a verse, a head, a solo — that
you replay by name. It is sugar for a zero-parameter pattern, so it shares all
the same scoping and calling rules; it just reads like song structure:

```text
section head { bar quarter { C E G _ }  bar quarter { F A C _ } }
section solo { repeat 4 { improv() } }

head            // play the head
solo            // then the solo
repeat 2 { head }   // and the head out, twice
```

Because a section is just a pattern, a bare `head` is a zero-arg call and you
can repeat it (`repeat 2 { head }`) or loop it (`for each 1..2 { head }`) like
anything else.

## for

`for` comes in two forms. The **bound** form names a variable that takes each
value in turn:

```text
for i in 1..4 {
    if i == 4 { bar whole { C7 } } else { bar whole { C } }
}
```

The **unbound** form uses the keyword `each` to iterate without a variable —
the natural way to write a counted repeat:

```text
for each 1..12 { groove() }   // repeat 12 times, no binding
```

What you loop over is one of:

- a **range** — inclusive, integer endpoints: `1..4`;
- a **bare sequence** — space-separated values, no brackets or commas:
  `1 2 3`, `C E G`, `Am7 Dm7 G7`;
- a **list** — a bracketed literal or a `let` binding that holds one.

```text
for i in C E G { bar quarter { i _ _ _ } }   // bare sequence, bound

let changes = [Am7, Dm7, G7, Cmaj7];
for ch in changes { bar quarter { ch ch ch ch } }   // list, bound

for each Am7 Dm7 G7 { bar quarter { C } }   // bare sequence, unbound
```

## repeat

`repeat N { … }` is sugar for the most common loop — a counted repeat with no
index. It reads better than `for each 1..N` when all you mean is "do this N
times":

```text
repeat 12 { groove() }        // same as: for each 1..12 { groove() }
```

`N` is any expression that evaluates to a number, so a `let` works:

```text
let choruses = 3;
repeat choruses { head() }
```

A `for`/`if` adapts to its context: in a track or pattern body it yields bars
and pattern calls; inside a bar it yields steps (advancing the cursor). Nesting
the two reads cleanly.

## if / else

```text
if i == 2 {
    bar whole { C7/G }      // voice the fifth in the bass
} else {
    bar whole { C7/E }
}
```

Comparisons (`==`, `!=`, `<`, `<=`, `>`, `>=`) work on same-typed values, and
boolean operators `&&`, `||`, `!` combine them.

## let bindings

`let` introduces an immutable binding, visible in its enclosing block and nested
blocks; inner bindings shadow outer ones:

```text
let changes = [Am6, Dm6, E7, Am6];
```

Lists are first-class values: a `let` may bind one, a `pattern` may take one as
a parameter and iterate it, and list literals may nest. Elements within a list
are of one uniform type (all notes, all chords, all numbers, …).

For the complete grammar, see the
[Language reference]({{< relref "/docs/language-reference" >}}).
