//go:build windows
// +build windows

package main

import (
	"unsafe"
	"syscall"
	"os/user"

	"golang.org/x/sys/windows"
	"github.com/go-ole/go-ole"
)

type IPackageManager struct {
	ole.IInspectable
}

type IPackageManagerVtbl struct {
	ole.IInspectableVtbl
	_[14] uintptr // skip defining unused methods
	FindPackagesByUserSecurityIdPackageFamilyName uintptr
}

type IIterable struct {
	ole.IInspectable
}

type IIterableVtbl struct {
	ole.IInspectableVtbl
	First uintptr
}

type IIterator struct {
	ole.IInspectable
}

type IIteratorVtbl struct {
	ole.IInspectableVtbl
	get_Current uintptr
}

type IPackage struct {
	ole.IInspectable
}

type IPackageVtbl struct {
	ole.IInspectableVtbl
	_[1] uintptr
	get_InstalledLocation uintptr
}

type IStorageFolder struct {
	ole.IInspectable
}

type IStorageItem struct {
	ole.IInspectable
}

type IStorageItemVtbl struct {
	ole.IInspectableVtbl
	_[6] uintptr
	get_Path uintptr
}

/* go-ole's wrapping of RoInitialize treats S_FALSE as an error */
var (
	modcombase         = windows.NewLazySystemDLL("combase.dll")
	procRoInitialize   = modcombase.NewProc("RoInitialize")
	procRoUninitialize = modcombase.NewProc("RoUninitialize")
)

func KdeConnectPathFromWindowsStore() (ret string, err error) {
	hr, _, _ := procRoInitialize.Call(uintptr(1)) // RO_INIT_MULTITHREADED
	if int32(hr) < 0 {
		return "", ole.NewError(hr)
	}
	defer procRoUninitialize.Call()

	// Get SID of running user - the package management APIs assume a search scoped to the system otherwise, which requires elevation
	me, err := user.Current()
	if err != nil {
		return "", err
	}

	inspectable, err := ole.RoActivateInstance("Windows.Management.Deployment.PackageManager")
	if err != nil {
		return "", err
	}
	defer inspectable.Release()
	unk, err := inspectable.QueryInterface(ole.NewGUID("9a7d4b65-5e8f-4fc7-a2e5-7f6925cb8b53"))
	if err != nil {
		return "", err
	}
	defer unk.Release()
	pm := (*IPackageManager)(unsafe.Pointer(unk))
	pmVtbl := (*IPackageManagerVtbl)(unsafe.Pointer(pm.RawVTable))
	
	var iterable *IIterable
	hsUid, _ := ole.NewHString(me.Uid)
	hsPackageFamilyName, _ := ole.NewHString("KDEe.V.KDEConnect_7vt06qxq7ptv8")
	defer ole.DeleteHString(hsUid)
	defer ole.DeleteHString(hsPackageFamilyName)
	hr, _, _ = syscall.Syscall6(pmVtbl.FindPackagesByUserSecurityIdPackageFamilyName, 4, uintptr(unsafe.Pointer(pm)), uintptr(hsUid), uintptr(hsPackageFamilyName), uintptr(unsafe.Pointer(&iterable)), 0, 0)
	if hr != 0 {
		return "", ole.NewError(hr)
	}
	defer iterable.Release()
	iterableVtbl := (*IIterableVtbl)(unsafe.Pointer(iterable.RawVTable))

	// TODO: Only the first result is looked at. Is it actually possible to have multiple KDE Connect installs from the Store?
	var iterator *IIterator
	hr, _, _ = syscall.Syscall(iterableVtbl.First, 2, uintptr(unsafe.Pointer(iterable)), uintptr(unsafe.Pointer(&iterator)), 0)
	if hr != 0 {
		return "", ole.NewError(hr)
	}
	defer iterator.Release()
	iteratorVtbl := (*IIteratorVtbl)(unsafe.Pointer(iterator.RawVTable))

	var ip *IPackage
	hr, _, _ = syscall.Syscall(iteratorVtbl.get_Current, 2, uintptr(unsafe.Pointer(iterator)), uintptr(unsafe.Pointer(&ip)), 0)
	if hr != 0 {
		return "", ole.NewError(hr)
	}
	defer ip.Release()
	ipVtbl := (*IPackageVtbl)(unsafe.Pointer(ip.RawVTable))

	var isf *IStorageFolder
	hr, _, _ = syscall.Syscall(ipVtbl.get_InstalledLocation, 2, uintptr(unsafe.Pointer(ip)), uintptr(unsafe.Pointer(&isf)), 0)
	if hr != 0 {
		return "", ole.NewError(hr)
	}
	defer isf.Release()
	unk, err = isf.QueryInterface(ole.NewGUID("4207a996-ca2f-42f7-bde8-8b10457a7f30"))
	if err != nil {
		return "", err
	}
	defer unk.Release()
	isi := (*IStorageItem)(unsafe.Pointer(unk))
	isiVtbl := (*IStorageItemVtbl)(unsafe.Pointer(isi.RawVTable))

	var kdeConnectPath ole.HString
	hr, _, _ = syscall.Syscall(isiVtbl.get_Path, 2, uintptr(unsafe.Pointer(isi)), uintptr(unsafe.Pointer(&kdeConnectPath)), 0)
	if hr != 0 {
		return "", ole.NewError(hr)
	}
	ret = kdeConnectPath.String()
	ole.DeleteHString(kdeConnectPath)

	return ret, nil
}
