package util

// This is copied from go/src/internal/bytealg, which includes versions
// optimized for various platforms.  Those optimizations are elided here so we
// don't have to maintain them.
func IndexByteString(s string, c byte) int {
	for i := 0; i < len(s); i++ {
		if s[i] == c {
			return i
		}
	}
	return -1
}

// IPUrl safely converts a bare IPv4 or IPv6 into URL form with brackets
//
// This is copied from net.JoinHostPort, but without the port
// Use  net.JoinHostPort if you have host and port.
func IPUrl(host string) string {
	// We assume that host is a literal IPv6 address if host has
	// colons.
	if IndexByteString(host, ':') >= 0 {
		return "[" + host + "]"
	}
	return host
}
