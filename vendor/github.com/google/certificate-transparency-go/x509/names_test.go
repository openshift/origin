// Copyright 2017 Google Inc. All Rights Reserved.
//
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package x509

import (
	"bytes"
	"encoding/hex"
	"net"
	"reflect"
	"strings"
	"testing"

	"github.com/google/certificate-transparency-go/asn1"
	"github.com/google/certificate-transparency-go/x509/pkix"
)

func TestParseGeneralNames(t *testing.T) {
	var tests = []struct {
		data    string // as hex
		want    GeneralNames
		wantErr string
	}{
		{
			data: ("3012" +
				("8210" + "7777772e676f6f676c652e636f2e756b")),
			want: GeneralNames{
				DNSNames: []string{"www.google.co.uk"},
			},
		},
		{
			data: ("3024" +
				("8210" + "7777772e676f6f676c652e636f2e756b") +
				("8610" + "7777772e676f6f676c652e636f2e756b")),
			want: GeneralNames{
				DNSNames: []string{"www.google.co.uk"},
				URIs:     []string{"www.google.co.uk"},
			},
		},
		{
			data:    "0a0101",
			wantErr: "failed to parse GeneralNames sequence",
		},
		{
			data:    "0a",
			wantErr: "failed to parse GeneralNames:",
		},
		{
			data:    "03000a0101",
			wantErr: "trailing data",
		},
		{
			data:    ("3005" + ("8703" + "010203")),
			wantErr: "invalid IP length",
		},
	}
	for _, test := range tests {
		inData := fromHex(test.data)
		var got GeneralNames
		err := parseGeneralNames(inData, &got)
		if err != nil {
			if test.wantErr == "" {
				t.Errorf("parseGeneralNames(%s)=%v; want nil", test.data, err)
			} else if !strings.Contains(err.Error(), test.wantErr) {
				t.Errorf("parseGeneralNames(%s)=%v; want %q", test.data, err, test.wantErr)
			}
			continue
		}
		if test.wantErr != "" {
			t.Errorf("parseGeneralNames(%s)=%+v,nil; want %q", test.data, got, test.wantErr)
			continue
		}
		if !reflect.DeepEqual(got, test.want) {
			t.Errorf("parseGeneralNames(%s)=%+v; want %+v", test.data, got, test.want)
		}
	}
}

func TestParseGeneralName(t *testing.T) {
	var tests = []struct {
		data     string // as hex
		withMask bool
		want     GeneralNames
		wantErr  string
	}{
		{
			data: ("a008" +
				("0603" + "551d0e") + // OID: subject-key-id
				("0a01" + "01")), // enum=1
			want: GeneralNames{
				OtherNames: []OtherName{
					{
						TypeID: OIDExtensionSubjectKeyId,
						Value: asn1.RawValue{
							Class:      asn1.ClassUniversal,
							Tag:        asn1.TagEnum,
							IsCompound: false,
							Bytes:      fromHex("01"),
							FullBytes:  fromHex("0a0101"),
						},
					},
				},
			},
		},
		{
			data: ("8008" +
				("0603" + "551d0e") + // OID: subject-key-id
				("0a01" + "01")), // enum=1
			wantErr: "not compound",
		},
		{
			data: ("a005" +
				("0603" + "551d0e")), // OID: subject-key-id
			wantErr: "sequence truncated",
		},
		{
			data: ("8110" + "77777740676f6f676c652e636f2e756b"),
			want: GeneralNames{
				EmailAddresses: []string{"www@google.co.uk"},
			},
		},
		{
			data: ("8210" + "7777772e676f6f676c652e636f2e756b"),
			want: GeneralNames{
				DNSNames: []string{"www.google.co.uk"},
			},
		},
		{
			data: ("844b" +
				("3049" +
					("310b" +
						("3009" +
							("0603" + "550406") +
							("1302" + "5553"))) + // "US"
					("3113" +
						("3011" +
							("0603" + "55040a") +
							("130a" + "476f6f676c6520496e63"))) + // "Google Inc"
					("3125" +
						("3023" +
							("0603" + "550403") +
							("131c" + "476f6f676c6520496e7465726e657420417574686f72697479204732"))))), // "GoogleInternet Authority G2"
			want: GeneralNames{
				DirectoryNames: []pkix.Name{
					{
						Country:      []string{"US"},
						Organization: []string{"Google Inc"},
						CommonName:   "Google Internet Authority G2",
						Names: []pkix.AttributeTypeAndValue{
							{Type: pkix.OIDCountry, Value: "US"},
							{Type: pkix.OIDOrganization, Value: "Google Inc"},
							{Type: pkix.OIDCommonName, Value: "Google Internet Authority G2"},
						},
					},
				},
			},
		},
		{
			data:    ("8410" + "7777772e676f6f676c652e636f2e756b"),
			wantErr: "failed to unmarshal GeneralNames.directoryName",
		},
		{
			data: ("8610" + "7777772e676f6f676c652e636f2e756b"),
			want: GeneralNames{
				URIs: []string{"www.google.co.uk"},
			},
		},
		{
			data: ("8704" + "01020304"),
			want: GeneralNames{
				IPNets: []net.IPNet{{IP: net.IP{1, 2, 3, 4}}},
			},
		},
		{
			data:     ("8708" + "01020304ffffff00"),
			withMask: true,
			want: GeneralNames{
				IPNets: []net.IPNet{{IP: net.IP{1, 2, 3, 4}, Mask: net.IPMask{0xff, 0xff, 0xff, 0x00}}},
			},
		},
		{
			data: ("8710" + "01020304111213142122232431323334"),
			want: GeneralNames{
				IPNets: []net.IPNet{{IP: net.IP{1, 2, 3, 4, 0x11, 0x12, 0x13, 0x14, 0x21, 0x22, 0x23, 0x24, 0x31, 0x32, 0x33, 0x34}}},
			},
		},
		{
			data:    ("8703" + "010203"),
			wantErr: "invalid IP length",
		},
		{
			data:     ("8707" + "01020304ffffff"),
			withMask: true,
			wantErr:  "invalid IP/mask length",
		},
		{
			data: ("8803" + "551d0e"), // OID: subject-key-id
			want: GeneralNames{
				RegisteredIDs: []asn1.ObjectIdentifier{OIDExtensionSubjectKeyId},
			},
		},
		{
			data:    ("8803" + "551d8e"),
			wantErr: "syntax error",
		},
		{
			data:    ("9003" + "551d8e"),
			wantErr: "unknown tag",
		},
		{
			data:    ("8803"),
			wantErr: "data truncated",
		},
	}

	for _, test := range tests {
		inData := fromHex(test.data)
		var got GeneralNames
		_, err := parseGeneralName(inData, &got, test.withMask)
		if err != nil {
			if test.wantErr == "" {
				t.Errorf("parseGeneralName(%s)=%v; want nil", test.data, err)
			} else if !strings.Contains(err.Error(), test.wantErr) {
				t.Errorf("parseGeneralName(%s)=%v; want %q", test.data, err, test.wantErr)
			}
			continue
		}
		if test.wantErr != "" {
			t.Errorf("parseGeneralName(%s)=%+v,nil; want %q", test.data, got, test.wantErr)
			continue
		}
		if !reflect.DeepEqual(got, test.want) {
			t.Errorf("parseGeneralName(%s)=%+v; want %+v", test.data, got, test.want)
		}
		if got.Empty() {
			t.Errorf("parseGeneralName(%s).Empty(%+v)=true; want false", test.data, got)
		}
		if gotLen, wantLen := got.Len(), 1; gotLen != wantLen {
			t.Errorf("parseGeneralName(%s).Len(%+v)=%d; want %d", test.data, got, gotLen, wantLen)
		}
		if !bytes.Equal(inData, fromHex(test.data)) {
			t.Errorf("parseGeneralName(%s) modified data to %x", test.data, inData)
		}

		// Wrap the GeneralName up in a SEQUENCE and check that we get the same result using parseGeneralNames.
		if test.withMask {
			continue
		}
		seqData := append([]byte{0x30, byte(len(inData))}, inData...)
		var gotSeq GeneralNames
		err = parseGeneralNames(seqData, &gotSeq)
		if err != nil {
			t.Errorf("parseGeneralNames(%x)=%v; want nil", seqData, err)
			continue
		}
		if !reflect.DeepEqual(gotSeq, test.want) {
			t.Errorf("parseGeneralNames(%x)=%+v; want %+v", seqData, gotSeq, test.want)
		}
	}
}

func fromHex(s string) []byte {
	d, _ := hex.DecodeString(s)
	return d
}
