package parser

import (
	"fmt"
	"io"
	"math"
	"strconv"

	"github.com/poolpOrg/earmuff/lexer"
	"github.com/poolpOrg/earmuff/midi"
	"github.com/poolpOrg/earmuff/types"
	"github.com/poolpOrg/go-harmony/chords"
	"github.com/poolpOrg/go-harmony/notes"
	"gitlab.com/gomidi/midi/v2/smf"
)

type Parser struct {
	s   *lexer.Scanner
	buf struct {
		tok lexer.Token
		lit string
		n   int
	}
	activePitches map[uint8]*types.Pitch
	barNo         uint32
}

func NewParser(r io.Reader) *Parser {
	return &Parser{s: lexer.NewScanner(r), activePitches: make(map[uint8]*types.Pitch), barNo: 0}
}

// scan returns the next token from the underlying scanner.
// If a token has been unscanned then read that instead.
func (p *Parser) scan() (tok lexer.Token, lit string) {
	// If we have a token on the buffer, then return it.
	if p.buf.n != 0 {
		p.buf.n = 0
		return p.buf.tok, p.buf.lit
	}

	// Otherwise read the next token from the scanner.
	tok, lit = p.s.Scan()

	// Save it to the buffer in case we unscan later.
	p.buf.tok, p.buf.lit = tok, lit

	return
}

// unscan pushes the previously read token back onto the buffer.
func (p *Parser) unscan() { p.buf.n = 1 }

// scanIgnoreWhitespace scans the next non-whitespace token.
func (p *Parser) scanIgnoreWhitespace() (tok lexer.Token, lit string) {
	for {
		tok, lit = p.scan()
		if tok == lexer.WHITESPACE || tok == lexer.COMMENT {
			continue
		}
		return tok, lit
	}
}

func (p *Parser) parseBpm() (float64, error) {
	if tok, lit := p.scanIgnoreWhitespace(); tok != lexer.NUMBER && tok != lexer.FLOAT {
		return 0, fmt.Errorf("found %q, expected number", lit)
	}

	beats, err := strconv.ParseFloat(p.buf.lit, 64)
	if err != nil {
		return 0, err
	}

	if tok, lit := p.scanIgnoreWhitespace(); tok != lexer.SEMICOLON {
		return 0, fmt.Errorf("found %q, expected ;", lit)
	}

	return beats, nil
}

func (p *Parser) parseTimeSignature() (*types.Signature, error) {
	if tok, lit := p.scanIgnoreWhitespace(); tok != lexer.NUMBER {
		return nil, fmt.Errorf("found %q, expected number", lit)
	}
	beats, err := strconv.ParseUint(p.buf.lit, 10, 8)
	if err != nil {
		return nil, err
	}

	if tok, lit := p.scanIgnoreWhitespace(); tok != lexer.NUMBER {
		return nil, fmt.Errorf("found %q, expected number", lit)
	}
	duration, err := strconv.ParseUint(p.buf.lit, 10, 8)
	if err != nil {
		return nil, err
	}

	if tok, lit := p.scanIgnoreWhitespace(); tok != lexer.SEMICOLON {
		return nil, fmt.Errorf("found %q, expected ;", lit)
	}
	return types.NewSignature(uint8(beats), uint8(duration)), nil
}

func (p *Parser) parseProject() (*types.Project, error) {
	project := types.NewProject()

	if tok, lit := p.scanIgnoreWhitespace(); tok != lexer.PROJECT {
		return nil, fmt.Errorf("found %q, expected PROJECT", lit)
	}

	if tok, lit := p.scanIgnoreWhitespace(); tok != lexer.IDENTIFIER && tok != lexer.STRING {
		return nil, fmt.Errorf("found %q, expected project name", lit)
	} else {
		project.SetName(lit)
	}

	if tok, lit := p.scanIgnoreWhitespace(); tok != lexer.BRACKET_OPEN {
		return nil, fmt.Errorf("found %q, expected {", lit)
	}

	for {
		tok, lit := p.scanIgnoreWhitespace()
		if tok == lexer.BRACKET_CLOSE {
			break
		}
		switch tok {
		case lexer.BPM:
			bpm, err := p.parseBpm()
			if err != nil {
				return nil, err
			}
			project.SetBPM(bpm)

		case lexer.TIME:
			timeSignature, err := p.parseTimeSignature()
			if err != nil {
				return nil, err
			}
			project.SetSignature(timeSignature)

		case lexer.TRACK:
			track, err := p.parseTrack(project)
			if err != nil {
				return nil, err
			}
			project.AddTrack(track)

		case lexer.COPYRIGHT:
			text, err := p.parseCopyright()
			if err != nil {
				return nil, err
			}
			project.SetCopyright(text)

		case lexer.TEXT:
			text, err := p.parseText()
			if err != nil {
				return nil, err
			}
			project.AddText(text)

		default:
			return nil, fmt.Errorf("found %q, expected TRACK or }", lit)
		}
	}

	//	if tok, lit := p.scanIgnoreWhitespace(); tok != lexer.BRACKET_CLOSE {
	//		return nil, fmt.Errorf("found %q, expected }", lit)
	//	}

	if tok, lit := p.scanIgnoreWhitespace(); tok != lexer.EOF {
		return nil, fmt.Errorf("found %q, expected EOF", lit)
	}

	return project, nil
}

func (p *Parser) parseTrack(project *types.Project) (*types.Track, error) {
	//XXX - for now clear active notes when entering a new track
	p.activePitches = make(map[uint8]*types.Pitch)
	p.barNo = 0

	track := types.NewTrack()
	track.SetBPM(project.GetBPM())
	track.SetSignature(project.GetSignature())

	if tok, lit := p.scanIgnoreWhitespace(); tok != lexer.IDENTIFIER && tok != lexer.STRING {
		return nil, fmt.Errorf("found %q, expected track name", lit)
	} else {
		track.SetName(lit)
	}

	if tok, lit := p.scanIgnoreWhitespace(); tok != lexer.BRACKET_OPEN {
		return nil, fmt.Errorf("found %q, expected {", lit)
	}

	for {
		tok, lit := p.scanIgnoreWhitespace()
		if tok == lexer.BRACKET_CLOSE {
			break
		}
		switch tok {
		case lexer.BPM:
			_, err := p.parseBpm()
			if err != nil {
				return nil, err
			}
		case lexer.TIME:
			_, err := p.parseTimeSignature()
			if err != nil {
				return nil, err
			}
		case lexer.BAR:
			bar, err := p.parseBar(track)
			if err != nil {
				return nil, err
			}
			track.AddBar(bar)
			p.barNo++

		case lexer.INSTRUMENT:
			text, err := p.parseInstrument()
			if err != nil {
				return nil, err
			}
			track.SetInstrument(text)

		case lexer.TEXT:
			text, err := p.parseText()
			if err != nil {
				return nil, err
			}
			track.AddText(text)

		case lexer.REPEAT:
			bars, err := p.parseRepeat(track)
			if err != nil {
				return nil, err
			}
			for _, bar := range bars {
				track.AddBar(bar)
				p.barNo++
			}

		default:
			return nil, fmt.Errorf("found %q, expected BAR or }", lit)
		}
	}
	return track, nil
}

func (p *Parser) parseBar(track *types.Track) (*types.Bar, error) {
	bar := types.NewBar()
	bar.SetBPM(track.GetBPM())
	bar.SetSignature(track.GetSignature())

	if tok, lit := p.scanIgnoreWhitespace(); tok != lexer.BRACKET_OPEN {
		return nil, fmt.Errorf("found %q, expected {", lit)
	}

	for {
		tok, lit := p.scanIgnoreWhitespace()
		if tok == lexer.BRACKET_CLOSE {
			break
		}
		switch tok {
		case lexer.BPM:
			_, err := p.parseBpm()
			if err != nil {
				return nil, err
			}
		case lexer.TIME:
			_, err := p.parseTimeSignature()
			if err != nil {
				return nil, err
			}

		case lexer.ON:
			err := p.parseOn(bar)
			if err != nil {
				return nil, err
			}

		default:
			return nil, fmt.Errorf("found %q, expected TIME, BEAT or }", lit)
		}
	}

	return bar, nil
}

func (p *Parser) parseRepeat(track *types.Track) ([]*types.Bar, error) {

	repeatCount := uint(0)
	if tok, lit := p.scanIgnoreWhitespace(); tok != lexer.NUMBER {
		return nil, fmt.Errorf("found %q, expected loop count", lit)
	} else {
		tmp, err := strconv.Atoi(lit)
		if err != nil {
			return nil, err
		}
		repeatCount = uint(tmp)
	}

	if tok, lit := p.scanIgnoreWhitespace(); tok != lexer.TIMES {
		return nil, fmt.Errorf("found %q, expected FOR", lit)
	}

	if tok, lit := p.scanIgnoreWhitespace(); tok != lexer.BRACKET_OPEN && tok != lexer.BAR {
		return nil, fmt.Errorf("found %q, expected { or BAR", lit)
	} else {

		bars := make([]*types.Bar, 0)
		if tok == lexer.BRACKET_OPEN {
			for {
				tok, lit := p.scanIgnoreWhitespace()
				if tok == lexer.BRACKET_CLOSE {
					break
				}
				switch tok {
				case lexer.BAR:
					bar, err := p.parseBar(track)
					if err != nil {
						return nil, err
					}
					bars = append(bars, bar)

				case lexer.REPEAT:
					nestedBars, err := p.parseRepeat(track)
					if err != nil {
						return nil, err
					}
					bars = append(bars, nestedBars...)

				default:
					return nil, fmt.Errorf("found %q, expected BAR or }", lit)
				}
			}
		} else {
			bar, err := p.parseBar(track)
			if err != nil {
				return nil, err
			}
			bars = append(bars, bar)
		}
		repeatedBars := make([]*types.Bar, 0)
		for i := 0; i < int(repeatCount); i++ {
			repeatedBars = append(repeatedBars, bars...)
		}

		return repeatedBars, nil
	}
}

func (p *Parser) parsePlayable(bar *types.Bar, duration uint16, tick uint32) ([]*types.Pitch, error) {
	playables := make([]*types.Pitch, 0)
	for {
		tok, lit := p.scanIgnoreWhitespace()
		if tok == lexer.SEMICOLON {
			break
		}
		switch tok {
		case lexer.CHORD:
			pitches, err := p.parseChord()
			if err != nil {
				return nil, err
			}
			for _, pitch := range pitches {
				pitch.SetDuration(duration)
				playables = append(playables, pitch)
			}

		case lexer.NOTE:
			pitch, err := p.parseNote()
			if err != nil {
				return nil, err
			}
			pitch.SetDuration(duration)
			playables = append(playables, pitch)

		case lexer.PERCUSSION:
			pitch, err := p.parsePercussion()
			if err != nil {
				return nil, err
			}
			pitch.SetDuration(duration)
			playables = append(playables, pitch)

		default:
			return nil, fmt.Errorf("found %q, expected ;", lit)
		}

		for _, p := range playables {
			p.SetTick(tick)
		}

		for {
			tok, _ := p.scanIgnoreWhitespace()
			if tok == lexer.SEMICOLON {
				p.unscan()
				break
			}
			switch tok {
			case lexer.VELOCITY:
				if tok, lit := p.scanIgnoreWhitespace(); tok != lexer.NUMBER {
					return nil, fmt.Errorf("found %q, expected NUMBER", lit)
				} else {
					tmp, err := strconv.ParseUint(lit, 10, 8)
					if err != nil {
						return nil, err
					}
					for _, p := range playables {
						p.SetVelocity(uint8(tmp))
					}

				}
			}
		}

	}

	for _, pitch := range playables {
		if active, exists := p.activePitches[pitch.GetValue()]; !exists {
			p.activePitches[pitch.GetValue()] = pitch
		} else {
			var clock = smf.MetricTicks(960)

			unit := clock.Ticks4th()
			switch bar.GetSignature().GetDuration() {
			case 1:
				unit = clock.Ticks4th() * 4
			case 2:
				unit = clock.Ticks4th() * 2
			case 4:
				unit = clock.Ticks4th()
			case 8:
				unit = clock.Ticks8th()
			case 16:
				unit = clock.Ticks16th()
			case 32:
				unit = clock.Ticks32th()
			case 64:
				unit = clock.Ticks64th()
			case 128:
				unit = clock.Ticks128th()
			}

			duration := unit
			switch active.GetDuration() {
			case 1:
				duration *= 4
			case 2:
				duration *= 2
			case 4:
				duration = unit
			case 8:
				duration = unit / 2
			case 16:
				duration = unit / 4
			case 32:
				duration = unit / 8
			case 64:
				duration = unit / 16
			case 128:
				duration = unit / 32
			}

			/// XXX - THIS NEEDS TO BE CHECKED WHEN INSERTING A BAR IN THE TRACK AS WE NEED BAR NO TO COMPUTE OVERLAPS
			//if active.GetTick()+duration > tick {
			//	return nil, fmt.Errorf("pitch overlap: %d (tick: %d / %d)", pitch, active.GetTick(), playable.GetTick())
			//}
		}

	}

	return playables, nil
}

func (p *Parser) parseChord() ([]*types.Pitch, error) {
	ret := make([]*types.Pitch, 0)

	if tok, lit := p.scanIgnoreWhitespace(); tok != lexer.IDENTIFIER {
		return nil, fmt.Errorf("found %q, expected chord name", lit)
	} else {
		chord, err := chords.Parse(lit)
		if err != nil {
			return nil, err
		}

		for _, note := range chord.Notes() {
			ret = append(ret, types.NewPitch(note.MIDI()))
		}
		return ret, nil
	}
}

func (p *Parser) parseNote() (*types.Pitch, error) {
	if tok, lit := p.scanIgnoreWhitespace(); tok != lexer.IDENTIFIER {
		return nil, fmt.Errorf("found %q, expected note name", lit)
	} else {
		note, err := notes.Parse(lit)
		if err != nil {
			return nil, err
		}
		return types.NewPitch(note.MIDI()), nil
	}
}

func (p *Parser) parsePercussion() (*types.Pitch, error) {
	if tok, lit := p.scanIgnoreWhitespace(); tok != lexer.IDENTIFIER && tok != lexer.STRING {
		return nil, fmt.Errorf("found %q, expected percussion name", lit)
	} else {
		percussion, err := midi.PercussionKeyMap(lit)
		if err != nil {
			return nil, err
		}

		return types.NewPitch(percussion), nil
	}
}

func (p *Parser) parseCopyright() (string, error) {
	if tok, lit := p.scanIgnoreWhitespace(); tok != lexer.IDENTIFIER && tok != lexer.STRING {
		return "", fmt.Errorf("found %q, expected string", lit)
	} else {
		if tok, _ := p.scanIgnoreWhitespace(); tok != lexer.SEMICOLON {
			return "", fmt.Errorf("found %q, expected ;", lit)
		}
		return lit, nil
	}
}

func (p *Parser) parseText() (string, error) {
	if tok, lit := p.scanIgnoreWhitespace(); tok != lexer.IDENTIFIER && tok != lexer.STRING {
		return "", fmt.Errorf("found %q, expected string", lit)
	} else {
		if tok, _ := p.scanIgnoreWhitespace(); tok != lexer.SEMICOLON {
			return "", fmt.Errorf("found %q, expected ;", lit)
		}
		return lit, nil
	}
}

func (p *Parser) parseName() (string, error) {
	if tok, lit := p.scanIgnoreWhitespace(); tok != lexer.IDENTIFIER && tok != lexer.STRING {
		return "", fmt.Errorf("found %q, expected string", lit)
	} else {
		if tok, _ := p.scanIgnoreWhitespace(); tok != lexer.SEMICOLON {
			return "", fmt.Errorf("found %q, expected ;", lit)
		}
		return lit, nil
	}
}

func (p *Parser) parseInstrument() (string, error) {
	if tok, lit := p.scanIgnoreWhitespace(); tok != lexer.IDENTIFIER && tok != lexer.STRING {
		return "", fmt.Errorf("found %q, expected string", lit)
	} else {
		if tok, _ := p.scanIgnoreWhitespace(); tok != lexer.SEMICOLON {
			return "", fmt.Errorf("found %q, expected ;", lit)
		}

		_, err := midi.InstrumentToPC(lit)
		if err != nil {
			return "", fmt.Errorf("found %q, unknown instrument", lit)
		}

		return lit, nil
	}
}

func (p *Parser) parseOn(bar *types.Bar) error {
	if tok, lit := p.scanIgnoreWhitespace(); tok != lexer.BEAT {
		return fmt.Errorf("found %q, expected beat", lit)
	}

	var beat uint8
	var delta float64
	if tok, lit := p.scanIgnoreWhitespace(); tok != lexer.NUMBER && tok != lexer.FLOAT {
		return fmt.Errorf("found %q, expected NUMBER", lit)
	} else if tok == lexer.NUMBER {
		tmp, err := strconv.ParseUint(lit, 10, 8)
		if err != nil {
			return err
		}
		if tmp == 0 || uint64(tmp) > uint64(bar.GetSignature().GetBeats()) {
			return fmt.Errorf("no such beat: %d", tmp)
		}
		beat = uint8(tmp)
		delta = 0.0
	} else {
		tmp, err := strconv.ParseFloat(lit, 32)
		if err != nil {
			return err
		}

		integer, fraction := math.Modf(tmp)

		if uint64(integer) == 0 || uint64(integer) > uint64(bar.GetSignature().GetBeats()) {
			return fmt.Errorf("no such beat: %f", tmp)
		}
		beat = uint8(integer)
		delta = fraction
	}

	ticksPerBeat := uint32(960)
	ticksPerSubdivision := uint32(ticksPerBeat) / uint32(bar.GetSignature().GetDuration())

	deltaTicks := float64(ticksPerSubdivision) * delta
	tick := uint32(beat-1)*uint32(ticksPerBeat) +
		uint32(deltaTicks)

	tok, lit := p.scanIgnoreWhitespace()
	switch tok {
	case lexer.PLAY:
		//		fmt.Println("play", tick)
		return p.parsePlay(bar, tick)
	case lexer.TEXT:
		//		fmt.Println("text", tick)
		return p.parseMetaText(bar, tick)
	case lexer.LYRIC:
		//		fmt.Println("lyric", tick)
		return p.parseMetaLyric(bar, tick)
	case lexer.MARKER:
		//		fmt.Println("marker", tick)
		return p.parseMetaMarker(bar, tick)
	case lexer.CUE:
		//		fmt.Println("cue", tick)
		return p.parseMetaCue(bar, tick)

	default:
		return fmt.Errorf("found %q, expected PLAY", lit)
	}
}

func (p *Parser) parsePlay(bar *types.Bar, tick uint32) error {

	tok, lit := p.scanIgnoreWhitespace()

	switch tok {
	case lexer.WHOLE:
		playables, err := p.parsePlayable(bar, 1, tick)
		if err != nil {
			return err
		}
		for _, playable := range playables {
			bar.AddTickable(playable)

		}
	case lexer.HALF:
		playables, err := p.parsePlayable(bar, 2, tick)
		if err != nil {
			return err
		}
		for _, playable := range playables {
			bar.AddTickable(playable)
		}
	case lexer.QUARTER:
		playables, err := p.parsePlayable(bar, 4, tick)
		if err != nil {
			return err
		}
		for _, playable := range playables {
			bar.AddTickable(playable)
		}
	case lexer.NUMBER:
		value, err := strconv.ParseUint(lit, 10, 16)
		if err != nil {
			return err
		}

		if value == 8 {
			if tok, lit := p.scanIgnoreWhitespace(); tok != lexer.TH {
				return fmt.Errorf("found %q, expected note name", lit)
			}
		} else if value == 16 {
			if tok, lit := p.scanIgnoreWhitespace(); tok != lexer.TH {
				return fmt.Errorf("found %q, expected note name", lit)
			}
		} else if value == 32 {
			if tok, lit := p.scanIgnoreWhitespace(); tok != lexer.ND {
				return fmt.Errorf("found %q, expected note name", lit)
			}
		} else if value == 64 {
			if tok, lit := p.scanIgnoreWhitespace(); tok != lexer.TH {
				return fmt.Errorf("found %q, expected note name", lit)
			}
		} else if value == 128 {
			if tok, lit := p.scanIgnoreWhitespace(); tok != lexer.TH {
				return fmt.Errorf("found %q, expected note name", lit)
			}
		} else {
			return fmt.Errorf("found %q, expected value", lit)
		}
		playables, err := p.parsePlayable(bar, uint16(value), tick)
		if err != nil {
			return err
		}

		for _, playable := range playables {
			bar.AddTickable(playable)
		}

	default:
		return fmt.Errorf("found %q, expected TIME, BEAT or }", lit)
	}

	return nil
}

func (p *Parser) parseMetaText(bar *types.Bar, tick uint32) error {
	if tok, lit := p.scanIgnoreWhitespace(); tok != lexer.IDENTIFIER && tok != lexer.STRING {
		return fmt.Errorf("found %q, expected string", lit)
	} else {
		if tok, _ := p.scanIgnoreWhitespace(); tok != lexer.SEMICOLON {
			return fmt.Errorf("found %q, expected ;", lit)
		}

		tickable := types.NewText(lit)
		tickable.SetTick(tick)
		bar.AddTickable(tickable)

		return nil
	}
}

func (p *Parser) parseMetaLyric(bar *types.Bar, tick uint32) error {
	if tok, lit := p.scanIgnoreWhitespace(); tok != lexer.IDENTIFIER && tok != lexer.STRING {
		return fmt.Errorf("found %q, expected string", lit)
	} else {
		if tok, _ := p.scanIgnoreWhitespace(); tok != lexer.SEMICOLON {
			return fmt.Errorf("found %q, expected ;", lit)
		}

		tickable := types.NewLyric(lit)
		tickable.SetTick(tick)
		bar.AddTickable(tickable)

		return nil
	}
}

func (p *Parser) parseMetaMarker(bar *types.Bar, tick uint32) error {
	if tok, lit := p.scanIgnoreWhitespace(); tok != lexer.IDENTIFIER && tok != lexer.STRING {
		return fmt.Errorf("found %q, expected string", lit)
	} else {
		if tok, _ := p.scanIgnoreWhitespace(); tok != lexer.SEMICOLON {
			return fmt.Errorf("found %q, expected ;", lit)
		}

		tickable := types.NewMarker(lit)
		tickable.SetTick(tick)
		bar.AddTickable(tickable)

		return nil
	}
}

func (p *Parser) parseMetaCue(bar *types.Bar, tick uint32) error {
	if tok, lit := p.scanIgnoreWhitespace(); tok != lexer.IDENTIFIER && tok != lexer.STRING {
		return fmt.Errorf("found %q, expected string", lit)
	} else {
		if tok, _ := p.scanIgnoreWhitespace(); tok != lexer.SEMICOLON {
			return fmt.Errorf("found %q, expected ;", lit)
		}

		tickable := types.NewCue(lit)
		tickable.SetTick(tick)
		bar.AddTickable(tickable)

		return nil
	}
}

func (p *Parser) Parse() (*types.Project, error) {
	return p.parseProject()
}
