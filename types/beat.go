package types

type Beat struct {
	durations []*Duration
}

func NewBeat() *Beat {
	return &Beat{
		durations: make([]*Duration, 0),
	}
}

func (beat *Beat) AddDuration(duration *Duration) {
	beat.durations = append(beat.durations, duration)
}

func (beat *Beat) GetDurations() []*Duration {
	return beat.durations
}
