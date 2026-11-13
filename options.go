package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"
)

const defaultHotkey = "Ctrl+Shift+F9"

const (
	modAlt      = 0x0001
	modControl  = 0x0002
	modShift    = 0x0004
	modNoRepeat = 0x4000
)

type hotkey struct {
	modifiers uint32
	key       uint32
	name      string
}

type options struct {
	out      string
	once     bool
	interval time.Duration
	count    int
	hotkey   hotkey
}

func parseOptions(args []string, help io.Writer) (options, error) {
	var opt options
	var shortcut string
	flags := flag.NewFlagSet("ossu", flag.ContinueOnError)
	flags.SetOutput(help)
	flags.StringVar(&opt.out, "out", "", "output folder (default: Windows Pictures\\OSSU; created only on capture)")
	flags.BoolVar(&opt.once, "once", false, "capture once immediately, without registering a hotkey")
	flags.DurationVar(&opt.interval, "interval", 0, "opt-in timed capture, at least 100ms; first capture after this delay")
	flags.IntVar(&opt.count, "count", 0, "positive capture limit, only with -interval (default: unlimited)")
	flags.StringVar(&shortcut, "hotkey", defaultHotkey, "Ctrl/Alt/Shift + A-Z, 0-9 or F1-F11; requires Ctrl or Alt")
	flags.Usage = func() {
		fmt.Fprintln(help, "OSSU: primary-display visible-desktop PNG capture (Windows x64).")
		fmt.Fprintln(help, "Usage: ossu [-out folder] [-hotkey Ctrl+Shift+F9 | -once | -interval 5s [-count 3]]")
		fmt.Fprintln(help, "Default: wait for the global hotkey. Ctrl+C stops. No automatic capture.")
		flags.PrintDefaults()
	}
	if err := flags.Parse(args); err != nil {
		return opt, err
	}
	if flags.NArg() != 0 {
		return opt, fmt.Errorf("unexpected arguments: %s (use -help)", strings.Join(flags.Args(), " "))
	}
	specified := make(map[string]bool)
	flags.Visit(func(f *flag.Flag) { specified[f.Name] = true })
	if specified["out"] && strings.TrimSpace(opt.out) == "" {
		return opt, errors.New("-out must name a folder")
	}
	if opt.once && specified["interval"] {
		return opt, errors.New("-once and -interval cannot be combined")
	}
	if specified["interval"] && opt.interval < 100*time.Millisecond {
		return opt, errors.New("-interval must be at least 100ms (for example -interval 5s)")
	}
	if specified["count"] && (!specified["interval"] || opt.count <= 0) {
		return opt, errors.New("-count must be positive and requires -interval")
	}
	if specified["hotkey"] && (opt.once || specified["interval"]) {
		return opt, errors.New("-hotkey is only valid in the default hotkey mode")
	}
	var err error
	opt.hotkey, err = parseHotkey(shortcut)
	return opt, err
}

func parseHotkey(value string) (hotkey, error) {
	var result hotkey
	parts := strings.Split(strings.ToUpper(strings.TrimSpace(value)), "+")
	if len(parts) < 2 {
		return result, errors.New("hotkey requires Ctrl or Alt plus A-Z, 0-9 or F1-F11")
	}
	for _, part := range parts[:len(parts)-1] {
		var modifier uint32
		switch strings.TrimSpace(part) {
		case "CTRL":
			modifier = modControl
		case "ALT":
			modifier = modAlt
		case "SHIFT":
			modifier = modShift
		default:
			return result, fmt.Errorf("unsupported hotkey modifier %q; use Ctrl, Alt or Shift (Windows-key shortcuts are reserved)", part)
		}
		if result.modifiers&modifier != 0 {
			return result, fmt.Errorf("duplicate hotkey modifier %q", part)
		}
		result.modifiers |= modifier
	}
	if result.modifiers&(modControl|modAlt) == 0 {
		return result, errors.New("hotkey requires Ctrl or Alt; Shift alone is not supported")
	}
	key := strings.TrimSpace(parts[len(parts)-1])
	switch {
	case len(key) == 1 && ((key[0] >= 'A' && key[0] <= 'Z') || (key[0] >= '0' && key[0] <= '9')):
		result.key = uint32(key[0])
	case strings.HasPrefix(key, "F"):
		n, err := strconv.Atoi(key[1:])
		if err != nil || n < 1 || n > 11 || key != fmt.Sprintf("F%d", n) {
			return result, errors.New("function hotkeys must be F1-F11; F12 is reserved for Windows debuggers")
		}
		result.key = uint32(0x70 + n - 1)
	default:
		return result, fmt.Errorf("unsupported hotkey key %q; use A-Z, 0-9 or F1-F11", key)
	}
	var names []string
	for _, mod := range []struct {
		mask uint32
		name string
	}{{modControl, "Ctrl"}, {modAlt, "Alt"}, {modShift, "Shift"}} {
		if result.modifiers&mod.mask != 0 {
			names = append(names, mod.name)
		}
	}
	result.name = strings.Join(append(names, key), "+")
	return result, nil
}
