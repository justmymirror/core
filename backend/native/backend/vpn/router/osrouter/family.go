// Copyright (c) 2026 Zane Schepke / WG Tunnel
//
// Portions of this file are derived from Tailscale
// Copyright (c) Tailscale Inc & contributors
// Licensed under the BSD 3-Clause License
// See NOTICE and LICENSES/BSD-3-Clause.txt in the project root

package osrouter

import "github.com/wgtunnel/backend/vpn/router"

// familyCaptures reports whether this config has a default route for the family.
func familyCaptures(c *router.Config, v4 bool) bool {
	if c == nil {
		return false
	}
	for _, rt := range c.Routes {
		if rt.Bits() != 0 {
			continue
		}
		if (v4 && rt.Addr().Is4()) || (!v4 && rt.Addr().Is6()) {
			return true
		}
	}
	return false
}

func hasFamilyAddress(c *router.Config, v4 bool) bool {
	if c == nil {
		return false
	}
	for _, a := range c.TunnelAddrs {
		if (v4 && a.Addr().Is4()) || (!v4 && a.Addr().Is6()) {
			return true
		}
	}
	return false
}

func familyUsable(c *router.Config, v4 bool) bool {
	if c == nil {
		return false
	}
	if v4 {
		return c.HasDefaultRouteV4()
	}
	return c.HasDefaultRouteV6()
}

// familyIsolated reports whether unmarked traffic for this family must stay
// off the physical NIC.
func familyIsolated(c *router.Config, v4 bool) bool {
	if c == nil {
		return false
	}
	if familyUsable(c, v4) || familyCaptures(c, v4) {
		return true
	}
	return familyUsable(c, !v4) && !hasFamilyAddress(c, v4)
}

// familyUnsupported is an isolated family the tun cannot carry.
func familyUnsupported(c *router.Config, v4 bool) bool {
	return familyIsolated(c, v4) && !familyUsable(c, v4)
}
