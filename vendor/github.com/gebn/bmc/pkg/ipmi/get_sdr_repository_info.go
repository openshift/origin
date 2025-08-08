package ipmi

import (
	"encoding/binary"
	"fmt"
	"time"

	"github.com/gebn/bmc/internal/pkg/bcd"

	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
)

// GetSDRRepositoryInfoRsp represents the response to a Get SDR Repository Info
// command, specified in section 27.9 and 33.9 of IPMI v1.5 and v2.0
// respectively. This command is useful for finding out how many SDRs are in the
// repository, and finding whether any changes were made during enumeration
// (meaning a re-retrieval is required).
type GetSDRRepositoryInfoRsp struct {
	layers.BaseLayer

	// Version indicates the command set supported by the SDR Repository Device.
	// This is little-endian packed BCD, and has not changed from 0x51 (i.e.
	// IPMI v1.5) since IPMI-over-LAN was introduced in v1.5.
	Version uint8

	// Records is the number of records in the SDR repository.
	Records uint16

	// FreeSpace is the space remaining in the SDR repository in bytes.
	FreeSpace uint16

	// LastAddition is the time when the last record was added to the
	// repository. This will be the zero value if never.
	LastAddition time.Time

	// LastErase is the time when the last record was deleted from the
	// repository, or the entire repository was cleared. This will be the zero
	// value if never.
	LastErase time.Time

	// Overflow indicates whether an SDR could not be written due to lack of
	// space.
	Overflow bool

	// SupportsModalUpdate indicates whether the controller must be put in an
	// SDR Repository update mode in order to be modified. False means
	// unspecified rather than unsupported, in which case the remote console
	// should attempt a Enter SDR Update Mode before falling back to a non-modal
	// update. If this is false and SupportsNonModalUpdate is true, a non-modal
	// update should be performed.
	SupportsModalUpdate bool

	// SupportsNonModalUpdate indicates whether the controller can be written to
	// at any time, without impacting other commands. False means unspecified
	// rather than unsupported; if SupportsModalUpdate is true, a modal update
	// should be performed.
	SupportsNonModalUpdate bool

	// SupportsDelete indicates whether the Delete SDR command is supported.
	SupportsDelete bool

	// SupportsPartialAdd indicates whether the Partial Add SDR command is
	// supported.
	SupportsPartialAdd bool

	// SupportsReserve indicates whether the Reserve SDR Repository command is
	// supported.
	SupportsReserve bool

	// SupportsGetAllocationInformation indicates whether the Get SDR Repository
	// Allocation Information command is supported.
	SupportsGetAllocationInformation bool
}

func (*GetSDRRepositoryInfoRsp) LayerType() gopacket.LayerType {
	return LayerTypeGetSDRRepositoryInfoRsp
}

func (i *GetSDRRepositoryInfoRsp) CanDecode() gopacket.LayerClass {
	return i.LayerType()
}

func (*GetSDRRepositoryInfoRsp) NextLayerType() gopacket.LayerType {
	return gopacket.LayerTypePayload
}

func (i *GetSDRRepositoryInfoRsp) DecodeFromBytes(data []byte, df gopacket.DecodeFeedback) error {
	if len(data) < 14 {
		df.SetTruncated()
		return fmt.Errorf("response must be 14 bytes, got %v", len(data))
	}

	i.BaseLayer.Contents = data[:14]
	i.BaseLayer.Payload = data[14:]

	i.Version = bcd.Decode(data[0]&0xf)*10 + bcd.Decode(data[0]>>4)
	i.Records = binary.LittleEndian.Uint16(data[1:3])
	i.FreeSpace = binary.LittleEndian.Uint16(data[3:5])
	i.LastAddition = time.Unix(int64(binary.LittleEndian.Uint32(data[5:9])), 0)
	i.LastErase = time.Unix(int64(binary.LittleEndian.Uint32(data[9:13])), 0)
	i.Overflow = data[13]&(1<<7) != 0
	i.SupportsModalUpdate = data[13]&(1<<6) != 0
	i.SupportsNonModalUpdate = data[13]&(1<<5) != 0
	i.SupportsDelete = data[13]&(1<<3) != 0
	i.SupportsPartialAdd = data[13]&(1<<2) != 0
	i.SupportsReserve = data[13]&(1<<1) != 0
	i.SupportsGetAllocationInformation = data[13]&1 != 0
	return nil
}

type GetSDRRepositoryInfoCmd struct {
	Rsp GetSDRRepositoryInfoRsp
}

// Name returns "Get SDR Repository Info".
func (*GetSDRRepositoryInfoCmd) Name() string {
	return "Get SDR Repository Info"
}

// Operation returns OperationGetSDRRepositoryInfoReq.
func (*GetSDRRepositoryInfoCmd) Operation() *Operation {
	return &OperationGetSDRRepositoryInfoReq
}

func (*GetSDRRepositoryInfoCmd) RemoteLUN() LUN {
	return LUNBMC
}

func (*GetSDRRepositoryInfoCmd) Request() gopacket.SerializableLayer {
	return nil
}

func (c *GetSDRRepositoryInfoCmd) Response() gopacket.DecodingLayer {
	return &c.Rsp
}
