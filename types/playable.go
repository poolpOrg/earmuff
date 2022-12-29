package types

type Playable interface {
	GetName() string
	SetDuration(uint16)
	SetVelocity(uint8)
	GetDuration() uint16
	GetVelocity() uint8
	GetNotes() []Note

	SetTick(uint32)
	GetTick() uint32
}
