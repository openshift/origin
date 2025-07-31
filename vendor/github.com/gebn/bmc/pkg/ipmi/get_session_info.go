package ipmi

import (
	"encoding/binary"
	"fmt"
	"net"

	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
)

// SessionIndex represents the first field of the Get Session Info request,
// defined in table 18-20 and 22-25 of IPMI v1.5 and v2.0 respectively. Although
// it is only used in this command, it has its own time as there are several
// sentinel values.
type SessionIndex uint8

const (
	// SessionIndexCurrent is a sentinel value for the Index field in a Get
	// Session Info request, requesting information about the session the
	// command was received over.
	SessionIndexCurrent SessionIndex = 0x00

	// SessionIndexHandle is a sentinel value for the Index field in a Get
	// Session Info request, requesting information about the specified session
	// handle rather than index. See type SessionHandle for its definition.
	SessionIndexHandle SessionIndex = 0xfe

	// SessionIndexID is a sentinel value for the Index field in a Get Session
	// Info request, requesting information about the specified session ID
	// rather than index.
	SessionIndexID SessionIndex = 0xff
)

// GetSessionInfoReq represents a Get Session Info request, specified in section
// 18.18 and 22.20 of IPMI v1.5 and v2.0 respectively. A session can be
// identified by one of its index, handle or ID. To get the current session,
// pass the zero-value. If identifying by handle, set Index to
// SessionIndexHandle. If identifying by ID, set Index to SessionIndexID.
type GetSessionInfoReq struct {
	layers.BaseLayer

	// Index is an offset into the logical sessions table maintained by the BMC.
	// Active sessions can be enumerated by incrementing this from 1 through to
	// the Active field of the response.
	Index SessionIndex

	// Handle is the handle of the session to request information for. This is
	// similar to the session ID, but only unique within the context of a
	// channel. See type SessionHandle for more details.
	Handle SessionHandle

	// ID is the ID of the session to request information for. For IPMI v2.0,
	// this is the ID assigned by the BMC, i.e. RemoteID, not the one generated
	// by the remote console.
	ID uint32
}

func (*GetSessionInfoReq) LayerType() gopacket.LayerType {
	return LayerTypeGetSessionInfoReq
}

func (g *GetSessionInfoReq) SerializeTo(b gopacket.SerializeBuffer, _ gopacket.SerializeOptions) error {
	length := 1
	switch g.Index {
	case SessionIndexHandle:
		length += 1
	case SessionIndexID:
		length += 4
	}
	bytes, err := b.PrependBytes(length)
	if err != nil {
		return err
	}
	bytes[0] = byte(g.Index)
	switch g.Index {
	case SessionIndexHandle:
		bytes[1] = byte(g.Handle)
	case SessionIndexID:
		binary.LittleEndian.PutUint32(bytes[1:], g.ID)
	}
	return nil
}

type GetSessionInfoRsp struct {
	layers.BaseLayer

	// Handle is the session handle of the requested session. In theory, this
	// should be 0x00 if no active session was found matching the coordinates in
	// the request, in which case only the Max and Active fields are valid. In
	// practice, Super Micro BMCs send a handle of 0x00 but then include
	// additional fields as if the session were valid. Checking UserID != 0 is a
	// more robust way to check if it and subsequent fields were included in the
	// response.
	Handle SessionHandle

	// Max is the highest number of simultaneous active sessions supported by
	// the BMC. This is independent of the session coordinates specified in the
	// request. It is a 6-bit uint on the wire.
	Max uint8

	// Active is the number of currently active sessions, <= Max. It is a 6-bit
	// uint on the wire. 0 means there are no active sessions.
	Active uint8

	// UserID is the ID of the user for the selected session. It is only
	// relevant if Handle is non-zero. This is a 6-bit uint on the wire. Zero
	// is reserved, and indicates no session was found.
	UserID uint8

	// PrivilegeLevel indicates the operating privilege level of the user in the
	// active session.
	PrivilegeLevel PrivilegeLevel

	// IsIPMIv2 applies to remote sessions, and indicates whether IPMI v1.5 or
	// v2.0 is in use. If this is true, it guarantees a LAN session.
	IsIPMIv2 bool

	// Channel is the channel number the session was activated over.
	Channel Channel

	// IP is the source IP address of the Activate Session command in IPMI v1.5,
	// or (presumably, to be consistent with Port) the RAKP Message 3 in IPMI
	// v2.0. This is 4 bytes on the wire; it is unclear what this will be if
	// communicating with the BMC over IPv6. This only applies to LAN sessions,
	// and the BMC is not required by the spec to store this.
	IP net.IP

	// MAC is the source MAC Address of the same packet used to populate IP. It
	// only applies to LAN sessions, and the BMC is not required by the spec to
	// store this.
	MAC net.HardwareAddr

	// Port is the source port number of the same packet used to populate IP. It
	// only applies to LAN sessions, and the BMC is not required by the spec to
	// store this.
	Port uint16
}

func (*GetSessionInfoRsp) LayerType() gopacket.LayerType {
	return LayerTypeGetSessionInfoRsp
}

func (g *GetSessionInfoRsp) CanDecode() gopacket.LayerClass {
	return g.LayerType()
}

func (*GetSessionInfoRsp) NextLayerType() gopacket.LayerType {
	return gopacket.LayerTypePayload
}

func (g *GetSessionInfoRsp) DecodeFromBytes(data []byte, df gopacket.DecodeFeedback) error {
	if len(data) < 3 {
		df.SetTruncated()
		return fmt.Errorf("expected at least 3 bytes, got %v", len(data))
	}

	g.Handle = SessionHandle(data[0])
	g.Max = data[1]
	g.Active = data[2]

	// a handle of 0x00 is meant to mean no active session, however some BMCs
	// (Super Micro) appear to either send this to mean they do not implement
	// handles, or as a valid handle value; the upshot is there *are* another 3
	// bytes with the user ID, privilege level, protocol aux data and channel
	// number
	if g.Handle == 0 && len(data) == 3 {
		g.UserID = 0
		g.PrivilegeLevel = 0
		g.IsIPMIv2 = false
		g.Channel = 0
		g.IP = nil
		g.MAC = nil
		g.Port = 0

		g.BaseLayer.Contents = data[:3]
		g.BaseLayer.Payload = data[3:]
		return nil
	}

	// the BMC has now violated the spec, so we behave as if g.Handle is
	// non-zero, and expect at least 6 bytes
	if len(data) < 6 {
		df.SetTruncated()
		return fmt.Errorf("expected at least 6 bytes for an active session, got %v", len(data))
	}

	g.UserID = data[3] & 0x3f
	g.PrivilegeLevel = PrivilegeLevel(data[4] & 0xf)
	g.IsIPMIv2 = (data[5]&0xf0)>>4 == 1
	g.Channel = Channel(data[5] & 0xf)

	if len(data) < 18 {
		g.IP = nil
		g.MAC = nil
		g.Port = 0

		g.BaseLayer.Contents = data[:6]
		g.BaseLayer.Payload = data[6:]
		return nil
	}

	// assume channel type == 802.3 LAN

	ip := [16]byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0xff, 0xff}
	copy(ip[12:], data[6:10])
	g.IP = ip[:]

	mac := [6]byte{}
	copy(mac[:], data[10:16])
	g.MAC = mac[:]

	g.Port = binary.LittleEndian.Uint16(data[16:18])

	g.BaseLayer.Contents = data[:18]
	g.BaseLayer.Payload = data[18:]
	return nil
}

type GetSessionInfoCmd struct {
	Req GetSessionInfoReq
	Rsp GetSessionInfoRsp
}

// Name returns "Get Session Info".
func (*GetSessionInfoCmd) Name() string {
	return "Get Session Info"
}

// Operation returns &OperationGetSessionInfoReq.
func (*GetSessionInfoCmd) Operation() *Operation {
	return &OperationGetSessionInfoReq
}

func (c *GetSessionInfoCmd) RemoteLUN() LUN {
	return LUNBMC
}

func (c *GetSessionInfoCmd) Request() gopacket.SerializableLayer {
	return &c.Req
}

func (c *GetSessionInfoCmd) Response() gopacket.DecodingLayer {
	return &c.Rsp
}
