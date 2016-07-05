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
			return nil, fmt.Errorf("Provided net id doesn't belong to range: [%d, %d]", min, max)
		}
		amap[netid] = true
	}

	return &NetIDAllocator{min: min, max: max, allocMap: amap}, nil
}

func (nia *NetIDAllocator) GetNetID() (uint, error) {
	var i uint
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
		return fmt.Errorf("Provided net id %d doesn't belong to the given range [%d, %d]", netid, nia.min, nia.max)
	}

	taken, found := nia.allocMap[netid]
	if !found || !taken {
		return fmt.Errorf("Provided net id %d is already available.", netid)
	}

	nia.allocMap[netid] = false
	return nil
}
