package netutils

import (
	"testing"
)

func TestAllocateIP(t *testing.T) {
	ipa, err := NewIPAllocator("10.1.2.0/24", nil)
	if err != nil {
		t.Fatal("Failed to initialize IP allocator: %v", err)
	}

	ip, err := ipa.GetIP()
	if err != nil {
		t.Fatal("Failed to get IP: ", err)
	}
	if ip.String() != "10.1.2.1/24" {
		t.Fatal("Did not get expected IP")
	}
	ip, err = ipa.GetIP()
	if err != nil {
		t.Fatal("Failed to get IP: ", err)
	}
	if ip.String() != "10.1.2.2/24" {
		t.Fatal("Did not get expected IP")
	}
	ip, err = ipa.GetIP()
	if err != nil {
		t.Fatal("Failed to get IP: ", err)
	}
	if ip.String() != "10.1.2.3/24" {
		t.Fatal("Did not get expected IP")
	}
}

func TestAllocateIPInUse(t *testing.T) {
	inUse := []string{"10.1.2.1/24", "10.1.2.2/24", "10.2.2.3/24", "Invalid"}
	ipa, err := NewIPAllocator("10.1.2.0/24", inUse)
	if err != nil {
		t.Fatal("Failed to initialize IP allocator: %v", err)
	}

	ip, err := ipa.GetIP()
	if err != nil {
		t.Fatal("Failed to get IP: ", err)
	}
	if ip.String() != "10.1.2.3/24" {
		t.Fatal("Did not get expected IP", ip)
	}
	ip, err = ipa.GetIP()
	if err != nil {
		t.Fatal("Failed to get IP: ", err)
	}
	if ip.String() != "10.1.2.4/24" {
		t.Fatal("Did not get expected IP", ip)
	}
}

func TestAllocateReleaseIP(t *testing.T) {
	ipa, err := NewIPAllocator("10.1.2.0/24", nil)
	if err != nil {
		t.Fatal("Failed to initialize IP allocator: %v", err)
	}

	ip, err := ipa.GetIP()
	if err != nil {
		t.Fatal("Failed to get IP: ", err)
	}
	if ip.String() != "10.1.2.1/24" {
		t.Fatal("Did not get expected IP")
	}

	if err := ipa.ReleaseIP(ip); err != nil {
		t.Fatal("Failed to release the IP")
	}
	ip, err = ipa.GetIP()
	if err != nil {
		t.Fatal("Failed to get IP: ", err)
	}
	if ip.String() != "10.1.2.1/24" {
		t.Fatal("Did not get expected IP")
	}
}
