package ipmi

import (
	"fmt"
)

// Channel specifies a channel number, which corresponds to an interface on the
// BMC. It can be thought of as a little like a port number, where each number
// supports a different media connection type (IPMB, LAN, serial etc.). Channels
// numbers are specified in section 6.3 of both IPMI v1.5 and IPMI v2.0; on the
// wire, they are a 4-bit uint. Channels' protocol and mediums can be discovered
// with the Get Channel Info command. Sessions allow multiplexing on a
// (session-based) channel.
type Channel uint8

// Valid returns whether a given channel number is valid, which is in the range
// 0 through 0xf. Values outside this range should be regarded as an indication
// of lack of support.
func (c Channel) Valid() bool {
	return c <= 0xf
}

func (c Channel) String() string {
	return fmt.Sprintf("%#x(%v)", uint8(c), c.name())
}

func (c Channel) name() string {
	switch {
	case c == ChannelPrimaryIPMB:
		return "Primary IPMB"
	case 0x1 <= c && c <= 0xB: // 0x7 for IPMI v1.5
		return "Implementation-specific"
	case c == ChannelPresentInterface:
		return "Present I/F"
	case c == ChannelSystemInterface:
		return "System Interface"
	default:
		return "Unknown"
	}
}

const (
	ChannelPrimaryIPMB Channel = 0x0

	// ChannelPresentInterface means the current channel this value is sent
	// over, or "this" channel.
	ChannelPresentInterface Channel = 0xe

	ChannelSystemInterface Channel = 0xf
)
