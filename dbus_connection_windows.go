//go:build windows
// +build windows

package main

import (
	"bytes"
	"crypto/sha1"
	"fmt"
	"io"
	"os"
	"reflect"
	"strings"
	"syscall"
	"time"
	"errors"
	"unsafe"

	"github.com/godbus/dbus/v5"
	"golang.org/x/sys/windows"
)

var (
	modkernel32          = windows.NewLazySystemDLL("kernel32.dll")
	procOpenFileMappingW = modkernel32.NewProc("OpenFileMappingW") //syscall only provides CreateFileMapping

	cachedDbusSectionName = ""
)

func normaliseAndHashDbusPath(kdeConnectPath string) string {
	if !strings.HasSuffix(kdeConnectPath, "\\") {
		kdeConnectPath = kdeConnectPath + "\\"
	}

	if strings.HasSuffix(kdeConnectPath, "\\bin\\") {
		kdeConnectPath = strings.TrimSuffix(kdeConnectPath, "bin\\")
	}

	kdeConnectPath = strings.ToLower(kdeConnectPath)

	sha1 := sha1.New()
	io.WriteString(sha1, kdeConnectPath)
	kdeConnectPath = fmt.Sprintf("%x", sha1.Sum(nil)) // produces a lower-case hash, which is required

	return kdeConnectPath
}

func readStringFromSection(sectionName string) (ret string, err error) {
	lpSectionName, err := syscall.UTF16PtrFromString(sectionName)
	if err != nil {
		return "", err
	}

	var hSharedMem uintptr
	for i := 0; i < 3; i++ {
		hSharedMem, _, err = syscall.Syscall(procOpenFileMappingW.Addr(), 3, windows.FILE_MAP_READ, 0, uintptr(unsafe.Pointer(lpSectionName)))
		if hSharedMem == 0 {
			time.Sleep(100 * time.Millisecond)
		} else {
			break
		}
	}
	if hSharedMem == 0 {
		return "", err
	}
	defer windows.CloseHandle(windows.Handle(hSharedMem))

	lpsharedAddr, err := syscall.MapViewOfFile(syscall.Handle(hSharedMem), windows.FILE_MAP_READ, 0, 0, 0)
	if err != nil {
		return "", err
	}
	defer syscall.UnmapViewOfFile(lpsharedAddr)

	// obtain section size - rounded up to the page size
	mbi := windows.MemoryBasicInformation{}
	err = windows.VirtualQueryEx(windows.CurrentProcess(), uintptr(lpsharedAddr), &mbi, unsafe.Sizeof(mbi))
	if err != nil {
		return "", err
	}

	if (mbi.RegionSize > 0x10000) { // if greater than 64Kb, which may already be too large, don't bother
		return "", errors.New("section size too large")
	}

	// get a byte[] representation of the mapping, using the same technique as syscall_unix.go
	// alternatively, unsafe.Slice((*byte)(unsafe.Pointer(sec)), mbi.RegionSize) works, with a "possible misuse of unsafe.Pointer" warning
	var bSharedAddr []byte
	hdr := (*reflect.SliceHeader)(unsafe.Pointer(&bSharedAddr))
	hdr.Data = lpsharedAddr
	hdr.Cap = int(mbi.RegionSize)
	hdr.Len = int(mbi.RegionSize)

	// copy section's contents into this process
	dbusAddress := make([]byte, len(bSharedAddr) + 1)
	copy(dbusAddress, bSharedAddr)
	dbusAddress[len(dbusAddress) - 1] = 0 // force null-termination somewhere

	// assuming a valid string, get the first null-terminator and cap the new slice's length to it
	strlen := bytes.IndexByte(dbusAddress, 0)
	return string(dbusAddress[:strlen]), nil
}

func DBusSessionBusForPlatform() (conn *dbus.Conn, err error) {
	// If $DBUS_SESSION_BUS_ADDRESS is set, just use that
	if this_env_sess_addr := os.Getenv("DBUS_SESSION_BUS_ADDRESS"); this_env_sess_addr != "" {
		return dbus.SessionBus()
	}

	var sectionName string = cachedDbusSectionName
	if sectionName == "" {
		kdeConnectPath, err := KdeConnectPathFromRunningIndicator()

		if err != nil {
			return nil, err
		}

		kdeConnectPath = normaliseAndHashDbusPath(kdeConnectPath)

		sectionName = "Local\\DBusDaemonAddressInfo-" + kdeConnectPath
	}

	if sectionName != "" {
		dbusAddress, err := readStringFromSection(sectionName)
		if err != nil {
			//cachedDbusSectionName = ""
			return nil, err
		}

		conn, err := dbus.Connect(dbusAddress)
		if err == nil {
			// Keep a copy of the computed section name to avoid the path lookups,
			// but not the section contents itself, as KDE Connect could be started again
			// from the same path but using a different port.
			// The odds of its install directory being changed on the other hand are low
			cachedDbusSectionName = sectionName
		} else {
			cachedDbusSectionName = ""
		}

		return conn, err
	}

	if err == nil {
		err = errors.New("could not determine KDE Connect's D-Bus daemon location")
	}

	return nil, err
}
