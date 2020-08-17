// Copyright 2018 Google Inc. All Rights Reserved.
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

package core

import (
	"fmt"

	ct "github.com/google/certificate-transparency-go"
	"github.com/google/certificate-transparency-go/trillian/util"
	"github.com/google/certificate-transparency-go/x509"
	"github.com/google/trillian"
)

func buildLogLeaf(logPrefix string, index int64, entry *ct.LeafEntry) (*trillian.LogLeaf, error) {
	logEntry, err := ct.LogEntryFromLeaf(index, entry)
	if _, ok := err.(x509.NonFatalErrors); !ok && err != nil {
		return nil, fmt.Errorf("failed to build LogEntry: %v", err)
	}
	// TODO(pavelkalinnikov): Verify the cert chain.

	var cert ct.ASN1Cert
	isPrecert := false
	switch {
	case logEntry.X509Cert != nil:
		cert = ct.ASN1Cert{Data: logEntry.X509Cert.Raw}
	case logEntry.Precert != nil:
		isPrecert = true
		cert = logEntry.Precert.Submitted
	default:
		return nil, fmt.Errorf("entry at %d is neither cert nor pre-cert", index)
	}

	leaf, err := util.BuildLogLeaf(logPrefix, logEntry.Leaf, logEntry.Index, cert, logEntry.Chain, isPrecert)
	if err != nil {
		return nil, fmt.Errorf("failed to build LogLeaf: %v", err)
	}
	return &leaf, nil
}
