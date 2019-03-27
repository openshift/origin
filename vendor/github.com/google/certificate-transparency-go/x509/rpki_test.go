// Copyright 2018 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package x509

import (
	"reflect"
	"strings"
	"testing"
)

func TestParseRPKIAddrBlocks(t *testing.T) {
	tests := []struct {
		desc    string
		in      string // hex-encoded
		want    []*IPAddressFamilyBlocks
		wantErr string
	}{
		{
			desc: "ValidSingleIPv4",
			in: "300e" + // SEQUENCE OF IPAddressFamily
				("300c" + // IPAddressFamily SEQUENCE
					("0402" + "0001") + // addressFamily OCTET STRING
					("3006" + // addressesOrRanges SEQUENCE OF IPAddressOrRange
						"0304" + "03" + "d596c8")), // addressPrefix BIT STRING
			want: []*IPAddressFamilyBlocks{
				{
					AFI: 1,
					AddressPrefixes: []IPAddressPrefix{
						{Bytes: fromHex("d596c8"), BitLength: 21},
					},
				},
			},
		},
		{
			desc: "ValidSingleIPv4WithSAFI",
			in: "300f" + // SEQUENCE OF IPAddressFamily
				("300d" + // IPAddressFamily SEQUENCE
					("0403" + "000120") + // addressFamily OCTET STRING
					("3006" + // addressesOrRanges SEQUENCE OF IPAddressOrRange
						"0304" + "03" + "d596c8")), // addressPrefix BIT STRING
			want: []*IPAddressFamilyBlocks{
				{
					AFI:  1,
					SAFI: 0x20,
					AddressPrefixes: []IPAddressPrefix{
						{Bytes: fromHex("d596c8"), BitLength: 21},
					},
				},
			},
		},
		{
			desc: "ValidSingleIPv4Inherit",
			in: "3008" + // SEQUENCE OF IPAddressFamily
				("3006" + // IPAddressFamily SEQUENCE
					("0402" + "0001") + // addressFamily OCTET STRING
					("0500")), // inherit NULL
			want: []*IPAddressFamilyBlocks{
				{
					AFI:               1,
					InheritFromIssuer: true,
				},
			},
		},
		{
			desc: "ValidMultipleIPv4",
			in: "3014" + // SEQUENCE OF IPAddressFamily
				("3012" + // IPAddressFamily SEQUENCE
					("0402" + "0001") + // addressFamily OCTET STRING
					("300c" + // addressesOrRanges SEQUENCE OF IPAddressOrRange
						"0304" + "03" + "d596c8" + // addressPrefix BIT STRING
						"0304" + "03" + "d696c8")), // addressPrefix BIT STRING
			want: []*IPAddressFamilyBlocks{
				{
					AFI: 1,
					AddressPrefixes: []IPAddressPrefix{
						{Bytes: fromHex("d596c8"), BitLength: 21},
						{Bytes: fromHex("d696c8"), BitLength: 21},
					},
				},
			},
		},
		{
			desc: "ValidSingleIPv4Range",
			in: "3016" + // SEQUENCE OF IPAddressFamily
				("3014" + // IPAddressFamily SEQUENCE
					("0402" + "0001") + // addressFamily OCTET STRING
					("300e" + // addressesOrRanges SEQUENCE OF IPAddressOrRange
						("300c" + // addressRange SEQUENCE
							"0304" + "03" + "d596c8" + // min BIT STRING
							"0304" + "03" + "d596d8"))), // max BIT STRING
			want: []*IPAddressFamilyBlocks{
				{
					AFI: 1,
					AddressRanges: []IPAddressRange{
						{
							Min: IPAddressPrefix{Bytes: fromHex("d596c8"), BitLength: 21},
							Max: IPAddressPrefix{Bytes: fromHex("d596d8"), BitLength: 21},
						},
					},
				},
			},
		},
		{
			desc: "InvalidOuterSequence",
			in: "310e" + // SET OF not SEQUENCE OF IPAddressFamily
				("300c" + // IPAddressFamily SEQUENCE
					("0402" + "0001") + // addressFamily OCTET STRING
					("3006" + // addressesOrRanges SEQUENCE OF IPAddressOrRange
						"0304" + "03" + "d596c8")), // addressPrefix BIT STRING
			wantErr: "failed to asn1.Unmarshal ipAddrBlocks ",
		},
		{
			desc: "TrailingOuterSequence",
			in: "300e" + // SEQUENCE OF IPAddressFamily
				("300c" + // IPAddressFamily SEQUENCE
					("0402" + "0001") + // addressFamily OCTET STRING
					("3006" + // addressesOrRanges SEQUENCE OF IPAddressOrRange
						"0304" + "03" + "d596c8") + // addressPrefix BIT STRING
					"ff"), // trailing byte
			wantErr: "trailing data after ipAddrBlocks ",
		},
		{
			desc: "InvalidInnerSequence",
			in: "300e" + // SEQUENCE OF IPAddressFamily
				("310c" + // SET not SEQUENCE
					("0402" + "0001") + // addressFamily OCTET STRING
					("3006" + // addressesOrRanges SEQUENCE OF IPAddressOrRange
						"0304" + "03" + "d596c8")), // addressPrefix BIT STRING
			wantErr: "failed to asn1.Unmarshal ipAddrBlocks ",
		},
		{
			desc: "TrailingInnerSequence",
			in: "300f" + // SEQUENCE OF IPAddressFamily
				("300c" + // IPAddressFamily SEQUENCE
					("0402" + "0001") + // addressFamily OCTET STRING
					("3006" + // addressesOrRanges SEQUENCE OF IPAddressOrRange
						"0304" + "03" + "d596c8")) + // addressPrefix BIT STRING
				"ff", // trailing byte
			wantErr: "failed to asn1.Unmarshal ipAddrBlocks ",
		},
		{
			desc: "InvalidAddressFamily",
			in: "300e" + // SEQUENCE OF IPAddressFamily
				("300c" + // IPAddressFamily SEQUENCE
					("0302" + "0001") + // BIT STRING not OCTET STRING
					("3006" + // addressesOrRanges SEQUENCE OF IPAddressOrRange
						"0304" + "03" + "d596c8")), // addressPrefix BIT STRING
			wantErr: "failed to asn1.Unmarshal ipAddrBlocks ",
		},
		{
			desc: "InvalidAddressFamilyTooShort",
			in: "300d" + // SEQUENCE OF IPAddressFamily
				("300b" + // IPAddressFamily SEQUENCE
					("0401" + "01") + // addressFamily OCTET STRING too short
					("3006" + // addressesOrRanges SEQUENCE OF IPAddressOrRange
						"0304" + "03" + "d596c8")), // addressPrefix BIT STRING
			wantErr: "invalid address family length (1)",
		},
		{
			desc: "InvalidAddressFamilyTooLong",
			in: "3010" + // SEQUENCE OF IPAddressFamily
				("300e" + // IPAddressFamily SEQUENCE
					("0404" + "00010a0b") + // addressFamily OCTET STRING too long
					("3006" + // addressesOrRanges SEQUENCE OF IPAddressOrRange
						"0304" + "03" + "d596c8")), // addressPrefix BIT STRING
			wantErr: "invalid address family length (4)",
		},
		{
			desc: "InvalidChoiceSequence",
			in: "300e" + // SEQUENCE OF IPAddressFamily
				("300c" + // IPAddressFamily SEQUENCE
					("0402" + "0001") + // addressFamily OCTET STRING
					("3106" + // SET not SEQUENCE
						"0304" + "03" + "d596c8")), // addressPrefix BIT STRING
			wantErr: "failed to asn1.Unmarshal ipAddrBlocks[0].ipAddressChoice",
		},
		{
			desc: "InvalidIPv4Prefix",
			in: "300e" + // SEQUENCE OF IPAddressFamily
				("300c" + // IPAddressFamily SEQUENCE
					("0402" + "0001") + // addressFamily OCTET STRING
					("3006" + // addressesOrRanges SEQUENCE OF IPAddressOrRange
						"8304" + "03d596c8")), // addressPrefix BIT STRING, Not class=Universal
			want:    []*IPAddressFamilyBlocks{{AFI: 1}},
			wantErr: "failed to asn1.Unmarshal ipAddrBlocks[0].ipAddressChoice.addressesOrRanges[0].addressPrefix",
		},
		{
			desc: "InvalidIPv4Range",
			in: "3016" + // SEQUENCE OF IPAddressFamily
				("3014" + // IPAddressFamily SEQUENCE
					("0402" + "0001") + // addressFamily OCTET STRING
					("300e" + // addressesOrRanges SEQUENCE OF IPAddressOrRange
						("310c" + // SET not SEQUENCE
							"0304" + "03" + "d596c8" + // addressPrefix BIT STRING
							"0304" + "03" + "d596d8"))), // addressPrefix BIT STRING
			want:    []*IPAddressFamilyBlocks{{AFI: 1}},
			wantErr: "unexpected ASN.1 type in ipAddrBlocks[0].ipAddressChoice.addressesOrRanges[0]",
		},
		{
			desc: "InvalidIPv4RangeContents",
			in: "3016" + // SEQUENCE OF IPAddressFamily
				("3014" + // IPAddressFamily SEQUENCE
					("0402" + "0001") + // addressFamily OCTET STRING
					("300e" + // addressesOrRanges SEQUENCE OF IPAddressOrRange
						("300c" +
							"0404" + "03d596c8" + // OCTET STRING not BIT STRING
							"0304" + "03" + "d596d8"))), // addressPrefix BIT STRING
			want:    []*IPAddressFamilyBlocks{{AFI: 1}},
			wantErr: "failed to asn1.Unmarshal ipAddrBlocks[0].ipAddressChoice.addressesOrRanges[0].addressRange",
		},
	}

	for _, test := range tests {
		t.Run(test.desc, func(t *testing.T) {
			var nfe NonFatalErrors
			got := parseRPKIAddrBlocks(fromHex(test.in), &nfe)
			if !reflect.DeepEqual(got, test.want) {
				t.Errorf("parseRPKIAddrBlocks(%s)=%+v,%v; want %+v,_", test.in, got, nfe, test.want)
			}
			if !strings.Contains(nfe.Error(), test.wantErr) {
				t.Errorf("parseRPKIAddrBlocks(%s)=_,%v; want _, err containing %q", test.in, nfe, test.wantErr)
			}
		})
	}
}

func TestParseRPKIASIdentifiers(t *testing.T) {
	tests := []struct {
		desc            string
		in              string // hex-encoded
		wantAS, wantRDI *ASIdentifiers
		wantErr         string
	}{
		{
			desc: "ValidASRange",
			in: ("3010" + // SEQUENCE
				("a00e" + // Tag:0 Class:Context-specific Compound:Y
					("300c" + // SEQUENCE OF ASIdOrRange
						("300a" + // SEQUENCE (ASRange)
							"0201" + "00" + // INTEGER
							"0205" + "00ffffffff")))), // INTEGER
			wantAS: &ASIdentifiers{
				ASIDRanges: []ASIDRange{
					{Min: 0, Max: 0x00ffffffff},
				},
			},
		},
		{
			desc: "ValidASNumbers",
			in: ("300b" + // SEQUENCE
				("a009" + // Tag:0 Class:Context-specific Compound:Y
					("3007" + // SEQUENCE OF ASIdOrRange
						"0201" + "01" + // INTEGER
						"0202" + "0123"))), // INTEGER
			wantAS: &ASIdentifiers{ASIDs: []int{1, 0x123}},
		},
		{
			desc: "ValidASInherit",
			in: ("3004" + // SEQUENCE
				("8002" + // Tag:0 Class:Context-specific Compound:N
					"0500")), // NULL
			wantAS: &ASIdentifiers{InheritFromIssuer: true},
		},
		{
			desc: "ValidRDIRange",
			in: ("3010" + // SEQUENCE
				("a10e" + // Tag:1 Class:Context-specific Compound:Y
					("300c" + // SEQUENCE OF ASIdOrRange
						("300a" + // SEQUENCE (ASRange)
							"0201" + "00" + // INTEGER
							"0205" + "00ffffffff")))), // INTEGER
			wantRDI: &ASIdentifiers{
				ASIDRanges: []ASIDRange{
					{Min: 0, Max: 0x00ffffffff},
				},
			},
		},
		{
			desc: "InvalidASRange",
			in: ("3110" + // SET not SEQUENCE
				("a00e" + // Tag:0 Class:Context-specific Compound:Y
					("300c" + // SEQUENCE OF ASIdOrRange
						("300a" + // SEQUENCE (ASRange)
							"0201" + "00" + // INTEGER
							"0205" + "00ffffffff")))), // INTEGER
			wantErr: "failed to asn1.Unmarshal ASIdentifiers extension",
		},
		{
			desc: "TrailingASRange",
			in: ("3010" + // SEQUENCE
				("a00e" + // Tag:0 Class:Context-specific Compound:Y
					("300c" + // SEQUENCE OF ASIdOrRange
						("300a" + // SEQUENCE (ASRange)
							"0201" + "00" + // INTEGER
							"0205" + "00ffffffff"))) + // INTEGER
				"ff"),
			wantErr: "trailing data after ASIdentifiers extension",
		},
		{
			desc: "InvalidAsIdsOrRanges",
			in: ("3010" + // SEQUENCE
				("a00e" + // Tag:0 Class:Context-specific Compound:Y
					("310c" + // SET not SEQUENCE OF ASIdOrRange
						("300a" + // SEQUENCE (ASRange)
							"0201" + "00" + // INTEGER
							"0205" + "00ffffffff")))), // INTEGER
			wantErr: "failed to asn1.Unmarshal ASIdentifiers.asIdsOrRanges",
		},
		{
			desc: "TrailingAsIdsOrRanges",
			in: ("3011" + // SEQUENCE
				("a00f" + // Tag:0 Class:Context-specific Compound:Y
					("300c" + // SEQUENCE OF ASIdOrRange
						("300a" + // SEQUENCE (ASRange)
							"0201" + "00" + // INTEGER
							"0205" + "00ffffffff")) + "ff")), // INTEGER
			wantErr: "trailing data after ASIdentifiers.asIdsOrRanges",
		},
		{
			desc: "InvalidAsIdsOrRangesType",
			in: ("3010" + // SEQUENCE
				("a00e" + // Tag:0 Class:Context-specific Compound:Y
					("300c" + // SEQUENCE OF ASIdOrRange
						("310a" + // SET not SEQUENCE (ASRange)
							"0201" + "00" + // INTEGER
							"0205" + "00ffffffff")))), // INTEGER
			wantAS:  &ASIdentifiers{},
			wantErr: "unexpected value in ASIdentifiers.asIdsOrRanges[0]",
		},
		{
			desc: "InvalidASRange",
			in: ("3010" + // SEQUENCE
				("a00e" + // Tag:0 Class:Context-specific Compound:Y
					("300c" + // SEQUENCE OF ASIdOrRange
						("100a" + // SEQUENCE (ASRange) but not Constructed:Y
							"0201" + "00" + // INTEGER
							"0205" + "00ffffffff")))), // INTEGER
			wantAS:  &ASIdentifiers{},
			wantErr: "failed to asn1.Unmarshal ASIdentifiers.asIdsOrRanges[0].range",
		},
		{
			desc: "InvalidASId",
			in: ("300b" + // SEQUENCE
				("a009" + // Tag:0 Class:Context-specific Compound:Y
					("3007" + // SEQUENCE OF ASIdOrRange
						"8201" + "01" + // INTEGER but Constructed:Y
						"0202" + "0123"))), // INTEGER
			wantAS:  &ASIdentifiers{ASIDs: []int{0x123}},
			wantErr: "failed to asn1.Unmarshal ASIdentifiers.asIdsOrRanges[0].id",
		},
	}

	for _, test := range tests {
		t.Run(test.desc, func(t *testing.T) {
			var nfe NonFatalErrors
			gotAS, gotRDI := parseRPKIASIdentifiers(fromHex(test.in), &nfe)
			if !reflect.DeepEqual(gotAS, test.wantAS) {
				t.Errorf("parseRPKIASIdentifiers(%s)=%+v,_,%v; want %+v,_", test.in, gotAS, nfe, test.wantAS)
			}
			if !reflect.DeepEqual(gotRDI, test.wantRDI) {
				t.Errorf("parseRPKIASIdentifiers(%s)=_,%+v,%v; want _,%+v", test.in, gotRDI, nfe, test.wantRDI)
			}
			if !strings.Contains(nfe.Error(), test.wantErr) {
				t.Errorf("parseRPKIASIdentifiers(%s)=_,_,%v; want _,_, err containing %q", test.in, nfe, test.wantErr)
			}
		})
	}
}
