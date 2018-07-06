// Copyright 2016 Google Inc. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package tls

import (
	"encoding/hex"
	"strings"
	"testing"

	"github.com/google/certificate-transparency-go/testdata"
)

func TestGenerateHash(t *testing.T) {
	var tests = []struct {
		in     string // hex encoded
		algo   HashAlgorithm
		want   string // hex encoded
		errstr string
	}{
		// Empty hash values
		{"", MD5, "d41d8cd98f00b204e9800998ecf8427e", ""},
		{"", SHA1, "da39a3ee5e6b4b0d3255bfef95601890afd80709", ""},
		{"", SHA224, "d14a028c2a3a2bc9476102bb288234c415a2b01f828ea62ac5b3e42f", ""},
		{"", SHA256, "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855", ""},
		{"", SHA384, "38b060a751ac96384cd9327eb1b1e36a21fdb71114be07434c0cc7bf63f6e1da274edebfe76f65fbd51ad2f14898b95b", ""},
		{"", SHA512, "cf83e1357eefb8bdf1542850d66d8007d620e4050b5715dc83f4a921d36ce9ce47d0d13c5d85f2b0ff8318d2877eec2f63b931bd47417a81a538327af927da3e", ""},
		{"", 999, "", "unsupported"},

		// Hashes of "abcd".
		{"61626364", MD5, testdata.AbcdMD5, ""},
		{"61626364", SHA1, testdata.AbcdSHA1, ""},
		{"61626364", SHA224, testdata.AbcdSHA224, ""},
		{"61626364", SHA256, testdata.AbcdSHA256, ""},
		{"61626364", SHA384, testdata.AbcdSHA384, ""},
		{"61626364", SHA512, testdata.AbcdSHA512, ""},
	}
	for _, test := range tests {
		got, _, err := generateHash(test.algo, testdata.FromHex(test.in))
		if test.errstr != "" {
			if err == nil {
				t.Errorf("generateHash(%s)=%s,nil; want error %q", test.in, hex.EncodeToString(got), test.errstr)
			} else if !strings.Contains(err.Error(), test.errstr) {
				t.Errorf("generateHash(%s)=nil,%q; want error %q", test.in, test.errstr, err.Error())
			}
			continue
		}
		if err != nil {
			t.Errorf("generateHash(%s)=nil,%q; want %s", test.in, err, test.want)
		} else if hex.EncodeToString(got) != test.want {
			t.Errorf("generateHash(%s)=%s,nil; want %s", test.in, hex.EncodeToString(got), test.want)
		}
	}
}
