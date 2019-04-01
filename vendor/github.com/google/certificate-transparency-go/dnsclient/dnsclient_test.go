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

package dnsclient

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"

	ct "github.com/google/certificate-transparency-go"
	"github.com/google/certificate-transparency-go/jsonclient"
	"github.com/google/certificate-transparency-go/tls"
)

var (
	rocketeerPubKey = dehex("3059301306072a8648ce3d020106082a8648ce3d03010703420004205b18c83cc18bb3310800bfa090572bb7478c6fb568b08e9078e9a073ea4f28212e9cc0f4161baaf9d5d7a980c34e2f523c9801254624252823772d05c2407a")
	rocketeerKeyID  = [...]byte{0xee, 0x4b, 0xbd, 0xb7, 0x75, 0xce, 0x60, 0xba, 0xe1, 0x42, 0x69, 0x1f, 0xab, 0xe1, 0x9e, 0x66, 0xa3, 0x0f, 0x7e, 0x5f, 0xb0, 0x72, 0xd8, 0x83, 0x00, 0xc4, 0x7b, 0x89, 0x7a, 0xa8, 0xfd, 0xcb}
)

type testRsp struct {
	q   string
	txt []string
	err error
}

func testClient(rsp testRsp) *DNSClient {
	dc, err := New("test.example.com", jsonclient.Options{PublicKeyDER: rocketeerPubKey})
	if err != nil {
		panic("failed to build hard-coded test client")
	}
	dc.resolve = func(ctx context.Context, name string) ([]string, error) { return rsp.txt, rsp.err }
	return dc
}

func TestGetSTH(t *testing.T) {
	ctx := context.Background()
	var tests = []struct {
		name    string
		rsp     testRsp
		want    ct.SignedTreeHead
		wantErr string
	}{
		{
			name: "Valid",
			rsp:  testRsp{txt: []string{"224556042.1520614811284.uEM4ZVdEWUXnGqNzFiaWEwEW8keaU8HUdah+X4UQSHo=.BAMARjBEAiBvy2bp3DLPLHjAlTEVTcn5W/QBYUjBz8xJDGsuorhppwIgUPm+sLrKeBuETSMYFsPdwuGzd/u8Y2GNs9kfVDFv1PY="}},
			want: ct.SignedTreeHead{
				Version:        0,
				TreeSize:       224556042,
				Timestamp:      1520614811284,
				SHA256RootHash: [...]byte{0xb8, 0x43, 0x38, 0x65, 0x57, 0x44, 0x59, 0x45, 0xe7, 0x1a, 0xa3, 0x73, 0x16, 0x26, 0x96, 0x13, 0x01, 0x16, 0xf2, 0x47, 0x9a, 0x53, 0xc1, 0xd4, 0x75, 0xa8, 0x7e, 0x5f, 0x85, 0x10, 0x48, 0x7a},
				TreeHeadSignature: ct.DigitallySigned{
					Algorithm: tls.SignatureAndHashAlgorithm{
						Hash:      tls.SHA256,
						Signature: tls.ECDSA,
					},
					Signature: dehex("304402206fcb66e9dc32cf2c78c09531154dc9f95bf4016148c1cfcc490c6b2ea2b869a7022050f9beb0baca781b844d231816c3ddc2e1b377fbbc63618db3d91f54316fd4f6"),
				},
			},
		},
		{
			name: "ValidButSplit",
			rsp:  testRsp{txt: []string{"224556042.", "1520614811284.uEM4ZVdE", "WUXnGqNzFiaWEwEW8keaU8HUdah+X4UQSHo=.BAMARjBEAiBvy2bp3DLPLHjAlTEVTcn5W/QBYUjBz8xJDGsuorhppwIgUPm+sLrKeBuETSMYFsPdwuGzd/u8Y2GNs9kfVDFv1PY="}},
			want: ct.SignedTreeHead{
				Version:        0,
				TreeSize:       224556042,
				Timestamp:      1520614811284,
				SHA256RootHash: [...]byte{0xb8, 0x43, 0x38, 0x65, 0x57, 0x44, 0x59, 0x45, 0xe7, 0x1a, 0xa3, 0x73, 0x16, 0x26, 0x96, 0x13, 0x01, 0x16, 0xf2, 0x47, 0x9a, 0x53, 0xc1, 0xd4, 0x75, 0xa8, 0x7e, 0x5f, 0x85, 0x10, 0x48, 0x7a},
				TreeHeadSignature: ct.DigitallySigned{
					Algorithm: tls.SignatureAndHashAlgorithm{
						Hash:      tls.SHA256,
						Signature: tls.ECDSA,
					},
					Signature: dehex("304402206fcb66e9dc32cf2c78c09531154dc9f95bf4016148c1cfcc490c6b2ea2b869a7022050f9beb0baca781b844d231816c3ddc2e1b377fbbc63618db3d91f54316fd4f6"),
				},
			},
		},
		{
			name:    "NoResponse",
			rsp:     testRsp{err: errors.New("a test error")},
			wantErr: "test error",
		},
		{
			name:    "InvalidSize",
			rsp:     testRsp{txt: []string{"22x4556042.1520614811284.uEM4ZVdEWUXnGqNzFiaWEwEW8keaU8HUdah+X4UQSHo=.BAMARjBEAiBvy2bp3DLPLHjAlTEVTcn5W/QBYUjBz8xJDGsuorhppwIgUPm+sLrKeBuETSMYFsPdwuGzd/u8Y2GNs9kfVDFv1PY="}},
			wantErr: "failed to parse result",
		},
		{
			name:    "SizeTooLarge",
			rsp:     testRsp{txt: []string{"99223372036854775807.1520614811284.uEM4ZVdEWUXnGqNzFiaWEwEW8keaU8HUdah+X4UQSHo=.BAMARjBEAiBvy2bp3DLPLHjAlTEVTcn5W/QBYUjBz8xJDGsuorhppwIgUPm+sLrKeBuETSMYFsPdwuGzd/u8Y2GNs9kfVDFv1PY="}},
			wantErr: "failed to parse tree size",
		},
		{
			name:    "TrailingText",
			rsp:     testRsp{txt: []string{"224556042.1520614811284.uEM4ZVdEWUXnGqNzFiaWEwEW8keaU8HUdah+X4UQSHo=.BAMARjBEAiBvy2bp3DLPLHjAlTEVTcn5W/QBYUjBz8xJDGsuorhppwIgUPm+sLrKeBuETSMYFsPdwuGzd/u8Y2GNs9kfVDFv1PY=XXX"}},
			wantErr: "failed to decode signature",
		},
		{
			name:    "InvalidTimestamp",
			rsp:     testRsp{txt: []string{"224556042.15x0614811284.uEM4ZVdEWUXnGqNzFiaWEwEW8keaU8HUdah+X4UQSHo=.BAMARjBEAiBvy2bp3DLPLHjAlTEVTcn5W/QBYUjBz8xJDGsuorhppwIgUPm+sLrKeBuETSMYFsPdwuGzd/u8Y2GNs9kfVDFv1PY="}},
			wantErr: "failed to parse result",
		},
		{
			name:    "TimestampTooLarge",
			rsp:     testRsp{txt: []string{"123.99223372036854775807.uEM4ZVdEWUXnGqNzFiaWEwEW8keaU8HUdah+X4UQSHo=.BAMARjBEAiBvy2bp3DLPLHjAlTEVTcn5W/QBYUjBz8xJDGsuorhppwIgUPm+sLrKeBuETSMYFsPdwuGzd/u8Y2GNs9kfVDFv1PY="}},
			wantErr: "failed to parse timestamp",
		},
		{
			name:    "RootHashTooShort",
			rsp:     testRsp{txt: []string{"224556042.1520614811284.AQID.BAMARjBEAiBvy2bp3DLPLHjAlTEVTcn5W/QBYUjBz8xJDGsuorhppwIgUPm+sLrKeBuETSMYFsPdwuGzd/u8Y2GNs9kfVDFv1PY="}},
			wantErr: "wrong size root hash",
		},
		{
			name:    "InvalidRootHash",
			rsp:     testRsp{txt: []string{"224556042.1520614811284.uE=4ZVdEWUXnGqNzFiaWEwEW8keaU8HUdah+X4UQSHo=.BAMARjBEAiBvy2bp3DLPLHjAlTEVTcn5W/QBYUjBz8xJDGsuorhppwIgUPm+sLrKeBuETSMYFsPdwuGzd/u8Y2GNs9kfVDFv1PY="}},
			wantErr: "failed to parse root hash",
		},
		{
			name:    "BadlyEncodedSignature",
			rsp:     testRsp{txt: []string{"224556042.1520614811284.uEM4ZVdEWUXnGqNzFiaWEwEW8keaU8HUdah+X4UQSHo=.BANARjBEAiBvy2bp3DLPLHjAlTEVTcn5W/QBYUjBz8xJDGsuorhppwIgUPm+sLrKeBuETSMYFsPdwuGzd/u8Y2GNs9kfVDFv1PY="}},
			wantErr: "failed to parse signature",
		},
		{
			name:    "SignatureTrailingData",
			rsp:     testRsp{txt: []string{"224556042.1520614811284.uEM4ZVdEWUXnGqNzFiaWEwEW8keaU8HUdah+X4UQSHo=.BAMARjBEAiBvy2bp3DLPLHjAlTEVTcn5W/QBYUjBz8xJDGsuorhppwIgUPm+sLrKeBuETSMYFsPdwuGzd/u8Y2GNs9kfVDFv1PYX"}},
			wantErr: "trailing data",
		},
		{
			name:    "SignatureDoesntVerify",
			rsp:     testRsp{txt: []string{"224556042.1521614811284.uEM4ZVdEWUXnGqNzFiaWEwEW8keaU8HUdah+X4UQSHo=.BAMARjBEAiBvy2bp3DLPLHjAlTEVTcn5W/QBYUjBz8xJDGsuorhppwIgUPm+sLrKeBuETSMYFsPdwuGzd/u8Y2GNs9kfVDFv1PY="}},
			wantErr: "failed to verify ECDSA signature",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			dc := testClient(test.rsp)
			got, err := dc.GetSTH(ctx)
			if err != nil {
				if test.wantErr == "" {
					t.Errorf("GetSTH()=nil,%v; want _, nil", err)
				} else if !strings.Contains(err.Error(), test.wantErr) {
					t.Errorf("GetSTH()=nil,%v; want _, err containing %q", err, test.wantErr)
				}
				return
			}
			if test.wantErr != "" {
				t.Errorf("GetSTH()=%+v,nil; want _, err containing %q", got, test.wantErr)
			}
			if !reflect.DeepEqual(got, &test.want) {
				t.Errorf("GetSTH()=%+v; want %+v", got, test.want)
			}
		})
	}
}

func testMultiClient(rsps []testRsp) *DNSClient {
	dc, err := New("a.b.c", jsonclient.Options{})
	if err != nil {
		panic("failed to build hard-coded test client")
	}
	next := 0
	dc.resolve = func(ctx context.Context, name string) ([]string, error) {
		if next > len(rsps) {
			panic("fell off the end of hard-coded test data")
		}
		which := next
		next++

		if rsps[which].q != "" && rsps[which].q != name {
			return nil, fmt.Errorf("unexpected query: got %q, want %q", name, rsps[which].q)
		}
		return rsps[which].txt, rsps[which].err
	}
	return dc
}

func TestGetSTHConsistency(t *testing.T) {
	ctx := context.Background()
	var tests = []struct {
		name    string
		rsps    []testRsp
		want    int
		wantErr string
	}{
		{
			name:    "NoFirstResponse",
			rsps:    []testRsp{{err: errors.New("a test error")}},
			wantErr: "test error",
		},
		{
			name:    "Not32Multiple",
			rsps:    []testRsp{{txt: []string{string(dehex("0102"))}}},
			wantErr: "not multiple of 32",
		},
		{
			name: "ValidSingle",
			rsps: []testRsp{
				{
					q:   "0.100.200.sth-consistency.a.b.c.",
					txt: []string{string(dehex("000102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f"))},
				},
				{
					q:   "1.100.200.sth-consistency.a.b.c.",
					err: errors.New("finished"),
				},
			},
			want: 1,
		},
		{
			name: "ValidEmpty",
			rsps: []testRsp{
				{
					q:   "0.100.200.sth-consistency.a.b.c.",
					txt: []string{""},
				},
				{
					q:   "1.100.200.sth-consistency.a.b.c.",
					err: errors.New("finished"),
				},
			},
			want: 0,
		},
		{
			name: "ValidMultipleRequests",
			rsps: []testRsp{
				{
					q:   "0.100.200.sth-consistency.a.b.c.",
					txt: []string{string(dehex("000102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f"))},
				},
				{
					q:   "1.100.200.sth-consistency.a.b.c.",
					txt: []string{string(dehex("000102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f"))},
				},
				{
					q:   "2.100.200.sth-consistency.a.b.c.",
					err: errors.New("finished"),
				},
			},
			want: 2,
		},
		{
			name: "ValidMultipleSplit",
			rsps: []testRsp{
				{
					q: "0.100.200.sth-consistency.a.b.c.",
					txt: []string{string(dehex("000102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f" +
						"000102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f"))},
				},
				{
					q:   "2.100.200.sth-consistency.a.b.c.",
					txt: []string{string(dehex("000102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f"))},
				},
				{
					q:   "3.100.200.sth-consistency.a.b.c.",
					err: errors.New("finished"),
				},
			},
			want: 3,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			dc := testMultiClient(test.rsps)
			proof, err := dc.GetSTHConsistency(ctx, 100, 200)
			if err != nil {
				if test.wantErr == "" {
					t.Errorf("GetSTHConsistency()=nil,%v; want _, nil", err)
				} else if !strings.Contains(err.Error(), test.wantErr) {
					t.Errorf("GetSTHConsistency()=nil,%v; want _, err containing %q", err, test.wantErr)
				}
				return
			}
			if test.wantErr != "" {
				t.Errorf("GetSTHConsistency()=_,nil; want _,err containing %q", test.wantErr)
			}
			if got := len(proof); got != test.want {
				t.Errorf("len(GetSTHConsistency())=%d; want %d", got, test.want)
			}
		})
	}
}

func TestGetProofByHash(t *testing.T) {
	ctx := context.Background()
	hash := dehex("000102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f")
	var tests = []struct {
		name    string
		rsps    []testRsp
		want    int
		wantErr string
	}{
		{
			name: "NoFirstResponse",
			rsps: []testRsp{
				{
					err: errors.New("a test error"),
				},
			},
			wantErr: "test error",
		},
		{
			name: "NoSecondResponse",
			rsps: []testRsp{
				{
					q:   "AAAQEAYEAUDAOCAJBIFQYDIOB4IBCEQTCQKRMFYYDENBWHA5DYPQ.hash.a.b.c.",
					txt: []string{"100"},
				},
				{
					q:   "0.100.200.tree.a.b.c.",
					err: errors.New("a test error"),
				},
			},
			wantErr: "test error",
		},
		{
			name: "InvalidIndex",
			rsps: []testRsp{
				{
					q:   "AAAQEAYEAUDAOCAJBIFQYDIOB4IBCEQTCQKRMFYYDENBWHA5DYPQ.hash.a.b.c.",
					txt: []string{"10x0"},
				},
			},
			wantErr: "failed to parse result",
		},
		{
			name: "IndexTooLarge",
			rsps: []testRsp{
				{
					q:   "AAAQEAYEAUDAOCAJBIFQYDIOB4IBCEQTCQKRMFYYDENBWHA5DYPQ.hash.a.b.c.",
					txt: []string{"99223372036854775807"},
				},
			},
			wantErr: "failed to parse result",
		},
		{
			name: "Not32Multiple",
			rsps: []testRsp{
				{
					q:   "AAAQEAYEAUDAOCAJBIFQYDIOB4IBCEQTCQKRMFYYDENBWHA5DYPQ.hash.a.b.c.",
					txt: []string{"100"},
				},
				{
					q:   "0.100.200.tree.a.b.c.",
					txt: []string{string(dehex("0102"))},
				},
			},
			wantErr: "not multiple of 32",
		},
		{
			name: "ValidSingle",
			rsps: []testRsp{
				{
					q:   "AAAQEAYEAUDAOCAJBIFQYDIOB4IBCEQTCQKRMFYYDENBWHA5DYPQ.hash.a.b.c.",
					txt: []string{"100"},
				},
				{
					q:   "0.100.200.tree.a.b.c.",
					txt: []string{string(dehex("000102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f"))},
				},
				{
					q:   "1.100.200.tree.a.b.c.",
					err: errors.New("finished"),
				},
			},
			want: 1,
		},
		{
			name: "ValidMultipleRequests",
			rsps: []testRsp{
				{
					q:   "AAAQEAYEAUDAOCAJBIFQYDIOB4IBCEQTCQKRMFYYDENBWHA5DYPQ.hash.a.b.c.",
					txt: []string{"100"},
				},
				{
					q:   "0.100.200.tree.a.b.c.",
					txt: []string{string(dehex("000102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f"))},
				},
				{
					q:   "1.100.200.tree.a.b.c.",
					txt: []string{string(dehex("000102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f"))},
				},
				{
					q:   "2.100.200.tree.a.b.c.",
					err: errors.New("finished"),
				},
			},
			want: 2,
		},
		{
			name: "ValidMultipleSplit",
			rsps: []testRsp{
				{
					q:   "AAAQEAYEAUDAOCAJBIFQYDIOB4IBCEQTCQKRMFYYDENBWHA5DYPQ.hash.a.b.c.",
					txt: []string{"100"},
				},
				{
					q: "0.100.200.tree.a.b.c.",
					txt: []string{string(dehex("000102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f" +
						"000102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f"))},
				},
				{
					q:   "2.100.200.tree.a.b.c.",
					txt: []string{string(dehex("000102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f"))},
				},
				{
					q:   "3.100.200.tree.a.b.c.",
					err: errors.New("finished"),
				},
			},
			want: 3,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			dc := testMultiClient(test.rsps)
			got, err := dc.GetProofByHash(ctx, hash, 200)
			if err != nil {
				if test.wantErr == "" {
					t.Errorf("GetProofByHash()=nil,%v; want _, nil", err)
				} else if !strings.Contains(err.Error(), test.wantErr) {
					t.Errorf("GetProofByHash()=nil,%v; want _, err containing %q", err, test.wantErr)
				}
				return
			}
			if test.wantErr != "" {
				t.Errorf("GetProofByHash()=_,nil; want _,err containing %q", test.wantErr)
			}
			if want := int64(100); got.LeafIndex != want {
				t.Errorf("GetProofByHash().LeafIndex=%d; want %d", got.LeafIndex, want)
			}
			if got := len(got.AuditPath); got != test.want {
				t.Errorf("len(GetProofByHash())=%d; want %d", got, test.want)
			}
		})
	}
}

func dehex(in string) []byte {
	data, err := hex.DecodeString(in)
	if err != nil {
		panic(fmt.Sprintf("error in hard-coded test data %q", in))
	}
	return data
}
