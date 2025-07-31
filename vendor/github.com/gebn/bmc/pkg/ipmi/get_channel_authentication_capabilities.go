package ipmi

import (
	"fmt"

	"github.com/gebn/bmc/pkg/iana"

	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
)

// GetChannelAuthenticationCapabilitiesReq defines a Get Channel Authentication
// Capabilities request. Its wire format is specified in section 18.12 of IPMI
// v1.5, and 22.13 of IPMI v2.0. This command is used to retrieve authentication
// algorithm support for a given channel at a given privilege level. In IPMIv2,
// this also reveals support for IPMI v1.5. This command can be sent outside a
// session. Inside a session, it is normally used as a keepalive. BMC
// implementations of IP-based channels must support this command using the IPMI
// v1.5 packet format, so it makes sense to always send it using v1.5
// encapsulation unless you know a-priori that the managed system supports IPMI
// v2.0. This command is typically the first one sent when looking to establish
// a session.
type GetChannelAuthenticationCapabilitiesReq struct {
	layers.BaseLayer

	// ExtendedData tells the BMC we understand IPMI v2.0 and want to discover
	// extended capabilities. In this case, the response will indicate support
	// for both IPMI v2.0 and IPMI v1.5. Ignored if the BMC only understands
	// IPMI v1.5.
	ExtendedData bool

	// Channel defines the channel whose authentication capabilities to
	// retrieve. Use ChannelPresentInterface to specify the current channel.
	Channel Channel

	// MaxPrivilege level indicates the user privilege level the remote console
	// intends to use on the channel. Specifying a higher privilege level may
	// mean the managed system chooses to respond with a stricter subset of
	// capabilities.
	MaxPrivilegeLevel PrivilegeLevel
}

func (*GetChannelAuthenticationCapabilitiesReq) LayerType() gopacket.LayerType {
	return LayerTypeGetChannelAuthenticationCapabilitiesReq
}

func (g *GetChannelAuthenticationCapabilitiesReq) SerializeTo(b gopacket.SerializeBuffer, _ gopacket.SerializeOptions) error {
	bytes, err := b.PrependBytes(2)
	if err != nil {
		return err
	}
	bytes[0] = uint8(g.Channel)
	if g.ExtendedData {
		bytes[0] |= 1 << 7
	}
	bytes[1] = uint8(g.MaxPrivilegeLevel)
	return nil
}

// GetChannelAuthenticationCapabilitiesRsp represents the response to a Get
// Channel Authentication Capabilities request.
type GetChannelAuthenticationCapabilitiesRsp struct {
	layers.BaseLayer

	// Channel is the channel number that these authentication capabilities
	// correspond to. This will never be ChannelPresentInterface.
	Channel Channel

	// ExtendedCapabilities indicates whether IPMI v2.0+ extended capabilities
	// are available. This will be false if the remote console did not request
	// extended data in its command.
	ExtendedCapabilities bool

	AuthenticationTypeOEM      bool
	AuthenticationTypePassword bool
	AuthenticationTypeMD5      bool
	AuthenticationTypeMD2      bool
	AuthenticationTypeNone     bool

	// TwoKeyLogin indicates whether the key-generating key K_G is not null.
	// This key is sometimes referred to as the "BMC Key" in the spec, however
	// this is misleading as the key "must be individually settable on each
	// channel that supports RMCP+" (22.25), so it is not global within the BMC.
	// This field applies to IPMI v2.0 only; will always be false for v1.5
	// (reserved bit).
	//
	// Two-key login is almost always disabled, as it effectively adds a second
	// password in addition to the user (or role in IPMI v2.0) password, which
	// must be known a-priori to establish a session.
	//
	// K_G is a 20 byte value used as the key for an HMAC during RMCP+ session
	// creation to produce the SIK. If K_G is null, K_[UID] (i.e. the user
	// password) is used instead. In this case, it is recommended for the user
	// password to have the 20 byte maximum length to lose as little security as
	// possible.
	TwoKeyLogin bool

	// PerMessageAuthentication being disabled means the BMC is only expecting
	// the Activate Session request to be authenticated - and likely only its
	// reply will be authenticated. Subsequent packets can use an authentication
	// type of NONE. A remote console is free to authenticate all packets it
	// sends (this one does), however the BMC can choose whether to validate
	// these, and if it is incorrect, it may still drop the packet.
	PerMessageAuthentication bool

	// UserLevelAuthentication being disabled means that commands requiring
	// only the User privilege level do not have to be authenticated,
	// regardless of PerMessageAuthentication - the idea being because these
	// commands are read-only. This library authenticates all packets
	// regardless.
	UserLevelAuthentication bool

	NonNullUsernamesEnabled bool
	NullUsernamesEnabled    bool
	AnonymousLoginEnabled   bool

	// SupportsV2 indicates whether the managed system supports IPMI v2.0 and
	// RMCP+.
	SupportsV2 bool

	// SupportsV1 indicates whether the managed system supports IPMI v1.5. Note
	// that this field was introduced in the v2.0 spec, and so will be false if
	// the BMC only supports v1.5. It is somewhat redundant if we receive this
	// response.
	SupportsV1 bool

	// OEM is the enterprise number of the organisation that specified the OEM
	// authentication type. This will be null if no such type is available,
	// displayed as "0(Unknown)".
	OEM iana.Enterprise

	// OEMData contains additional OEM-defined information for the OEM
	// authentication type. This will be null if no such type is available.
	OEMData byte
}

func (*GetChannelAuthenticationCapabilitiesRsp) LayerType() gopacket.LayerType {
	return LayerTypeGetChannelAuthenticationCapabilitiesRsp
}

func (g *GetChannelAuthenticationCapabilitiesRsp) CanDecode() gopacket.LayerClass {
	return g.LayerType()
}

func (*GetChannelAuthenticationCapabilitiesRsp) NextLayerType() gopacket.LayerType {
	return gopacket.LayerTypePayload
}

func (g *GetChannelAuthenticationCapabilitiesRsp) DecodeFromBytes(data []byte, df gopacket.DecodeFeedback) error {
	if len(data) < 8 {
		df.SetTruncated()
		return fmt.Errorf("invalid command response, length %v less than 8",
			len(data))
	}

	g.BaseLayer.Contents = data[:8]
	g.BaseLayer.Payload = data[8:]

	g.Channel = Channel(data[0])

	g.ExtendedCapabilities = data[1]&(1<<7) != 0
	g.AuthenticationTypeOEM = data[1]&(1<<5) != 0
	g.AuthenticationTypePassword = data[1]&(1<<4) != 0
	g.AuthenticationTypeMD5 = data[1]&(1<<2) != 0
	g.AuthenticationTypeMD2 = data[1]&(1<<1) != 0
	g.AuthenticationTypeNone = data[1]&1 != 0

	g.TwoKeyLogin = data[2]&(1<<5) != 0
	g.PerMessageAuthentication = data[2]&(1<<4) != 0
	g.UserLevelAuthentication = data[2]&(1<<3) != 0
	g.NonNullUsernamesEnabled = data[2]&(1<<2) != 0
	g.NullUsernamesEnabled = data[2]&(1<<1) != 0
	g.AnonymousLoginEnabled = data[2]&1 != 0

	g.SupportsV2 = data[3]&(1<<1) != 0
	g.SupportsV1 = data[3]&1 != 0

	// effectively a 3-byte implementation of binary.LittleEndian.Uint32()
	g.OEM = iana.Enterprise(uint32(data[4]) | uint32(data[5])<<8 | uint32(data[6])<<16)
	g.OEMData = uint8(data[7])

	return nil
}

type GetChannelAuthenticationCapabilitiesCmd struct {
	Req GetChannelAuthenticationCapabilitiesReq
	Rsp GetChannelAuthenticationCapabilitiesRsp
}

// Name returns "Get Channel Authentication Capabilities".
func (*GetChannelAuthenticationCapabilitiesCmd) Name() string {
	return "Get Channel Authentication Capabilities"
}

// Operation returns OperationGetChannelAuthenticationCapabilitiesReq.
func (*GetChannelAuthenticationCapabilitiesCmd) Operation() *Operation {
	return &OperationGetChannelAuthenticationCapabilitiesReq
}

func (c *GetChannelAuthenticationCapabilitiesCmd) RemoteLUN() LUN {
	return LUNBMC
}

func (c *GetChannelAuthenticationCapabilitiesCmd) Request() gopacket.SerializableLayer {
	return &c.Req
}

func (c *GetChannelAuthenticationCapabilitiesCmd) Response() gopacket.DecodingLayer {
	return &c.Rsp
}
