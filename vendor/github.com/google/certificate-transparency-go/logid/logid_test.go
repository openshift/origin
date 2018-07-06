// Copyright 2017 Google Inc. All Rights Reserved.
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

package logid

import (
	"crypto/sha256"
	"encoding/base64"
	"log"
	"reflect"
	"testing"
)

var (
	wanted LogID = [32]byte{
		'c', 't', 'l', 'o', 'g', '_', 'i', 'd', '_', 'f', 'o', 'r', '_', 't', 'e', 's',
		't', 'i', 'n', 'g', '_', 'p', 'u', 'r', 'p', 'o', 's', 'e', 's', '!', '!', '1',
	}
)

func toBytes(s string, l int) []byte {
	bs := []byte(s)
	if len(bs) != l {
		log.Fatalf("string %q is %d bytes long, wanted length %d", s, len(bs), l)
	}
	return bs
}

func TestFromBytes(t *testing.T) {
	tests := []struct {
		in      string
		size    int
		want    *LogID
		wantErr bool
	}{
		{"ctlog_id_for_testing_purposes!!1", sha256.Size, &wanted, false},
		// "invalid CT LogID - expected 32 bytes but got 16"
		{"ctlog_id_for_tes", 16, nil, true},
		// "invalid CT LogID - expected 32 bytes but got 37"
		{"ctlog_id_for_testing_purposes!!1extra", 37, nil, true},
	}

	for _, test := range tests {
		got, err := FromBytes(toBytes(test.in, test.size))
		if gotErr := (err != nil); gotErr != test.wantErr {
			t.Errorf("FromBytes(%q): got err? %t, want? %t (err=%v)", test.in, gotErr, test.wantErr, err)
		}
		if err == nil && !reflect.DeepEqual(&got, test.want) {
			t.Errorf("FromBytes(%q): got %v, wanted %v", test.in, got, test.want)
		}
	}
}

func TestFromB64(t *testing.T) {
	tests := []struct {
		in      string
		want    *LogID
		wantErr bool
	}{
		{"Y3Rsb2dfaWRfZm9yX3Rlc3RpbmdfcHVycG9zZXMhITE=", &wanted, false},
		// "illegal base64 data at input byte 4"
		{"garbage", nil, true},
		// "invalid CT LogID - expected 32 bytes but got 1"
		{"ab==", nil, true},
		// "invalid CT LogID - expected 32 bytes but got 36"
		{"this+input+decodes+to+36+bytes+which+is+too+long", nil, true},
	}

	for _, test := range tests {
		got, err := FromB64(test.in)
		if gotErr := (err != nil); gotErr != test.wantErr {
			t.Errorf("FromB64(%q): got err? %t, want? %t (err=%v)", test.in, gotErr, test.wantErr, err)
		}
		if err == nil && !reflect.DeepEqual(&got, test.want) {
			t.Errorf("FromB64(%q): got %v, wanted %v", test.in, got, test.want)
		}
	}
}

func TestFromPubKeyB64(t *testing.T) {
	tests := []struct {
		desc    string
		b64     string
		want    string
		wantErr bool
	}{
		{
			desc:    "not base64",
			b64:     "Not valid b64",
			want:    "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=",
			wantErr: true,
		},
		{
			desc:    "ECDSA",
			b64:     "MFkwEwYHKoZIzj0CAQYIKoZIzj0DAQcDQgAEfahLEimAoz2t01p3uMziiLOl/fHTDM0YDOhBRuiBARsV4UvxG2LdNgoIGLrtCzWE0J5APC2em4JlvR8EEEFMoA==",
			want:    "pLkJkLQYWBSHuxOizGdwCjw1mAT5G9+443fNDsgN3BA=",
			wantErr: false,
		},
		{
			desc:    "RSA",
			b64:     "MIIBIjANBgkqhkiG9w0BAQEFAAOCAQ8AMIIBCgKCAQEAolpIHxdSlTXLo1s6H1OCdpSj/4DyHDc8wLG9wVmLqy1lk9fz4ATVmm+/1iN2Nk8jmctUKK2MFUtlWXZBSpym97M7frGlSaQXUWyA3CqQUEuIJOmlEjKTBEiQAvpfDjCHjlV2Be4qTM6jamkJbiWtgnYPhJL6ONaGTiSPm7Byy57iaz/hbckldSOIoRhYBiMzeNoA0DiRZ9KmfSeXZ1rB8y8X5urSW+iBzf2SaOfzBvDpcoTuAaWx2DPazoOl28fP1hZ+kHUYvxbcMjttjauCFx+JII0dmuZNIwjfeG/GBb9frpSX219k1O4Wi6OEbHEr8at/XQ0y7gTikOxBn/s5wQIDAQAB",
			want:    "rDua7X+pZ0dXFZ5tfVdWcvnZgQCUHpve/+yhMTt1eC0=",
			wantErr: false,
		},
	}

	for _, test := range tests {
		logID, err := FromPubKeyB64(test.b64)
		if gotErr := err != nil; gotErr != test.wantErr {
			t.Errorf("%s: got? %t want? %t (err=%v)", test.desc, gotErr, test.wantErr, err)
			continue
		}
		if got := base64.StdEncoding.EncodeToString(logID.Bytes()); got != test.want {
			t.Errorf("%s: got? %s want? %s", test.desc, got, test.want)
		}
	}
}
