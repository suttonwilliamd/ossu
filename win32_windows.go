package main

import (
	"errors"
	"fmt"
	"path/filepath"
	"syscall"
	"unsafe"
)

var (
	user32   = syscall.NewLazyDLL("user32.dll")
	gdi32    = syscall.NewLazyDLL("gdi32.dll")
	kernel32 = syscall.NewLazyDLL("kernel32.dll")
	shell32  = syscall.NewLazyDLL("shell32.dll")
	ole32    = syscall.NewLazyDLL("ole32.dll")
)

func winError(operation string, err error) error {
	if err == nil || err == syscall.Errno(0) {
		return fmt.Errorf("%s failed (Windows supplied no error code)", operation)
	}
	return fmt.Errorf("%s: %w", operation, err)
}

func addError(target *error, err error) {
	if err != nil {
		if *target == nil {
			*target = err
		} else {
			*target = errors.Join(*target, err)
		}
	}
}

func defaultOutput() (string, error) {
	// FOLDERID_Pictures; do not create or silently substitute another folder.
	pictures := syscall.GUID{Data1: 0x33e28130, Data2: 0x4e1e, Data3: 0x4676, Data4: [8]byte{0x83, 0x5a, 0x98, 0x39, 0x5c, 0x3b, 0xc3, 0xbb}}
	var path *uint16
	result, _, _ := shell32.NewProc("SHGetKnownFolderPath").Call(
		uintptr(unsafe.Pointer(&pictures)), 0x4000, 0, uintptr(unsafe.Pointer(&path))) // KF_FLAG_DONT_VERIFY, not CREATE
	if path != nil {
		defer ole32.NewProc("CoTaskMemFree").Call(uintptr(unsafe.Pointer(path)))
	}
	if int32(result) < 0 || path == nil {
		return "", fmt.Errorf("resolve Windows Pictures folder (HRESULT 0x%08x); specify -out explicitly", uint32(result))
	}
	return filepath.Join(syscall.UTF16ToString(unsafe.Slice(path, 32768)), "OSSU"), nil
}
