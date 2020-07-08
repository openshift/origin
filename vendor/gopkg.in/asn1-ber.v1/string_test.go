package ber

import (
	"testing"
)

func TestIA5String(t *testing.T) {
	for _, test := range []struct {
		value       string
		expectedErr string
	}{
		{"asdfgh", ""},
		{"asdfgå", "invalid character for IA5String at pos 5: å"},
	} {
		pkt := NewString(ClassUniversal, TypePrimitive, TagIA5String, test.value, test.value)
		dec, err := DecodePacketErr(pkt.Bytes())
		if err == nil {
			if test.expectedErr != "" {
				t.Errorf("got unexpected error for `%s`: %s", test.value, err)
			}
			if dec.Value.(string) != test.value {
				t.Errorf("did not get back original value: %s <=> %s", dec.Value.(string), test.value)
			}
		} else if err.Error() != test.expectedErr {
			t.Errorf("got unexpected error for `%s`: %s", test.value, err)
		}
	}
}

func TestPrintableString(t *testing.T) {
	for _, test := range []struct {
		value       string
		expectedErr string
	}{
		{"asdfgh", ""},
		{"asdfgå", "invalid character in position 5"},
	} {
		pkt := NewString(ClassUniversal, TypePrimitive, TagPrintableString, test.value, test.value)
		dec, err := DecodePacketErr(pkt.Bytes())
		if err == nil {
			if test.expectedErr != "" {
				t.Errorf("got unexpected error for `%s`: %s", test.value, err)
			}
			if dec.Value.(string) != test.value {
				t.Errorf("did not get back original value: %s <=> %s", dec.Value.(string), test.value)
			}
		} else if err.Error() != test.expectedErr {
			t.Errorf("got unexpected error for `%s`: %s", test.value, err)
		}
	}
}

func TestUTF8String(t *testing.T) {
	for _, test := range []struct {
		value       string
		expectedErr string
	}{
		{"åäöüß", ""},
		{"asdfg\xFF", "invalid UTF-8 string"},
	} {
		pkt := NewString(ClassUniversal, TypePrimitive, TagUTF8String, test.value, test.value)
		dec, err := DecodePacketErr(pkt.Bytes())
		if err == nil {
			if test.expectedErr != "" {
				t.Errorf("got unexpected error for `%s`: %s", test.value, err)
			}
			if dec.Value.(string) != test.value {
				t.Errorf("did not get back original value: %s <=> %s", dec.Value.(string), test.value)
			}
		} else if err.Error() != test.expectedErr {
			t.Errorf("got unexpected error for `%s`: %s", test.value, err)
		}
	}
}
