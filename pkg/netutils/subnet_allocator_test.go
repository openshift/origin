package netutils

import (
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
		t.Fatal("Did not get expected subnet")
	}
	sn, err = sna.GetNetwork()
	if err != nil {
		t.Fatal("Failed to get network: ", err)
	}
	if sn.String() != "10.1.1.0/24" {
		t.Fatal("Did not get expected subnet")
	}
	sn, err = sna.GetNetwork()
	if err != nil {
		t.Fatal("Failed to get network: ", err)
	}
	if sn.String() != "10.1.2.0/24" {
		t.Fatal("Did not get expected subnet")
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
		t.Fatal("Did not get expected subnet")
	}
	sn, err = sna.GetNetwork()
	if err != nil {
		t.Fatal("Failed to get network: ", err)
	}
	if sn.String() != "10.1.3.0/24" {
		t.Fatal("Did not get expected subnet")
	}
}

func TestAllocateReleaseSubnet(t *testing.T) {
	sna, err := NewSubnetAllocator("10.1.0.0/16", 8, nil)
	if err != nil {
		t.Fatal("Failed to initialize IP allocator: ", err)
	}

	sn, err := sna.GetNetwork()
	if err != nil {
		t.Fatal("Failed to get network: ", err)
	}
	if sn.String() != "10.1.0.0/24" {
		t.Fatal("Did not get expected subnet")
	}

	if err := sna.ReleaseNetwork(sn); err != nil {
		t.Fatal("Failed to release the subnet")
	}

	sn, err = sna.GetNetwork()
	if err != nil {
		t.Fatal("Failed to get network: ", err)
	}
	if sn.String() != "10.1.0.0/24" {
		t.Fatal("Did not get expected subnet")
	}
}

func TestGenerateGateway(t *testing.T) {
	sna, err := NewSubnetAllocator("10.1.0.0/16", 8, nil)
	if err != nil {
		t.Fatal("Failed to initialize IP allocator: ", err)
	}

	sn, err := sna.GetNetwork()
	if err != nil {
		t.Fatal("Failed to get network: ", err)
	}
	if sn.String() != "10.1.0.0/24" {
		t.Fatal("Did not get expected subnet")
	}

	gatewayIP := GenerateDefaultGateway(sn)
	t.Log(gatewayIP)
	if gatewayIP.String() != "10.1.0.1" {
		t.Fatal("Did not get expected gateway IP Address")
	}
}
