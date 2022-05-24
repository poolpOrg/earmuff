package types

type Duration struct {
	duration uint8
}

func NewDuration(duration uint8) *Duration {
	return &Duration{duration: duration}
}
