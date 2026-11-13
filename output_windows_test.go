package main

import (
	"context"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"io"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

var fixedTime = time.Date(2026, 11, 13, 12, 34, 56, 123456789, time.UTC)

// SYNTHETIC: generated content and a fixed clock; never captures a desktop.
func TestSyntheticConcurrentCollisionsPreserveFiles(t *testing.T) {
	out := t.TempDir()
	sentinel := filepath.Join(out, "OSSU_2026-11-13_12-34-56.123456789.png")
	if err := os.WriteFile(sentinel, []byte("existing user file"), 0600); err != nil {
		t.Fatal(err)
	}
	var workers sync.WaitGroup
	for i := range 12 {
		workers.Go(func() {
			payload := fmt.Sprintf("capture %d", i)
			path, err := writeCapture(out, fixedTime, func(w io.Writer) error {
				_, err := io.WriteString(w, payload)
				return err
			})
			if err != nil {
				t.Error(err)
				return
			}
			got, err := os.ReadFile(path)
			if err != nil || string(got) != payload || path == sentinel {
				t.Errorf("capture overwritten/corrupt at %s: %q, %v; want %q", path, got, err, payload)
			}
		})
	}
	workers.Wait()
	got, err := os.ReadFile(sentinel)
	if err != nil || string(got) != "existing user file" {
		t.Fatalf("existing user file changed: %q, %v", got, err)
	}
}

// SYNTHETIC: injected encoder errors/cancellation must delete only our partial file.
func TestSyntheticFailedWriteCleanup(t *testing.T) {
	for _, cancel := range []bool{false, true} {
		t.Run(fmt.Sprintf("cancel=%t", cancel), func(t *testing.T) {
			out := t.TempDir()
			sentinel := filepath.Join(out, "OSSU_2026-11-13_12-34-56.123456789.png")
			if err := os.WriteFile(sentinel, []byte("keep me"), 0600); err != nil {
				t.Fatal(err)
			}
			failure := errors.New("injected encoder failure")
			path, err := writeCapture(out, fixedTime, func(w io.Writer) error {
				if _, err := io.WriteString(w, "partial data"); err != nil {
					return err
				}
				if cancel {
					ctx, stop := context.WithCancel(context.Background())
					stop()
					_, err := (contextWriter{ctx: ctx, out: w}).Write([]byte("must not be written"))
					return err
				}
				return failure
			})
			wantError := failure
			if cancel {
				wantError = context.Canceled
			}
			if path != "" || !errors.Is(err, wantError) {
				t.Fatalf("failed write reported success: path=%q err=%v", path, err)
			}
			entries, err := os.ReadDir(out)
			if err != nil {
				t.Fatal(err)
			}
			for _, entry := range entries {
				if entry.Name() != filepath.Base(sentinel) {
					t.Errorf("partial file survived: %s", entry.Name())
				}
			}
			got, err := os.ReadFile(sentinel)
			if err != nil || string(got) != "keep me" {
				t.Fatalf("cleanup damaged existing file: %q, %v", got, err)
			}
		})
	}
}

// SYNTHETIC: a tiny generated image checks encoded pixels, not Windows capture.
func TestSyntheticPNGAndOutputErrors(t *testing.T) {
	out := filepath.Join(t.TempDir(), "new folder ü")
	img := image.NewRGBA(image.Rect(0, 0, 2, 1))
	img.SetRGBA(0, 0, color.RGBA{R: 200, G: 50, B: 30, A: 255})
	img.SetRGBA(1, 0, color.RGBA{R: 10, G: 80, B: 240, A: 255})
	path, err := savePNG(context.Background(), out, img, fixedTime)
	if err != nil {
		t.Fatal(err)
	}
	file, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	decoded, decodeErr := png.Decode(file)
	closeErr := file.Close()
	if decodeErr != nil || closeErr != nil {
		t.Fatalf("decode=%v close=%v", decodeErr, closeErr)
	}
	for x := range 2 {
		if decoded.At(x, 0) != img.At(x, 0) {
			t.Errorf("PNG pixel %d: got %v want %v", x, decoded.At(x, 0), img.At(x, 0))
		}
	}
	if result, err := savePNG(context.Background(), path, img, fixedTime); err == nil || result != "" {
		t.Fatalf("file as output directory reported success: %q, %v", result, err)
	}
	ctx, stop := context.WithCancel(context.Background())
	stop()
	unused := filepath.Join(out, "must not be created")
	if result, err := savePNG(ctx, unused, img, fixedTime); err != context.Canceled || result != "" {
		t.Fatalf("cancelled capture: %q, %v", result, err)
	}
	if _, err := os.Stat(unused); !os.IsNotExist(err) {
		t.Fatalf("cancelled capture created output directory: %v", err)
	}
}
