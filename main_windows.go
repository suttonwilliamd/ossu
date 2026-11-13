package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"syscall"
	"time"
)

func main() {
	os.Exit(run(os.Args[1:]))
}

func run(args []string) int {
	opt, err := parseOptions(args, os.Stderr)
	if err == flag.ErrHelp {
		return 0
	}
	if err == nil && runtime.GOARCH != "amd64" {
		err = fmt.Errorf("this milestone supports Windows x64 only; build with GOARCH=amd64")
	}
	if err == nil && opt.out == "" {
		opt.out, err = defaultOutput()
	}
	if err == nil {
		opt.out, err = filepath.Abs(opt.out)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "ossu:", err)
		return 1
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	// A launcher can pass down Windows' ignore-Ctrl+C flag. Explicitly clear
	// it in THIS process so the registered Go handler can cancel every mode.
	if ok, _, e := kernel32.NewProc("SetConsoleCtrlHandler").Call(0, 0); ok == 0 {
		fmt.Fprintln(os.Stderr, "ossu:", winError("enable Ctrl+C processing", e))
		return 1
	}
	logger := log.New(os.Stdout, "", log.LstdFlags)
	capture := func() error {
		saved, err := capturePrimary(ctx, opt.out)
		if err != nil {
			return err
		}
		logger.Printf("Saved %s (%dx%d)", saved.path, saved.width, saved.height)
		return nil
	}
	switch {
	case opt.once:
		err = capture()
	case opt.interval != 0:
		logger.Printf("Timed capture: first in %s, then %s after each save; count=%s. Output: %s. Ctrl+C stops.", opt.interval, opt.interval, countDescription(opt.count), opt.out)
		err = runInterval(ctx, opt.interval, opt.count, capture)
	default:
		err = runHotkey(ctx, opt.hotkey, func() {
			logger.Printf("Waiting for %s (no key repeat). Output: %s. Ctrl+C stops.", opt.hotkey.name, opt.out)
		}, capture)
	}
	if err == context.Canceled {
		logger.Print("Stopped.")
		return 0
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "ossu:", err)
		return 1
	}
	return 0
}

func countDescription(count int) string {
	if count == 0 {
		return "unlimited"
	}
	return fmt.Sprint(count)
}

func runInterval(ctx context.Context, interval time.Duration, count int, capture func() error) error {
	timer := time.NewTimer(interval)
	defer timer.Stop()
	for completed := 0; count == 0 || completed < count; completed++ {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-timer.C:
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		if err := capture(); err != nil {
			return err
		}
		timer.Reset(interval)
	}
	return nil
}
