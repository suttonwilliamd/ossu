package main

import (
	"context"
	"fmt"
	"runtime"
	"unsafe"
)

var (
	registerHotKey   = user32.NewProc("RegisterHotKey")
	unregisterHotKey = user32.NewProc("UnregisterHotKey")
	peekMessage      = user32.NewProc("PeekMessageW")
	msgWait          = user32.NewProc("MsgWaitForMultipleObjectsEx")
)

const wmHotkey = 0x0312

type message struct {
	window         uintptr
	id             uint32
	wParam, lParam uintptr
	time           uint32
	pointX, pointY int32
	private        uint32
}

func runHotkey(ctx context.Context, key hotkey, ready func(), capture func() error) (err error) {
	// hWnd=NULL binds registration, the message queue and unregistration to
	// this OS thread. Never move between Go scheduler threads during that lifetime.
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	if ok, _, e := registerHotKey.Call(0, 1, uintptr(key.modifiers|modNoRepeat), uintptr(key.key)); ok == 0 {
		return fmt.Errorf("register hotkey %s: %w; another app or Windows may own it; close the conflicting app or choose -hotkey Ctrl+Alt+F10 (no fallback used)", key.name, winError("RegisterHotKey", e))
	}
	defer func() {
		if ok, _, e := unregisterHotKey.Call(0, 1); ok == 0 {
			addError(&err, winError("unregister global hotkey", e))
		}
	}()
	ready()
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		var msg message
		pending := false
		for {
			ok, _, _ := peekMessage.Call(uintptr(unsafe.Pointer(&msg)), 0, 0, 0, 1) // PM_REMOVE
			if ok == 0 {
				break
			}
			if msg.id == 0x0012 { // WM_QUIT
				return nil
			}
			pending = pending || (msg.id == wmHotkey && msg.wParam == 1)
		}
		if pending {
			if err := ctx.Err(); err != nil {
				return err
			}
			if err := capture(); err != nil {
				return err
			}
			// Discard presses received while busy rather than replaying a backlog.
			for {
				ok, _, _ := peekMessage.Call(uintptr(unsafe.Pointer(&msg)), 0, wmHotkey, wmHotkey, 1)
				if ok == 0 {
					break
				}
			}
		}
		// No hidden window or input hook. Bound idle cancellation latency to 50ms
		// without a shutdown goroutine that can post into a reused thread's queue.
		if result, _, e := msgWait.Call(0, 0, 50, 0x04ff, 0x0004); result == 0xffffffff { // QS_ALLINPUT, MWMO_INPUTAVAILABLE
			return winError("wait for global hotkey", e)
		}
	}
}
