package types

type Pitch struct {
	duration uint16
	value    uint8
	tick     uint32
	velocity uint8
}

func NewPitch(value uint8) *Pitch {
	return &Pitch{
		value:    value,
		velocity: 64,
	}
}

func (pitch *Pitch) SetDuration(duration uint16) {
	pitch.duration = duration
}

func (pitch *Pitch) GetDuration() uint16 {
	return pitch.duration
}

func (pitch *Pitch) SetVelocity(velocity uint8) {
	pitch.velocity = velocity
}

func (pitch *Pitch) GetVelocity() uint8 {
	return pitch.velocity
}

func (pitch *Pitch) GetValue() uint8 {
	return pitch.value
}

func (pitch *Pitch) SetTick(tick uint32) {
	pitch.tick = tick
}

func (pitch *Pitch) GetTick() uint32 {
	return pitch.tick
}
