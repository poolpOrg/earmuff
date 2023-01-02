package midi

import (
	"fmt"
	"strings"
)

// https://www.midi.org/specifications-old/item/gm-level-1-sound-set

func GetInstrumentFamilies() []string {
	return []string{
		"piano",
		"chromatic percussion",
		"organ",
		"guitar",
		"bass",
		"strings",
		"ensemble",
		"brass",
		"reed",
		"pipe",
		"synth lead",
		"synth pad",
		"synth effects",
		"ethnic",
		"percussive",
		"sound effects",
	}
}

func GetInstruments() []string {
	return []string{
		"acoustic grand piano",
		"bright acoustic piano",
		"electric grand piano",
		"honky-tonk piano",
		"electric piano 1",
		"electric piano 2",
		"hapsichord",
		"clavi",

		"celesta",
		"glockenspiel",
		"music box",
		"vibraphone",
		"marimba",
		"xylophone",
		"tubular bells",
		"dulcimer",

		"drawbar organ",
		"percussive organ",
		"rock organ",
		"church organ",
		"reed organ",
		"accordion",
		"harmonica",
		"tango accordion",

		"acoustic guitar (nylon)",
		"acoustic guitar (steel)",
		"electric guitar (jazz)",
		"electric guitar (clean)",
		"electric guitar (muted)",
		"overdriven guitar",
		"distortion guitar",
		"guitar harmonics",

		"acoustic bass",
		"electric bass (finger)",
		"electric bass (pick)",
		"fretless bass",
		"slap bass 1",
		"slap bass 2",
		"synth bass 1",
		"synth bass 2",

		"violin",
		"viola",
		"cello",
		"contrabass",
		"tremolo strings",
		"pizzicato strings",
		"orchestral harp",
		"timpani",

		"string ensemble 1",
		"string ensemble 2",
		"synthstrings 1",
		"synthstrings 2",
		"choir aahs",
		"voice oohs",
		"synth voice",
		"orchestra hit",

		"trumpet",
		"trombone",
		"tuba",
		"muted trumpet",
		"french horn",
		"brass section",
		"synthbrass 1",
		"synthbrass 2",

		"soprano sax",
		"alto sax",
		"tenor sax",
		"baritone sax",
		"oboe",
		"english horn",
		"bassoon",
		"clarinet",

		"piccolo",
		"flute",
		"recorder",
		"pan flute",
		"blown bottle",
		"shakuhachi",
		"whistle",
		"ocarina",

		"lead 1 (square)",
		"lead 2 (sawtooth)",
		"lead 3 (calliope)",
		"lead 4 (chiff)",
		"lead 5 (charang)",
		"lead 6 (voice)",
		"lead 7 (fifths)",
		"lead 8 (bass + lead)",

		"pad 1 (new age)",
		"pad 2 (warm)",
		"pad 3 (polysynth)",
		"pad 4 (choir)",
		"pad 5 (bowed)",
		"pad 6 (metallic)",
		"pad 7 (halo)",
		"pad 8 (sweep)",

		"FX 1 (rain)",
		"FX 2 (soundtrack)",
		"FX 3 (crystal)",
		"FX 4 (atmosphere)",
		"FX 5 (brightness)",
		"FX 6 (goblins)",
		"FX 7 (echoes)",
		"FX 8 (sci-fi)",

		"sitar",
		"banjo",
		"shamisen",
		"koto",
		"kalimba",
		"bag pipe",
		"fiddle",
		"shanai",

		"tinkle bell",
		"agogo",
		"steel drums",
		"woodblock",
		"taiko drum",
		"melodic tom",
		"synth drum",
		"reverse cymbal",

		"guitar fret noise",
		"breath noise",
		"seashore",
		"bird tweet",
		"telephone ring",
		"helicopter",
		"applause",
		"gunshot",
	}
}

func IntrumentToFamily(instrument string) (string, error) {
	pc, err := InstrumentToPC(instrument)
	if err != nil {
		return "", err
	}
	return GetInstrumentFamilies()[pc%8], nil
}

func InstrumentToPC(instrumentName string) (uint8, error) {
	for offset, instrument := range GetInstruments() {
		if strings.ToLower(instrumentName) == instrument {
			return uint8(offset + 1), nil
		}
	}

	for offset, instrumentFamily := range GetInstrumentFamilies() {
		if strings.ToLower(instrumentName) == instrumentFamily {
			return uint8(offset*8 + 1), nil
		}
	}

	return 0, fmt.Errorf("unknown instrument %s", instrumentName)

}
