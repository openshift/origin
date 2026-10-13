// © Broadcom. All Rights Reserved.
// The term “Broadcom” refers to Broadcom Inc. and/or its subsidiaries.
// SPDX-License-Identifier: Apache-2.0

package types

import (
	"fmt"
	"io"
	"math"
	"strconv"

	"github.com/vmware/govmomi/vim25/xml"
)

// ByteSlice implements vCenter compatibile xml encoding and decoding for a byte slice.
// vCenter encodes each byte of the array in its own xml element, whereas
// Go encodes the entire byte array in a single xml element.
type ByteSlice []byte

// MarshalXML implements xml.Marshaler
func (b ByteSlice) MarshalXML(e *xml.Encoder, field xml.StartElement) error {
	start := xml.StartElement{
		Name: field.Name,
	}
	for i := range b {
		// Using int8() here to output a signed byte (issue #3615)
		if err := e.EncodeElement(int8(b[i]), start); err != nil {
			return err
		}
	}
	return nil
}

// UnmarshalXML implements xml.Unmarshaler
func (b *ByteSlice) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	for {
		t, err := d.Token()
		if err == io.EOF {
			break
		}

		if c, ok := t.(xml.CharData); ok {
			n, err := strconv.ParseInt(string(c), 10, 16)
			if err != nil {
				return err
			}
			if n > math.MaxUint8 {
				return fmt.Errorf("parsing %q: uint8 overflow", start.Name.Local)
			}
			*b = append(*b, byte(n))
		}
	}

	return nil
}
