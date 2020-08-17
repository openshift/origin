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

// Code to handle encoding / decoding various data structures used in RFC 6962. Does not
// contain the low level serialization.

import (
	"crypto"
	"crypto/sha256"

	"github.com/google/certificate-transparency-go/x509"
)

const millisPerNano int64 = 1000 * 1000

// GetCTLogID takes the key manager for a log and returns the LogID. (see RFC 6962 S3.2)
// In CT V1 the log id is a hash of the public key.
func GetCTLogID(pk crypto.PublicKey) ([sha256.Size]byte, error) {
	pubBytes, err := x509.MarshalPKIXPublicKey(pk)
	if err != nil {
		return [sha256.Size]byte{}, err
	}
	return sha256.Sum256(pubBytes), nil
}
