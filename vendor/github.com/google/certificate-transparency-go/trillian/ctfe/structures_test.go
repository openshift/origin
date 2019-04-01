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

package ctfe

import (
	"crypto"
	"testing"
	"time"

	"github.com/google/certificate-transparency-go/trillian/testdata"
	"github.com/google/trillian/crypto/keys/pem"
)

var (
	fixedTime       = time.Date(2017, 9, 7, 12, 15, 23, 0, time.UTC)
	fixedTimeMillis = uint64(fixedTime.UnixNano() / millisPerNano)
	demoLogID       = [32]byte{19, 56, 222, 93, 229, 36, 102, 128, 227, 214, 3, 121, 93, 175, 126, 236, 97, 217, 34, 32, 40, 233, 98, 27, 46, 179, 164, 251, 84, 10, 60, 57}
	fakeSignature   = []byte("signed")
)

func TestGetCTLogID(t *testing.T) {
	pk, err := pem.UnmarshalPublicKey(testdata.DemoPublicKey)
	if err != nil {
		t.Fatalf("unexpected error loading public key: %v", err)
	}

	got, err := GetCTLogID(pk)
	if err != nil {
		t.Fatalf("error getting logid: %v", err)
	}

	if want := demoLogID; got != want {
		t.Errorf("logID: \n%v want \n%v", got, want)
	}
}

// Creates a fake signer for use in interaction tests.
// It will always return fakeSig when asked to sign something.
func setupSigner(fakeSig []byte) (crypto.Signer, error) {
	key, err := pem.UnmarshalPublicKey(testdata.DemoPublicKey)
	if err != nil {
		return nil, err
	}

	return testdata.NewSignerWithFixedSig(key, fakeSig), nil
}
