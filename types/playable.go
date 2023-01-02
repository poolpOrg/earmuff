package types

type Playable interface {
	SetDuration(uint16)
	SetVelocity(uint8)
	GetDuration() uint16
	GetVelocity() uint8
	GetPitches() []uint8

	SetTick(uint32)
	GetTick() uint32
}
