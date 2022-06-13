package types

type Playable interface {
	GetName() string
	SetDuration(uint16)
	GetDuration() uint16
	GetNotes() []Note

	SetTick(uint32)
	GetTick() uint32
}
