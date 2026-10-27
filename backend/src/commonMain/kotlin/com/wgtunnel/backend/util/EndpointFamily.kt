package com.wgtunnel.backend.util

import com.wgtunnel.backend.Tunnel
import com.wgtunnel.backend.enums.FamilyOverride
import com.wgtunnel.backend.enums.Ipv6Support

/** Family for the first bootstrap, before any peer has a current endpoint. */
internal fun initialBootstrapFamily(
    ipStrategy: Tunnel.IpStrategy,
    networkIpv6: Ipv6Support,
): FamilyOverride =
    if (ipStrategy is Tunnel.IpStrategy.PreferIpv6 && networkIpv6 == Ipv6Support.Available) {
        FamilyOverride.PreferIpv6
    } else {
        FamilyOverride.MatchCurrent
    }

/**
 * Family for DDNS recovery. Force v4 only when IPv6 peers sit on a network known to lack IPv6, an
 * unknown reading keeps them on their current family.
 */
internal fun ddnsRecoveryFamily(networkIpv6: Ipv6Support, hasIpv6Peers: Boolean): FamilyOverride =
    if (networkIpv6 == Ipv6Support.Unavailable && hasIpv6Peers) {
        FamilyOverride.ForceIpv4
    } else {
        FamilyOverride.MatchCurrent
    }

/** Family for a bounce with a fresh DNS request. Ipv4Only tunnels are pulled back to v4. */
internal fun freshBounceFamily(
    ipStrategy: Tunnel.IpStrategy,
    networkIpv6: Ipv6Support,
    hasIpv6Peers: Boolean,
): FamilyOverride =
    if (networkIpv6 == Ipv6Support.Unavailable && hasIpv6Peers) {
        FamilyOverride.ForceIpv4
    } else if (ipStrategy is Tunnel.IpStrategy.Ipv4Only) {
        FamilyOverride.ForceIpv4
    } else {
        FamilyOverride.MatchCurrent
    }
