package nfnetlink

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"syscall"
	"unsafe"
)

// The native or local byte order
var native = nativeByteOrder()

func nativeByteOrder() binary.ByteOrder {
	var x uint32 = 0x01020304
	if *(*byte)(unsafe.Pointer(&x)) == 0x01 {
		return binary.BigEndian
	} else {
		return binary.LittleEndian
	}
}

// Length in bytes of NfGenHdr structure
const NFGEN_HDRLEN = 4

// General address family dependent message header
type NfGenHdr struct {
	Family  uint8  // AF_XXX
	Version uint8  // nfnetlink version
	ResID   uint16 // resource id
}

// Netfilter netlink message
type NfNlMessage struct {
	syscall.NlMsghdr // Netlink message header
	NfGenHdr         // nfnetlink general header

	attrs   []*NLAttr          // list of attributes
	attrMap map[uint16]*NLAttr // mapping of attributes by type
	nls     *NetlinkSocket     // Socket this message will be transmitted on
}

// NewNfNlMsg create and return a new NfNlMessage associated with socket s
func (s *NetlinkSocket) NewNfNlMsg() *NfNlMessage {
	return &NfNlMessage{
		nls:     s,
		attrMap: make(map[uint16]*NLAttr),
	}
}

// Send transmits the message on the associated netlink socket
func (m *NfNlMessage) Send() error {
	return m.nls.Send(m)
}

func (m *NfNlMessage) String() string {
	bb := new(bytes.Buffer)
	fmt.Fprintf(bb, "[L: %d T: %04x F: %04x S: %d P: %d | ", m.Len, m.Type, m.Flags, m.Seq, m.Pid)
	fmt.Fprintf(bb, "F: %d V: %d R: %d |", m.Family, m.Version, m.ResID)
	for _, a := range m.attrs {
		fmt.Fprintf(bb, " %v", a)
	}
	fmt.Fprintf(bb, "]")

	return bb.String()
}

// Serialize the message and return the raw bytes
func (m *NfNlMessage) Serialize() []byte {
	m.updateLen()
	bb := new(bytes.Buffer)
	binary.Write(bb, native, &m.NlMsghdr)
	bb.WriteByte(m.Family)
	bb.WriteByte(m.Version)
	binary.Write(bb, binary.BigEndian, m.ResID)

	for _, a := range m.attrs {
		a.WriteTo(bb)
	}

	return bb.Bytes()
}

// updateLen sets the header Len value to the correct value for the current content
func (m *NfNlMessage) updateLen() {
	m.Len = syscall.NLMSG_HDRLEN + NFGEN_HDRLEN
	for _, a := range m.attrs {
		m.Len += uint32(a.Size())
	}
}

// Round the length of a netlink message up to align it properly.
func nlmAlignOf(msglen int) int {
	return (msglen + syscall.NLMSG_ALIGNTO - 1) & ^(syscall.NLMSG_ALIGNTO - 1)
}

// AddAttributeData creates and appends a new NLAttr from the provided type and payload data
func (m *NfNlMessage) AddAttributeData(atype uint16, data []byte) {
	m.AddAttribute(NewAttr(atype, data))
}

// AddAttributeFields creates and appends a new NLAttr by serializing the provided fields
// as the payload data for the new attribute
func (m *NfNlMessage) AddAttributeFields(atype uint16, fields ...interface{}) error {
	attr, err := NewAttrFromFields(atype, fields...)
	if err != nil {
		return err
	}
	m.AddAttribute(attr)
	return nil
}

// AddAttribute appends the provided NLAttr attribute to this message
func (m *NfNlMessage) AddAttribute(attr *NLAttr) {
	m.attrs = append(m.attrs, attr)
	m.attrMap[attr.Type] = attr
}

// AttrByType returns an attribute of the given type if this message contains one, or nil otherwise.
func (m *NfNlMessage) AttrByType(atype uint16) *NLAttr {
	return m.attrMap[atype]
}

// parse reads serialized bytes from r and parses a netlink message starting at the NfGenHdr and
// stores it in the current message m.
func (m *NfNlMessage) parse(r *bytes.Reader, hdr syscall.NlMsghdr) error {
	m.NlMsghdr = hdr
	if err := binary.Read(r, binary.BigEndian, &m.NfGenHdr); err != nil {
		return err
	}
	return m.parseAttributes(r)
}

// parseAttributes reads serialized attributes from r and parses each one into an NLAttr instance which is
// appended to this message.
func (m *NfNlMessage) parseAttributes(r *bytes.Reader) error {
	for r.Len() >= syscall.NLA_HDRLEN {
		attr, err := ParseAttr(r)
		if err != nil {
			return err
		}
		m.AddAttribute(attr)
	}
	return nil
}
