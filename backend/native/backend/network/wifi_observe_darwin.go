//go:build darwin && cgo

package network

import (
	"strings"

	"github.com/wgtunnel/backend/log"
)

// applyObservedWifiReport stores a name read by the user-session LaunchAgent and
// refreshes the monitor. The name is kept only while the underlay interface and gateway still match
// the ones captured here.
func applyObservedWifiReport(ssid string) {
	ssid = strings.TrimSpace(ssid)
	if len(ssid) > maxReportedSSIDLen {
		return
	}
	d := LookupPhysicalDefault4()
	if d.IfName == "" {
		d = LookupPhysicalDefault6()
	}
	if ssid == "" || ssid == UnknownSSID {
		if d.IfName == "" {
			setObservedWifi(nil)
		} else {
			setObservedWifi(&ObservedWifi{IfName: d.IfName, Gateway: d.Gateway})
		}
		log.Debug(tag, "observed wifi name cleared if=%s", d.IfName)
	} else if d.IfName == "" {
		setObservedWifi(nil)
		log.Debug(tag, "observed wifi name dropped, no underlay")
	} else {
		setObservedWifi(&ObservedWifi{SSID: ssid, IfName: d.IfName, Gateway: d.Gateway})
		log.Debug(tag, "observed wifi name if=%s", d.IfName)
	}

	monitorMu.Lock()
	m := monitor
	monitorMu.Unlock()
	if dm, ok := m.(*darwinMonitor); ok {
		dm.refresh()
	}
}
