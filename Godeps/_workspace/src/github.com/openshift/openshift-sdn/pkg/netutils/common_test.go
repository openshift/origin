package netutils

import (
	"net"
	"testing"
)

func TestConversion(t *testing.T) {
	ip := net.ParseIP("10.1.2.3")
	if ip == nil {
		t.Fatal("Failed to parse IP")
	}

	u := IPToUint32(ip)
	t.Log(u)
	ip2 := Uint32ToIP(u)
	t.Log(ip2)

	if !ip2.Equal(ip) {
		t.Fatal("Conversion back and forth failed")
	}
}
