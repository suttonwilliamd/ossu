package main

import (
	"io"
	"testing"
	"time"
)

// SYNTHETIC: parser behavior only; no hotkey registration or desktop capture.
func TestSyntheticHotkeys(t *testing.T) {
	valid := []struct {
		input          string
		modifiers, key uint32
	}{
		{"Ctrl+Shift+F9", modControl | modShift, 0x78},
		{" shift + alt + f11 ", modAlt | modShift, 0x7a},
		{"Ctrl+Alt+7", modControl | modAlt, '7'},
		{"Alt+a", modAlt, 'A'},
	}
	for _, test := range valid {
		t.Run(test.input, func(t *testing.T) {
			got, err := parseHotkey(test.input)
			if err != nil || got.modifiers != test.modifiers || got.key != test.key {
				t.Fatalf("hotkey = %+v, %v; want modifiers=%x key=%x", got, err, test.modifiers, test.key)
			}
		})
	}
	for _, input := range []string{"", "F9", "Shift+F9", "Ctrl+F12", "Ctrl+F0", "Ctrl+F01", "Win+F9", "Ctrl+Ctrl+F9", "Ctrl++F9", "Ctrl+Shift", "Ctrl+PrintScreen", "Ctrl+F9+F10", "Ctrl+é"} {
		t.Run("reject_"+input, func(t *testing.T) {
			if _, err := parseHotkey(input); err == nil {
				t.Fatalf("accepted unsupported hotkey %q", input)
			}
		})
	}
}

// SYNTHETIC: reject ambiguous modes before any native API or directory creation.
func TestSyntheticModes(t *testing.T) {
	for _, args := range [][]string{
		{"-once", "-interval", "1s"}, {"-interval", "0"}, {"-interval", "-1s"},
		{"-interval", "99ms"}, {"-count", "1"}, {"-count", "0"},
		{"-once", "-count", "1"}, {"-interval", "1s", "-count", "0"},
		{"-interval", "1s", "-count", "-1"}, {"-once", "-hotkey", "Ctrl+F9"},
		{"-interval", "1s", "-hotkey", "Ctrl+F9"}, {"-out", ""},
		{"-hotkey", "Ctrl+F12"}, {"unexpected"}, {"-unknown"},
	} {
		if _, err := parseOptions(args, io.Discard); err == nil {
			t.Errorf("accepted invalid mode %q", args)
		}
	}
	defaults, err := parseOptions(nil, io.Discard)
	if err != nil || defaults.once || defaults.interval != 0 || defaults.count != 0 || defaults.hotkey.key != 0x78 {
		t.Fatalf("default must wait for Ctrl+Shift+F9, not auto-capture: %+v, %v", defaults, err)
	}
	once, err := parseOptions([]string{"-once", "-out", "captures"}, io.Discard)
	if err != nil || !once.once || once.interval != 0 || once.out != "captures" {
		t.Fatalf("once mode: %+v, %v", once, err)
	}
	batch, err := parseOptions([]string{"-interval", "100ms", "-count", "3"}, io.Discard)
	if err != nil || batch.once || batch.interval != 100*time.Millisecond || batch.count != 3 {
		t.Fatalf("bounded interval mode: %+v, %v", batch, err)
	}
}
