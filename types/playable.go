package types

import "github.com/poolpOrg/go-harmony/notes"

type Playable interface {
	GetName() string
	SetDuration(uint16)
	SetVelocity(uint8)
	GetDuration() uint16
	GetVelocity() uint8
	GetNotes() []notes.Note

	SetTick(uint32)
	GetTick() uint32
}
