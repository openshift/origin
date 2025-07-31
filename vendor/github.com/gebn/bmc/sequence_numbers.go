package bmc

// sequenceNumbers maintains a pair of sequence numbers for a session. In IPMI
// v1.5, there is one set for all packets. In IPMI v2.0, there is one set for
// authenticated packets, and another for unauthenticated packets. The first
// packet in a particular direction has sequence number 1 ("pre-increment"), so
// to get the next sequence number, increment the value, then read it. The
// sequence number must be incremented when retrying packets. "inbound" and
// "outbound" are relative to the BMC, to be consistent with the spec.
type sequenceNumbers struct {

	// Inbound is the sequence number of the last packet the remote console sent
	// to the managed system.
	Inbound uint32

	// Outbound is the sequence number of the last packet the managed system
	// sent to the remote console.
	Outbound uint32
}
