// Copyright 2017 Google Inc. All Rights Reserved.
//
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package x509

import (
	"encoding/pem"
	"math/big"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/google/certificate-transparency-go/asn1"
	"github.com/google/certificate-transparency-go/x509/pkix"
)

func TestParseCertificateList(t *testing.T) {
	var tests = []struct {
		desc    string
		data    string // as hex
		want    TBSCertList
		wantErr string
	}{
		{
			desc: "valid-certlist",
			data: ("3082026c" + // SEQUENCE CertificateList
				("30820154" + // SEQUENCE TBSCertList
					("0201" + "01") + // version 2(0x01)
					("300d" + // SEQUENCE AlgorithmIdentifier
						("0609" + "2a864886f70d01010b") + // OID: sha256WithRSA
						"0500") + // NULL
					("3049" + // SEQUENCE Name
						("310b" +
							("3009" +
								("0603" + "550406") + // OID: country
								("1302" + "5553"))) + // "US"
						("3113" +
							("3011" +
								("0603" + "55040a") + // OID: organization
								("130a" + "476f6f676c6520496e63"))) + // "Google Inc"
						("3125" +
							("3023" +
								("0603" + "550403") + // OID: commonName
								("131c" + "476f6f676c6520496e7465726e657420417574686f72697479204732")))) +
					("170d" + "3137303632393031303030325a") + // UTCTime
					("170d" + "3137303730393031303030325a") + // UTCTime
					("3081a4" + // SEQUENCE OF
						("3027" +
							("0208" + "764bedd38afd51f7") + // serial number
							("170d" + "3137303131333134313835385a") + // revocation time
							("300c" +
								("300a" +
									("0603" + "551d15") +
									("0403" + "0a0103")))) +
						("3027" +
							("0208" + "3b772e5f1202118e") +
							("170d" + "3137303531303130353530375a") +
							("300c" +
								("300a" +
									("0603" + "551d15") +
									("0403" + "0a0101")))) +
						("3027" +
							("0208" + "0b54e3090079ad4b") +
							("170d" + "3137303431323038353331375a") +
							("300c" +
								("300a" +
									("0603" + "551d15") +
									("0403" + "0a0101")))) +
						("3027" +
							("0208" + "31da3380182af9b2") +
							("170d" + "3136303931353230323231335a") +
							("300c" +
								("300a" +
									("0603" + "551d15") +
									("0403" + "0a0103"))))) +
					("a030" +
						("302e" +
							("301f" +
								("0603" + "551d23") + // OID: authority-key-id
								("0418" +
									("3016" +
										"8014" + "4add06161bbcf668b576f581b6bb621aba5a812f"))) +
							("300b" +
								("0603" + "551d14") + // OID: CRL-number
								("0404" + "02020623"))))) +
				("300d" +
					("0609" + "2a864886f70d01010b") + // OID: sha256WithRSA
					"0500") + // NULL
				("03820101" + // BIT STRING length 0x101
					"004dcde29667973239cca344c58b72128fb5c5db03efdc75cfb7d9a0410ec03c8cd21160b449cd80224f41ca9d91529295ef7d0179ca4b08bb688cecce13cc07b20ecd87ffde1bc356554083c40bea7a387dacc54b3848b3710acf2fa613d007b12afc37f0a77082655b8dbb6683ba2fc52555e9f74bb5ba9429377ff38e193e799fc05c4c9bbcee29492945a732db67ba3575a79a83427a1f6d18d9ede01c544f3ccd68e5680a9b5418e03e1d80b3e77e69860982a4d21c6b111b07c87fe32c561e871554896b37651d5aaf42b2d092ce8d4dd4ae1d7a97091c0a06c03d71580e0557a51408513fde3012f02dac76536822a564faa2553048729633b68f1fc369")),
			want: TBSCertList{
				Version: 1,
				Signature: pkix.AlgorithmIdentifier{
					Algorithm:  oidSignatureSHA256WithRSA,
					Parameters: asn1.RawValue{Class: 0, Tag: 5, Bytes: []byte{}, FullBytes: []byte{5, 0}},
				},
				Issuer: pkix.RDNSequence{
					[]pkix.AttributeTypeAndValue{
						{Type: pkix.OIDCountry, Value: "US"},
					},
					[]pkix.AttributeTypeAndValue{
						{Type: pkix.OIDOrganization, Value: "Google Inc"},
					},
					[]pkix.AttributeTypeAndValue{
						{Type: pkix.OIDCommonName, Value: "Google Internet Authority G2"},
					},
				},
				ThisUpdate: time.Date(2017, 6, 29, 01, 0, 2, 0, time.UTC),
				NextUpdate: time.Date(2017, 7, 9, 01, 0, 2, 0, time.UTC),
				RevokedCertificates: []*RevokedCertificate{
					{
						RevokedCertificate: pkix.RevokedCertificate{
							SerialNumber:   big.NewInt(0x764bedd38afd51f7),
							RevocationTime: time.Date(2017, 1, 13, 14, 18, 58, 0, time.UTC),
						},
						RevocationReason: AffiliationChanged,
					},
					{
						RevokedCertificate: pkix.RevokedCertificate{
							SerialNumber:   big.NewInt(0x3b772e5f1202118e),
							RevocationTime: time.Date(2017, 5, 10, 10, 55, 7, 0, time.UTC),
						},
						RevocationReason: KeyCompromise,
					},
					{
						RevokedCertificate: pkix.RevokedCertificate{
							SerialNumber:   big.NewInt(0x0b54e3090079ad4b),
							RevocationTime: time.Date(2017, 4, 12, 8, 53, 17, 0, time.UTC),
						},
						RevocationReason: KeyCompromise,
					},
					{
						RevokedCertificate: pkix.RevokedCertificate{
							SerialNumber:   big.NewInt(0x31da3380182af9b2),
							RevocationTime: time.Date(2016, 9, 15, 20, 22, 13, 0, time.UTC),
						},
						RevocationReason: AffiliationChanged,
					},
				},
				AuthorityKeyID: fromHex("4add06161bbcf668b576f581b6bb621aba5a812f"),
				CRLNumber:      1571,
				BaseCRLNumber:  -1,
			},
		},
		{
			desc: "invalid-cert-critical-ext-revocation-time",
			data: ("3082026f" + // SEQUENCE CertificateList
				("30820157" + // SEQUENCE TBSCertList
					("0201" + "01") + // version 2(0x01)
					("300d" + // SEQUENCE AlgorithmIdentifier
						("0609" + "2a864886f70d01010b") + // OID: sha256WithRSA
						"0500") + // NULL
					("3049" + // SEQUENCE Name
						("310b" +
							("3009" +
								("0603" + "550406") + // OID: country
								("1302" + "5553"))) + // "US"
						("3113" +
							("3011" +
								("0603" + "55040a") + // OID: organization
								("130a" + "476f6f676c6520496e63"))) + // "Google Inc"
						("3125" +
							("3023" +
								("0603" + "550403") + // OID: commonName
								("131c" + "476f6f676c6520496e7465726e657420417574686f72697479204732")))) +
					("170d" + "3137303632393031303030325a") + // UTCTime
					("170d" + "3137303730393031303030325a") + // UTCTime
					("3081a7" + // SEQUENCE OF
						("302a" +
							("0208" + "764bedd38afd51f7") + // serial number
							("170d" + "3137303131333134313835385a") + // revocation time
							("300f" +
								("300d" +
									("0603" + "551d15") +
									("0101ff") + // INVALID critical: true
									("0403" + "0a0103")))) +
						("3027" +
							("0208" + "3b772e5f1202118e") +
							("170d" + "3137303531303130353530375a") +
							("300c" +
								("300a" +
									("0603" + "551d15") +
									("0403" + "0a0101")))) +
						("3027" +
							("0208" + "0b54e3090079ad4b") +
							("170d" + "3137303431323038353331375a") +
							("300c" +
								("300a" +
									("0603" + "551d15") +
									("0403" + "0a0101")))) +
						("3027" +
							("0208" + "31da3380182af9b2") +
							("170d" + "3136303931353230323231335a") +
							("300c" +
								("300a" +
									("0603" + "551d15") +
									("0403" + "0a0103"))))) +
					("a030" +
						("302e" +
							("301f" +
								("0603" + "551d23") + // OID: authority-key-id
								("0418" +
									("3016" +
										"8014" + "4add06161bbcf668b576f581b6bb621aba5a812f"))) +
							("300b" +
								("0603" + "551d14") + // OID: CRL-number
								("0404" + "02020623"))))) +
				("300d" +
					("0609" + "2a864886f70d01010b") + // OID: sha256WithRSA
					"0500") + // NULL
				("03820101" + // BIT STRING length 0x101
					"004dcde29667973239cca344c58b72128fb5c5db03efdc75cfb7d9a0410ec03c8cd21160b449cd80224f41ca9d91529295ef7d0179ca4b08bb688cecce13cc07b20ecd87ffde1bc356554083c40bea7a387dacc54b3848b3710acf2fa613d007b12afc37f0a77082655b8dbb6683ba2fc52555e9f74bb5ba9429377ff38e193e799fc05c4c9bbcee29492945a732db67ba3575a79a83427a1f6d18d9ede01c544f3ccd68e5680a9b5418e03e1d80b3e77e69860982a4d21c6b111b07c87fe32c561e871554896b37651d5aaf42b2d092ce8d4dd4ae1d7a97091c0a06c03d71580e0557a51408513fde3012f02dac76536822a564faa2553048729633b68f1fc369")),
			wantErr: "marked critical",
		},
		{
			desc: "invalid-unknown-critical-ext",
			data: ("308201c9" + // SEQUENCE CertificateList
				("3081b2" + // SEQUENCE TBSCertList
					("0201" + "01") + // version 2(0x01)
					("300d" + // SEQUENCE AlgorithmIdentifier
						("0609" + "2a864886f70d01010b") + // OID: sha256WithRSA
						"0500") + // NULL
					("3049" + // SEQUENCE Name
						("310b" +
							("3009" +
								("0603" + "550406") + // OID: country
								("1302" + "5553"))) + // "US"
						("3113" +
							("3011" +
								("0603" + "55040a") + // OID: organization
								("130a" + "476f6f676c6520496e63"))) + // "Google Inc"
						("3125" +
							("3023" +
								("0603" + "550403") + // OID: commonName
								("131c" + "476f6f676c6520496e7465726e657420417574686f72697479204732")))) +
					("170d" + "3137303632393031303030325a") + // UTCTime
					("170d" + "3137303730393031303030325a") + // UTCTime
					("3000") + // no revoked certs
					("a033" +
						("3031" +
							("301f" +
								("0603" + "551d23") + // OID: authority-key-id
								("0418" +
									("3016" +
										"8014" + "4add06161bbcf668b576f581b6bb621aba5a812f"))) +
							("300e" +
								("0603" + "551d1f") + // OID: unknown
								("0101ff") + // critical: true
								("0404" + "02020623"))))) +
				("300d" +
					("0609" + "2a864886f70d01010b") + // OID: sha256WithRSA
					"0500") + // NULL
				("03820101" + // BIT STRING length 0x101
					"004dcde29667973239cca344c58b72128fb5c5db03efdc75cfb7d9a0410ec03c8cd21160b449cd80224f41ca9d91529295ef7d0179ca4b08bb688cecce13cc07b20ecd87ffde1bc356554083c40bea7a387dacc54b3848b3710acf2fa613d007b12afc37f0a77082655b8dbb6683ba2fc52555e9f74bb5ba9429377ff38e193e799fc05c4c9bbcee29492945a732db67ba3575a79a83427a1f6d18d9ede01c544f3ccd68e5680a9b5418e03e1d80b3e77e69860982a4d21c6b111b07c87fe32c561e871554896b37651d5aaf42b2d092ce8d4dd4ae1d7a97091c0a06c03d71580e0557a51408513fde3012f02dac76536822a564faa2553048729633b68f1fc369")),
			wantErr: "unhandled critical extension",
		},
		{
			desc: "invalid-unknown-ext-trailing-data",
			data: ("308201c9" + // SEQUENCE CertificateList
				("3081b2" + // SEQUENCE TBSCertList
					("0201" + "01") + // version 2(0x01)
					("300d" + // SEQUENCE AlgorithmIdentifier
						("0609" + "2a864886f70d01010b") + // OID: sha256WithRSA
						"0500") + // NULL
					("3049" + // SEQUENCE Name
						("310b" +
							("3009" +
								("0603" + "550406") + // OID: country
								("1302" + "5553"))) + // "US"
						("3113" +
							("3011" +
								("0603" + "55040a") + // OID: organization
								("130a" + "476f6f676c6520496e63"))) + // "Google Inc"
						("3125" +
							("3023" +
								("0603" + "550403") + // OID: commonName
								("131c" + "476f6f676c6520496e7465726e657420417574686f72697479204732")))) +
					("170d" + "3137303632393031303030325a") + // UTCTime
					("170d" + "3137303730393031303030325a") + // UTCTime
					("3000") + // no revoked certs
					("a033" +
						("3031" +
							("301f" +
								("0603" + "551d23") + // OID: authority-key-id
								("0418" +
									("3016" +
										"8014" + "4add06161bbcf668b576f581b6bb621aba5a812f"))) +
							("300e" +
								("0603" + "551d1f") + // OID: unknown
								("010100") + // critical: false
								("0404" + "02020623"))))) +
				("300d" +
					("0609" + "2a864886f70d01010b") + // OID: sha256WithRSA
					"0500") + // NULL
				("03820101" + // BIT STRING length 0x101
					"004dcde29667973239cca344c58b72128fb5c5db03efdc75cfb7d9a0410ec03c8cd21160b449cd80224f41ca9d91529295ef7d0179ca4b08bb688cecce13cc07b20ecd87ffde1bc356554083c40bea7a387dacc54b3848b3710acf2fa613d007b12afc37f0a77082655b8dbb6683ba2fc52555e9f74bb5ba9429377ff38e193e799fc05c4c9bbcee29492945a732db67ba3575a79a83427a1f6d18d9ede01c544f3ccd68e5680a9b5418e03e1d80b3e77e69860982a4d21c6b111b07c87fe32c561e871554896b37651d5aaf42b2d092ce8d4dd4ae1d7a97091c0a06c03d71580e0557a51408513fde3012f02dac76536822a564faa2553048729633b68f1fc369") +
				"00"),
			wantErr: "trailing data",
		},
		{
			desc:    "invalid-wrong-asn1",
			data:    "0a0101",
			wantErr: "structure error",
		},
		// The following example is used as the template for other variations
		{
			desc: "valid-empty-certlist",
			data: ("308201c6" + // SEQUENCE CertificateList
				("3081af" + // SEQUENCE TBSCertList
					("0201" + "01") + // version 2(0x01)
					("300d" + // SEQUENCE AlgorithmIdentifier
						("0609" + "2a864886f70d01010b") + // OID: sha256WithRSA
						"0500") + // NULL
					("3049" + // SEQUENCE Name
						("310b" +
							("3009" +
								("0603" + "550406") + // OID: country
								("1302" + "5553"))) + // "US"
						("3113" +
							("3011" +
								("0603" + "55040a") + // OID: organization
								("130a" + "476f6f676c6520496e63"))) + // "Google Inc"
						("3125" +
							("3023" +
								("0603" + "550403") + // OID: commonName
								("131c" + "476f6f676c6520496e7465726e657420417574686f72697479204732")))) +
					("170d" + "3137303632393031303030325a") + // UTCTime
					("170d" + "3137303730393031303030325a") + // UTCTime
					("3000") + // SEQUENCE OF no revoked certs
					("a030" +
						("302e" +
							("301f" +
								("0603" + "551d23") + // OID: authority-key-id
								("0418" +
									("3016" +
										"8014" + "4add06161bbcf668b576f581b6bb621aba5a812f"))) +
							("300b" +
								("0603" + "551d14") + // OID: CRL-number
								("0404" + "02020623"))))) +
				("300d" +
					("0609" + "2a864886f70d01010b") + // OID: sha256WithRSA
					"0500") + // NULL
				("03820101" + // BIT STRING length 0x101
					"004dcde29667973239cca344c58b72128fb5c5db03efdc75cfb7d9a0410ec03c8cd21160b449cd80224f41ca9d91529295ef7d0179ca4b08bb688cecce13cc07b20ecd87ffde1bc356554083c40bea7a387dacc54b3848b3710acf2fa613d007b12afc37f0a77082655b8dbb6683ba2fc52555e9f74bb5ba9429377ff38e193e799fc05c4c9bbcee29492945a732db67ba3575a79a83427a1f6d18d9ede01c544f3ccd68e5680a9b5418e03e1d80b3e77e69860982a4d21c6b111b07c87fe32c561e871554896b37651d5aaf42b2d092ce8d4dd4ae1d7a97091c0a06c03d71580e0557a51408513fde3012f02dac76536822a564faa2553048729633b68f1fc369")),
			want: TBSCertList{
				Version: 1,
				Signature: pkix.AlgorithmIdentifier{
					Algorithm:  oidSignatureSHA256WithRSA,
					Parameters: asn1.RawValue{Class: 0, Tag: 5, Bytes: []byte{}, FullBytes: []byte{5, 0}},
				},
				Issuer: pkix.RDNSequence{
					[]pkix.AttributeTypeAndValue{
						{Type: pkix.OIDCountry, Value: "US"},
					},
					[]pkix.AttributeTypeAndValue{
						{Type: pkix.OIDOrganization, Value: "Google Inc"},
					},
					[]pkix.AttributeTypeAndValue{
						{Type: pkix.OIDCommonName, Value: "Google Internet Authority G2"},
					},
				},
				ThisUpdate:          time.Date(2017, 6, 29, 01, 0, 2, 0, time.UTC),
				NextUpdate:          time.Date(2017, 7, 9, 01, 0, 2, 0, time.UTC),
				RevokedCertificates: []*RevokedCertificate{},
				AuthorityKeyID:      fromHex("4add06161bbcf668b576f581b6bb621aba5a812f"),
				CRLNumber:           1571,
				BaseCRLNumber:       -1,
			},
		},
		{
			desc: "valid-delta-crl-indicator-ext",
			data: ("308201d6" + // SEQUENCE CertificateList
				("3081bf" + // SEQUENCE TBSCertList
					("0201" + "01") + // version 2(0x01)
					("300d" + // SEQUENCE AlgorithmIdentifier
						("0609" + "2a864886f70d01010b") + // OID: sha256WithRSA
						"0500") + // NULL
					("3049" + // SEQUENCE Name
						("310b" +
							("3009" +
								("0603" + "550406") + // OID: country
								("1302" + "5553"))) + // "US"
						("3113" +
							("3011" +
								("0603" + "55040a") + // OID: organization
								("130a" + "476f6f676c6520496e63"))) + // "Google Inc"
						("3125" +
							("3023" +
								("0603" + "550403") + // OID: commonName
								("131c" + "476f6f676c6520496e7465726e657420417574686f72697479204732")))) +
					("170d" + "3137303632393031303030325a") + // UTCTime
					("170d" + "3137303730393031303030325a") + // UTCTime
					("3000") + // SEQUENCE OF no revoked certs
					("a040" +
						("303e" +
							("301f" +
								("0603" + "551d23") + // OID: authority-key-id
								("0418" +
									("3016" +
										"8014" + "4add06161bbcf668b576f581b6bb621aba5a812f"))) +
							("300e" +
								("0603" + "551d1b") + // OID: delta-crl-indicator
								("0101ff") + // critical: true
								("0404" + "02020120")) +
							("300b" +
								("0603" + "551d14") + // OID: CRL-number
								("0404" + "02020623"))))) +
				("300d" +
					("0609" + "2a864886f70d01010b") + // OID: sha256WithRSA
					"0500") + // NULL
				("03820101" + // BIT STRING length 0x101
					"004dcde29667973239cca344c58b72128fb5c5db03efdc75cfb7d9a0410ec03c8cd21160b449cd80224f41ca9d91529295ef7d0179ca4b08bb688cecce13cc07b20ecd87ffde1bc356554083c40bea7a387dacc54b3848b3710acf2fa613d007b12afc37f0a77082655b8dbb6683ba2fc52555e9f74bb5ba9429377ff38e193e799fc05c4c9bbcee29492945a732db67ba3575a79a83427a1f6d18d9ede01c544f3ccd68e5680a9b5418e03e1d80b3e77e69860982a4d21c6b111b07c87fe32c561e871554896b37651d5aaf42b2d092ce8d4dd4ae1d7a97091c0a06c03d71580e0557a51408513fde3012f02dac76536822a564faa2553048729633b68f1fc369")),
			want: TBSCertList{
				Version: 1,
				Signature: pkix.AlgorithmIdentifier{
					Algorithm:  oidSignatureSHA256WithRSA,
					Parameters: asn1.RawValue{Class: 0, Tag: 5, Bytes: []byte{}, FullBytes: []byte{5, 0}},
				},
				Issuer: pkix.RDNSequence{
					[]pkix.AttributeTypeAndValue{
						{Type: pkix.OIDCountry, Value: "US"},
					},
					[]pkix.AttributeTypeAndValue{
						{Type: pkix.OIDOrganization, Value: "Google Inc"},
					},
					[]pkix.AttributeTypeAndValue{
						{Type: pkix.OIDCommonName, Value: "Google Internet Authority G2"},
					},
				},
				ThisUpdate:          time.Date(2017, 6, 29, 01, 0, 2, 0, time.UTC),
				NextUpdate:          time.Date(2017, 7, 9, 01, 0, 2, 0, time.UTC),
				RevokedCertificates: []*RevokedCertificate{},
				AuthorityKeyID:      fromHex("4add06161bbcf668b576f581b6bb621aba5a812f"),
				CRLNumber:           1571,
				BaseCRLNumber:       288,
			},
		},
		{
			desc: "invalid-delta-crl-indicator-ext-non-critical",
			data: ("308201d6" + // SEQUENCE CertificateList
				("3081bf" + // SEQUENCE TBSCertList
					("0201" + "01") + // version 2(0x01)
					("300d" + // SEQUENCE AlgorithmIdentifier
						("0609" + "2a864886f70d01010b") + // OID: sha256WithRSA
						"0500") + // NULL
					("3049" + // SEQUENCE Name
						("310b" +
							("3009" +
								("0603" + "550406") + // OID: country
								("1302" + "5553"))) + // "US"
						("3113" +
							("3011" +
								("0603" + "55040a") + // OID: organization
								("130a" + "476f6f676c6520496e63"))) + // "Google Inc"
						("3125" +
							("3023" +
								("0603" + "550403") + // OID: commonName
								("131c" + "476f6f676c6520496e7465726e657420417574686f72697479204732")))) +
					("170d" + "3137303632393031303030325a") + // UTCTime
					("170d" + "3137303730393031303030325a") + // UTCTime
					("3000") + // SEQUENCE OF no revoked certs
					("a040" +
						("303e" +
							("301f" +
								("0603" + "551d23") + // OID: authority-key-id
								("0418" +
									("3016" +
										"8014" + "4add06161bbcf668b576f581b6bb621aba5a812f"))) +
							("300e" +
								("0603" + "551d1b") + // OID: delta-crl-indicator
								("010100") + // INVALID: critical: false
								("0404" + "02020120")) +
							("300b" +
								("0603" + "551d14") + // OID: CRL-number
								("0404" + "02020623"))))) +
				("300d" +
					("0609" + "2a864886f70d01010b") + // OID: sha256WithRSA
					"0500") + // NULL
				("03820101" + // BIT STRING length 0x101
					"004dcde29667973239cca344c58b72128fb5c5db03efdc75cfb7d9a0410ec03c8cd21160b449cd80224f41ca9d91529295ef7d0179ca4b08bb688cecce13cc07b20ecd87ffde1bc356554083c40bea7a387dacc54b3848b3710acf2fa613d007b12afc37f0a77082655b8dbb6683ba2fc52555e9f74bb5ba9429377ff38e193e799fc05c4c9bbcee29492945a732db67ba3575a79a83427a1f6d18d9ede01c544f3ccd68e5680a9b5418e03e1d80b3e77e69860982a4d21c6b111b07c87fe32c561e871554896b37651d5aaf42b2d092ce8d4dd4ae1d7a97091c0a06c03d71580e0557a51408513fde3012f02dac76536822a564faa2553048729633b68f1fc369")),
			wantErr: "marked non-critical",
		},
		{
			desc: "invalid-delta-crl-indicator-ext-wrong-asn1",
			data: ("308201d6" + // SEQUENCE CertificateList
				("3081bf" + // SEQUENCE TBSCertList
					("0201" + "01") + // version 2(0x01)
					("300d" + // SEQUENCE AlgorithmIdentifier
						("0609" + "2a864886f70d01010b") + // OID: sha256WithRSA
						"0500") + // NULL
					("3049" + // SEQUENCE Name
						("310b" +
							("3009" +
								("0603" + "550406") + // OID: country
								("1302" + "5553"))) + // "US"
						("3113" +
							("3011" +
								("0603" + "55040a") + // OID: organization
								("130a" + "476f6f676c6520496e63"))) + // "Google Inc"
						("3125" +
							("3023" +
								("0603" + "550403") + // OID: commonName
								("131c" + "476f6f676c6520496e7465726e657420417574686f72697479204732")))) +
					("170d" + "3137303632393031303030325a") + // UTCTime
					("170d" + "3137303730393031303030325a") + // UTCTime
					("3000") + // SEQUENCE OF no revoked certs
					("a040" +
						("303e" +
							("301f" +
								("0603" + "551d23") + // OID: authority-key-id
								("0418" +
									("3016" +
										"8014" + "4add06161bbcf668b576f581b6bb621aba5a812f"))) +
							("300e" +
								("0603" + "551d1b") + // OID: delta-crl-indicator
								("0101ff") + // critical: true
								("0404" + "0a020123")) + // INVALID: tag ENUM not int
							("300b" +
								("0603" + "551d14") + // OID: CRL-number
								("0404" + "02020623"))))) +
				("300d" +
					("0609" + "2a864886f70d01010b") + // OID: sha256WithRSA
					"0500") + // NULL
				("03820101" + // BIT STRING length 0x101
					"004dcde29667973239cca344c58b72128fb5c5db03efdc75cfb7d9a0410ec03c8cd21160b449cd80224f41ca9d91529295ef7d0179ca4b08bb688cecce13cc07b20ecd87ffde1bc356554083c40bea7a387dacc54b3848b3710acf2fa613d007b12afc37f0a77082655b8dbb6683ba2fc52555e9f74bb5ba9429377ff38e193e799fc05c4c9bbcee29492945a732db67ba3575a79a83427a1f6d18d9ede01c544f3ccd68e5680a9b5418e03e1d80b3e77e69860982a4d21c6b111b07c87fe32c561e871554896b37651d5aaf42b2d092ce8d4dd4ae1d7a97091c0a06c03d71580e0557a51408513fde3012f02dac76536822a564faa2553048729633b68f1fc369")),
			wantErr: "failed to unmarshal",
		},
		{
			desc: "invalid-delta-crl-indicator-ext-trailing-data",
			data: ("308201d6" + // SEQUENCE CertificateList
				("3081bf" + // SEQUENCE TBSCertList
					("0201" + "01") + // version 2(0x01)
					("300d" + // SEQUENCE AlgorithmIdentifier
						("0609" + "2a864886f70d01010b") + // OID: sha256WithRSA
						"0500") + // NULL
					("3049" + // SEQUENCE Name
						("310b" +
							("3009" +
								("0603" + "550406") + // OID: country
								("1302" + "5553"))) + // "US"
						("3113" +
							("3011" +
								("0603" + "55040a") + // OID: organization
								("130a" + "476f6f676c6520496e63"))) + // "Google Inc"
						("3125" +
							("3023" +
								("0603" + "550403") + // OID: commonName
								("131c" + "476f6f676c6520496e7465726e657420417574686f72697479204732")))) +
					("170d" + "3137303632393031303030325a") + // UTCTime
					("170d" + "3137303730393031303030325a") + // UTCTime
					("3000") + // SEQUENCE OF no revoked certs
					("a040" +
						("303e" +
							("301f" +
								("0603" + "551d23") + // OID: authority-key-id
								("0418" +
									("3016" +
										"8014" + "4add06161bbcf668b576f581b6bb621aba5a812f"))) +
							("300e" +
								("0603" + "551d1b") + // OID: delta-crl-indicator
								("0101ff") + // critical: true
								("0404" + "020101DD")) + // INVALID: trailing data
							("300b" +
								("0603" + "551d14") + // OID: CRL-number
								("0404" + "02020623"))))) +
				("300d" +
					("0609" + "2a864886f70d01010b") + // OID: sha256WithRSA
					"0500") + // NULL
				("03820101" + // BIT STRING length 0x101
					"004dcde29667973239cca344c58b72128fb5c5db03efdc75cfb7d9a0410ec03c8cd21160b449cd80224f41ca9d91529295ef7d0179ca4b08bb688cecce13cc07b20ecd87ffde1bc356554083c40bea7a387dacc54b3848b3710acf2fa613d007b12afc37f0a77082655b8dbb6683ba2fc52555e9f74bb5ba9429377ff38e193e799fc05c4c9bbcee29492945a732db67ba3575a79a83427a1f6d18d9ede01c544f3ccd68e5680a9b5418e03e1d80b3e77e69860982a4d21c6b111b07c87fe32c561e871554896b37651d5aaf42b2d092ce8d4dd4ae1d7a97091c0a06c03d71580e0557a51408513fde3012f02dac76536822a564faa2553048729633b68f1fc369")),
			wantErr: "trailing data",
		},
		{
			desc: "invalid-delta-crl-indicator-ext-negative",
			data: ("308201d6" + // SEQUENCE CertificateList
				("3081bf" + // SEQUENCE TBSCertList
					("0201" + "01") + // version 2(0x01)
					("300d" + // SEQUENCE AlgorithmIdentifier
						("0609" + "2a864886f70d01010b") + // OID: sha256WithRSA
						"0500") + // NULL
					("3049" + // SEQUENCE Name
						("310b" +
							("3009" +
								("0603" + "550406") + // OID: country
								("1302" + "5553"))) + // "US"
						("3113" +
							("3011" +
								("0603" + "55040a") + // OID: organization
								("130a" + "476f6f676c6520496e63"))) + // "Google Inc"
						("3125" +
							("3023" +
								("0603" + "550403") + // OID: commonName
								("131c" + "476f6f676c6520496e7465726e657420417574686f72697479204732")))) +
					("170d" + "3137303632393031303030325a") + // UTCTime
					("170d" + "3137303730393031303030325a") + // UTCTime
					("3000") + // SEQUENCE OF no revoked certs
					("a040" +
						("303e" +
							("301f" +
								("0603" + "551d23") + // OID: authority-key-id
								("0418" +
									("3016" +
										"8014" + "4add06161bbcf668b576f581b6bb621aba5a812f"))) +
							("300e" +
								("0603" + "551d1b") + // OID: delta-crl-indicator
								("0101ff") + // critical: true
								("0404" + "02028120")) + // INVALID: negative base CRL
							("300b" +
								("0603" + "551d14") + // OID: CRL-number
								("0404" + "02020623"))))) +
				("300d" +
					("0609" + "2a864886f70d01010b") + // OID: sha256WithRSA
					"0500") + // NULL
				("03820101" + // BIT STRING length 0x101
					"004dcde29667973239cca344c58b72128fb5c5db03efdc75cfb7d9a0410ec03c8cd21160b449cd80224f41ca9d91529295ef7d0179ca4b08bb688cecce13cc07b20ecd87ffde1bc356554083c40bea7a387dacc54b3848b3710acf2fa613d007b12afc37f0a77082655b8dbb6683ba2fc52555e9f74bb5ba9429377ff38e193e799fc05c4c9bbcee29492945a732db67ba3575a79a83427a1f6d18d9ede01c544f3ccd68e5680a9b5418e03e1d80b3e77e69860982a4d21c6b111b07c87fe32c561e871554896b37651d5aaf42b2d092ce8d4dd4ae1d7a97091c0a06c03d71580e0557a51408513fde3012f02dac76536822a564faa2553048729633b68f1fc369")),
			wantErr: "negative",
		},
		{
			desc: "invalid-crl-number-ext-critical",
			data: ("308201c9" + // SEQUENCE CertificateList
				("3081b2" + // SEQUENCE TBSCertList
					("0201" + "01") + // version 2(0x01)
					("300d" + // SEQUENCE AlgorithmIdentifier
						("0609" + "2a864886f70d01010b") + // OID: sha256WithRSA
						"0500") + // NULL
					("3049" + // SEQUENCE Name
						("310b" +
							("3009" +
								("0603" + "550406") + // OID: country
								("1302" + "5553"))) + // "US"
						("3113" +
							("3011" +
								("0603" + "55040a") + // OID: organization
								("130a" + "476f6f676c6520496e63"))) + // "Google Inc"
						("3125" +
							("3023" +
								("0603" + "550403") + // OID: commonName
								("131c" + "476f6f676c6520496e7465726e657420417574686f72697479204732")))) +
					("170d" + "3137303632393031303030325a") + // UTCTime
					("170d" + "3137303730393031303030325a") + // UTCTime
					("3000") + // no revoked certs
					("a033" +
						("3031" +
							("301f" +
								("0603" + "551d23") + // OID: authority-key-id
								("0418" +
									("3016" +
										"8014" + "4add06161bbcf668b576f581b6bb621aba5a812f"))) +
							("300e" +
								("0603" + "551d14") + // OID: CRL-number
								("0101ff") + // critical: true
								("0404" + "02020623"))))) +
				("300d" +
					("0609" + "2a864886f70d01010b") + // OID: sha256WithRSA
					"0500") + // NULL
				("03820101" + // BIT STRING length 0x101
					"004dcde29667973239cca344c58b72128fb5c5db03efdc75cfb7d9a0410ec03c8cd21160b449cd80224f41ca9d91529295ef7d0179ca4b08bb688cecce13cc07b20ecd87ffde1bc356554083c40bea7a387dacc54b3848b3710acf2fa613d007b12afc37f0a77082655b8dbb6683ba2fc52555e9f74bb5ba9429377ff38e193e799fc05c4c9bbcee29492945a732db67ba3575a79a83427a1f6d18d9ede01c544f3ccd68e5680a9b5418e03e1d80b3e77e69860982a4d21c6b111b07c87fe32c561e871554896b37651d5aaf42b2d092ce8d4dd4ae1d7a97091c0a06c03d71580e0557a51408513fde3012f02dac76536822a564faa2553048729633b68f1fc369")),
			wantErr: "marked critical",
		},
		{
			desc: "invalid-crl-number-ext-trailing-data",
			data: ("308201c6" + // SEQUENCE CertificateList
				("3081af" + // SEQUENCE TBSCertList
					("0201" + "01") + // version 2(0x01)
					("300d" + // SEQUENCE AlgorithmIdentifier
						("0609" + "2a864886f70d01010b") + // OID: sha256WithRSA
						"0500") + // NULL
					("3049" + // SEQUENCE Name
						("310b" +
							("3009" +
								("0603" + "550406") + // OID: country
								("1302" + "5553"))) + // "US"
						("3113" +
							("3011" +
								("0603" + "55040a") + // OID: organization
								("130a" + "476f6f676c6520496e63"))) + // "Google Inc"
						("3125" +
							("3023" +
								("0603" + "550403") + // OID: commonName
								("131c" + "476f6f676c6520496e7465726e657420417574686f72697479204732")))) +
					("170d" + "3137303632393031303030325a") + // UTCTime
					("170d" + "3137303730393031303030325a") + // UTCTime
					("3000") + // no revoked certs
					("a030" +
						("302e" +
							("301f" +
								("0603" + "551d23") + // OID: authority-key-id
								("0418" +
									("3016" +
										"8014" + "4add06161bbcf668b576f581b6bb621aba5a812f"))) +
							("300b" +
								("0603" + "551d14") + // OID: CRL-number
								("0404" + "0201" + "0623"))))) + // INVALID: trailing data
				("300d" +
					("0609" + "2a864886f70d01010b") + // OID: sha256WithRSA
					"0500") + // NULL
				("03820101" + // BIT STRING length 0x101
					"004dcde29667973239cca344c58b72128fb5c5db03efdc75cfb7d9a0410ec03c8cd21160b449cd80224f41ca9d91529295ef7d0179ca4b08bb688cecce13cc07b20ecd87ffde1bc356554083c40bea7a387dacc54b3848b3710acf2fa613d007b12afc37f0a77082655b8dbb6683ba2fc52555e9f74bb5ba9429377ff38e193e799fc05c4c9bbcee29492945a732db67ba3575a79a83427a1f6d18d9ede01c544f3ccd68e5680a9b5418e03e1d80b3e77e69860982a4d21c6b111b07c87fe32c561e871554896b37651d5aaf42b2d092ce8d4dd4ae1d7a97091c0a06c03d71580e0557a51408513fde3012f02dac76536822a564faa2553048729633b68f1fc369")),
			wantErr: "trailing data",
		},
		{
			desc: "invalid-crl-number-ext-negative",
			data: ("308201c6" + // SEQUENCE CertificateList
				("3081af" + // SEQUENCE TBSCertList
					("0201" + "01") + // version 2(0x01)
					("300d" + // SEQUENCE AlgorithmIdentifier
						("0609" + "2a864886f70d01010b") + // OID: sha256WithRSA
						"0500") + // NULL
					("3049" + // SEQUENCE Name
						("310b" +
							("3009" +
								("0603" + "550406") + // OID: country
								("1302" + "5553"))) + // "US"
						("3113" +
							("3011" +
								("0603" + "55040a") + // OID: organization
								("130a" + "476f6f676c6520496e63"))) + // "Google Inc"
						("3125" +
							("3023" +
								("0603" + "550403") + // OID: commonName
								("131c" + "476f6f676c6520496e7465726e657420417574686f72697479204732")))) +
					("170d" + "3137303632393031303030325a") + // UTCTime
					("170d" + "3137303730393031303030325a") + // UTCTime
					("3000") + // no revoked certs
					("a030" +
						("302e" +
							("301f" +
								("0603" + "551d23") + // OID: authority-key-id
								("0418" +
									("3016" +
										"8014" + "4add06161bbcf668b576f581b6bb621aba5a812f"))) +
							("300b" +
								("0603" + "551d14") + // OID: CRL-number
								("0404" + "0202" + "8623"))))) + // INVALID: negative value
				("300d" +
					("0609" + "2a864886f70d01010b") + // OID: sha256WithRSA
					"0500") + // NULL
				("03820101" + // BIT STRING length 0x101
					"004dcde29667973239cca344c58b72128fb5c5db03efdc75cfb7d9a0410ec03c8cd21160b449cd80224f41ca9d91529295ef7d0179ca4b08bb688cecce13cc07b20ecd87ffde1bc356554083c40bea7a387dacc54b3848b3710acf2fa613d007b12afc37f0a77082655b8dbb6683ba2fc52555e9f74bb5ba9429377ff38e193e799fc05c4c9bbcee29492945a732db67ba3575a79a83427a1f6d18d9ede01c544f3ccd68e5680a9b5418e03e1d80b3e77e69860982a4d21c6b111b07c87fe32c561e871554896b37651d5aaf42b2d092ce8d4dd4ae1d7a97091c0a06c03d71580e0557a51408513fde3012f02dac76536822a564faa2553048729633b68f1fc369")),
			wantErr: "negative",
		},
		{
			desc: "invalid-crl-number-ext-wrong-asn1",
			data: ("308201c6" + // SEQUENCE CertificateList
				("3081af" + // SEQUENCE TBSCertList
					("0201" + "01") + // version 2(0x01)
					("300d" + // SEQUENCE AlgorithmIdentifier
						("0609" + "2a864886f70d01010b") + // OID: sha256WithRSA
						"0500") + // NULL
					("3049" + // SEQUENCE Name
						("310b" +
							("3009" +
								("0603" + "550406") + // OID: country
								("1302" + "5553"))) + // "US"
						("3113" +
							("3011" +
								("0603" + "55040a") + // OID: organization
								("130a" + "476f6f676c6520496e63"))) + // "Google Inc"
						("3125" +
							("3023" +
								("0603" + "550403") + // OID: commonName
								("131c" + "476f6f676c6520496e7465726e657420417574686f72697479204732")))) +
					("170d" + "3137303632393031303030325a") + // UTCTime
					("170d" + "3137303730393031303030325a") + // UTCTime
					("3000") + // no revoked certs
					("a030" +
						("302e" +
							("301f" +
								("0603" + "551d23") + // OID: authority-key-id
								("0418" +
									("3016" +
										"8014" + "4add06161bbcf668b576f581b6bb621aba5a812f"))) +
							("300b" +
								("0603" + "551d14") + // OID: CRL-number
								("0404" + "0a02" + "0623"))))) + // INVALID: enum tag
				("300d" +
					("0609" + "2a864886f70d01010b") + // OID: sha256WithRSA
					"0500") + // NULL
				("03820101" + // BIT STRING length 0x101
					"004dcde29667973239cca344c58b72128fb5c5db03efdc75cfb7d9a0410ec03c8cd21160b449cd80224f41ca9d91529295ef7d0179ca4b08bb688cecce13cc07b20ecd87ffde1bc356554083c40bea7a387dacc54b3848b3710acf2fa613d007b12afc37f0a77082655b8dbb6683ba2fc52555e9f74bb5ba9429377ff38e193e799fc05c4c9bbcee29492945a732db67ba3575a79a83427a1f6d18d9ede01c544f3ccd68e5680a9b5418e03e1d80b3e77e69860982a4d21c6b111b07c87fe32c561e871554896b37651d5aaf42b2d092ce8d4dd4ae1d7a97091c0a06c03d71580e0557a51408513fde3012f02dac76536822a564faa2553048729633b68f1fc369")),
			wantErr: "structure error",
		},
		{
			desc: "invalid-auth-key-id-ext-trailing-data",
			data: ("308201c6" + // SEQUENCE CertificateList
				("3081af" + // SEQUENCE TBSCertList
					("0201" + "01") + // version 2(0x01)
					("300d" + // SEQUENCE AlgorithmIdentifier
						("0609" + "2a864886f70d01010b") + // OID: sha256WithRSA
						"0500") + // NULL
					("3049" + // SEQUENCE Name
						("310b" +
							("3009" +
								("0603" + "550406") + // OID: country
								("1302" + "5553"))) + // "US"
						("3113" +
							("3011" +
								("0603" + "55040a") + // OID: organization
								("130a" + "476f6f676c6520496e63"))) + // "Google Inc"
						("3125" +
							("3023" +
								("0603" + "550403") + // OID: commonName
								("131c" + "476f6f676c6520496e7465726e657420417574686f72697479204732")))) +
					("170d" + "3137303632393031303030325a") + // UTCTime
					("170d" + "3137303730393031303030325a") + // UTCTime
					("3000") + // no revoked certs
					("a030" +
						("302e" +
							("301f" +
								("0603" + "551d23") + // OID: authority-key-id
								("0418" +
									("3015" +
										"8013" + "4add06161bbcf668b576f581b6bb621aba5a812f"))) + // INVALID: trailing data
							("300b" +
								("0603" + "551d14") + // OID: CRL-number
								("0404" + "0202" + "0623"))))) +
				("300d" +
					("0609" + "2a864886f70d01010b") + // OID: sha256WithRSA
					"0500") + // NULL
				("03820101" + // BIT STRING length 0x101
					"004dcde29667973239cca344c58b72128fb5c5db03efdc75cfb7d9a0410ec03c8cd21160b449cd80224f41ca9d91529295ef7d0179ca4b08bb688cecce13cc07b20ecd87ffde1bc356554083c40bea7a387dacc54b3848b3710acf2fa613d007b12afc37f0a77082655b8dbb6683ba2fc52555e9f74bb5ba9429377ff38e193e799fc05c4c9bbcee29492945a732db67ba3575a79a83427a1f6d18d9ede01c544f3ccd68e5680a9b5418e03e1d80b3e77e69860982a4d21c6b111b07c87fe32c561e871554896b37651d5aaf42b2d092ce8d4dd4ae1d7a97091c0a06c03d71580e0557a51408513fde3012f02dac76536822a564faa2553048729633b68f1fc369")),
			wantErr: "trailing data",
		},
		{
			desc: "invalid-auth-key-id-ext-wrong-asn1",
			data: ("308201c6" + // SEQUENCE CertificateList
				("3081af" + // SEQUENCE TBSCertList
					("0201" + "01") + // version 2(0x01)
					("300d" + // SEQUENCE AlgorithmIdentifier
						("0609" + "2a864886f70d01010b") + // OID: sha256WithRSA
						"0500") + // NULL
					("3049" + // SEQUENCE Name
						("310b" +
							("3009" +
								("0603" + "550406") + // OID: country
								("1302" + "5553"))) + // "US"
						("3113" +
							("3011" +
								("0603" + "55040a") + // OID: organization
								("130a" + "476f6f676c6520496e63"))) + // "Google Inc"
						("3125" +
							("3023" +
								("0603" + "550403") + // OID: commonName
								("131c" + "476f6f676c6520496e7465726e657420417574686f72697479204732")))) +
					("170d" + "3137303632393031303030325a") + // UTCTime
					("170d" + "3137303730393031303030325a") + // UTCTime
					("3000") + // no revoked certs
					("a030" +
						("302e" +
							("301f" +
								("0603" + "551d23") + // OID: authority-key-id
								("0418" +
									("3116" + // INVALID: set not sequence
										"8014" + "4add06161bbcf668b576f581b6bb621aba5a812f"))) +
							("300b" +
								("0603" + "551d14") + // OID: CRL-number
								("0404" + "0202" + "0623"))))) +
				("300d" +
					("0609" + "2a864886f70d01010b") + // OID: sha256WithRSA
					"0500") + // NULL
				("03820101" + // BIT STRING length 0x101
					"004dcde29667973239cca344c58b72128fb5c5db03efdc75cfb7d9a0410ec03c8cd21160b449cd80224f41ca9d91529295ef7d0179ca4b08bb688cecce13cc07b20ecd87ffde1bc356554083c40bea7a387dacc54b3848b3710acf2fa613d007b12afc37f0a77082655b8dbb6683ba2fc52555e9f74bb5ba9429377ff38e193e799fc05c4c9bbcee29492945a732db67ba3575a79a83427a1f6d18d9ede01c544f3ccd68e5680a9b5418e03e1d80b3e77e69860982a4d21c6b111b07c87fe32c561e871554896b37651d5aaf42b2d092ce8d4dd4ae1d7a97091c0a06c03d71580e0557a51408513fde3012f02dac76536822a564faa2553048729633b68f1fc369")),
			wantErr: "failed to unmarshal",
		},

		{
			desc: "valid-auth-info-access-ext-ca-issuer",
			data: ("308201ee" + // SEQUENCE CertificateList
				("3081d7" + // SEQUENCE TBSCertList
					("0201" + "01") + // version 2(0x01)
					("300d" + // SEQUENCE AlgorithmIdentifier
						("0609" + "2a864886f70d01010b") + // OID: sha256WithRSA
						"0500") + // NULL
					("3049" + // SEQUENCE Name
						("310b" +
							("3009" +
								("0603" + "550406") + // OID: country
								("1302" + "5553"))) + // "US"
						("3113" +
							("3011" +
								("0603" + "55040a") + // OID: organization
								("130a" + "476f6f676c6520496e63"))) + // "Google Inc"
						("3125" +
							("3023" +
								("0603" + "550403") + // OID: commonName
								("131c" + "476f6f676c6520496e7465726e657420417574686f72697479204732")))) +
					("170d" + "3137303632393031303030325a") + // UTCTime
					("170d" + "3137303730393031303030325a") + // UTCTime
					("3000") + // SEQUENCE OF no revoked certs
					("a058" +
						("3056" +
							("301f" +
								("0603" + "551d23") + // OID: authority-key-id
								("0418" +
									("3016" +
										"8014" + "4add06161bbcf668b576f581b6bb621aba5a812f"))) +
							("3026" +
								("0608" + "2b06010505070101") + // OID: authority-info-access
								("041a" +
									("3018" +
										("3016" +
											("0608" + "2b06010505073002") + // OID: CA issuers
											("860a" + "687474703a2f2f777777"))))) + // 'http://www'
							("300b" +
								("0603" + "551d14") + // OID: CRL-number
								("0404" + "02020623"))))) +
				("300d" +
					("0609" + "2a864886f70d01010b") + // OID: sha256WithRSA
					"0500") + // NULL
				("03820101" + // BIT STRING length 0x101
					"004dcde29667973239cca344c58b72128fb5c5db03efdc75cfb7d9a0410ec03c8cd21160b449cd80224f41ca9d91529295ef7d0179ca4b08bb688cecce13cc07b20ecd87ffde1bc356554083c40bea7a387dacc54b3848b3710acf2fa613d007b12afc37f0a77082655b8dbb6683ba2fc52555e9f74bb5ba9429377ff38e193e799fc05c4c9bbcee29492945a732db67ba3575a79a83427a1f6d18d9ede01c544f3ccd68e5680a9b5418e03e1d80b3e77e69860982a4d21c6b111b07c87fe32c561e871554896b37651d5aaf42b2d092ce8d4dd4ae1d7a97091c0a06c03d71580e0557a51408513fde3012f02dac76536822a564faa2553048729633b68f1fc369")),
			want: TBSCertList{
				Version: 1,
				Signature: pkix.AlgorithmIdentifier{
					Algorithm:  oidSignatureSHA256WithRSA,
					Parameters: asn1.RawValue{Class: 0, Tag: 5, Bytes: []byte{}, FullBytes: []byte{5, 0}},
				},
				Issuer: pkix.RDNSequence{
					[]pkix.AttributeTypeAndValue{
						{Type: pkix.OIDCountry, Value: "US"},
					},
					[]pkix.AttributeTypeAndValue{
						{Type: pkix.OIDOrganization, Value: "Google Inc"},
					},
					[]pkix.AttributeTypeAndValue{
						{Type: pkix.OIDCommonName, Value: "Google Internet Authority G2"},
					},
				},
				ThisUpdate:            time.Date(2017, 6, 29, 01, 0, 2, 0, time.UTC),
				NextUpdate:            time.Date(2017, 7, 9, 01, 0, 2, 0, time.UTC),
				RevokedCertificates:   []*RevokedCertificate{},
				AuthorityKeyID:        fromHex("4add06161bbcf668b576f581b6bb621aba5a812f"),
				CRLNumber:             1571,
				BaseCRLNumber:         -1,
				IssuingCertificateURL: []string{"http://www"},
			},
		},
		{
			desc: "valid-auth-info-access-ext-ocsp-server",
			data: ("308201ee" + // SEQUENCE CertificateList
				("3081d7" + // SEQUENCE TBSCertList
					("0201" + "01") + // version 2(0x01)
					("300d" + // SEQUENCE AlgorithmIdentifier
						("0609" + "2a864886f70d01010b") + // OID: sha256WithRSA
						"0500") + // NULL
					("3049" + // SEQUENCE Name
						("310b" +
							("3009" +
								("0603" + "550406") + // OID: country
								("1302" + "5553"))) + // "US"
						("3113" +
							("3011" +
								("0603" + "55040a") + // OID: organization
								("130a" + "476f6f676c6520496e63"))) + // "Google Inc"
						("3125" +
							("3023" +
								("0603" + "550403") + // OID: commonName
								("131c" + "476f6f676c6520496e7465726e657420417574686f72697479204732")))) +
					("170d" + "3137303632393031303030325a") + // UTCTime
					("170d" + "3137303730393031303030325a") + // UTCTime
					("3000") + // SEQUENCE OF no revoked certs
					("a058" +
						("3056" +
							("301f" +
								("0603" + "551d23") + // OID: authority-key-id
								("0418" +
									("3016" +
										"8014" + "4add06161bbcf668b576f581b6bb621aba5a812f"))) +
							("3026" +
								("0608" + "2b06010505070101") + // OID: authority-info-access
								("041a" +
									("3018" +
										("3016" +
											("0608" + "2b06010505073001") + // OID: OCSP
											("860a" + "687474703a2f2f777777"))))) + // 'http://www'
							("300b" +
								("0603" + "551d14") + // OID: CRL-number
								("0404" + "02020623"))))) +
				("300d" +
					("0609" + "2a864886f70d01010b") + // OID: sha256WithRSA
					"0500") + // NULL
				("03820101" + // BIT STRING length 0x101
					"004dcde29667973239cca344c58b72128fb5c5db03efdc75cfb7d9a0410ec03c8cd21160b449cd80224f41ca9d91529295ef7d0179ca4b08bb688cecce13cc07b20ecd87ffde1bc356554083c40bea7a387dacc54b3848b3710acf2fa613d007b12afc37f0a77082655b8dbb6683ba2fc52555e9f74bb5ba9429377ff38e193e799fc05c4c9bbcee29492945a732db67ba3575a79a83427a1f6d18d9ede01c544f3ccd68e5680a9b5418e03e1d80b3e77e69860982a4d21c6b111b07c87fe32c561e871554896b37651d5aaf42b2d092ce8d4dd4ae1d7a97091c0a06c03d71580e0557a51408513fde3012f02dac76536822a564faa2553048729633b68f1fc369")),
			want: TBSCertList{
				Version: 1,
				Signature: pkix.AlgorithmIdentifier{
					Algorithm:  oidSignatureSHA256WithRSA,
					Parameters: asn1.RawValue{Class: 0, Tag: 5, Bytes: []byte{}, FullBytes: []byte{5, 0}},
				},
				Issuer: pkix.RDNSequence{
					[]pkix.AttributeTypeAndValue{
						{Type: pkix.OIDCountry, Value: "US"},
					},
					[]pkix.AttributeTypeAndValue{
						{Type: pkix.OIDOrganization, Value: "Google Inc"},
					},
					[]pkix.AttributeTypeAndValue{
						{Type: pkix.OIDCommonName, Value: "Google Internet Authority G2"},
					},
				},
				ThisUpdate:          time.Date(2017, 6, 29, 01, 0, 2, 0, time.UTC),
				NextUpdate:          time.Date(2017, 7, 9, 01, 0, 2, 0, time.UTC),
				RevokedCertificates: []*RevokedCertificate{},
				AuthorityKeyID:      fromHex("4add06161bbcf668b576f581b6bb621aba5a812f"),
				CRLNumber:           1571,
				BaseCRLNumber:       -1,
				OCSPServer:          []string{"http://www"},
			},
		},
		{
			desc: "valid-auth-info-access-ext-non-uri-ignored",
			data: ("308201ee" + // SEQUENCE CertificateList
				("3081d7" + // SEQUENCE TBSCertList
					("0201" + "01") + // version 2(0x01)
					("300d" + // SEQUENCE AlgorithmIdentifier
						("0609" + "2a864886f70d01010b") + // OID: sha256WithRSA
						"0500") + // NULL
					("3049" + // SEQUENCE Name
						("310b" +
							("3009" +
								("0603" + "550406") + // OID: country
								("1302" + "5553"))) + // "US"
						("3113" +
							("3011" +
								("0603" + "55040a") + // OID: organization
								("130a" + "476f6f676c6520496e63"))) + // "Google Inc"
						("3125" +
							("3023" +
								("0603" + "550403") + // OID: commonName
								("131c" + "476f6f676c6520496e7465726e657420417574686f72697479204732")))) +
					("170d" + "3137303632393031303030325a") + // UTCTime
					("170d" + "3137303730393031303030325a") + // UTCTime
					("3000") + // SEQUENCE OF no revoked certs
					("a058" +
						("3056" +
							("301f" +
								("0603" + "551d23") + // OID: authority-key-id
								("0418" +
									("3016" +
										"8014" + "4add06161bbcf668b576f581b6bb621aba5a812f"))) +
							("3026" +
								("0608" + "2b06010505070101") + // OID: authority-info-access
								("041a" +
									("3018" +
										("3016" +
											("0608" + "2b06010505073001") + // OID: OCSP
											("820a" + "687474703a2f2f777777"))))) + // dNSName: 'http://www'
							("300b" +
								("0603" + "551d14") + // OID: CRL-number
								("0404" + "02020623"))))) +
				("300d" +
					("0609" + "2a864886f70d01010b") + // OID: sha256WithRSA
					"0500") + // NULL
				("03820101" + // BIT STRING length 0x101
					"004dcde29667973239cca344c58b72128fb5c5db03efdc75cfb7d9a0410ec03c8cd21160b449cd80224f41ca9d91529295ef7d0179ca4b08bb688cecce13cc07b20ecd87ffde1bc356554083c40bea7a387dacc54b3848b3710acf2fa613d007b12afc37f0a77082655b8dbb6683ba2fc52555e9f74bb5ba9429377ff38e193e799fc05c4c9bbcee29492945a732db67ba3575a79a83427a1f6d18d9ede01c544f3ccd68e5680a9b5418e03e1d80b3e77e69860982a4d21c6b111b07c87fe32c561e871554896b37651d5aaf42b2d092ce8d4dd4ae1d7a97091c0a06c03d71580e0557a51408513fde3012f02dac76536822a564faa2553048729633b68f1fc369")),
			want: TBSCertList{
				Version: 1,
				Signature: pkix.AlgorithmIdentifier{
					Algorithm:  oidSignatureSHA256WithRSA,
					Parameters: asn1.RawValue{Class: 0, Tag: 5, Bytes: []byte{}, FullBytes: []byte{5, 0}},
				},
				Issuer: pkix.RDNSequence{
					[]pkix.AttributeTypeAndValue{
						{Type: pkix.OIDCountry, Value: "US"},
					},
					[]pkix.AttributeTypeAndValue{
						{Type: pkix.OIDOrganization, Value: "Google Inc"},
					},
					[]pkix.AttributeTypeAndValue{
						{Type: pkix.OIDCommonName, Value: "Google Internet Authority G2"},
					},
				},
				ThisUpdate:          time.Date(2017, 6, 29, 01, 0, 2, 0, time.UTC),
				NextUpdate:          time.Date(2017, 7, 9, 01, 0, 2, 0, time.UTC),
				RevokedCertificates: []*RevokedCertificate{},
				AuthorityKeyID:      fromHex("4add06161bbcf668b576f581b6bb621aba5a812f"),
				CRLNumber:           1571,
				BaseCRLNumber:       -1,
			},
		},
		{
			desc: "invalid-auth-info-access-ext-wrong-asn1",
			data: ("308201ee" + // SEQUENCE CertificateList
				("3081d7" + // SEQUENCE TBSCertList
					("0201" + "01") + // version 2(0x01)
					("300d" + // SEQUENCE AlgorithmIdentifier
						("0609" + "2a864886f70d01010b") + // OID: sha256WithRSA
						"0500") + // NULL
					("3049" + // SEQUENCE Name
						("310b" +
							("3009" +
								("0603" + "550406") + // OID: country
								("1302" + "5553"))) + // "US"
						("3113" +
							("3011" +
								("0603" + "55040a") + // OID: organization
								("130a" + "476f6f676c6520496e63"))) + // "Google Inc"
						("3125" +
							("3023" +
								("0603" + "550403") + // OID: commonName
								("131c" + "476f6f676c6520496e7465726e657420417574686f72697479204732")))) +
					("170d" + "3137303632393031303030325a") + // UTCTime
					("170d" + "3137303730393031303030325a") + // UTCTime
					("3000") + // SEQUENCE OF no revoked certs
					("a058" +
						("3056" +
							("301f" +
								("0603" + "551d23") + // OID: authority-key-id
								("0418" +
									("3016" +
										"8014" + "4add06161bbcf668b576f581b6bb621aba5a812f"))) +
							("3026" +
								("0608" + "2b06010505070101") + // OID: authority-info-access
								("041a" +
									("3018" +
										("3116" + // INVALID: set not sequence
											("0608" + "2b06010505073002") + // OID: CA issuers
											("860a" + "687474703a2f2f777777"))))) + // 'http://www'
							("300b" +
								("0603" + "551d14") + // OID: CRL-number
								("0404" + "02020623"))))) +
				("300d" +
					("0609" + "2a864886f70d01010b") + // OID: sha256WithRSA
					"0500") + // NULL
				("03820101" + // BIT STRING length 0x101
					"004dcde29667973239cca344c58b72128fb5c5db03efdc75cfb7d9a0410ec03c8cd21160b449cd80224f41ca9d91529295ef7d0179ca4b08bb688cecce13cc07b20ecd87ffde1bc356554083c40bea7a387dacc54b3848b3710acf2fa613d007b12afc37f0a77082655b8dbb6683ba2fc52555e9f74bb5ba9429377ff38e193e799fc05c4c9bbcee29492945a732db67ba3575a79a83427a1f6d18d9ede01c544f3ccd68e5680a9b5418e03e1d80b3e77e69860982a4d21c6b111b07c87fe32c561e871554896b37651d5aaf42b2d092ce8d4dd4ae1d7a97091c0a06c03d71580e0557a51408513fde3012f02dac76536822a564faa2553048729633b68f1fc369")),
			wantErr: "failed to unmarshal",
		},
		{
			desc: "invalid-auth-info-access-ext-trailing-data",
			data: ("308201ee" + // SEQUENCE CertificateList
				("3081d7" + // SEQUENCE TBSCertList
					("0201" + "01") + // version 2(0x01)
					("300d" + // SEQUENCE AlgorithmIdentifier
						("0609" + "2a864886f70d01010b") + // OID: sha256WithRSA
						"0500") + // NULL
					("3049" + // SEQUENCE Name
						("310b" +
							("3009" +
								("0603" + "550406") + // OID: country
								("1302" + "5553"))) + // "US"
						("3113" +
							("3011" +
								("0603" + "55040a") + // OID: organization
								("130a" + "476f6f676c6520496e63"))) + // "Google Inc"
						("3125" +
							("3023" +
								("0603" + "550403") + // OID: commonName
								("131c" + "476f6f676c6520496e7465726e657420417574686f72697479204732")))) +
					("170d" + "3137303632393031303030325a") + // UTCTime
					("170d" + "3137303730393031303030325a") + // UTCTime
					("3000") + // SEQUENCE OF no revoked certs
					("a058" +
						("3056" +
							("301f" +
								("0603" + "551d23") + // OID: authority-key-id
								("0418" +
									("3016" +
										"8014" + "4add06161bbcf668b576f581b6bb621aba5a812f"))) +
							("3026" +
								("0608" + "2b06010505070101") + // OID: authority-info-access
								("041a" +
									("3017" +
										("3015" +
											("0608" + "2b06010505073002") + // OID: CA issuers
											("8609" + "687474703a2f2f7777"))) + "77")) + // INVALID: trailing data
							("300b" +
								("0603" + "551d14") + // OID: CRL-number
								("0404" + "02020623"))))) +
				("300d" +
					("0609" + "2a864886f70d01010b") + // OID: sha256WithRSA
					"0500") + // NULL
				("03820101" + // BIT STRING length 0x101
					"004dcde29667973239cca344c58b72128fb5c5db03efdc75cfb7d9a0410ec03c8cd21160b449cd80224f41ca9d91529295ef7d0179ca4b08bb688cecce13cc07b20ecd87ffde1bc356554083c40bea7a387dacc54b3848b3710acf2fa613d007b12afc37f0a77082655b8dbb6683ba2fc52555e9f74bb5ba9429377ff38e193e799fc05c4c9bbcee29492945a732db67ba3575a79a83427a1f6d18d9ede01c544f3ccd68e5680a9b5418e03e1d80b3e77e69860982a4d21c6b111b07c87fe32c561e871554896b37651d5aaf42b2d092ce8d4dd4ae1d7a97091c0a06c03d71580e0557a51408513fde3012f02dac76536822a564faa2553048729633b68f1fc369")),
			wantErr: "trailing data",
		},
		{
			desc: "valid-issuer-alt-name-ext",
			data: ("308201d6" + // SEQUENCE CertificateList
				("3081bf" + // SEQUENCE TBSCertList
					("0201" + "01") + // version 2(0x01)
					("300d" + // SEQUENCE AlgorithmIdentifier
						("0609" + "2a864886f70d01010b") + // OID: sha256WithRSA
						"0500") + // NULL
					("3049" + // SEQUENCE Name
						("310b" +
							("3009" +
								("0603" + "550406") + // OID: country
								("1302" + "5553"))) + // "US"
						("3113" +
							("3011" +
								("0603" + "55040a") + // OID: organization
								("130a" + "476f6f676c6520496e63"))) + // "Google Inc"
						("3125" +
							("3023" +
								("0603" + "550403") + // OID: commonName
								("131c" + "476f6f676c6520496e7465726e657420417574686f72697479204732")))) +
					("170d" + "3137303632393031303030325a") + // UTCTime
					("170d" + "3137303730393031303030325a") + // UTCTime
					("3000") + // SEQUENCE OF no revoked certs
					("a040" +
						("303e" +
							("301f" +
								("0603" + "551d23") + // OID: authority-key-id
								("0418" +
									("3016" +
										"8014" + "4add06161bbcf668b576f581b6bb621aba5a812f"))) +
							("300e" +
								("0603" + "551d12") + // OID: issuer-alt-name
								("0407" +
									("3005" +
										"8203" + "777777"))) + // [2] dNSName = 'www'
							("300b" +
								("0603" + "551d14") + // OID: CRL-number
								("0404" + "02020623"))))) +
				("300d" +
					("0609" + "2a864886f70d01010b") + // OID: sha256WithRSA
					"0500") + // NULL
				("03820101" + // BIT STRING length 0x101
					"004dcde29667973239cca344c58b72128fb5c5db03efdc75cfb7d9a0410ec03c8cd21160b449cd80224f41ca9d91529295ef7d0179ca4b08bb688cecce13cc07b20ecd87ffde1bc356554083c40bea7a387dacc54b3848b3710acf2fa613d007b12afc37f0a77082655b8dbb6683ba2fc52555e9f74bb5ba9429377ff38e193e799fc05c4c9bbcee29492945a732db67ba3575a79a83427a1f6d18d9ede01c544f3ccd68e5680a9b5418e03e1d80b3e77e69860982a4d21c6b111b07c87fe32c561e871554896b37651d5aaf42b2d092ce8d4dd4ae1d7a97091c0a06c03d71580e0557a51408513fde3012f02dac76536822a564faa2553048729633b68f1fc369")),
			want: TBSCertList{
				Version: 1,
				Signature: pkix.AlgorithmIdentifier{
					Algorithm:  oidSignatureSHA256WithRSA,
					Parameters: asn1.RawValue{Class: 0, Tag: 5, Bytes: []byte{}, FullBytes: []byte{5, 0}},
				},
				Issuer: pkix.RDNSequence{
					[]pkix.AttributeTypeAndValue{
						{Type: pkix.OIDCountry, Value: "US"},
					},
					[]pkix.AttributeTypeAndValue{
						{Type: pkix.OIDOrganization, Value: "Google Inc"},
					},
					[]pkix.AttributeTypeAndValue{
						{Type: pkix.OIDCommonName, Value: "Google Internet Authority G2"},
					},
				},
				ThisUpdate:          time.Date(2017, 6, 29, 01, 0, 2, 0, time.UTC),
				NextUpdate:          time.Date(2017, 7, 9, 01, 0, 2, 0, time.UTC),
				RevokedCertificates: []*RevokedCertificate{},
				AuthorityKeyID:      fromHex("4add06161bbcf668b576f581b6bb621aba5a812f"),
				CRLNumber:           1571,
				BaseCRLNumber:       -1,
				IssuerAltNames:      GeneralNames{DNSNames: []string{"www"}},
			},
		},
		{
			desc: "invalid-issuer-alt-name-ext",
			data: ("308201d6" + // SEQUENCE CertificateList
				("3081bf" + // SEQUENCE TBSCertList
					("0201" + "01") + // version 2(0x01)
					("300d" + // SEQUENCE AlgorithmIdentifier
						("0609" + "2a864886f70d01010b") + // OID: sha256WithRSA
						"0500") + // NULL
					("3049" + // SEQUENCE Name
						("310b" +
							("3009" +
								("0603" + "550406") + // OID: country
								("1302" + "5553"))) + // "US"
						("3113" +
							("3011" +
								("0603" + "55040a") + // OID: organization
								("130a" + "476f6f676c6520496e63"))) + // "Google Inc"
						("3125" +
							("3023" +
								("0603" + "550403") + // OID: commonName
								("131c" + "476f6f676c6520496e7465726e657420417574686f72697479204732")))) +
					("170d" + "3137303632393031303030325a") + // UTCTime
					("170d" + "3137303730393031303030325a") + // UTCTime
					("3000") + // SEQUENCE OF no revoked certs
					("a040" +
						("303e" +
							("301f" +
								("0603" + "551d23") + // OID: authority-key-id
								("0418" +
									("3016" +
										"8014" + "4add06161bbcf668b576f581b6bb621aba5a812f"))) +
							("300e" +
								("0603" + "551d12") + // OID: issuer-alt-name
								("0407" +
									("3005" +
										"8903" + "777777"))) + // INVALID: tag 9 not used
							("300b" +
								("0603" + "551d14") + // OID: CRL-number
								("0404" + "02020623"))))) +
				("300d" +
					("0609" + "2a864886f70d01010b") + // OID: sha256WithRSA
					"0500") + // NULL
				("03820101" + // BIT STRING length 0x101
					"004dcde29667973239cca344c58b72128fb5c5db03efdc75cfb7d9a0410ec03c8cd21160b449cd80224f41ca9d91529295ef7d0179ca4b08bb688cecce13cc07b20ecd87ffde1bc356554083c40bea7a387dacc54b3848b3710acf2fa613d007b12afc37f0a77082655b8dbb6683ba2fc52555e9f74bb5ba9429377ff38e193e799fc05c4c9bbcee29492945a732db67ba3575a79a83427a1f6d18d9ede01c544f3ccd68e5680a9b5418e03e1d80b3e77e69860982a4d21c6b111b07c87fe32c561e871554896b37651d5aaf42b2d092ce8d4dd4ae1d7a97091c0a06c03d71580e0557a51408513fde3012f02dac76536822a564faa2553048729633b68f1fc369")),
			wantErr: "failed to parse",
		},
		{
			desc: "valid-freshest-crl-ext",
			data: ("308201e3" + // SEQUENCE CertificateList
				("3081cc" + // SEQUENCE TBSCertList
					("0201" + "01") + // version 2(0x01)
					("300d" + // SEQUENCE AlgorithmIdentifier
						("0609" + "2a864886f70d01010b") + // OID: sha256WithRSA
						"0500") + // NULL
					("3049" + // SEQUENCE Name
						("310b" +
							("3009" +
								("0603" + "550406") + // OID: country
								("1302" + "5553"))) + // "US"
						("3113" +
							("3011" +
								("0603" + "55040a") + // OID: organization
								("130a" + "476f6f676c6520496e63"))) + // "Google Inc"
						("3125" +
							("3023" +
								("0603" + "550403") + // OID: commonName
								("131c" + "476f6f676c6520496e7465726e657420417574686f72697479204732")))) +
					("170d" + "3137303632393031303030325a") + // UTCTime
					("170d" + "3137303730393031303030325a") + // UTCTime
					("3000") + // SEQUENCE OF no revoked certs
					("a04d" +
						("304b" +
							("301f" +
								("0603" + "551d23") + // OID: authority-key-id
								("0418" +
									("3016" +
										"8014" + "4add06161bbcf668b576f581b6bb621aba5a812f"))) +
							("301b" +
								("0603" + "551d2e") + // OID: freshest-crl
								("0414" +
									("3012" +
										("3010" +
											("a00e" +
												("a00c" +
													("860a" + "687474703a2f2f777777"))))))) + // uRI='http://www'
							("300b" +
								("0603" + "551d14") + // OID: CRL-number
								("0404" + "02020623"))))) +
				("300d" +
					("0609" + "2a864886f70d01010b") + // OID: sha256WithRSA
					"0500") + // NULL
				("03820101" + // BIT STRING length 0x101
					"004dcde29667973239cca344c58b72128fb5c5db03efdc75cfb7d9a0410ec03c8cd21160b449cd80224f41ca9d91529295ef7d0179ca4b08bb688cecce13cc07b20ecd87ffde1bc356554083c40bea7a387dacc54b3848b3710acf2fa613d007b12afc37f0a77082655b8dbb6683ba2fc52555e9f74bb5ba9429377ff38e193e799fc05c4c9bbcee29492945a732db67ba3575a79a83427a1f6d18d9ede01c544f3ccd68e5680a9b5418e03e1d80b3e77e69860982a4d21c6b111b07c87fe32c561e871554896b37651d5aaf42b2d092ce8d4dd4ae1d7a97091c0a06c03d71580e0557a51408513fde3012f02dac76536822a564faa2553048729633b68f1fc369")),
			want: TBSCertList{
				Version: 1,
				Signature: pkix.AlgorithmIdentifier{
					Algorithm:  oidSignatureSHA256WithRSA,
					Parameters: asn1.RawValue{Class: 0, Tag: 5, Bytes: []byte{}, FullBytes: []byte{5, 0}},
				},
				Issuer: pkix.RDNSequence{
					[]pkix.AttributeTypeAndValue{
						{Type: pkix.OIDCountry, Value: "US"},
					},
					[]pkix.AttributeTypeAndValue{
						{Type: pkix.OIDOrganization, Value: "Google Inc"},
					},
					[]pkix.AttributeTypeAndValue{
						{Type: pkix.OIDCommonName, Value: "Google Internet Authority G2"},
					},
				},
				ThisUpdate:                   time.Date(2017, 6, 29, 01, 0, 2, 0, time.UTC),
				NextUpdate:                   time.Date(2017, 7, 9, 01, 0, 2, 0, time.UTC),
				RevokedCertificates:          []*RevokedCertificate{},
				AuthorityKeyID:               fromHex("4add06161bbcf668b576f581b6bb621aba5a812f"),
				CRLNumber:                    1571,
				BaseCRLNumber:                -1,
				FreshestCRLDistributionPoint: []string{"http://www"},
			},
		},
		{
			desc: "invalid-freshest-crl-ext",
			data: ("308201e3" + // SEQUENCE CertificateList
				("3081cc" + // SEQUENCE TBSCertList
					("0201" + "01") + // version 2(0x01)
					("300d" + // SEQUENCE AlgorithmIdentifier
						("0609" + "2a864886f70d01010b") + // OID: sha256WithRSA
						"0500") + // NULL
					("3049" + // SEQUENCE Name
						("310b" +
							("3009" +
								("0603" + "550406") + // OID: country
								("1302" + "5553"))) + // "US"
						("3113" +
							("3011" +
								("0603" + "55040a") + // OID: organization
								("130a" + "476f6f676c6520496e63"))) + // "Google Inc"
						("3125" +
							("3023" +
								("0603" + "550403") + // OID: commonName
								("131c" + "476f6f676c6520496e7465726e657420417574686f72697479204732")))) +
					("170d" + "3137303632393031303030325a") + // UTCTime
					("170d" + "3137303730393031303030325a") + // UTCTime
					("3000") + // SEQUENCE OF no revoked certs
					("a04d" +
						("304b" +
							("301f" +
								("0603" + "551d23") + // OID: authority-key-id
								("0418" +
									("3016" +
										"8014" + "4add06161bbcf668b576f581b6bb621aba5a812f"))) +
							("301b" +
								("0603" + "551d2e") + // OID: freshest-crl
								("0414" +
									("3112" + // INVALID: set-of not sequence-of
										("3010" +
											("a00e" +
												("a00c" +
													("860a" + "687474703a2f2f777777"))))))) + // uRI='http://www'
							("300b" +
								("0603" + "551d14") + // OID: CRL-number
								("0404" + "02020623"))))) +
				("300d" +
					("0609" + "2a864886f70d01010b") + // OID: sha256WithRSA
					"0500") + // NULL
				("03820101" + // BIT STRING length 0x101
					"004dcde29667973239cca344c58b72128fb5c5db03efdc75cfb7d9a0410ec03c8cd21160b449cd80224f41ca9d91529295ef7d0179ca4b08bb688cecce13cc07b20ecd87ffde1bc356554083c40bea7a387dacc54b3848b3710acf2fa613d007b12afc37f0a77082655b8dbb6683ba2fc52555e9f74bb5ba9429377ff38e193e799fc05c4c9bbcee29492945a732db67ba3575a79a83427a1f6d18d9ede01c544f3ccd68e5680a9b5418e03e1d80b3e77e69860982a4d21c6b111b07c87fe32c561e871554896b37651d5aaf42b2d092ce8d4dd4ae1d7a97091c0a06c03d71580e0557a51408513fde3012f02dac76536822a564faa2553048729633b68f1fc369")),
			wantErr: "structure error",
		},
		{
			desc: "valid-issuing-dp-ext",
			data: ("308201d7" + // SEQUENCE CertificateList
				("3081c0" + // SEQUENCE TBSCertList
					("0201" + "01") + // version 2(0x01)
					("300d" + // SEQUENCE AlgorithmIdentifier
						("0609" + "2a864886f70d01010b") + // OID: sha256WithRSA
						"0500") + // NULL
					("3049" + // SEQUENCE Name
						("310b" +
							("3009" +
								("0603" + "550406") + // OID: country
								("1302" + "5553"))) + // "US"
						("3113" +
							("3011" +
								("0603" + "55040a") + // OID: organization
								("130a" + "476f6f676c6520496e63"))) + // "Google Inc"
						("3125" +
							("3023" +
								("0603" + "550403") + // OID: commonName
								("131c" + "476f6f676c6520496e7465726e657420417574686f72697479204732")))) +
					("170d" + "3137303632393031303030325a") + // UTCTime
					("170d" + "3137303730393031303030325a") + // UTCTime
					("3000") + // SEQUENCE OF no revoked certs
					("a041" +
						("303f" +
							("301f" +
								("0603" + "551d23") + // OID: authority-key-id
								("0418" +
									("3016" +
										"8014" + "4add06161bbcf668b576f581b6bb621aba5a812f"))) +
							("300f" +
								("0603" + "551d1c") + // OID: issuing-distribution-point
								("0101ff") + // critical: true
								("0405" +
									("3003" + // SEQUENCE
										"8101" + "ff"))) + // [1]: onlyContainsUserCerts: true
							("300b" +
								("0603" + "551d14") + // OID: CRL-number
								("0404" + "02020623"))))) +
				("300d" +
					("0609" + "2a864886f70d01010b") + // OID: sha256WithRSA
					"0500") + // NULL
				("03820101" + // BIT STRING length 0x101
					"004dcde29667973239cca344c58b72128fb5c5db03efdc75cfb7d9a0410ec03c8cd21160b449cd80224f41ca9d91529295ef7d0179ca4b08bb688cecce13cc07b20ecd87ffde1bc356554083c40bea7a387dacc54b3848b3710acf2fa613d007b12afc37f0a77082655b8dbb6683ba2fc52555e9f74bb5ba9429377ff38e193e799fc05c4c9bbcee29492945a732db67ba3575a79a83427a1f6d18d9ede01c544f3ccd68e5680a9b5418e03e1d80b3e77e69860982a4d21c6b111b07c87fe32c561e871554896b37651d5aaf42b2d092ce8d4dd4ae1d7a97091c0a06c03d71580e0557a51408513fde3012f02dac76536822a564faa2553048729633b68f1fc369")),
			want: TBSCertList{
				Version: 1,
				Signature: pkix.AlgorithmIdentifier{
					Algorithm:  oidSignatureSHA256WithRSA,
					Parameters: asn1.RawValue{Class: 0, Tag: 5, Bytes: []byte{}, FullBytes: []byte{5, 0}},
				},
				Issuer: pkix.RDNSequence{
					[]pkix.AttributeTypeAndValue{
						{Type: pkix.OIDCountry, Value: "US"},
					},
					[]pkix.AttributeTypeAndValue{
						{Type: pkix.OIDOrganization, Value: "Google Inc"},
					},
					[]pkix.AttributeTypeAndValue{
						{Type: pkix.OIDCommonName, Value: "Google Internet Authority G2"},
					},
				},
				ThisUpdate:               time.Date(2017, 6, 29, 01, 0, 2, 0, time.UTC),
				NextUpdate:               time.Date(2017, 7, 9, 01, 0, 2, 0, time.UTC),
				RevokedCertificates:      []*RevokedCertificate{},
				AuthorityKeyID:           fromHex("4add06161bbcf668b576f581b6bb621aba5a812f"),
				CRLNumber:                1571,
				BaseCRLNumber:            -1,
				IssuingDistributionPoint: IssuingDistributionPoint{OnlyContainsUserCerts: true},
			},
		},
		{
			desc: "invalid-issuing-dp-ext",
			data: ("308201d7" + // SEQUENCE CertificateList
				("3081c0" + // SEQUENCE TBSCertList
					("0201" + "01") + // version 2(0x01)
					("300d" + // SEQUENCE AlgorithmIdentifier
						("0609" + "2a864886f70d01010b") + // OID: sha256WithRSA
						"0500") + // NULL
					("3049" + // SEQUENCE Name
						("310b" +
							("3009" +
								("0603" + "550406") + // OID: country
								("1302" + "5553"))) + // "US"
						("3113" +
							("3011" +
								("0603" + "55040a") + // OID: organization
								("130a" + "476f6f676c6520496e63"))) + // "Google Inc"
						("3125" +
							("3023" +
								("0603" + "550403") + // OID: commonName
								("131c" + "476f6f676c6520496e7465726e657420417574686f72697479204732")))) +
					("170d" + "3137303632393031303030325a") + // UTCTime
					("170d" + "3137303730393031303030325a") + // UTCTime
					("3000") + // SEQUENCE OF no revoked certs
					("a041" +
						("303f" +
							("301f" +
								("0603" + "551d23") + // OID: authority-key-id
								("0418" +
									("3016" +
										"8014" + "4add06161bbcf668b576f581b6bb621aba5a812f"))) +
							("300f" +
								("0603" + "551d1c") + // OID: issuing-distribution-point
								("0101ff") + // critical: true
								("0405" +
									("3103" + // INVALID: SET not SEQUENCE
										"8101" + "ff"))) + // [1]: onlyContainsUserCerts: true
							("300b" +
								("0603" + "551d14") + // OID: CRL-number
								("0404" + "02020623"))))) +
				("300d" +
					("0609" + "2a864886f70d01010b") + // OID: sha256WithRSA
					"0500") + // NULL
				("03820101" + // BIT STRING length 0x101
					"004dcde29667973239cca344c58b72128fb5c5db03efdc75cfb7d9a0410ec03c8cd21160b449cd80224f41ca9d91529295ef7d0179ca4b08bb688cecce13cc07b20ecd87ffde1bc356554083c40bea7a387dacc54b3848b3710acf2fa613d007b12afc37f0a77082655b8dbb6683ba2fc52555e9f74bb5ba9429377ff38e193e799fc05c4c9bbcee29492945a732db67ba3575a79a83427a1f6d18d9ede01c544f3ccd68e5680a9b5418e03e1d80b3e77e69860982a4d21c6b111b07c87fe32c561e871554896b37651d5aaf42b2d092ce8d4dd4ae1d7a97091c0a06c03d71580e0557a51408513fde3012f02dac76536822a564faa2553048729633b68f1fc369")),
			wantErr: "failed to unmarshal",
		},
	}

	for _, test := range tests {
		inData := fromHex(test.data)
		got, err := ParseCertificateList(inData)
		if err != nil {
			if test.wantErr == "" {
				t.Errorf("ParseCertificateList(%q)=%+v,%v; want _,nil", test.desc, got, err)
			} else if !strings.Contains(err.Error(), test.wantErr) {
				t.Errorf("ParseCertificateList(%q)=%+v,%v; want _,%q", test.desc, got, err, test.wantErr)
			}
			continue
		}
		if test.wantErr != "" {
			t.Errorf("ParseCertificateList(%q)=%+v,nil; want _,%q", test.desc, got, test.wantErr)
			continue
		}

		// Zero out unparsed extensions before comparison to make test data simpler.
		got.TBSCertList.Raw = nil
		got.TBSCertList.Extensions = nil
		for _, rc := range got.TBSCertList.RevokedCertificates {
			rc.Extensions = nil
		}

		if !reflect.DeepEqual(got.TBSCertList, test.want) {
			t.Errorf("ParseCertificateList(%q)=%+v; want %+v", test.desc, got.TBSCertList, test.want)
		}
	}
}

func TestParseRevokedCertificate(t *testing.T) {
	var tests = []struct {
		desc    string
		data    string // as hex
		want    RevokedCertificate
		wantErr string
	}{
		// CRL Reason
		{
			desc: "valid-reason-ext",
			data: ("3027" + // sequence
				("0208" + "3b772e5f1202118e") + // serial number
				("170d" + "3137303531303130353530375a") + // revocation time
				("300c" + // extensions
					("300a" + // extension
						("0603" + "551d15") + // OID: reason
						("0403" + // octet string
							"0a01" + "01")))), // enum:1
			want: RevokedCertificate{
				RevokedCertificate: pkix.RevokedCertificate{
					SerialNumber:   big.NewInt(4284944556325212558),
					RevocationTime: time.Date(2017, 05, 10, 10, 55, 07, 0, time.UTC),
					Extensions: []pkix.Extension{
						{
							Id:       OIDExtensionCRLReasons,
							Critical: false,
							Value:    fromHex("0a01" + "01"),
						},
					},
				},
				RevocationReason: KeyCompromise,
			},
		},
		{
			desc: "invalid-reason-ext-wrong-type",
			data: ("3027" + // sequence
				("0208" + "3b772e5f1202118e") + // serial number
				("170d" + "3137303531303130353530375a") + // revocation time
				("300c" + // extensions
					("300a" + // extension
						("0603" + "551d15") + // OID: reason
						("0403" + // octet string
							"0201" + "01")))), // int:1
			wantErr: "tags don't match",
		},
		{
			desc: "invalid-reason-ext-trailing-data",
			data: ("3028" + // sequence
				("0208" + "3b772e5f1202118e") + // serial number
				("170d" + "3137303531303130353530375a") + // revocation time
				("300d" + // extensions
					("300b" + // extension
						("0603" + "551d15") + // OID: reason
						("0404" + // octet string
							"0a01" + "01" + "aa")))), // enum:1
			wantErr: "trailing data",
		},
		{
			desc: "invalid-reason-ext-critical",
			data: ("302b" + // sequence
				("0208" + "3b772e5f1202118e") + // serial number
				("170d" + "3137303531303130353530375a") + // revocation time
				("3010" + // extensions
					("300e" + // extension
						("0603" + "551d15") + // OID: reason
						("0101ff") + // critical: true
						("0404" + // octet string
							"0a01" + "01" + "aa")))), // enum:1
			wantErr: "marked critical",
		},
		// Invalidity Date
		{
			desc: "valid-invalidity-date-ext",
			data: ("3033" + // sequence
				("0208" + "3b772e5f1202118e") + // serial number
				("170d" + "3137303531303130353530375a") + // revocation time
				("3018" + // extensions
					("3016" + // extension
						("0603" + "551d18") + // OID: invalidity date
						("040f" + // octet string
							"170d" + "3137303531303130353530375a")))),
			want: RevokedCertificate{
				RevokedCertificate: pkix.RevokedCertificate{
					SerialNumber:   big.NewInt(4284944556325212558),
					RevocationTime: time.Date(2017, 05, 10, 10, 55, 07, 0, time.UTC),
					Extensions: []pkix.Extension{
						{
							Id:       OIDExtensionInvalidityDate,
							Critical: false,
							Value:    fromHex("170d" + "3137303531303130353530375a"),
						},
					},
				},
				InvalidityDate: time.Date(2017, 05, 10, 10, 55, 07, 0, time.UTC),
			},
		},
		{
			desc: "invalid-invalidity-date-ext-wrong-type",
			data: ("3027" + // sequence
				("0208" + "3b772e5f1202118e") + // serial number
				("170d" + "3137303531303130353530375a") + // revocation time
				("300c" + // extensions
					("300a" + // extension
						("0603" + "551d18") + // OID: invalidity date
						("0403" + // octet string
							"0a01" + "01")))), // enum:1
			wantErr: "failed to parse",
		},
		{
			desc: "invalid-invalidity-date-ext-trailing-data",
			data: ("3036" + // sequence
				("0208" + "3b772e5f1202118e") + // serial number
				("170d" + "3137303531303130353530375a") + // revocation time
				("301b" + // extensions
					("3019" + // extension
						("0603" + "551d18") + // OID: invalidity date
						("0412" + // octet string
							"170d" + "3137303531303130353530375a" + "0a0101")))),
			wantErr: "trailing data",
		},
		{
			desc: "invalid-invalidity-date-ext-critical",
			data: ("3036" + // sequence
				("0208" + "3b772e5f1202118e") + // serial number
				("170d" + "3137303531303130353530375a") + // revocation time
				("301b" + // extensions
					("3019" + // extension
						("0603" + "551d18") + // OID: invalidity date
						("0101ff") + // critical: true
						("040f" + // octet string
							"170d" + "3137303531303130353530375a")))),
			wantErr: "marked critical",
		},
		// Issuer
		{
			desc: "valid-issuer-ext",
			data: ("303b" + // sequence
				("0208" + "3b772e5f1202118e") + // serial number
				("170d" + "3137303531303130353530375a") + // revocation time
				("3020" + // extensions
					("301e" + // extension
						("0603" + "551d1d") + // OID: issuer
						("0101ff") + // critical: true
						("0414" + // octet string
							("3012" +
								("8210" + "7777772e676f6f676c652e636f2e756b")))))), // "www.google.co.uk"
			want: RevokedCertificate{
				RevokedCertificate: pkix.RevokedCertificate{
					SerialNumber:   big.NewInt(4284944556325212558),
					RevocationTime: time.Date(2017, 05, 10, 10, 55, 07, 0, time.UTC),
					Extensions: []pkix.Extension{
						{
							Id:       OIDExtensionCertificateIssuer,
							Critical: true,
							Value: fromHex("3012" +
								("8210" + "7777772e676f6f676c652e636f2e756b")),
						},
					},
				},
				Issuer: GeneralNames{
					DNSNames: []string{"www.google.co.uk"},
				},
			},
		},
		{
			desc: "invalid-issuer-ext-wrong-type",
			data: ("302a" + // sequence
				("0208" + "3b772e5f1202118e") + // serial number
				("170d" + "3137303531303130353530375a") + // revocation time
				("300f" + // extensions
					("300d" + // extension
						("0603" + "551d1d") + // OID: issuer
						("0101ff") + // critical: true
						("0403" + // octet string
							"0a01" + "01")))), // enum:1
			wantErr: "failed to parse",
		},
		{
			desc: "invalid-issuer-ext-non-critical",
			data: ("303b" + // sequence
				("0208" + "3b772e5f1202118e") + // serial number
				("170d" + "3137303531303130353530375a") + // revocation time
				("3020" + // extensions
					("301e" + // extension
						("0603" + "551d1d") + // OID: issuer
						("010100") + // critical: false
						("0414" + // octet string
							("3012" +
								("8210" + "7777772e676f6f676c652e636f2e756b")))))), // "www.google.co.uk"
			wantErr: "marked non-critical",
		},
		// Unknown extension
		{
			desc: "valid-unknown-ext",
			data: ("3027" + // sequence
				("0208" + "3b772e5f1202118e") + // serial number
				("170d" + "3137303531303130353530375a") + // revocation time
				("300c" + // extensions
					("300a" + // extension
						("0603" + "551d14") + // OID: CRL number
						("0403" + // octet string
							"0a01" + "01")))), // enum:1
			want: RevokedCertificate{
				RevokedCertificate: pkix.RevokedCertificate{
					SerialNumber:   big.NewInt(4284944556325212558),
					RevocationTime: time.Date(2017, 05, 10, 10, 55, 07, 0, time.UTC),
					Extensions: []pkix.Extension{
						{
							Id:       OIDExtensionCRLNumber,
							Critical: false,
							Value:    fromHex("0a01" + "01"),
						},
					},
				},
			},
		},
		{
			desc: "invalid-unknown-ext-critical",
			data: ("302a" + // sequence
				("0208" + "3b772e5f1202118e") + // serial number
				("170d" + "3137303531303130353530375a") + // revocation time
				("300f" + // extensions
					("300d" + // extension
						("0603" + "551d14") + // OID: CRL number
						("0101ff") + // critical: true
						("0403" + // octet string
							"0a01" + "01")))), // enum:1
			wantErr: "unhandled critical extension",
		},
	}

	for _, test := range tests {
		inData := fromHex(test.data)
		var pkixCert pkix.RevokedCertificate
		if _, err := asn1.Unmarshal(inData, &pkixCert); err != nil {
			t.Errorf("asn1.Unmarshal(%s)=_,%v; want _,nil", test.data, err)
			continue
		}
		var errs Errors
		got := parseRevokedCertificate(pkixCert, &errs)
		if len(errs.Errs) > 0 {
			err := errs.Errs[0]
			if test.wantErr == "" {
				t.Errorf("parseRevokedCertificate(%q)=%+v,%v; want _,nil", test.desc, got, err)
			} else if !strings.Contains(err.Error(), test.wantErr) {
				t.Errorf("parseRevokedCertificate(%q)=%+v,%v; want _,%q", test.desc, got, err, test.wantErr)
			}
			continue
		}
		if test.wantErr != "" {
			t.Errorf("parseRevokedCertificate(%q)=%+v,nil; want _,%q", test.desc, got, test.wantErr)
			continue
		}
		if !reflect.DeepEqual(got, &test.want) {
			t.Errorf("parseRevokedCertificate(%q)=%+v; want %+v", test.desc, got, test.want)
		}
	}
}

func TestParseIssuingDistributionPoint(t *testing.T) {
	var tests = []struct {
		data    string // as hex
		want    IssuingDistributionPoint
		wantErr string
	}{
		{
			data: ("3003" + "8101ff"),
			want: IssuingDistributionPoint{OnlyContainsUserCerts: true},
		},
		{
			data: ("3003" + "8201ff"),
			want: IssuingDistributionPoint{OnlyContainsCACerts: true},
		},
		{
			data: ("3003" + "8501ff"),
			want: IssuingDistributionPoint{OnlyContainsAttributeCerts: true},
		},
		{
			data: ("3006" + "810100" + "8501ff"),
			want: IssuingDistributionPoint{OnlyContainsAttributeCerts: true},
		},
		{
			data: ("3009" + // SEQUENCE
				("a007" + // tag [0] = distributionPoint / DistributionPointName
					("a005" + // CHOICE [0] = fullName / GeneralNames
						"8203" + "777777"))), // CHOICE [2] = dNSName
			want: IssuingDistributionPoint{
				DistributionPoint: distributionPointName{
					FullName: []asn1.RawValue{
						{
							Class:      asn1.ClassContextSpecific,
							Tag:        2,
							IsCompound: false,
							Bytes:      fromHex("777777"),
							FullBytes:  fromHex("8203777777"),
						},
					},
				},
			},
		},
		{
			data: ("3019" + // SEQUENCE
				("a017" + // tag [0] = distributionPoint / DistributionPointName
					("a115" + // CHOICE [1] = nameRelativeToCRLIssuer / RelativeDistinguishedName
						("3113" + // SET OF
							("3011" + // SEQUENCE
								("0603" + "55040a") + // OID: organization
								("130a" + "476f6f676c6520496e63")))))), // "Google Inc"
			want: IssuingDistributionPoint{
				DistributionPoint: distributionPointName{
					RelativeName: pkix.RDNSequence{
						pkix.RelativeDistinguishedNameSET{
							pkix.AttributeTypeAndValue{
								Type:  pkix.OIDOrganization,
								Value: "Google Inc",
							},
						},
					},
				},
			},
		},
		{
			data:    ("3006" + "8101ff" + "8501ff"),
			wantErr: "multiple cert",
		},
		{
			data:    ("3003" + "8501ff" + "00"),
			wantErr: "trailing data",
		},
		{
			data:    ("3103" + "8101ff"), // INVALID: SET not SEQUENCE
			wantErr: "failed to unmarshal",
		},
		{
			data: ("3009" + // SEQUENCE
				("a007" + // tag [0] = distributionPoint / DistributionPointName
					("a005" + // CHOICE [0] = fullName / GeneralNames
						"8903" + "777777"))), // INVALID: choice 9 not allowed
			wantErr: "failed to unmarshal GeneralName",
		},
	}
	for _, test := range tests {
		inData := fromHex(test.data)
		var got IssuingDistributionPoint
		var gn GeneralNames
		var errs Errors
		parseIssuingDistributionPoint(inData, &got, &gn, &errs)
		if !errs.Empty() {
			err := errs.Errs[0]
			if test.wantErr == "" {
				t.Errorf("asn1.Unmarshal(%s)=_,%v; want _,nil", test.data, err)
			} else if !strings.Contains(err.Error(), test.wantErr) {
				t.Errorf("asn1.Unmarshal(%s)=_,%v; want _,%q", test.data, err, test.wantErr)
			}
			continue
		}
		if test.wantErr != "" {
			t.Errorf("asn1.Unmarshal(%s)=%+v,nil; want _,%q", test.data, got, test.wantErr)
			continue
		}
		if !reflect.DeepEqual(got, test.want) {
			t.Errorf("asn1.Unmarshal(%s)=%+v; want %+v", test.data, got, test.want)
		}
	}
}

// CRL for Google Internet Authority G2:
//   Certificate Revocation List (CRL):
//           Version 2 (0x1)
//       Signature Algorithm: sha256WithRSAEncryption
//           Issuer: /C=US/O=Google Inc/CN=Google Internet Authority G2
//           Last Update: Jun 29 01:00:02 2017 GMT
//           Next Update: Jul  9 01:00:02 2017 GMT
//           CRL extensions:
//               X509v3 Authority Key Identifier:
//                   keyid:4A:DD:06:16:1B:BC:F6:68:B5:76:F5:81:B6:BB:62:1A:BA:5A:81:2F
//               X509v3 CRL Number:
//                   1571
//   Revoked Certificates:
//       Serial Number: 764BEDD38AFD51F7
//           Revocation Date: Jan 13 14:18:58 2017 GMT
//           CRL entry extensions:
//               X509v3 CRL Reason Code:
//                   Affiliation Changed
//       Serial Number: 3B772E5F1202118E
//           Revocation Date: May 10 10:55:07 2017 GMT
//           CRL entry extensions:
//               X509v3 CRL Reason Code:
//                   Key Compromise
//       Serial Number: 0B54E3090079AD4B
//           Revocation Date: Apr 12 08:53:17 2017 GMT
//           CRL entry extensions:
//               X509v3 CRL Reason Code:
//                   Key Compromise
//       Serial Number: 31DA3380182AF9B2
//           Revocation Date: Sep 15 20:22:13 2016 GMT
//           CRL entry extensions:
//               X509v3 CRL Reason Code:
//                   Affiliation Changed
//       Signature Algorithm: sha256WithRSAEncryption
//            4d:cd:e2:96:67:97:32:39:cc:a3:44:c5:8b:72:12:8f:b5:c5:
//            db:03:ef:dc:75:cf:b7:d9:a0:41:0e:c0:3c:8c:d2:11:60:b4:
//            49:cd:80:22:4f:41:ca:9d:91:52:92:95:ef:7d:01:79:ca:4b:
//            08:bb:68:8c:ec:ce:13:cc:07:b2:0e:cd:87:ff:de:1b:c3:56:
//            55:40:83:c4:0b:ea:7a:38:7d:ac:c5:4b:38:48:b3:71:0a:cf:
//            2f:a6:13:d0:07:b1:2a:fc:37:f0:a7:70:82:65:5b:8d:bb:66:
//            83:ba:2f:c5:25:55:e9:f7:4b:b5:ba:94:29:37:7f:f3:8e:19:
//            3e:79:9f:c0:5c:4c:9b:bc:ee:29:49:29:45:a7:32:db:67:ba:
//            35:75:a7:9a:83:42:7a:1f:6d:18:d9:ed:e0:1c:54:4f:3c:cd:
//            68:e5:68:0a:9b:54:18:e0:3e:1d:80:b3:e7:7e:69:86:09:82:
//            a4:d2:1c:6b:11:1b:07:c8:7f:e3:2c:56:1e:87:15:54:89:6b:
//            37:65:1d:5a:af:42:b2:d0:92:ce:8d:4d:d4:ae:1d:7a:97:09:
//            1c:0a:06:c0:3d:71:58:0e:05:57:a5:14:08:51:3f:de:30:12:
//            f0:2d:ac:76:53:68:22:a5:64:fa:a2:55:30:48:72:96:33:b6:
//            8f:1f:c3:69
const giag2CRL = `-----BEGIN X509 CRL-----
MIICbDCCAVQCAQEwDQYJKoZIhvcNAQELBQAwSTELMAkGA1UEBhMCVVMxEzARBgNV
BAoTCkdvb2dsZSBJbmMxJTAjBgNVBAMTHEdvb2dsZSBJbnRlcm5ldCBBdXRob3Jp
dHkgRzIXDTE3MDYyOTAxMDAwMloXDTE3MDcwOTAxMDAwMlowgaQwJwIIdkvt04r9
UfcXDTE3MDExMzE0MTg1OFowDDAKBgNVHRUEAwoBAzAnAgg7dy5fEgIRjhcNMTcw
NTEwMTA1NTA3WjAMMAoGA1UdFQQDCgEBMCcCCAtU4wkAea1LFw0xNzA0MTIwODUz
MTdaMAwwCgYDVR0VBAMKAQEwJwIIMdozgBgq+bIXDTE2MDkxNTIwMjIxM1owDDAK
BgNVHRUEAwoBA6AwMC4wHwYDVR0jBBgwFoAUSt0GFhu89mi1dvWBtrtiGrpagS8w
CwYDVR0UBAQCAgYjMA0GCSqGSIb3DQEBCwUAA4IBAQBNzeKWZ5cyOcyjRMWLchKP
tcXbA+/cdc+32aBBDsA8jNIRYLRJzYAiT0HKnZFSkpXvfQF5yksIu2iM7M4TzAey
Ds2H/94bw1ZVQIPEC+p6OH2sxUs4SLNxCs8vphPQB7Eq/Dfwp3CCZVuNu2aDui/F
JVXp90u1upQpN3/zjhk+eZ/AXEybvO4pSSlFpzLbZ7o1daeag0J6H20Y2e3gHFRP
PM1o5WgKm1QY4D4dgLPnfmmGCYKk0hxrERsHyH/jLFYehxVUiWs3ZR1ar0Ky0JLO
jU3Urh16lwkcCgbAPXFYDgVXpRQIUT/eMBLwLax2U2gipWT6olUwSHKWM7aPH8Np
-----END X509 CRL-----`

// Certificate for GIAG2:
//     Data:
//         Version: 3 (0x2)
//         Serial Number:
//             01:00:21:25:88:b0:fa:59:a7:77:ef:05:7b:66:27:df
//     Signature Algorithm: sha256WithRSAEncryption
//         Issuer: C=US, O=GeoTrust Inc., CN=GeoTrust Global CA
//         Validity
//             Not Before: May 22 11:32:37 2017 GMT
//             Not After : Dec 31 23:59:59 2018 GMT
//         Subject: C=US, O=Google Inc, CN=Google Internet Authority G2
//         Subject Public Key Info:
//             Public Key Algorithm: rsaEncryption
//                 Public-Key: (2048 bit)
//                 Modulus:
//                     00:9c:2a:04:77:5c:d8:50:91:3a:06:a3:82:e0:d8:
//                     50:48:bc:89:3f:f1:19:70:1a:88:46:7e:e0:8f:c5:
//                     f1:89:ce:21:ee:5a:fe:61:0d:b7:32:44:89:a0:74:
//                     0b:53:4f:55:a4:ce:82:62:95:ee:eb:59:5f:c6:e1:
//                     05:80:12:c4:5e:94:3f:bc:5b:48:38:f4:53:f7:24:
//                     e6:fb:91:e9:15:c4:cf:f4:53:0d:f4:4a:fc:9f:54:
//                     de:7d:be:a0:6b:6f:87:c0:d0:50:1f:28:30:03:40:
//                     da:08:73:51:6c:7f:ff:3a:3c:a7:37:06:8e:bd:4b:
//                     11:04:eb:7d:24:de:e6:f9:fc:31:71:fb:94:d5:60:
//                     f3:2e:4a:af:42:d2:cb:ea:c4:6a:1a:b2:cc:53:dd:
//                     15:4b:8b:1f:c8:19:61:1f:cd:9d:a8:3e:63:2b:84:
//                     35:69:65:84:c8:19:c5:46:22:f8:53:95:be:e3:80:
//                     4a:10:c6:2a:ec:ba:97:20:11:c7:39:99:10:04:a0:
//                     f0:61:7a:95:25:8c:4e:52:75:e2:b6:ed:08:ca:14:
//                     fc:ce:22:6a:b3:4e:cf:46:03:97:97:03:7e:c0:b1:
//                     de:7b:af:45:33:cf:ba:3e:71:b7:de:f4:25:25:c2:
//                     0d:35:89:9d:9d:fb:0e:11:79:89:1e:37:c5:af:8e:
//                     72:69
//                 Exponent: 65537 (0x10001)
//         X509v3 extensions:
//             X509v3 Authority Key Identifier:
//                 keyid:C0:7A:98:68:8D:89:FB:AB:05:64:0C:11:7D:AA:7D:65:B8:CA:CC:4E
//
//             X509v3 Subject Key Identifier:
//                 4A:DD:06:16:1B:BC:F6:68:B5:76:F5:81:B6:BB:62:1A:BA:5A:81:2F
//             X509v3 Key Usage: critical
//                 Certificate Sign, CRL Sign
//             Authority Information Access:
//                 OCSP - URI:http://g.symcd.com
//
//             X509v3 Basic Constraints: critical
//                 CA:TRUE, pathlen:0
//             X509v3 CRL Distribution Points:
//
//                 Full Name:
//                   URI:http://g.symcb.com/crls/gtglobal.crl
//
//             X509v3 Certificate Policies:
//                 Policy: 1.3.6.1.4.1.11129.2.5.1
//                 Policy: 2.23.140.1.2.2
//
//             X509v3 Extended Key Usage:
//                 TLS Web Server Authentication, TLS Web Client Authentication
//     Signature Algorithm: sha256WithRSAEncryption
//          ca:49:e5:ac:d7:64:64:77:5b:be:71:fa:cf:f4:1e:23:c7:9a:
//          69:63:54:5f:eb:4c:d6:19:28:23:64:66:8e:1c:c7:87:80:64:
//          5f:04:8b:26:af:98:df:0a:70:bc:bc:19:3d:ee:7b:33:a9:7f:
//          bd:f4:05:d4:70:bb:05:26:79:ea:9a:c7:98:b9:07:19:65:34:
//          cc:3c:e9:3f:c5:01:fa:6f:0c:7e:db:7a:70:5c:4c:fe:2d:00:
//          f0:ca:be:2d:8e:b4:a8:80:fb:01:13:88:cb:9c:3f:e5:bb:77:
//          ca:3a:67:36:f3:ce:d5:27:02:72:43:a0:bd:6e:02:f1:47:05:
//          71:3e:01:59:e9:11:9e:1a:f3:84:0f:80:a6:a2:78:35:2f:b6:
//          c7:a2:7f:17:7c:e1:8b:56:ae:ee:67:88:51:27:30:60:a5:62:
//          52:c3:37:d5:3b:ea:85:2a:01:38:87:a2:cf:70:ad:a4:7a:c9:
//          c4:e7:ca:c5:da:bc:23:32:f2:fe:18:c2:7b:e0:df:3b:2f:d4:
//          d0:10:e6:96:4c:fb:44:b7:21:64:0d:b9:00:94:30:12:26:87:
//          58:98:39:05:38:0f:cc:82:48:0c:0a:47:66:ee:bf:b4:5f:c4:
//          ff:70:a8:e1:7f:8b:79:2b:b8:65:32:a3:b9:b7:31:e9:0a:f5:
//          f6:1f:32:dc
const giag2Cert = `-----BEGIN CERTIFICATE-----
MIIEKDCCAxCgAwIBAgIQAQAhJYiw+lmnd+8Fe2Yn3zANBgkqhkiG9w0BAQsFADBC
MQswCQYDVQQGEwJVUzEWMBQGA1UEChMNR2VvVHJ1c3QgSW5jLjEbMBkGA1UEAxMS
R2VvVHJ1c3QgR2xvYmFsIENBMB4XDTE3MDUyMjExMzIzN1oXDTE4MTIzMTIzNTk1
OVowSTELMAkGA1UEBhMCVVMxEzARBgNVBAoTCkdvb2dsZSBJbmMxJTAjBgNVBAMT
HEdvb2dsZSBJbnRlcm5ldCBBdXRob3JpdHkgRzIwggEiMA0GCSqGSIb3DQEBAQUA
A4IBDwAwggEKAoIBAQCcKgR3XNhQkToGo4Lg2FBIvIk/8RlwGohGfuCPxfGJziHu
Wv5hDbcyRImgdAtTT1WkzoJile7rWV/G4QWAEsRelD+8W0g49FP3JOb7kekVxM/0
Uw30SvyfVN59vqBrb4fA0FAfKDADQNoIc1Fsf/86PKc3Bo69SxEE630k3ub5/DFx
+5TVYPMuSq9C0svqxGoassxT3RVLix/IGWEfzZ2oPmMrhDVpZYTIGcVGIvhTlb7j
gEoQxirsupcgEcc5mRAEoPBhepUljE5SdeK27QjKFPzOImqzTs9GA5eXA37Asd57
r0Uzz7o+cbfe9CUlwg01iZ2d+w4ReYkeN8WvjnJpAgMBAAGjggERMIIBDTAfBgNV
HSMEGDAWgBTAephojYn7qwVkDBF9qn1luMrMTjAdBgNVHQ4EFgQUSt0GFhu89mi1
dvWBtrtiGrpagS8wDgYDVR0PAQH/BAQDAgEGMC4GCCsGAQUFBwEBBCIwIDAeBggr
BgEFBQcwAYYSaHR0cDovL2cuc3ltY2QuY29tMBIGA1UdEwEB/wQIMAYBAf8CAQAw
NQYDVR0fBC4wLDAqoCigJoYkaHR0cDovL2cuc3ltY2IuY29tL2NybHMvZ3RnbG9i
YWwuY3JsMCEGA1UdIAQaMBgwDAYKKwYBBAHWeQIFATAIBgZngQwBAgIwHQYDVR0l
BBYwFAYIKwYBBQUHAwEGCCsGAQUFBwMCMA0GCSqGSIb3DQEBCwUAA4IBAQDKSeWs
12Rkd1u+cfrP9B4jx5ppY1Rf60zWGSgjZGaOHMeHgGRfBIsmr5jfCnC8vBk97nsz
qX+99AXUcLsFJnnqmseYuQcZZTTMPOk/xQH6bwx+23pwXEz+LQDwyr4tjrSogPsB
E4jLnD/lu3fKOmc2887VJwJyQ6C9bgLxRwVxPgFZ6RGeGvOED4Cmong1L7bHon8X
fOGLVq7uZ4hRJzBgpWJSwzfVO+qFKgE4h6LPcK2kesnE58rF2rwjMvL+GMJ74N87
L9TQEOaWTPtEtyFkDbkAlDASJodYmDkFOA/MgkgMCkdm7r+0X8T/cKjhf4t5K7hl
MqO5tzHpCvX2HzLc
-----END CERTIFICATE-----`

func TestParseGIAG2CertificateList(t *testing.T) {
	certList, err := ParseCertificateList([]byte(giag2CRL))
	if err != nil {
		t.Fatalf("error parsing: %s", err)
	}
	if got, want := len(certList.TBSCertList.RevokedCertificates), 4; got != want {
		t.Errorf("len(ParseCertificateList(crl).TBSCertList.RevokedCertificates) = %d; want %d", got, want)
	}

	when := time.Date(2017, 7, 7, 12, 0, 0, 0, time.UTC)
	if certList.ExpiredAt(when) {
		t.Errorf("certList.ExpiredAt(%v)=true; want false", when)
	}
	if got, want := certList.TBSCertList.CRLNumber, 1571; got != want {
		t.Errorf("ParseCertificateList(crl).TBSCertList.CRLNumber = %d; want %d", got, want)
	}

	pemBlock, _ := pem.Decode([]byte(giag2Cert))
	giag2, err := ParseCertificate(pemBlock.Bytes)
	if err != nil {
		t.Fatalf("error parsing GIAG2 cert: %v", err)
	}
	if err := giag2.CheckCertificateListSignature(certList); err != nil {
		t.Errorf("CheckCertificateListSignature(giag2CRL)=%v; want nil", err)
	}
}
