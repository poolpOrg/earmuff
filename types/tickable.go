package types

type Tickable interface {
	SetTick(uint32)
	GetTick() uint32
}
