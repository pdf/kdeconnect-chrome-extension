//go:build windows
// +build windows

package main

import (
	"errors"
	"unsafe"
	"path/filepath"

	"golang.org/x/sys/windows"
)

const (
	WTS_CURRENT_SERVER_HANDLE = 0
	WTS_CURRENT_SESSION       = 0xFFFFFFFF
	WTSTypeProcessInfoLevel0  = 0
)

type WTS_PROCESS_INFO struct {
	SessionId    uint32
	ProcessId    uint32
	pProcessName *uint16
	pUserSid     uintptr // incomplete: avoid defining a struct for something unneeded
}

var (
	libwtsapi32                  = windows.NewLazySystemDLL("wtsapi32.dll")
	procWTSEnumerateProcessesExW = libwtsapi32.NewProc("WTSEnumerateProcessesExW")
	procWTSFreeMemoryExW         = libwtsapi32.NewProc("WTSFreeMemoryExW")
)

func wtsFreeMemoryExW(WTSTypeClass uint32, pMemory unsafe.Pointer, NumberOfEntries uint32) {
	procWTSFreeMemoryExW.Call(uintptr(WTSTypeClass), uintptr(pMemory), uintptr(NumberOfEntries))
}

func wtsEnumerateProcessesExW(hServer windows.Handle, pLevel *uint32, SessionId uint32, ppProcessInfo unsafe.Pointer, pCount *uint32) (ret bool, lastErr error) {
	r1, _, lastErr := procWTSEnumerateProcessesExW.Call(uintptr(hServer), uintptr(unsafe.Pointer(pLevel)), uintptr(SessionId), uintptr(unsafe.Pointer(ppProcessInfo)), uintptr(unsafe.Pointer(pCount)))
	return r1 != 0, lastErr
}

func KdeConnectPathFromRunningIndicator() (ret string, err error) {
	var (
		Level        uint32 = WTSTypeProcessInfoLevel0
		pProcessInfo *WTS_PROCESS_INFO
		count        uint32

		hostSessionId uint32 = WTS_CURRENT_SESSION
	)
	// Only look at processes started in the same Windows session as the host is running in
	windows.ProcessIdToSessionId(windows.GetCurrentProcessId(), &hostSessionId)

	r1, lastErr := wtsEnumerateProcessesExW(WTS_CURRENT_SERVER_HANDLE, &Level, hostSessionId, unsafe.Pointer(&pProcessInfo), &count)
	if !r1 {
		return "", lastErr
	}
	defer wtsFreeMemoryExW(Level, unsafe.Pointer(pProcessInfo), count)

	size := unsafe.Sizeof(WTS_PROCESS_INFO{})
	for i := uint32(0); i < count; i++ {
		p := *(*WTS_PROCESS_INFO)(unsafe.Pointer(uintptr(unsafe.Pointer(pProcessInfo)) + (uintptr(size) * uintptr(i))))
		procName := windows.UTF16PtrToString(p.pProcessName)
		if procName == "kdeconnect-indicator.exe" || procName == "kdeconnectd.exe" || procName == "kdeconnect-app.exe" {
			hProcess, _ := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION, false, p.ProcessId)
			if hProcess != 0 {
				var exeNameBuf [261]uint16 // MAX_PATH + 1
				exeNameLen := uint32(len(exeNameBuf) - 1)
				err = windows.QueryFullProcessImageName(hProcess, 0, &exeNameBuf[0], &exeNameLen)
				windows.CloseHandle(hProcess)
				if err == nil {
					exeName := windows.UTF16ToString(exeNameBuf[:exeNameLen])
					return filepath.Dir(exeName), nil
				}
			}
		}
	}

	return "", errors.New("could not find KDE Connect processes")
}
