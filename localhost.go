package main

import (
	"fmt"
	"net"
)

// findAvailableLocalhostIP finds the next available IP address in the 127.0.0.0/8 range.
// It attempts to bind to port 53 (DNS port) on UDP to check availability.
func findAvailableLocalhostIP(lastIP net.IP) (net.IP, error) {
	for {
		// Increment IP
		lastIP = net.ParseIP(incrementIP(lastIP.String()))

		// Check if IP is in 127.0.0.0/8 range
		if lastIP[12] != 127 {
			return nil, fmt.Errorf("no available IPs found in the 127.0.0.0/8 range")
		}

		// Check if port 53 is available on this IP
		if isPortAvailable(lastIP.String(), 53) {
			return lastIP, nil
		}
	}
}

// incrementIP increments an IP address.
func incrementIP(ipStr string) string {
	ip := net.ParseIP(ipStr)
	for j := len(ip) - 1; j >= 0; j-- {
		ip[j]++
		if ip[j] > 0 {
			break
		}
	}
	return ip.String()
}

// Check if a specific IP and port combination is available
func isPortAvailable(host string, port int) bool {
	addr := fmt.Sprintf("%s:%d", host, port)
	ln, err := net.ListenPacket("udp", addr)
	if err != nil {
		return false
	}
	ln.Close()
	return true
}
