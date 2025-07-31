package ipmi

import (
	"errors"
	"fmt"

	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
)

// SetSessionPrivilegeLevelReq implements the Set Session Privilege Level command, specified in section
// 18.16 of v1.5 and 22.18 of v2.0.
type SetSessionPrivilegeLevelReq struct {
	layers.BaseLayer

	// PrivilegeLevel indicates the privilege level to switch to.
	// Omitting this field will retrieve the current privilege level without modification.
	// PrivilegeLevelHighest and PrivilegeLevelCallback are invalid values.
	PrivilegeLevel PrivilegeLevel
}

func (*SetSessionPrivilegeLevelReq) LayerType() gopacket.LayerType {
	return LayerTypeSetSessionPrivilegeLevelReq
}

func (c *SetSessionPrivilegeLevelReq) SerializeTo(b gopacket.SerializeBuffer, _ gopacket.SerializeOptions) error {
	if c.PrivilegeLevel == PrivilegeLevelCallback {
		// reserved according to the specification
		return errors.New("Set Session Privilege Level can't be 0x01(CALLBACK level)")
	}

	bytes, err := b.PrependBytes(1)
	if err != nil {
		return err
	}
	bytes[0] = uint8(c.PrivilegeLevel) & 0xF
	return nil
}

type SetSessionPrivilegeLevelRsp struct {
	layers.BaseLayer

	// PrivilegeLevel indicates the new (possibly updated) privilege level
	// of the user in the active session.
	PrivilegeLevel PrivilegeLevel
}

func (*SetSessionPrivilegeLevelRsp) LayerType() gopacket.LayerType {
	return LayerTypeSetSessionPrivilegeLevelRsp
}

func (r *SetSessionPrivilegeLevelRsp) CanDecode() gopacket.LayerClass {
	return r.LayerType()
}

func (*SetSessionPrivilegeLevelRsp) NextLayerType() gopacket.LayerType {
	return gopacket.LayerTypePayload
}

func (r *SetSessionPrivilegeLevelRsp) DecodeFromBytes(data []byte, df gopacket.DecodeFeedback) error {
	if len(data) != 1 { // in case of non-zero status code
		df.SetTruncated()
		return fmt.Errorf("Set Session Privilege Level Response must be 1 byte, got %v", len(data))
	}

	r.PrivilegeLevel = PrivilegeLevel(data[0] & 0xF)
	return nil
}

type SetSessionPrivilegeLevelCmd struct {
	Req SetSessionPrivilegeLevelReq
	Rsp SetSessionPrivilegeLevelRsp
}

// Name returns "Set Session Privilege Level".
func (*SetSessionPrivilegeLevelCmd) Name() string {
	return "Set Session Privilege Level"
}

// Operation returns OperationSetSessionPrivilegeLevelReq.
func (*SetSessionPrivilegeLevelCmd) Operation() *Operation {
	return &OperationSetSessionPrivilegeLevelReq
}

func (c *SetSessionPrivilegeLevelCmd) RemoteLUN() LUN {
	return LUNBMC
}

func (c *SetSessionPrivilegeLevelCmd) Request() gopacket.SerializableLayer {
	return &c.Req
}

func (c *SetSessionPrivilegeLevelCmd) Response() gopacket.DecodingLayer {
	return &c.Rsp
}
