package types

type Duration struct {
	value    uint16
	playable Playable
}

func NewDuration(value uint16) *Duration {
	return &Duration{value: value}
}

func (duration *Duration) GetDuration() uint16 {
	return duration.value
}

func (duration *Duration) GetDurationName() string {
	switch duration.value {
	case 1:
		return "whole"
	case 2:
		return "half"
	case 4:
		return "quarter"
	case 8:
		return "8th"
	case 16:
		return "16th"
	case 32:
		return "32nd"
	case 64:
		return "64th"
	case 128:
		return "128th"
	case 256:
		return "256th"
	}
	return ""
}

func (duration *Duration) SetPlayable(playable Playable) {
	duration.playable = playable
}

func (duration *Duration) GetPlayable() Playable {
	return duration.playable
}
