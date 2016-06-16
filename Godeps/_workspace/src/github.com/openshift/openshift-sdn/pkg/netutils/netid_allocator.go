package netutils

import (
	"fmt"
)

type NetIDAllocator struct {
	min      uint32
	max      uint32
	allocMap map[uint32]bool
}

func NewNetIDAllocator(min uint32, max uint32, inUse []uint32) (*NetIDAllocator, error) {
	if max <= min {
		return nil, fmt.Errorf("Min should be lesser than max value (Min: %d, Max: %d)", min, max)
	}

	amap := make(map[uint32]bool)
	for _, netid := range inUse {
		if netid < min || netid > max {
			return nil, fmt.Errorf("Provided net id doesn't belong to range: [%d, %d]", min, max)
		}
		amap[netid] = true
	}

	return &NetIDAllocator{min: min, max: max, allocMap: amap}, nil
}

func (nia *NetIDAllocator) GetNetID() (uint32, error) {
	var i uint32
	for i = nia.min; i <= nia.max; i++ {
		taken, found := nia.allocMap[i]
		if !found || !taken {
			nia.allocMap[i] = true
			return i, nil
		}
	}

	return 0, fmt.Errorf("No NetIDs available.")
}

func (nia *NetIDAllocator) ReleaseNetID(netid uint32) error {
	if nia.min > netid || nia.max < netid {
		return fmt.Errorf("Provided net id %d doesn't belong to the given range [%d, %d]", netid, nia.min, nia.max)
	}

	taken, found := nia.allocMap[netid]
	if !found || !taken {
		return fmt.Errorf("Provided net id %d is already available.", netid)
	}

	nia.allocMap[netid] = false
	return nil
}
