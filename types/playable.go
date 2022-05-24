package types

type Playable interface {
	Type() string
	Name() string
}

type Chord struct {
	name string
}

func NewChord(name string) *Chord {
	return &Chord{
		name: name,
	}
}

func (chord *Chord) Type() string {
	return "Chord"
}

func (chord *Chord) Name() string {
	return chord.name
}

type Note struct {
	name string
}

func NewNote(name string) *Note {
	return &Note{
		name: name,
	}
}

func (note *Note) Type() string {
	return "Note"
}

func (note *Note) Name() string {
	return note.name
}

type Rest struct {
	name string
}

func NewRest() *Rest {
	return &Rest{}
}

func (rest *Rest) Type() string {
	return "Rest"
}

func (rest *Rest) Name() string {
	return ""
}
