package util

import (
	"strings"

	"github.com/amnezia-vpn/amneziawg-go/v3/device"
	wireproxyawg "github.com/artem-russkikh/wireproxy-awg"
	"github.com/wgtunnel/backend/log"
)

// KickHandshakes sends one keepalive to each peer once the device is up. Without a
// session the keepalive starts a handshake, so health is known right away instead of
// waiting for traffic.
func KickHandshakes(dev *device.Device, peers []wireproxyawg.PeerConfig, tag string) {
	for _, p := range peers {
		// No endpoint means the remote side dials in. Sending would fail and be
		// reported as a handshake failure.
		if p.Endpoint == nil || *p.Endpoint == "" {
			continue
		}
		var pk device.NoisePublicKey
		if err := pk.FromHex(p.PublicKey); err != nil {
			log.Error(tag, "handshake kick: invalid peer key: %v", err)
			continue
		}
		if peer := dev.LookupPeer(pk); peer != nil {
			peer.SendKeepalive()
		}
	}
}

// ForceHandshakes starts a new handshake with every peer that has an endpoint, even
// one with a session that still looks valid. A keepalive on a session the server has
// dropped is never answered and never leads to a rekey. The current keys stay in use
// until the new handshake completes. Peers are read from the device, so endpoints
// changed by recovery are used.
func ForceHandshakes(dev *device.Device, tag string) {
	uapi, err := dev.IpcGet()
	if err != nil {
		log.Error(tag, "force handshake: read device: %v", err)
		return
	}

	var key string
	var hasEndpoint bool
	force := func() {
		if key == "" || !hasEndpoint {
			return
		}
		var pk device.NoisePublicKey
		if err := pk.FromHex(key); err != nil {
			log.Error(tag, "force handshake: invalid peer key: %v", err)
			return
		}
		if peer := dev.LookupPeer(pk); peer != nil {
			_ = peer.SendHandshakeInitiation(false)
		}
	}

	for _, line := range strings.Split(uapi, "\n") {
		k, v, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		switch k {
		case "public_key":
			force()
			key, hasEndpoint = v, false
		case "endpoint":
			hasEndpoint = v != ""
		}
	}
	force()
}
