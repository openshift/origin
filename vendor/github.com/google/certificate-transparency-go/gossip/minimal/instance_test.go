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

package minimal

import (
	"context"
	"strings"
	"testing"
)

func TestNewGossiperFromFile(t *testing.T) {
	ctx := context.Background()
	var tests = []struct {
		name     string
		filename string
		wantErr  string
	}{
		{name: "OK", filename: "testdata/test.cfg"},
		{name: "EmptyFilename", filename: "", wantErr: "no such file"},
		{name: "MissingFile", filename: "testdata/nofile", wantErr: "no such file"},
		{name: "FailToParse", filename: "testdata/Makefile", wantErr: "failed to parse"},
		{name: "NoDestLog", filename: "testdata/no-dest-log.cfg", wantErr: "no dest log"},
		{name: "NoSourceLog", filename: "testdata/no-source-log.cfg", wantErr: "no source log"},
		{name: "NoSourceName", filename: "testdata/no-source-name.cfg", wantErr: "no log name provided"},
		{name: "InvalidSourcePubKey", filename: "testdata/invalid-source-pubkey.cfg", wantErr: "invalid public key"},
		{name: "InvalidSourceDuration", filename: "testdata/invalid-source-duration.cfg", wantErr: "MinReqInterval"},
		{name: "DupSourceName", filename: "testdata/dup-source-name.cfg", wantErr: "duplicate source logs"},
		{name: "NoDestName", filename: "testdata/no-dest-name.cfg", wantErr: "no log name provided"},
		{name: "NoPrivateKey", filename: "testdata/no-private-key.cfg", wantErr: "no private key"},
		{name: "InvalidPrivateKey", filename: "testdata/invalid-private-key.cfg", wantErr: "failed to unmarshal"},
		{name: "WrongPasswordPrivateKey", filename: "testdata/wrong-password-private-key.cfg", wantErr: "failed to decrypt"},
		{name: "NoRootCert", filename: "testdata/no-root-cert.cfg", wantErr: "failed to parse root"},
		{name: "InvalidRootCert", filename: "testdata/invalid-root-cert.cfg", wantErr: "failed to parse root"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := NewGossiperFromFile(ctx, test.filename, nil)
			if err != nil {
				if test.wantErr == "" {
					t.Errorf("NewGossiperFromFile(%v)=nil,%v; want _,nil", test.filename, err)
				} else if !strings.Contains(err.Error(), test.wantErr) {
					t.Errorf("NewGossiperFromFile(%v)=nil,%v; want nil,err containing %q", test.filename, err, test.wantErr)
				}
				return
			}
			if test.wantErr != "" {
				t.Errorf("NewGossiperFromFile(%+v)=%+v,nil; want nil,err containing %q", test.filename, got, test.wantErr)
			}
		})
	}
}
