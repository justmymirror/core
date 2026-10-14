// Copyright (c) 2026 Zane Schepke / WG Tunnel

package osfirewall

import (
	"slices"
	"strings"
)

// vmIface is one host interface considered for the macOS VM pass.
// A bridge carries the member names reported by ifconfig. An empty member
// list is treated as unknown and is not passed.
type vmIface struct {
	name    string
	bridge  bool
	members []string
}

// vmPassIfaces returns the virtual NICs a NAT guest arrives on.
func vmPassIfaces(ifaces []vmIface) []string {
	var out []string
	for _, ifc := range ifaces {
		if !safeIface(ifc.name) {
			continue
		}
		if ifc.bridge || strings.HasPrefix(strings.ToLower(ifc.name), "bridge") {
			if bridgeIsVirtualOnly(ifc.members) {
				out = append(out, ifc.name)
			}
			continue
		}
		if isDirectVMNIC(ifc.name) {
			out = append(out, ifc.name)
		}
	}
	slices.Sort(out)
	return out
}

func isDirectVMNIC(name string) bool {
	n := strings.ToLower(name)
	switch {
	case strings.HasPrefix(n, "vmnet"),
		strings.HasPrefix(n, "vmenet"),
		strings.HasPrefix(n, "vnic"),
		strings.HasPrefix(n, "vboxnet"),
		strings.HasPrefix(n, "feth"):
		return true
	default:
		return false
	}
}

func isPhysicalBridgeMember(name string) bool {
	n := strings.ToLower(name)
	return strings.HasPrefix(n, "en") ||
		strings.HasPrefix(n, "awdl") ||
		strings.HasPrefix(n, "llw") ||
		strings.HasPrefix(n, "ap") ||
		strings.HasPrefix(n, "p2p")
}

func bridgeIsVirtualOnly(members []string) bool {
	if len(members) == 0 {
		return false
	}
	for _, m := range members {
		if !safeIface(m) || isPhysicalBridgeMember(m) || strings.HasPrefix(strings.ToLower(m), "bridge") {
			return false
		}
	}
	return true
}

func parseBridgeMembers(out string) []string {
	var members []string
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "member:") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 2 || !safeIface(fields[1]) {
			continue
		}
		members = append(members, fields[1])
	}
	return members
}

func safeIface(name string) bool {
	if name == "" {
		return false
	}
	for _, c := range name {
		if !(c == '.' || c == '-' || c == '_' ||
			(c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9')) {
			return false
		}
	}
	return true
}
