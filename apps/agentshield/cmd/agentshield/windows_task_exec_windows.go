package main

import (
	"errors"
	"path/filepath"
	"syscall"
	"unsafe"
)

func windowsTaskExecutable() (string, error) {
	var buffer [32768]uint16
	proc := syscall.NewLazyDLL("kernel32.dll").NewProc("GetSystemDirectoryW")
	n, _, _ := proc.Call(uintptr(unsafe.Pointer(&buffer[0])), uintptr(len(buffer)))
	if n == 0 || n >= uintptr(len(buffer)) {
		return "", errors.New("task-query: Windows system directory unavailable")
	}
	return filepath.Join(syscall.UTF16ToString(buffer[:n]), "schtasks.exe"), nil
}
