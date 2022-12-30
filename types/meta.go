package types

type Text struct {
	value string
	tick  uint32
}

func NewText(value string) *Text {
	return &Text{
		value: value,
	}
}

func (text *Text) GetValue() string {
	return text.value
}

func (text *Text) SetTick(tick uint32) {
	text.tick = tick
}

func (text *Text) GetTick() uint32 {
	return text.tick
}

type Lyric struct {
	value string
	tick  uint32
}

func NewLyric(value string) *Lyric {
	return &Lyric{
		value: value,
	}
}

func (lyric *Lyric) GetValue() string {
	return lyric.value
}

func (lyric *Lyric) SetTick(tick uint32) {
	lyric.tick = tick
}

func (lyric *Lyric) GetTick() uint32 {
	return lyric.tick
}

type Marker struct {
	value string
	tick  uint32
}

func NewMarker(value string) *Marker {
	return &Marker{
		value: value,
	}
}

func (marker *Marker) GetValue() string {
	return marker.value
}

func (marker *Marker) SetTick(tick uint32) {
	marker.tick = tick
}

func (marker *Marker) GetTick() uint32 {
	return marker.tick
}

type Cue struct {
	value string
	tick  uint32
}

func NewCue(value string) *Cue {
	return &Cue{
		value: value,
	}
}

func (cue *Cue) GetValue() string {
	return cue.value
}

func (cue *Cue) SetTick(tick uint32) {
	cue.tick = tick
}

func (cue *Cue) GetTick() uint32 {
	return cue.tick
}
