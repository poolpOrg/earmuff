package lsp

import (
	"github.com/poolpOrg/go-harmony/chords"
	"github.com/poolpOrg/go-harmony/notes"
)

// notesParse returns the MIDI key for a note literal, or an error if it is not
// a valid note spelling.
func notesParse(s string) (uint8, error) {
	n, err := notes.Parse(s)
	if err != nil {
		return 0, err
	}
	return n.MIDI(), nil
}

// chordParse returns the MIDI keys and quality name for a chord literal.
func chordParse(s string) ([]uint8, string, error) {
	c, err := chords.Parse(s)
	if err != nil {
		return nil, "", err
	}
	var keys []uint8
	for _, n := range c.Notes() {
		keys = append(keys, n.MIDI())
	}
	return keys, c.Name(), nil
}
