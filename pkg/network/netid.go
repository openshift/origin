package network

// Accessor methods to annotate NetNamespace for multitenant support
import "fmt"

const (
	// Maximum VXLAN Virtual Network Identifier(VNID) as per RFC#7348
	MaxVNID = uint32((1 << 24) - 1)
	// VNID: 1 to 9 are internally reserved for any special cases in the future
	MinVNID = uint32(10)
	// VNID: 0 reserved for default namespace and can reach any network in the cluster
	GlobalVNID = uint32(0)
)

// Check if the given vnid is valid or not
func ValidVNID(vnid uint32) error {
	if vnid == GlobalVNID {
		return nil
	}
	if vnid < MinVNID {
		return fmt.Errorf("VNID must be greater than or equal to %d", MinVNID)
	}
	if vnid > MaxVNID {
		return fmt.Errorf("VNID must be less than or equal to %d", MaxVNID)
	}
	return nil
}
