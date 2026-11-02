//go:build windows

package main

import (
	"syscall"
	"unsafe"
)

// systemPrefersDark reads the Windows app mode (Settings, Personalization,
// Colors: "Choose your default app mode"). Any failure reads as light.
func systemPrefersDark() bool {
	sub, err := syscall.UTF16PtrFromString(`Software\Microsoft\Windows\CurrentVersion\Themes\Personalize`)
	if err != nil {
		return false
	}
	var key syscall.Handle
	if syscall.RegOpenKeyEx(syscall.HKEY_CURRENT_USER, sub, 0, syscall.KEY_READ, &key) != nil {
		return false
	}
	defer syscall.RegCloseKey(key)
	name, err := syscall.UTF16PtrFromString("AppsUseLightTheme")
	if err != nil {
		return false
	}
	var typ, val uint32
	size := uint32(unsafe.Sizeof(val))
	if syscall.RegQueryValueEx(key, name, nil, &typ, (*byte)(unsafe.Pointer(&val)), &size) != nil || typ != syscall.REG_DWORD {
		return false
	}
	return val == 0
}
