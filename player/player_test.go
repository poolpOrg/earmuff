package player

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestExpandTemplate_Substitutes(t *testing.T) {
	name, args := expandTemplate("fluidsynth -a coreaudio sf.sf2 {}", "/tmp/x.mid")
	if name != "fluidsynth" {
		t.Fatalf("name = %q, want fluidsynth", name)
	}
	want := []string{"-a", "coreaudio", "sf.sf2", "/tmp/x.mid"}
	if len(args) != len(want) {
		t.Fatalf("args = %v, want %v", args, want)
	}
	for i := range want {
		if args[i] != want[i] {
			t.Fatalf("args = %v, want %v", args, want)
		}
	}
}

func TestExpandTemplate_AppendsWhenNoPlaceholder(t *testing.T) {
	name, args := expandTemplate("timidity", "/tmp/x.mid")
	if name != "timidity" || len(args) != 1 || args[0] != "/tmp/x.mid" {
		t.Fatalf("got %q %v, want timidity [/tmp/x.mid]", name, args)
	}
}

func TestResolve_OverrideWins(t *testing.T) {
	t.Setenv("EARMUFF_PLAYER", "fromenv {}")
	name, args, err := resolve("explicit {}", "/tmp/x.mid")
	if err != nil {
		t.Fatal(err)
	}
	if name != "explicit" || args[0] != "/tmp/x.mid" {
		t.Fatalf("override did not win: %q %v", name, args)
	}
}

func TestResolve_EnvUsedWhenNoOverride(t *testing.T) {
	t.Setenv("EARMUFF_PLAYER", "fromenv -x {}")
	name, args, err := resolve("", "/tmp/x.mid")
	if err != nil {
		t.Fatal(err)
	}
	if name != "fromenv" || args[0] != "-x" || args[1] != "/tmp/x.mid" {
		t.Fatalf("env not used: %q %v", name, args)
	}
}

func TestFindSoundFont_EnvOverride(t *testing.T) {
	dir := t.TempDir()
	sf := filepath.Join(dir, "test.sf2")
	if err := os.WriteFile(sf, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("EARMUFF_SOUNDFONT", sf)
	got, ok := findSoundFont()
	if !ok || got != sf {
		t.Fatalf("findSoundFont = %q,%v, want %q,true", got, ok, sf)
	}
}

func TestFindSoundFont_MissingEnvFileIgnored(t *testing.T) {
	t.Setenv("EARMUFF_SOUNDFONT", "/nonexistent/nope.sf2")
	// Should not panic; returns whatever system path exists (likely none here).
	_, _ = findSoundFont()
}

func TestPlatformPlayer_MatchesOS(t *testing.T) {
	name, args, ok := platformPlayer("/tmp/x.mid")
	switch runtime.GOOS {
	case "darwin":
		// afplay is part of macOS; expect it present in CI/dev.
		if ok && name == "" {
			t.Fatal("darwin player resolved to empty name")
		}
		if ok && (len(args) == 0 || args[len(args)-1] != "/tmp/x.mid") {
			t.Fatalf("darwin args missing path: %v", args)
		}
	default:
		// On other platforms we just assert it doesn't panic and is consistent.
		if ok && len(args) == 0 {
			t.Fatalf("player %q returned no args", name)
		}
	}
}

func TestResolve_NoPlayerError(t *testing.T) {
	// Clear overrides; force the no-fluidsynth-soundfont path by pointing the
	// soundfont env at nothing and relying on no override/env player.
	t.Setenv("EARMUFF_PLAYER", "")
	t.Setenv("EARMUFF_SOUNDFONT", "/nonexistent.sf2")
	// We can't reliably remove the platform player (afplay exists on macOS), so
	// only assert the error path's message construction directly.
	err := noPlayerError()
	if err == nil {
		t.Fatal("noPlayerError returned nil")
	}
}
