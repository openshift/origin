package vnid

import (
	"fmt"
	"strconv"
	"strings"
)

const (
	// Maximum VXLAN Virtual Network Identifier(VNID) as per RFC#7348
	MaxVNID = uint((1 << 24) - 1)
	// VNID: 1 to 9 are internally reserved for any special cases in the future
	MinVNID = 10
	// VNID: 0 reserved for default namespace and can reach any network in the cluster
	GlobalVNID = uint(0)
)

type VNIDRange struct {
	Base uint
	Size uint
}

func ValidVNID(vnid uint) error {
	if vnid == GlobalVNID {
		return nil
	}
	if vnid < MinVNID {
		return fmt.Errorf("must be greater than or equal to %d", MinVNID)
	}
	if vnid > MaxVNID {
		return fmt.Errorf("must be less than or equal to %d", MaxVNID)
	}
	return nil
}

// Contains tests whether a given vnid falls within the Range.
func (r *VNIDRange) Contains(vnid uint) bool {
	return (vnid >= r.Base) && ((vnid - r.Base) < r.Size)
}

func (r *VNIDRange) String() string {
	if r.Size == 0 {
		return ""
	}
	return fmt.Sprintf("%d-%d", r.Base, r.Base+r.Size-1)
}

func (r *VNIDRange) Set(base, size uint) error {
	if base < MinVNID {
		return fmt.Errorf("invalid vnid base, must be greater than %d", MinVNID)
	}
	if size == 0 {
		return fmt.Errorf("invalid vnid size, must be greater than zero")
	}
	if (base + size - 1) > MaxVNID {
		return fmt.Errorf("vnid range exceeded max value %d", MaxVNID)
	}

	r.Base = base
	r.Size = size
	return nil
}

func NewVNIDRange(base, size uint) (*VNIDRange, error) {
	r := &VNIDRange{}
	err := r.Set(base, size)
	if err != nil {
		return nil, err
	}
	return r, nil
}

// Parse range string of the form "min-max", inclusive at both ends
// and returns VNIDRange object.
func ParseVNIDRange(value string) (*VNIDRange, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil, fmt.Errorf("invalid range string")
	}

	hyphenIndex := strings.Index(value, "-")
	if hyphenIndex == -1 {
		return nil, fmt.Errorf("expected hyphen in port range")
	}

	var err error
	var low, high int
	low, err = strconv.Atoi(value[:hyphenIndex])
	if err == nil {
		high, err = strconv.Atoi(value[hyphenIndex+1:])
	}
	if err != nil {
		return nil, fmt.Errorf("unable to parse vnid range: %s", value)
	}
	return NewVNIDRange(uint(low), uint(high-low+1))
}
