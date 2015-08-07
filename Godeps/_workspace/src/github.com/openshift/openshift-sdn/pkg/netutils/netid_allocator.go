package netutils

import (
	"fmt"
)

type NetIDAllocator struct {
	min      uint
	max      uint
	allocMap map[uint]bool
}

func NewNetIDAllocator(min uint, max uint, inUse []uint) (*NetIDAllocator, error) {
	if max <= min {
		return nil, fmt.Errorf("Min should be lesser than max value (Min: %d, Max: %d)", min, max)
	}

	amap := make(map[uint]bool)
	for _, netid := range inUse {
		if netid < min || netid > max {
			fmt.Println("Provided net id doesn't belong to range: ", min, max)
			continue
		}
		amap[netid] = true
	}

	return &NetIDAllocator{min: min, max: max, allocMap: amap}, nil
}

func (nia *NetIDAllocator) GetNetID() (uint, error) {
	var i uint
	// We exclude the last address as it is reserved for broadcast
	for i = nia.min; i <= nia.max; i++ {
		taken, found := nia.allocMap[i]
		if !found || !taken {
			nia.allocMap[i] = true
			return i, nil
		}
	}

	return 0, fmt.Errorf("No NetIDs available.")
}

func (nia *NetIDAllocator) ReleaseNetID(netid uint) error {
	if nia.min > netid || nia.max < netid {
		return fmt.Errorf("Provided net id %v doesn't belong to the given range (%v-%v)", netid, nia.min, nia.max)
	}

	taken, found := nia.allocMap[netid]
	if !found || !taken {
		return fmt.Errorf("Provided net id %v is already available.", netid)
	}

	nia.allocMap[netid] = false
	return nil
}
