package dbus

import (
	"bytes"
	"encoding/binary"
	"testing"
)

type pixmap struct {
	Width  int
	Height int
	Pixels []uint8
}

type property struct {
	IconName    string
	Pixmaps     []pixmap
	Title       string
	Description string
}

func TestDecodeArrayEmptyStruct(t *testing.T) {
	buf := bytes.NewBuffer(nil)
	msg := &Message{
		Type:  0x02,
		Flags: 0x00,
		Headers: map[HeaderField]Variant{
			0x06: Variant{
				sig: Signature{
					str: "s",
				},
				value: ":1.391",
			},
			0x05: Variant{
				sig: Signature{
					str: "u",
				},
				value: uint32(2),
			},
			0x08: Variant{
				sig: Signature{
					str: "g",
				},
				value: Signature{
					str: "v",
				},
			},
		},
		Body: []interface{}{
			Variant{
				sig: Signature{
					str: "(sa(iiay)ss)",
				},
				value: property{
					IconName:    "iconname",
					Pixmaps:     []pixmap{},
					Title:       "title",
					Description: "description",
				},
			},
		},
		serial: 0x00000003,
	}
	err := msg.EncodeTo(buf, binary.LittleEndian)
	if err != nil {
		t.Fatal(err)
	}
	msg, err = DecodeMessage(buf)
	if err != nil {
		t.Fatal(err)
	}
}
