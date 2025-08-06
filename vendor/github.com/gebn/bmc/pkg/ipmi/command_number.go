package ipmi

import (
	"fmt"
)

// CommandNumber represents a particular function that can be requested.
// CommandNumber identifiers are only unique within a given network function.
// See Appendix G in the v1.5 and v2.0 specs for assignments. This is a 1 byte
// uint on the wire. It has the "Number" suffix in order to avoid colliding with
// the Command interface, which is much more likely to be interacted with by
// users, so wins the shorter name, even if this is called Command in the spec.
type CommandNumber uint8

func (c CommandNumber) String() string {
	// cannot do much better than this without the context of the NetFn; we use
	// hex as this is what can be found in the spec
	return fmt.Sprintf("%#x", uint8(c))
}
