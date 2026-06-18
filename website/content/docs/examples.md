---
title: Examples
weight: 50
---

# Examples

The [`examples/`](https://github.com/poolpOrg/earmuff/tree/main/examples)
directory holds complete, compilable pieces. Each one exercises a different
corner of the language:

| File | Shows |
| --- | --- |
| `12bars-blues.ear` | patterns, loops, drums |
| `minor-swing.ear` | nested loops, comping |
| `nuages.ear` | syncopation on a fine grid |
| `lofi.ear` | lists, dynamics, off-beats |
| `small-jazz.ear` | a small swing arrangement |
| `bach-prelude.ear` | a classical fragment |
| `mozart-nachtmusik.ear` | a classical fragment |

Compile or play any of them as described in
[Command line]({{< relref "/docs/command-line" >}}):

```sh
earmuff examples/minor-swing.ear
earmuff -pdf nuages.pdf examples/nuages.ear
```

A taste — the lofi keys part comps off-beat stabs over a list of changes,
accenting the second hit of each pair:

```text
track "keys" instrument "electric piano 1" v mp {
    let changes = [Cmaj7, Em7, Am7, Dm7, Fmaj7, Em7, Am7, G7];

    for ch in changes {
        bar 8 v p { _ ch _ ch:v mf _ ch _ ch }
    }
}
```

Browse the
[`examples/`](https://github.com/poolpOrg/earmuff/tree/main/examples) directory
for the rest.
