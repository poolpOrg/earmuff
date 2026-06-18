// Package player plays a Standard MIDI File through whatever synth is available
// on the host. It resolves a player command in priority order:
//
//  1. an explicit override command (the -player flag), if non-empty
//  2. the EARMUFF_PLAYER environment variable
//  3. a platform-native player (afplay on macOS, timidity/wildmidi on Linux,
//     the file association on Windows)
//  4. fluidsynth, but only when a SoundFont can be located (fluidsynth has no
//     built-in instruments, so without one it plays silence)
//
// Override commands are templates: the first "{}" is replaced with the MIDI
// file path; if there is no "{}", the path is appended as a final argument.
package player

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

// Play renders smf to a temporary MIDI file and plays it. override is the
// -player flag value (may be empty). It returns a descriptive error if no
// player could be found or playback failed.
func Play(smf []byte, override string) error {
	path, cleanup, err := writeTemp(smf)
	if err != nil {
		return err
	}
	defer cleanup()

	name, args, err := resolve(override, path)
	if err != nil {
		return err
	}

	cmd := exec.Command(name, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("playback with %q failed: %w", name, err)
	}
	return nil
}

// resolve returns the command name and args to play the MIDI file at path.
func resolve(override, path string) (string, []string, error) {
	// 1. explicit override flag
	if tmpl := strings.TrimSpace(override); tmpl != "" {
		name, args := expandTemplate(tmpl, path)
		return name, args, nil
	}
	// 2. EARMUFF_PLAYER env
	if tmpl := strings.TrimSpace(os.Getenv("EARMUFF_PLAYER")); tmpl != "" {
		name, args := expandTemplate(tmpl, path)
		return name, args, nil
	}
	// 3. platform-native player
	if name, args, ok := platformPlayer(path); ok {
		return name, args, nil
	}
	// 4. fluidsynth, only with a soundfont
	if name, args, ok := fluidsynthPlayer(path); ok {
		return name, args, nil
	}
	// 5. last-resort fallback (e.g. `open` on macOS)
	if name, args, ok := fallbackPlayer(path); ok {
		return name, args, nil
	}
	return "", nil, noPlayerError()
}

// expandTemplate splits a command template into name+args, substituting the
// MIDI path for the first "{}" (or appending it if absent).
func expandTemplate(tmpl, path string) (string, []string) {
	fields := strings.Fields(tmpl)
	substituted := false
	for i, f := range fields {
		if strings.Contains(f, "{}") {
			fields[i] = strings.ReplaceAll(f, "{}", path)
			substituted = true
		}
	}
	if !substituted {
		fields = append(fields, path)
	}
	return fields[0], fields[1:]
}

// platformPlayer returns the OS-native player if one is available.
//
// Note: macOS has no zero-install headless SMF player — afplay handles audio
// files (wav/aiff/mp3/…), NOT MIDI sequences. So on macOS we rely on
// fluidsynth (handled in resolve's step 4) and fall back to `open`, which hands
// the file to the default app (GUI, non-blocking).
func platformPlayer(path string) (string, []string, bool) {
	switch runtime.GOOS {
	case "linux":
		for _, c := range []string{"timidity", "wildmidi"} {
			if p, err := exec.LookPath(c); err == nil {
				return p, []string{path}, true
			}
		}
	case "windows":
		// Play via the .mid file association, waiting for it to finish.
		if p, err := exec.LookPath("cmd"); err == nil {
			return p, []string{"/c", "start", "/wait", "", path}, true
		}
	}
	return "", nil, false
}

// fallbackPlayer is a last resort used only when nothing else is available. On
// macOS, `open` hands the file to the default app so the user at least hears
// something without installing a synth.
func fallbackPlayer(path string) (string, []string, bool) {
	if runtime.GOOS == "darwin" {
		if p, err := exec.LookPath("open"); err == nil {
			return p, []string{path}, true
		}
	}
	return "", nil, false
}

// soundFontPaths are common exact locations a General MIDI SoundFont may live.
var soundFontPaths = []string{
	"/usr/share/sounds/sf2/FluidR3_GM.sf2",
	"/usr/share/sounds/sf2/default-GM.sf2",
	"/usr/share/soundfonts/FluidR3_GM.sf2",
	"/usr/share/soundfonts/default.sf2",
	"/opt/homebrew/share/fluid-soundfont/FluidR3_GM.sf2",
	"/usr/local/share/fluid-soundfont/FluidR3_GM.sf2",
}

// soundFontGlobs catch SoundFonts at versioned or less-predictable locations,
// e.g. the one Homebrew ships inside the fluid-synth Cellar.
var soundFontGlobs = []string{
	"/opt/homebrew/Cellar/fluid-synth/*/share/fluid-synth/sf2/*.sf2",
	"/usr/local/Cellar/fluid-synth/*/share/fluid-synth/sf2/*.sf2",
	"/opt/homebrew/share/soundfonts/*.sf2",
	"/usr/share/sounds/sf2/*.sf2",
	"/usr/share/soundfonts/*.sf2",
}

// findSoundFont locates a SoundFont: EARMUFF_SOUNDFONT wins, then known exact
// paths, then well-known glob locations.
func findSoundFont() (string, bool) {
	if sf := strings.TrimSpace(os.Getenv("EARMUFF_SOUNDFONT")); sf != "" {
		if _, err := os.Stat(sf); err == nil {
			return sf, true
		}
	}
	for _, p := range soundFontPaths {
		if _, err := os.Stat(p); err == nil {
			return p, true
		}
	}
	for _, g := range soundFontGlobs {
		if matches, _ := filepath.Glob(g); len(matches) > 0 {
			return matches[0], true
		}
	}
	return "", false
}

// fluidsynthPlayer returns a fluidsynth invocation, but only if both fluidsynth
// and a SoundFont are present — otherwise fluidsynth would play silence.
func fluidsynthPlayer(path string) (string, []string, bool) {
	fs, err := exec.LookPath("fluidsynth")
	if err != nil {
		return "", nil, false
	}
	sf, ok := findSoundFont()
	if !ok {
		return "", nil, false
	}
	// -n no MIDI-in, -i no interactive shell, -q quiet: play the file then exit.
	args := []string{"-niq"}
	// Be explicit about the audio driver where the default is unreliable.
	switch runtime.GOOS {
	case "darwin":
		args = append(args, "-a", "coreaudio")
	}
	args = append(args, sf, path)
	return fs, args, true
}

// noPlayerError explains how to configure playback when nothing was found.
func noPlayerError() error {
	hint := "no MIDI player found"
	switch runtime.GOOS {
	case "darwin":
		hint += " (expected afplay)"
	case "linux":
		hint += " (install timidity, or fluidsynth + a SoundFont)"
	}
	return fmt.Errorf("%s; set EARMUFF_PLAYER=\"<cmd> {}\" or pass -player, "+
		"or set EARMUFF_SOUNDFONT to a .sf2 for fluidsynth", hint)
}

func writeTemp(smf []byte) (string, func(), error) {
	f, err := os.CreateTemp("", "earmuff-*.mid")
	if err != nil {
		return "", func() {}, err
	}
	if _, err := f.Write(smf); err != nil {
		f.Close()
		os.Remove(f.Name())
		return "", func() {}, err
	}
	f.Close()
	return f.Name(), func() { os.Remove(f.Name()) }, nil
}
