//go:build android
// +build android

// gateway_android.go - Android-specific gateway discovery via UDP fallback
//
// On Android /proc/net/route is inaccessible (permission denied or redacted).
// We fall back to opening a dummy UDP socket to a public address and
// reading our local source IP, which gives the default‐route interface.

package gateway

import (
    "fmt"
    "net"
)

// discoverViaUDP opens a dummy UDP connection to infer the
// local interface IP used by Android’s default gateway.
func discoverViaUDP() (net.IP, error) {
    // We choose Google DNS here; any routeable address works.
    conn, err := net.Dial("udp", "8.8.8.8:80")
    if err != nil {
        return nil, fmt.Errorf("android gateway discovery via UDP failed: %w", err)
    }
    defer conn.Close()

    udpAddr, ok := conn.LocalAddr().(*net.UDPAddr)
    if !ok || udpAddr.IP == nil {
        return nil, fmt.Errorf("invalid local UDP address: %v", conn.LocalAddr())
    }
    return udpAddr.IP, nil
}

// discoverGatewayOSSpecific implements DiscoverGateway on Android.
// Instead of probing /proc/net/route (unavailable), it returns
// the UDP-derived IP––which Syncthing-Android treats as the gateway IP.
func discoverGatewayOSSpecific() (net.IP, error) {
    return discoverViaUDP()
}

// discoverGatewayInterfaceOSSpecific implements DiscoverInterface on Android.
// It likewise returns the UDP-derived IP of the default‐route interface.
func discoverGatewayInterfaceOSSpecific() (net.IP, error) {
    return discoverViaUDP()
}
