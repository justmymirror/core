// Copyright (c) 2026 Zane Schepke / WG Tunnel
//
// Portions of this file are derived from Tailscale
// Copyright (c) Tailscale Inc & contributors
// Licensed under the BSD 3-Clause License
// See NOTICE and LICENSES/BSD-3-Clause.txt in the project root

//go:build windows

package osrouter

import (
	"fmt"
	"runtime"
	"syscall"
	"time"
	"unsafe"

	"github.com/wgtunnel/backend/log"
	"golang.org/x/sys/windows"
	"golang.zx2c4.com/wireguard/windows/tunnel/winipcfg"
)

// NLM_NETWORK_CATEGORY. Public is 0. Domain authenticated stays as Windows set it.
const (
	categoryPrivate = 1
	categoryDomain  = 2
)

// INetworkListManager, INetworkConnection, and INetwork slots past IDispatch.
// IUnknown is 3 and IDispatch adds 4, so the first method of each interface is 7.
//
//	INetworkListManager: GetNetworkConnections = 9
//	INetworkConnection:  GetNetwork = 7, GetAdapterId = 12
//	INetwork:            GetCategory = 18, SetCategory = 19
const (
	nlmGetNetworkConnections = 9
	ncoGetNetwork            = 7
	ncoGetAdapterId          = 12
	netGetCategory           = 18
	netSetCategory           = 19
	clsctxInprocServer       = 1
)

var (
	procCoCreateInstance = windows.NewLazySystemDLL("ole32.dll").NewProc("CoCreateInstance")

	clsidNetworkListManager = windows.GUID{
		Data1: 0xdcb00c01, Data2: 0x570f, Data3: 0x4a9b,
		Data4: [8]byte{0x8d, 0x69, 0x19, 0x9f, 0xdb, 0xa5, 0x72, 0x3b},
	}
	iidINetworkListManager = windows.GUID{
		Data1: 0xdcb00000, Data2: 0x570f, Data3: 0x4a9b,
		Data4: [8]byte{0x8d, 0x69, 0x19, 0x9f, 0xdb, 0xa5, 0x72, 0x3b},
	}
)

// ensurePrivateNetwork starts one background pass that marks the tunnel
// adapter private once Network List Manager knows about it. Close cancels it
// via context. A later Set does not start a second pass.
func (r *windowsRouter) ensurePrivateNetwork() {
	r.privateMu.Lock()
	if r.privateStarted {
		r.privateMu.Unlock()
		return
	}
	r.privateStarted = true
	r.privateMu.Unlock()

	go r.watchPrivateNetwork()
}

func (r *windowsRouter) watchPrivateNetwork() {
	// NLA publishes a new adapter late, so we poll it like Tailscale.
	const tries = 20
	for i := range tries {
		if r.ctx.Err() != nil {
			return
		}
		found, err := setAdapterPrivate(r.luid)
		if err != nil {
			log.Debug(tag, "setPrivateNetwork(try=%d): %v", i, err)
		} else if found {
			if i > 0 {
				log.Debug(tag, "setPrivateNetwork(try=%d): success", i)
			} else {
				log.Debug(tag, "setPrivateNetwork: success")
			}
			return
		}
		if i == tries-1 {
			if err == nil {
				log.Debug(tag, "setPrivateNetwork: adapter LUID %v not found after %d tries", r.luid, tries)
			}
			return
		}
		timer := time.NewTimer(time.Second)
		select {
		case <-r.ctx.Done():
			timer.Stop()
			return
		case <-timer.C:
		}
	}
}

// setAdapterPrivate sets the adapter's NLA category to private.
// It returns (false, nil) when the adapter is not in NLA yet.
func setAdapterPrivate(luid winipcfg.LUID) (bool, error) {
	adapterID, err := luid.GUID()
	if err != nil {
		return false, fmt.Errorf("luid.GUID: %w", err)
	}

	// COM apartment membership is per OS thread. Pin this call so
	// CoInitializeEx and the vtable calls stay on the thread that joined.
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	hr := windows.CoInitializeEx(0, windows.COINIT_MULTITHREADED)
	if hr == nil {
		defer windows.CoUninitialize()
	}

	var nlm unsafe.Pointer
	r, _, _ := procCoCreateInstance.Call(
		uintptr(unsafe.Pointer(&clsidNetworkListManager)),
		0,
		uintptr(clsctxInprocServer),
		uintptr(unsafe.Pointer(&iidINetworkListManager)),
		uintptr(unsafe.Pointer(&nlm)),
	)
	if int32(r) < 0 || nlm == nil {
		return false, fmt.Errorf("CoCreateInstance(NLM): 0x%x", uint32(r))
	}
	defer comRelease(nlm)

	var enum unsafe.Pointer
	hrCall, _, _ := syscall.SyscallN(
		comSlot(nlm, nlmGetNetworkConnections),
		uintptr(nlm),
		uintptr(unsafe.Pointer(&enum)),
	)
	if int32(hrCall) < 0 || enum == nil {
		return false, fmt.Errorf("GetNetworkConnections: 0x%x", uint32(hrCall))
	}
	defer comRelease(enum)

	for {
		var conn unsafe.Pointer
		var fetched uint32
		// IEnumNetworkConnections.Next is slot 8 (IDispatch 7 + get__NewEnum).
		hrCall, _, _ = syscall.SyscallN(
			comSlot(enum, 8),
			uintptr(enum),
			1,
			uintptr(unsafe.Pointer(&conn)),
			uintptr(unsafe.Pointer(&fetched)),
		)
		if int32(hrCall) < 0 {
			comRelease(conn)
			return false, fmt.Errorf("IEnumNetworkConnections.Next: 0x%x", uint32(hrCall))
		}
		if fetched == 0 || conn == nil {
			return false, nil
		}
		found, err := setPrivateIfMatch(conn, *adapterID)
		comRelease(conn)
		if err != nil || found {
			return found, err
		}
	}
}

func setPrivateIfMatch(conn unsafe.Pointer, adapterID windows.GUID) (bool, error) {
	var id windows.GUID
	hr, _, _ := syscall.SyscallN(comSlot(conn, ncoGetAdapterId), uintptr(conn), uintptr(unsafe.Pointer(&id)))
	if int32(hr) < 0 {
		return false, fmt.Errorf("GetAdapterId: 0x%x", uint32(hr))
	}
	if id != adapterID {
		return false, nil
	}

	var net unsafe.Pointer
	hr, _, _ = syscall.SyscallN(comSlot(conn, ncoGetNetwork), uintptr(conn), uintptr(unsafe.Pointer(&net)))
	if int32(hr) < 0 || net == nil {
		return false, fmt.Errorf("GetNetwork: 0x%x", uint32(hr))
	}
	defer comRelease(net)

	var cat int32
	hr, _, _ = syscall.SyscallN(comSlot(net, netGetCategory), uintptr(net), uintptr(unsafe.Pointer(&cat)))
	if int32(hr) < 0 {
		return false, fmt.Errorf("GetCategory: 0x%x", uint32(hr))
	}
	if cat == categoryPrivate || cat == categoryDomain {
		return true, nil
	}

	hr, _, _ = syscall.SyscallN(comSlot(net, netSetCategory), uintptr(net), uintptr(categoryPrivate))
	if int32(hr) < 0 {
		return false, fmt.Errorf("SetCategory: 0x%x", uint32(hr))
	}
	return true, nil
}

func comSlot(obj unsafe.Pointer, slot int) uintptr {
	vtbl := *(*unsafe.Pointer)(obj)
	entry := unsafe.Add(vtbl, uintptr(slot)*unsafe.Sizeof(uintptr(0)))
	return *(*uintptr)(entry)
}

func comRelease(obj unsafe.Pointer) {
	if obj == nil {
		return
	}
	syscall.SyscallN(comSlot(obj, 2), uintptr(obj))
}
