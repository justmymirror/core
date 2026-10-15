package network

import "sync"

// maxReportedSSIDLen rejects an oversized IPC payload. A real 802.11 SSID is
// at most 32 octets so the extra room is for a UTF-8 decoding of those octets.
const maxReportedSSIDLen = 128

// ObservedWifi is a Wi-Fi name reported by the LaunchAgent, bound
// to the underlay it was read on. An empty SSID is an explicit clear for that
// underlay. A later gateway or interface change must not keep the name.
type ObservedWifi struct {
	SSID    string
	IfName  string
	Gateway string
}

var (
	observedMu   sync.Mutex
	observedWifi *ObservedWifi
)

func setObservedWifi(v *ObservedWifi) {
	observedMu.Lock()
	observedWifi = v
	observedMu.Unlock()
}

func getObservedWifi() *ObservedWifi {
	observedMu.Lock()
	defer observedMu.Unlock()
	if observedWifi == nil {
		return nil
	}
	cp := *observedWifi
	return &cp
}

// ResolveWifiSSID chooses the name for a Wi-Fi underlay.
//
// A report that matches this interface and gateway wins, including an empty
// report that clears the name. A report for a different underlay is ignored
// and the previous name is not reused, so joining another network cannot keep
// the last one. With no report at all, a known prevSSID is the cache. The
// daemon does not read a file for the name. The LaunchAgent is the only
// source of a new one.
func ResolveWifiSSID(ifName, gateway, prevSSID string, observed *ObservedWifi) string {
	if observed != nil && observed.IfName == ifName && observed.Gateway == gateway {
		if observed.SSID != "" {
			return observed.SSID
		}
		return UnknownSSID
	}
	if observed != nil {
		return UnknownSSID
	}
	if prevSSID != "" && prevSSID != UnknownSSID {
		return prevSSID
	}
	return UnknownSSID
}
