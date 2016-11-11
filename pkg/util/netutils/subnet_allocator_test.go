package netutils

import (
	"fmt"
	"net"
	"testing"
)

func TestAllocateSubnet(t *testing.T) {
	sna, err := NewSubnetAllocator("10.1.0.0/16", 8, nil)
	if err != nil {
		t.Fatal("Failed to initialize subnet allocator: ", err)
	}

	sn, err := sna.GetNetwork()
	if err != nil {
		t.Fatal("Failed to get network: ", err)
	}
	if sn.String() != "10.1.0.0/24" {
		t.Fatalf("Did not get expected subnet (n=%d, sn=%s)", 0, sn.String())
	}
	sn, err = sna.GetNetwork()
	if err != nil {
		t.Fatal("Failed to get network: ", err)
	}
	if sn.String() != "10.1.1.0/24" {
		t.Fatalf("Did not get expected subnet (n=%d, sn=%s)", 1, sn.String())
	}
	sn, err = sna.GetNetwork()
	if err != nil {
		t.Fatal("Failed to get network: ", err)
	}
	if sn.String() != "10.1.2.0/24" {
		t.Fatalf("Did not get expected subnet (n=%d, sn=%s)", 2, sn.String())
	}
}

// 10.1.SSSSSSHH.HHHHHHHH
func TestAllocateSubnetLargeHostBits(t *testing.T) {
	sna, err := NewSubnetAllocator("10.1.0.0/16", 10, nil)
	if err != nil {
		t.Fatal("Failed to initialize subnet allocator: ", err)
	}

	sn, err := sna.GetNetwork()
	if err != nil {
		t.Fatal("Failed to get network: ", err)
	}
	if sn.String() != "10.1.0.0/22" {
		t.Fatalf("Did not get expected subnet (n=%d, sn=%s)", 0, sn.String())
	}
	sn, err = sna.GetNetwork()
	if err != nil {
		t.Fatal("Failed to get network: ", err)
	}
	if sn.String() != "10.1.4.0/22" {
		t.Fatalf("Did not get expected subnet (n=%d, sn=%s)", 1, sn.String())
	}
	sn, err = sna.GetNetwork()
	if err != nil {
		t.Fatal("Failed to get network: ", err)
	}
	if sn.String() != "10.1.8.0/22" {
		t.Fatalf("Did not get expected subnet (n=%d, sn=%s)", 2, sn.String())
	}
	sn, err = sna.GetNetwork()
	if err != nil {
		t.Fatal("Failed to get network: ", err)
	}
	if sn.String() != "10.1.12.0/22" {
		t.Fatalf("Did not get expected subnet (n=%d, sn=%s)", 3, sn.String())
	}
}

// 10.1.SSSSSSSS.SSHHHHHH
func TestAllocateSubnetLargeSubnetBits(t *testing.T) {
	sna, err := NewSubnetAllocator("10.1.0.0/16", 6, nil)
	if err != nil {
		t.Fatal("Failed to initialize subnet allocator: ", err)
	}

	for n := 0; n < 256; n++ {
		sn, err := sna.GetNetwork()
		if err != nil {
			t.Fatal("Failed to get network: ", err)
		}
		if sn.String() != fmt.Sprintf("10.1.%d.0/26", n) {
			t.Fatalf("Did not get expected subnet (n=%d, sn=%s)", n, sn.String())
		}
	}

	for n := 0; n < 256; n++ {
		sn, err := sna.GetNetwork()
		if err != nil {
			t.Fatal("Failed to get network: ", err)
		}
		if sn.String() != fmt.Sprintf("10.1.%d.64/26", n) {
			t.Fatalf("Did not get expected subnet (n=%d, sn=%s)", n+256, sn.String())
		}
	}

	sn, err := sna.GetNetwork()
	if err != nil {
		t.Fatal("Failed to get network: ", err)
	}
	if sn.String() != "10.1.0.128/26" {
		t.Fatalf("Did not get expected subnet (n=%d, sn=%s)", 512, sn.String())
	}
}

// 10.000000SS.SSSSSSHH.HHHHHHHH
func TestAllocateSubnetOverlapping(t *testing.T) {
	sna, err := NewSubnetAllocator("10.0.0.0/14", 10, nil)
	if err != nil {
		t.Fatal("Failed to initialize subnet allocator: ", err)
	}

	for n := 0; n < 4; n++ {
		sn, err := sna.GetNetwork()
		if err != nil {
			t.Fatal("Failed to get network: ", err)
		}
		if sn.String() != fmt.Sprintf("10.%d.0.0/22", n) {
			t.Fatalf("Did not get expected subnet (n=%d, sn=%s)", n, sn.String())
		}
	}

	for n := 0; n < 4; n++ {
		sn, err := sna.GetNetwork()
		if err != nil {
			t.Fatal("Failed to get network: ", err)
		}
		if sn.String() != fmt.Sprintf("10.%d.4.0/22", n) {
			t.Fatalf("Did not get expected subnet (n=%d, sn=%s)", n+4, sn.String())
		}
	}

	sn, err := sna.GetNetwork()
	if err != nil {
		t.Fatal("Failed to get network: ", err)
	}
	if sn.String() != "10.0.8.0/22" {
		t.Fatalf("Did not get expected subnet (n=%d, sn=%s)", 8, sn.String())
	}
}

// 10.1.HHHHHHHH.HHHHHHHH
func TestAllocateSubnetNoSubnetBits(t *testing.T) {
	sna, err := NewSubnetAllocator("10.1.0.0/16", 16, nil)
	if err != nil {
		t.Fatal("Failed to initialize subnet allocator: ", err)
	}

	sn, err := sna.GetNetwork()
	if err != nil {
		t.Fatal("Failed to get network: ", err)
	}
	if sn.String() != "10.1.0.0/16" {
		t.Fatalf("Did not get expected subnet (sn=%s)", sn.String())
	}

	sn, err = sna.GetNetwork()
	if err == nil {
		t.Fatalf("Unexpectedly succeeded in getting network (sn=%s)", sn.String())
	}
}

func TestAllocateSubnetInvalidHostBitsOrCIDR(t *testing.T) {
	_, err := NewSubnetAllocator("10.1.0.0/16", 18, nil)
	if err == nil {
		t.Fatal("Unexpectedly succeeded in initializing subnet allocator")
	}

	_, err = NewSubnetAllocator("10.1.0.0/16", 0, nil)
	if err == nil {
		t.Fatal("Unexpectedly succeeded in initializing subnet allocator")
	}

	_, err = NewSubnetAllocator("10.1.0.0/33", 16, nil)
	if err == nil {
		t.Fatal("Unexpectedly succeeded in initializing subnet allocator")
	}
}

func TestAllocateSubnetInUse(t *testing.T) {
	inUse := []string{"10.1.0.0/24", "10.1.2.0/24", "10.2.2.2/24", "Invalid"}
	sna, err := NewSubnetAllocator("10.1.0.0/16", 8, inUse)
	if err != nil {
		t.Fatal("Failed to initialize IP allocator: ", err)
	}

	sn, err := sna.GetNetwork()
	if err != nil {
		t.Fatal("Failed to get network: ", err)
	}
	if sn.String() != "10.1.1.0/24" {
		t.Fatalf("Did not get expected subnet (sn=%s)", sn.String())
	}
	sn, err = sna.GetNetwork()
	if err != nil {
		t.Fatal("Failed to get network: ", err)
	}
	if sn.String() != "10.1.3.0/24" {
		t.Fatalf("Did not get expected subnet (sn=%s)", sn.String())
	}
}

func TestAllocateReleaseSubnet(t *testing.T) {
	sna, err := NewSubnetAllocator("10.1.0.0/16", 14, nil)
	if err != nil {
		t.Fatal("Failed to initialize IP allocator: ", err)
	}

	var releaseSn *net.IPNet

	for i := 0; i < 4; i++ {
		sn, err := sna.GetNetwork()
		if err != nil {
			t.Fatal("Failed to get network: ", err)
		}
		if sn.String() != fmt.Sprintf("10.1.%d.0/18", i*64) {
			t.Fatalf("Did not get expected subnet (i=%d, sn=%s)", i, sn.String())
		}
		if i == 2 {
			releaseSn = sn
		}
	}

	sn, err := sna.GetNetwork()
	if err == nil {
		t.Fatalf("Unexpectedly succeeded in getting network (sn=%s)", sn.String())
	}

	if err := sna.ReleaseNetwork(releaseSn); err != nil {
		t.Fatalf("Failed to release the subnet (releaseSn=%s): %v", releaseSn.String(), err)
	}

	sn, err = sna.GetNetwork()
	if err != nil {
		t.Fatal("Failed to get network: ", err)
	}
	if sn.String() != releaseSn.String() {
		t.Fatalf("Did not get expected subnet (sn=%s)", sn.String())
	}

	sn, err = sna.GetNetwork()
	if err == nil {
		t.Fatalf("Unexpectedly succeeded in getting network (sn=%s)", sn.String())
	}
}
