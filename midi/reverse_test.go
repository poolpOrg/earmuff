package midi

import "testing"

func TestPCToInstrument_RoundTrip(t *testing.T) {
	for _, name := range []string{"piano", "violin", "acoustic bass", "synth drum"} {
		pc, err := InstrumentToPC(name)
		if err != nil {
			t.Fatalf("InstrumentToPC(%q): %v", name, err)
		}
		got, err := PCToInstrument(pc)
		if err != nil {
			t.Fatalf("PCToInstrument(%d): %v", pc, err)
		}
		// InstrumentToPC lowercases; the canonical list may differ in case.
		if back, _ := InstrumentToPC(got); back != pc {
			t.Fatalf("round-trip %q -> pc %d -> %q -> pc %d", name, pc, got, back)
		}
	}
}

func TestKeyToPercussion_RoundTrip(t *testing.T) {
	for _, name := range []string{"acoustic snare", "closed hi-hat", "crash cymbal 1"} {
		key, err := PercussionKeyMap(name)
		if err != nil {
			t.Fatalf("PercussionKeyMap(%q): %v", name, err)
		}
		got, err := KeyToPercussion(key)
		if err != nil {
			t.Fatalf("KeyToPercussion(%d): %v", key, err)
		}
		if back, _ := PercussionKeyMap(got); back != key {
			t.Fatalf("round-trip %q -> key %d -> %q -> key %d", name, key, got, back)
		}
	}
}

func TestReverse_OutOfRange(t *testing.T) {
	if _, err := PCToInstrument(0); err == nil {
		t.Fatal("PCToInstrument(0) should error")
	}
	if _, err := KeyToPercussion(0); err == nil {
		t.Fatal("KeyToPercussion(0) should error")
	}
}
