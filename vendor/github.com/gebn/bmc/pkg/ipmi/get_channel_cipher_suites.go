package ipmi

import (
	"fmt"

	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
)

// GetChannelCipherSuitesReq is defined in section 22.15 of IPMI v2.0. It is
// used to retrieve the authentication, integrity and confidentiality
// algorithms supported by a given channel on the BMC. Support for this command
// is mandatory for IPMI v2.0 BMCs implementing sessions, and it may be sent at
// all privilege levels, both inside and outside a session.
//
// The spec indicates BMCs may narrow the suites they accept at higher
// privilege levels (a given privilege level supports a subset of suites
// available at less-privileged levels), however there is no known mechanism to
// find out which suites apply to which privilege levels. We would expect there
// to be a MaxPrivilegeLevel field in this request.
type GetChannelCipherSuitesReq struct {
	layers.BaseLayer

	// Channel defines the channel whose supported cipher suites to retrieve.
	// Use ChannelPresentInterface to specify the current channel.
	Channel Channel

	// PayloadType indicates the type of payload that will be sent over the
	// channel when the session is established, which can influence cipher
	// suite availability. This is primarily useful for OEM support. If in
	// doubt, leave as the default PayloadTypeIPMI.
	PayloadType PayloadType

	// We only implement "list algorithms by Cipher Suite", not "list supported
	// algorithms". Although potentially more efficient, the response format of
	// the latter is not documented, is not implemented by ipmitool, and
	// returned an unintelligible response on a test BMC.

	// ListIndex is the 6-bit 0-based offset (0 through 63) into the cipher
	// suite record data, which is returned 16 bytes at a time. This means the
	// total length of cipher suite records cannot exceed 1024 bytes.
	ListIndex uint8
}

func (*GetChannelCipherSuitesReq) LayerType() gopacket.LayerType {
	return LayerTypeGetChannelCipherSuitesReq
}

func (c *GetChannelCipherSuitesReq) SerializeTo(b gopacket.SerializeBuffer, _ gopacket.SerializeOptions) error {
	bytes, err := b.PrependBytes(3)
	if err != nil {
		return err
	}
	bytes[0] = uint8(c.Channel & 0x0f)
	bytes[1] = uint8(c.PayloadType & 0x3f)
	// force list algos by cipher suite (see struct definition for rationale)
	bytes[2] = uint8(1<<7 | c.ListIndex&0x3f)
	return nil
}

// GetChannelCipherSuitesRsp represents the response to a Get Channel Cipher
// Suites request.
type GetChannelCipherSuitesRsp struct {
	layers.BaseLayer

	// Channel is the channel number that these cipher suites correspond to.
	// This will never be ChannelPresentInterface.
	Channel Channel

	// CipherSuiteRecordsChunk is up to 16 bytes of Cipher Suite Record data,
	// that may begin mid-way through a record, so is typically interpreted
	// after prior indices' data has been retrieved. Note this references data
	// in the decoded packet, so should be appended to a buffer before reading
	// the next packet.
	CipherSuiteRecordsChunk []byte
}

func (*GetChannelCipherSuitesRsp) LayerType() gopacket.LayerType {
	return LayerTypeGetChannelCipherSuitesRsp
}

func (c *GetChannelCipherSuitesRsp) CanDecode() gopacket.LayerClass {
	return c.LayerType()
}

func (*GetChannelCipherSuitesRsp) NextLayerType() gopacket.LayerType {
	return gopacket.LayerTypePayload
}

func (c *GetChannelCipherSuitesRsp) DecodeFromBytes(data []byte, df gopacket.DecodeFeedback) error {
	if len(data) < 1 {
		df.SetTruncated()
		return fmt.Errorf("invalid command response, length %v less than 1, expected channel number",
			len(data))
	}

	cipherSuiteRecordChunkEnd := len(data)
	if cipherSuiteRecordChunkEnd > 17 {
		// we have trailing bytes, cap interpreted length
		cipherSuiteRecordChunkEnd = 17
	}

	c.BaseLayer.Contents = data[:cipherSuiteRecordChunkEnd]
	c.BaseLayer.Payload = data[cipherSuiteRecordChunkEnd:]

	c.Channel = Channel(data[0])
	c.CipherSuiteRecordsChunk = data[1:cipherSuiteRecordChunkEnd]

	return nil
}

type GetChannelCipherSuitesCmd struct {
	Req GetChannelCipherSuitesReq
	Rsp GetChannelCipherSuitesRsp
}

// Name returns "Get Channel Cipher Suites".
func (*GetChannelCipherSuitesCmd) Name() string {
	return "Get Channel Cipher Suites"
}

// Operation returns &OperationGetChannelCipherSuitesReq.
func (*GetChannelCipherSuitesCmd) Operation() *Operation {
	return &OperationGetChannelCipherSuitesReq
}

func (c *GetChannelCipherSuitesCmd) RemoteLUN() LUN {
	return LUNBMC
}

func (c *GetChannelCipherSuitesCmd) Request() gopacket.SerializableLayer {
	return &c.Req
}

func (c *GetChannelCipherSuitesCmd) Response() gopacket.DecodingLayer {
	return &c.Rsp
}
