package util

import (
	"sync"

	"github.com/amnezia-vpn/amneziawg-go/v3/device"
)

// DeviceRole says how resume handling treats a live device.
type DeviceRole int

const (
	// RoleDirect sends over its own OS UDP sockets.
	RoleDirect DeviceRole = iota
	// RoleOuter is a multihop outer device. Its sockets carry the inner device's traffic.
	RoleOuter
	// RoleInner sends through an outer device's in-process netstack, so it has no OS
	// sockets for sleep to break.
	RoleInner
)

type LiveDevice struct {
	Dev  *device.Device
	Role DeviceRole
}

var (
	liveMu      sync.Mutex
	liveDevices = map[*device.Device]DeviceRole{}
)

// RegisterDevice tracks a device that is up.
func RegisterDevice(dev *device.Device, role DeviceRole) {
	liveMu.Lock()
	liveDevices[dev] = role
	liveMu.Unlock()
}

// UnregisterDevice must run before the device is closed.
func UnregisterDevice(dev *device.Device) {
	liveMu.Lock()
	delete(liveDevices, dev)
	liveMu.Unlock()
}

// LiveDevices returns the devices that are up, outer devices first so an inner
// handshake does not wait behind a stale outer session.
func LiveDevices() []LiveDevice {
	liveMu.Lock()
	defer liveMu.Unlock()
	devs := make([]LiveDevice, 0, len(liveDevices))
	for dev, role := range liveDevices {
		if role == RoleOuter {
			devs = append(devs, LiveDevice{Dev: dev, Role: role})
		}
	}
	for dev, role := range liveDevices {
		if role != RoleOuter {
			devs = append(devs, LiveDevice{Dev: dev, Role: role})
		}
	}
	return devs
}
