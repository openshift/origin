package nfnetlink

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"syscall"
)

// NLAttr represents a single netlink attribute.
type NLAttr struct {
	Type uint16
	Data []byte
}

// nlaAlignOf returns attrlen aligned to a 4 byte boundary
func nlaAlignOf(attrlen int) int {
	return (attrlen + syscall.NLA_ALIGNTO - 1) & ^(syscall.NLA_ALIGNTO - 1)
}

// NewAttrFromFields creates and returns a new NLAttr instance by serializing the provided
// fields into a slice of bytes which is stored as the Data element of the attribute.
func NewAttrFromFields(atype uint16, fields ...interface{}) (*NLAttr, error) {
	b := new(bytes.Buffer)
	for _, f := range fields {
		if err := binary.Write(b, binary.BigEndian, f); err != nil {
			return nil, err
		}
	}
	return NewAttr(atype, b.Bytes()), nil
}

// NewAttr creates and returns a new NLAttr instance from the provided type and data payload
func NewAttr(atype uint16, data []byte) *NLAttr {
	return &NLAttr{
		Type: atype,
		Data: data,
	}
}

func (a *NLAttr) String() string {
	return fmt.Sprintf("(%d %s)", a.Type, hex.EncodeToString(a.Data))
}

// ParseAttr reads a serialized attribute from r and parses it into an NLAttr instance.
func ParseAttr(r *bytes.Reader) (*NLAttr, error) {
	attr := &NLAttr{}
	if err := attr.parse(r); err != nil {
		return nil, err
	}
	return attr, nil
}

// parse reads a serialized attribute from r and parses it into this NLAttr instance.
func (a *NLAttr) parse(r *bytes.Reader) error {
	if r.Len() < syscall.NLA_HDRLEN {
		return errors.New("Truncated attribute")
	}
	var alen uint16
	binary.Read(r, native, &alen)
	binary.Read(r, native, &a.Type)

	if alen < syscall.NLA_HDRLEN || int(alen-syscall.NLA_HDRLEN) > r.Len() {
		return errors.New("Truncated attribute")
	}
	alen -= syscall.NLA_HDRLEN
	if alen == 0 {
		a.Data = nil
		return nil
	}

	a.Data = make([]byte, alen)
	r.Read(a.Data)
	padlen := nlaAlignOf(int(alen)) - int(alen)
	for i := 0; i < padlen; i++ {
		r.ReadByte()
	}
	return nil
}

// Size returns the size in bytes of this attribute when serialized
func (a *NLAttr) Size() int {
	return syscall.NLA_HDRLEN + nlaAlignOf(len(a.Data))
}

// serialize the attribute and return the raw bytes
func (a *NLAttr) serialize() []byte {
	bs := new(bytes.Buffer)
	a.WriteTo(bs)
	return bs.Bytes()
}

// WriteTo serializes the attribute instance into the provided bytes.Buffer
func (a *NLAttr) WriteTo(b *bytes.Buffer) {
	alen := syscall.NLA_HDRLEN + len(a.Data)
	binary.Write(b, native, uint16(alen))
	binary.Write(b, native, a.Type)
	b.Write(a.Data)
	a.writePadding(b)
}

// ReadFields parses the attribute data into the provided array of
// fields using binary.Read() to parse each individual field.
func (a *NLAttr) ReadFields(fields ...interface{}) error {
	r := bytes.NewReader(a.Data)
	for _, f := range fields {
		if err := binary.Read(r, binary.BigEndian, f); err != nil {
			return err
		}
	}
	return nil
}

// writePadding is called while serializing the attribute instance to write
// an appropriate number of '0' bytes to the buffer b so that the length of
// data in the buffer is 4 byte aligned
func (a *NLAttr) writePadding(b *bytes.Buffer) {
	padlen := a.Size() - (syscall.NLA_HDRLEN + len(a.Data))
	for i := 0; i < padlen; i++ {
		b.WriteByte(0)
	}
}
