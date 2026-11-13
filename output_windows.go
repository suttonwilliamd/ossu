package main

import (
	"context"
	"fmt"
	"image"
	"image/png"
	"io"
	"os"
	"path/filepath"
	"syscall"
	"time"
	"unsafe"
)

var setFileInformationByHandle = kernel32.NewProc("SetFileInformationByHandle")

func savePNG(ctx context.Context, out string, pixels image.Image, at time.Time) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	return writeCapture(out, at, func(w io.Writer) error {
		encoder := png.Encoder{CompressionLevel: png.BestSpeed}
		return encoder.Encode(contextWriter{ctx: ctx, out: w}, pixels)
	})
}

type contextWriter struct {
	ctx context.Context
	out io.Writer
}

func (w contextWriter) Write(data []byte) (int, error) {
	if err := w.ctx.Err(); err != nil {
		return 0, err
	}
	return w.out.Write(data)
}

// writeCapture never opens an existing file. Failed writes are deleted by HANDLE,
// while still exclusively owned, not by a path that another process could replace.
func writeCapture(out string, at time.Time, encode func(io.Writer) error) (string, error) {
	if err := os.MkdirAll(out, 0755); err != nil {
		return "", fmt.Errorf("create output folder %q: %w", out, err)
	}
	base := "OSSU_" + at.Format("2006-01-02_15-04-05.000000000")
	for suffix := range 10000 {
		name := base + ".png"
		if suffix != 0 {
			name = fmt.Sprintf("%s_%04d.png", base, suffix)
		}
		path := filepath.Join(out, name)
		wide, err := syscall.UTF16PtrFromString(path)
		if err != nil {
			return "", fmt.Errorf("output filename: %w", err)
		}
		// DELETE access permits failure cleanup on this exact handle. Do not
		// share writes/deletion, so a partial file cannot be replaced underneath us.
		handle, err := syscall.CreateFile(wide, syscall.GENERIC_WRITE|0x00010000,
			syscall.FILE_SHARE_READ, nil, syscall.CREATE_NEW, syscall.FILE_ATTRIBUTE_NORMAL, 0)
		if err == syscall.ERROR_FILE_EXISTS || err == syscall.ERROR_ALREADY_EXISTS {
			continue
		}
		if err != nil {
			return "", fmt.Errorf("create screenshot %q (check folder access/free space): %w", path, err)
		}
		file := os.NewFile(uintptr(handle), path)
		if err := encode(file); err != nil {
			if err != context.Canceled {
				err = fmt.Errorf("write screenshot %q: %w", path, err)
			}
			remove := byte(1) // FILE_DISPOSITION_INFO.DeleteFile
			ok, _, removeErr := setFileInformationByHandle.Call(uintptr(handle), 4,
				uintptr(unsafe.Pointer(&remove)), unsafe.Sizeof(remove))
			if ok == 0 {
				addError(&err, fmt.Errorf("partial file retained at %q: %w", path, winError("mark failed screenshot for deletion", removeErr)))
			}
			addError(&err, file.Close())
			return "", err
		}
		if err := file.Close(); err != nil {
			return "", fmt.Errorf("close screenshot %q: %w", path, err)
		}
		return path, nil
	}
	return "", fmt.Errorf("too many timestamp collisions in %q; no existing files overwritten", out)
}
