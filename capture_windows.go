package main

import (
	"context"
	"fmt"
	"image"
	"runtime"
	"time"
	"unsafe"
)

var (
	setThreadDPI       = user32.NewProc("SetThreadDpiAwarenessContext")
	monitorFromPoint   = user32.NewProc("MonitorFromPoint")
	getMonitorInfo     = user32.NewProc("GetMonitorInfoW")
	getDC              = user32.NewProc("GetDC")
	releaseDC          = user32.NewProc("ReleaseDC")
	createCompatibleDC = gdi32.NewProc("CreateCompatibleDC")
	deleteDC           = gdi32.NewProc("DeleteDC")
	createDIBSection   = gdi32.NewProc("CreateDIBSection")
	selectObject       = gdi32.NewProc("SelectObject")
	deleteObject       = gdi32.NewProc("DeleteObject")
	bitBlt             = gdi32.NewProc("BitBlt")
	gdiFlush           = gdi32.NewProc("GdiFlush")
)

type rect struct{ left, top, right, bottom int32 }

type monitorInfo struct {
	size          uint32
	monitor, work rect
	flags         uint32
}

type bitmapInfoHeader struct {
	size                             uint32
	width, height                    int32
	planes, bitCount                 uint16
	compression, imageSize           uint32
	xPixelsPerMeter, yPixelsPerMeter int32
	colorsUsed, colorsImportant      uint32
}

type savedCapture struct {
	path          string
	width, height int
}

func capturePrimary(ctx context.Context, out string) (saved savedCapture, err error) {
	if err = ctx.Err(); err != nil {
		return saved, err
	}
	// DPI contexts and GDI DC ownership are thread-affine. The hotkey loop may
	// already hold this thread; LockOSThread is nestable.
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	if findErr := setThreadDPI.Find(); findErr != nil {
		return saved, fmt.Errorf("per-monitor DPI capture requires Windows 10 1703 or newer: %w", findErr)
	}
	previous, _, callErr := setThreadDPI.Call(^uintptr(3)) // DPI_AWARENESS_CONTEXT_PER_MONITOR_AWARE_V2 (-4)
	if previous == 0 {
		return saved, winError("set native-pixel DPI context (requires Windows 10 1703+)", callErr)
	}
	defer func() {
		if ok, _, e := setThreadDPI.Call(previous); ok == 0 {
			addError(&err, winError("restore DPI context", e))
		}
	}()
	monitor, _, callErr := monitorFromPoint.Call(0, 1) // origin, MONITOR_DEFAULTTOPRIMARY
	if monitor == 0 {
		return saved, winError("find primary monitor", callErr)
	}
	info := monitorInfo{size: uint32(unsafe.Sizeof(monitorInfo{}))}
	if ok, _, e := getMonitorInfo.Call(monitor, uintptr(unsafe.Pointer(&info))); ok == 0 {
		return saved, winError("read primary-monitor bounds", e)
	}
	width := int(info.monitor.right) - int(info.monitor.left)
	height := int(info.monitor.bottom) - int(info.monitor.top)
	if width <= 0 || height <= 0 || width > int(^uint(0)>>1)/4/height {
		return saved, fmt.Errorf("invalid primary-monitor dimensions %dx%d", width, height)
	}
	screen, _, callErr := getDC.Call(0)
	if screen == 0 {
		return saved, winError("get visible desktop DC", callErr)
	}
	defer func() {
		if ok, _, e := releaseDC.Call(0, screen); ok == 0 {
			addError(&err, winError("release desktop DC", e))
		}
	}()
	memory, _, callErr := createCompatibleDC.Call(screen)
	if memory == 0 {
		return saved, winError("create capture DC", callErr)
	}
	var bitmap, previousBitmap uintptr
	defer func() {
		if previousBitmap != 0 && previousBitmap != ^uintptr(0) {
			if ok, _, e := selectObject.Call(memory, previousBitmap); ok == 0 || ok == ^uintptr(0) {
				addError(&err, winError("restore capture DC bitmap", e))
			}
		}
		// Destroy the DC before the bitmap even if restoring its selection failed.
		if ok, _, e := deleteDC.Call(memory); ok == 0 {
			addError(&err, winError("delete capture DC", e))
		}
		if bitmap != 0 {
			if ok, _, e := deleteObject.Call(bitmap); ok == 0 {
				addError(&err, winError("delete capture bitmap", e))
			}
		}
	}()
	header := bitmapInfoHeader{size: 40, width: int32(width), height: -int32(height), planes: 1, bitCount: 32}
	var bits unsafe.Pointer
	bitmap, _, callErr = createDIBSection.Call(screen, uintptr(unsafe.Pointer(&header)), 0,
		uintptr(unsafe.Pointer(&bits)), 0, 0) // top-down 32-bit BI_RGB, DIB_RGB_COLORS
	if bitmap == 0 || bits == nil {
		return saved, winError("allocate capture bitmap", callErr)
	}
	previousBitmap, _, callErr = selectObject.Call(memory, bitmap)
	if previousBitmap == 0 || previousBitmap == ^uintptr(0) {
		return saved, winError("select capture bitmap", callErr)
	}
	capturedAt := time.Now()
	if ok, _, e := bitBlt.Call(memory, 0, 0, uintptr(width), uintptr(height), screen,
		uintptr(info.monitor.left), uintptr(info.monitor.top), 0x00cc0020|0x40000000); ok == 0 { // SRCCOPY | CAPTUREBLT
		return saved, winError("capture visible primary desktop", e)
	}
	if ok, _, e := gdiFlush.Call(); ok == 0 {
		return saved, winError("finish GDI capture", e)
	}
	// Convert BGRA to opaque RGBA in the DIB itself, avoiding a full-screen copy.
	// The encoder finishes before the deferred bitmap deletion frees these bytes.
	pixels := unsafe.Slice((*byte)(bits), width*height*4)
	for y := range height {
		if err = ctx.Err(); err != nil {
			return saved, err
		}
		row := pixels[y*width*4 : (y+1)*width*4]
		for x := 0; x < len(row); x += 4 {
			row[x], row[x+2], row[x+3] = row[x+2], row[x], 255
		}
	}
	img := image.RGBA{Pix: pixels, Stride: width * 4, Rect: image.Rect(0, 0, width, height)}
	saved.width, saved.height = width, height
	saved.path, err = savePNG(ctx, out, &img, capturedAt)
	return saved, err
}
