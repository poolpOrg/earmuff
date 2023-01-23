# earmuff

WIP: this is a work in progress, incomplete and very buggys


## What is earmuff ?

earmuff is a language to program music as code and either interpret source as a MIDI stream or compile source as an SMF file.


## What can you do with earmuff ?

You can write music using your favorite IDE (vscode, vim, emacs, ...),
any of your usual development tools to match your workflow (git, diff, patch, ...),
and either playback the music by having it stream to a synthesizer or build a MIDI file to be imported in your other applications.


## How does it look like ?

You can have a look at the `examples/` directory that comes with the project for some samples,
but basicall you describe a project,
its tracks and bars within each tracks.

It supports repetitions,
recognizes note and chord names,
and is able to perform a few sanity checks on the structure of bars and tracks.


```
project "small project" {
    bpm 120;
    time 4 4;

    track "lead piano" {
        instrument "piano";

        bar {
            on beat 1 play quarter note C;
            on beat 2 play quarter note E;
            on beat 3 play quarter note G;
        }
        bar {
            on beat 1 play quarter note F;
            on beat 2 play quarter note A;
            on beat 3 play quarter note C;
        }
    }

    track "rythm guitar" {
        instrument "guitar";

        bar { on beat 1 play whole chord C7; }
        bar { on beat 1 play whole chord F7; }
    }

    track "bass" {
        instrument "bass";

        bar {
            on beat 1 play quarter note C2;
            on beat 3 play quarter note E2;
        }
        bar {
            on beat 1 play quarter note F2;
            on beat 3 play quarter note A2;
        }
    }

    track "drums" {
        instrument "steel drums";

        repeat 2 times bar {
            on beat 1 play quarter percussion "open hi-hat";
            on beat 1 play quarter percussion "acoustic snare";
            on beat 1 play quarter percussion "crash cymbal 1";
            on beat 2 play quarter percussion "closed hi-hat";
            on beat 3 play quarter percussion "closed hi-hat";
            on beat 4 play quarter percussion "closed hi-hat";
        }
    }
}
```

## Is it intended to replace scores ?

Nope,it is intended to be an intermediate format to read and produce MIDI.

Because MIDI can be used by a wide variety of software,
including software that manipulate scores...
they can either be converted to earmuff or the other way around.

This makes it easier to use development tools,
write code that generates earmuff dynamically,
then export as scores.