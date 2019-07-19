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

package client_test

import (
	"context"
	"encoding/pem"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/golang/protobuf/ptypes"
	tspb "github.com/golang/protobuf/ptypes/timestamp"
	ct "github.com/google/certificate-transparency-go"
	"github.com/google/certificate-transparency-go/client"
	"github.com/google/certificate-transparency-go/client/configpb"
	"github.com/google/certificate-transparency-go/testdata"
	"github.com/google/certificate-transparency-go/x509"
	"github.com/google/certificate-transparency-go/x509util"
)

func TestNewTemporalLogClient(t *testing.T) {
	ts0, _ := ptypes.TimestampProto(time.Date(2010, 9, 19, 11, 00, 00, 00, time.UTC))
	ts1, _ := ptypes.TimestampProto(time.Date(2011, 9, 19, 11, 00, 00, 00, time.UTC))
	ts2, _ := ptypes.TimestampProto(time.Date(2012, 9, 19, 11, 00, 00, 00, time.UTC))
	ts2_5, _ := ptypes.TimestampProto(time.Date(2013, 3, 19, 11, 00, 00, 00, time.UTC))
	ts3, _ := ptypes.TimestampProto(time.Date(2013, 9, 19, 11, 00, 00, 00, time.UTC))
	ts4, _ := ptypes.TimestampProto(time.Date(2014, 9, 19, 11, 00, 00, 00, time.UTC))

	tests := []struct {
		cfg     configpb.TemporalLogConfig
		wantErr string
	}{
		{
			cfg: configpb.TemporalLogConfig{
				Shard: []*configpb.LogShardConfig{
					{Uri: "one", NotAfterStart: nil, NotAfterLimit: nil},
				},
			},
		},
		{
			cfg: configpb.TemporalLogConfig{
				Shard: []*configpb.LogShardConfig{
					{Uri: "one", NotAfterStart: ts0, NotAfterLimit: ts1},
					{Uri: "two", NotAfterStart: ts1, NotAfterLimit: ts2},
					{Uri: "three", NotAfterStart: ts2, NotAfterLimit: ts3},
					{Uri: "four", NotAfterStart: ts3, NotAfterLimit: ts4},
				},
			},
		},
		{
			cfg: configpb.TemporalLogConfig{
				Shard: []*configpb.LogShardConfig{
					{Uri: "one", NotAfterStart: nil, NotAfterLimit: ts1},
					{Uri: "two", NotAfterStart: ts1, NotAfterLimit: ts2},
					{Uri: "three", NotAfterStart: ts2, NotAfterLimit: ts3},
					{Uri: "four", NotAfterStart: ts3, NotAfterLimit: ts4},
				},
			},
		},
		{
			cfg: configpb.TemporalLogConfig{
				Shard: []*configpb.LogShardConfig{
					{Uri: "one", NotAfterStart: nil, NotAfterLimit: ts1},
					{Uri: "two", NotAfterStart: ts1, NotAfterLimit: ts2},
					{Uri: "three", NotAfterStart: ts2, NotAfterLimit: ts3},
					{Uri: "four", NotAfterStart: ts3, NotAfterLimit: nil},
				},
			},
		},
		{
			cfg: configpb.TemporalLogConfig{
				Shard: []*configpb.LogShardConfig{
					{Uri: "one", NotAfterStart: ts0, NotAfterLimit: ts1},
					{Uri: "two", NotAfterStart: ts1, NotAfterLimit: ts2},
					{Uri: "three", NotAfterStart: ts2, NotAfterLimit: ts3},
					{Uri: "four", NotAfterStart: ts3, NotAfterLimit: nil},
				},
			},
		},
		{
			cfg: configpb.TemporalLogConfig{
				Shard: []*configpb.LogShardConfig{
					{Uri: "one", NotAfterStart: ts0, NotAfterLimit: ts1},
					{Uri: "two", NotAfterStart: ts1, NotAfterLimit: ts2},
					{Uri: "three", NotAfterStart: ts2, NotAfterLimit: ts3},
					{Uri: "threeA", NotAfterStart: ts2_5, NotAfterLimit: ts3},
					{Uri: "four", NotAfterStart: ts3, NotAfterLimit: ts4},
				},
			},
			wantErr: "previous interval ended at",
		},
		{
			cfg: configpb.TemporalLogConfig{
				Shard: []*configpb.LogShardConfig{
					{Uri: "one", NotAfterStart: ts0, NotAfterLimit: ts1},
					{Uri: "three", NotAfterStart: ts2, NotAfterLimit: ts3},
					{Uri: "two", NotAfterStart: ts1, NotAfterLimit: ts2},
					{Uri: "four", NotAfterStart: ts3, NotAfterLimit: nil},
				},
			},
			wantErr: "previous interval ended at",
		},
		{
			cfg: configpb.TemporalLogConfig{
				Shard: []*configpb.LogShardConfig{
					{Uri: "one", NotAfterStart: nil, NotAfterLimit: ts1},
					{Uri: "two", NotAfterStart: ts1, NotAfterLimit: ts1},
					{Uri: "three", NotAfterStart: ts2, NotAfterLimit: ts3},
					{Uri: "four", NotAfterStart: ts3, NotAfterLimit: ts4},
				},
			},
			wantErr: "inverted",
		},
		{
			cfg: configpb.TemporalLogConfig{
				Shard: []*configpb.LogShardConfig{
					{Uri: "one", NotAfterStart: ts0, NotAfterLimit: ts1},
					{Uri: "two", NotAfterStart: ts1, NotAfterLimit: nil},
					{Uri: "three", NotAfterStart: ts2, NotAfterLimit: ts3},
					{Uri: "four", NotAfterStart: ts3, NotAfterLimit: nil},
				},
			},
			wantErr: "no upper bound",
		},
		{
			cfg: configpb.TemporalLogConfig{
				Shard: []*configpb.LogShardConfig{
					{Uri: "one", NotAfterStart: ts0, NotAfterLimit: ts1},
					{Uri: "two", NotAfterStart: ts1, NotAfterLimit: ts2},
					{Uri: "three", NotAfterStart: nil, NotAfterLimit: ts3},
					{Uri: "four", NotAfterStart: ts3, NotAfterLimit: nil},
				},
			},
			wantErr: "has no lower bound",
		},
		{
			wantErr: "empty",
		},
		{
			cfg:     configpb.TemporalLogConfig{Shard: []*configpb.LogShardConfig{}},
			wantErr: "empty",
		},
		{
			cfg: configpb.TemporalLogConfig{
				Shard: []*configpb.LogShardConfig{
					{Uri: "one", NotAfterStart: ts1, NotAfterLimit: ts1},
				},
			},
			wantErr: "inverted",
		},
		{
			cfg: configpb.TemporalLogConfig{
				Shard: []*configpb.LogShardConfig{
					{Uri: "one", NotAfterStart: ts2, NotAfterLimit: ts1},
				},
			},
			wantErr: "inverted",
		},
		{
			cfg: configpb.TemporalLogConfig{
				Shard: []*configpb.LogShardConfig{
					{Uri: "one", NotAfterStart: &tspb.Timestamp{Seconds: -1, Nanos: -1}, NotAfterLimit: ts2},
				},
			},
			wantErr: "failed to parse",
		},
		{
			cfg: configpb.TemporalLogConfig{
				Shard: []*configpb.LogShardConfig{
					{Uri: "one", NotAfterStart: ts1, NotAfterLimit: &tspb.Timestamp{Seconds: -1, Nanos: -1}},
				},
			},
			wantErr: "failed to parse",
		},
		{
			cfg: configpb.TemporalLogConfig{
				Shard: []*configpb.LogShardConfig{
					{
						Uri:           "one",
						NotAfterStart: nil,
						NotAfterLimit: nil,
						PublicKeyDer:  []byte{0x01, 0x02},
					},
				},
			},
			wantErr: "invalid public key",
		},
	}
	for _, test := range tests {
		_, err := client.NewTemporalLogClient(test.cfg, nil)
		if err != nil {
			if test.wantErr == "" {
				t.Errorf("NewTemporalLogClient(%+v)=nil,%v; want _,nil", test.cfg, err)
			} else if !strings.Contains(err.Error(), test.wantErr) {
				t.Errorf("NewTemporalLogClient(%+v)=nil,%v; want _,%q", test.cfg, err, test.wantErr)
			}
			continue
		}
		if test.wantErr != "" {
			t.Errorf("NewTemporalLogClient(%+v)=_, nil; want _,%q", test.cfg, test.wantErr)
		}
	}
}

func TestIndexByDate(t *testing.T) {
	time0 := time.Date(2010, 9, 19, 11, 00, 00, 00, time.UTC)
	time1 := time.Date(2011, 9, 19, 11, 00, 00, 00, time.UTC)
	time1_9 := time.Date(2012, 9, 19, 10, 59, 59, 00, time.UTC)
	time2 := time.Date(2012, 9, 19, 11, 00, 00, 00, time.UTC)
	time2_5 := time.Date(2013, 3, 19, 11, 00, 00, 00, time.UTC)
	time3 := time.Date(2013, 9, 19, 11, 00, 00, 00, time.UTC)
	time4 := time.Date(2014, 9, 19, 11, 00, 00, 00, time.UTC)

	ts0, _ := ptypes.TimestampProto(time0)
	ts1, _ := ptypes.TimestampProto(time1)
	ts2, _ := ptypes.TimestampProto(time2)
	ts3, _ := ptypes.TimestampProto(time3)
	ts4, _ := ptypes.TimestampProto(time4)

	allCfg := configpb.TemporalLogConfig{
		Shard: []*configpb.LogShardConfig{
			{Uri: "zero", NotAfterStart: nil, NotAfterLimit: ts0},
			{Uri: "one", NotAfterStart: ts0, NotAfterLimit: ts1},
			{Uri: "two", NotAfterStart: ts1, NotAfterLimit: ts2},
			{Uri: "three", NotAfterStart: ts2, NotAfterLimit: ts3},
			{Uri: "four", NotAfterStart: ts3, NotAfterLimit: ts4},
			{Uri: "five", NotAfterStart: ts4, NotAfterLimit: nil},
		},
	}
	uptoCfg := configpb.TemporalLogConfig{
		Shard: []*configpb.LogShardConfig{
			{Uri: "zero", NotAfterStart: nil, NotAfterLimit: ts0},
			{Uri: "one", NotAfterStart: ts0, NotAfterLimit: ts1},
			{Uri: "two", NotAfterStart: ts1, NotAfterLimit: ts2},
			{Uri: "three", NotAfterStart: ts2, NotAfterLimit: ts3},
			{Uri: "four", NotAfterStart: ts3, NotAfterLimit: ts4},
		},
	}
	fromCfg :=
		configpb.TemporalLogConfig{
			Shard: []*configpb.LogShardConfig{
				{Uri: "zero", NotAfterStart: ts0, NotAfterLimit: ts1},
				{Uri: "one", NotAfterStart: ts1, NotAfterLimit: ts2},
				{Uri: "two", NotAfterStart: ts2, NotAfterLimit: ts3},
				{Uri: "three", NotAfterStart: ts3, NotAfterLimit: ts4},
				{Uri: "four", NotAfterStart: ts4, NotAfterLimit: nil},
			},
		}
	boundedCfg :=
		configpb.TemporalLogConfig{
			Shard: []*configpb.LogShardConfig{
				{Uri: "zero", NotAfterStart: ts0, NotAfterLimit: ts1},
				{Uri: "one", NotAfterStart: ts1, NotAfterLimit: ts2},
				{Uri: "two", NotAfterStart: ts2, NotAfterLimit: ts3},
				{Uri: "three", NotAfterStart: ts3, NotAfterLimit: ts4},
			},
		}

	tests := []struct {
		cfg     configpb.TemporalLogConfig
		when    time.Time
		want    int
		wantErr bool
	}{
		{cfg: allCfg, when: time.Date(2000, 9, 19, 11, 00, 00, 00, time.UTC), want: 0},
		{cfg: allCfg, when: time0, want: 1},
		{cfg: allCfg, when: time1, want: 2},
		{cfg: allCfg, when: time1_9, want: 2},
		{cfg: allCfg, when: time2, want: 3},
		{cfg: allCfg, when: time2_5, want: 3},
		{cfg: allCfg, when: time3, want: 4},
		{cfg: allCfg, when: time4, want: 5},
		{cfg: allCfg, when: time.Date(2015, 9, 19, 11, 00, 00, 00, time.UTC), want: 5},

		{cfg: uptoCfg, when: time.Date(2000, 9, 19, 11, 00, 00, 00, time.UTC), want: 0},
		{cfg: uptoCfg, when: time0, want: 1},
		{cfg: uptoCfg, when: time1, want: 2},
		{cfg: uptoCfg, when: time2, want: 3},
		{cfg: uptoCfg, when: time2_5, want: 3},
		{cfg: uptoCfg, when: time3, want: 4},
		{cfg: uptoCfg, when: time4, wantErr: true},
		{cfg: uptoCfg, when: time.Date(2015, 9, 19, 11, 00, 00, 00, time.UTC), wantErr: true},

		{cfg: fromCfg, when: time.Date(2000, 9, 19, 11, 00, 00, 00, time.UTC), wantErr: true},
		{cfg: fromCfg, when: time0, want: 0},
		{cfg: fromCfg, when: time1, want: 1},
		{cfg: fromCfg, when: time2, want: 2},
		{cfg: fromCfg, when: time2_5, want: 2},
		{cfg: fromCfg, when: time3, want: 3},
		{cfg: fromCfg, when: time4, want: 4},
		{cfg: fromCfg, when: time.Date(2015, 9, 19, 11, 00, 00, 00, time.UTC), want: 4},

		{cfg: boundedCfg, when: time.Date(2000, 9, 19, 11, 00, 00, 00, time.UTC), wantErr: true},
		{cfg: boundedCfg, when: time0, want: 0},
		{cfg: boundedCfg, when: time1, want: 1},
		{cfg: boundedCfg, when: time2, want: 2},
		{cfg: boundedCfg, when: time2_5, want: 2},
		{cfg: boundedCfg, when: time3, want: 3},
		{cfg: boundedCfg, when: time4, wantErr: true},
		{cfg: boundedCfg, when: time.Date(2015, 9, 19, 11, 00, 00, 00, time.UTC), wantErr: true},
	}
	for _, test := range tests {
		tlc, err := client.NewTemporalLogClient(test.cfg, nil)
		if err != nil {
			t.Errorf("NewTemporalLogClient(%+v)=nil, %v; want _,nil", test.cfg, err)
			continue
		}
		got, err := tlc.IndexByDate(test.when)
		if err != nil {
			if !test.wantErr {
				t.Errorf("NewTemporalLogClient(%+v).idxByDate()=%d,%v; want %d,nil", test.cfg, got, err, test.want)
			}
			continue
		}
		if test.wantErr {
			t.Errorf("NewTemporalLogClient(%+v).idxByDate(%v)=%d, nil; want _, 'no log found'", test.cfg, test.when, got)
		}
		if got != test.want {
			t.Errorf("NewTemporalLogClient(%+v).idxByDate(%v)=%d, nil; want %d, nil", test.cfg, test.when, got, test.want)
		}
	}
}

func TestTemporalAddChain(t *testing.T) {
	hs := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/ct/v1/add-chain":
			data, _ := sctToJSON(testdata.TestCertProof)
			w.Write(data)
		case "/ct/v1/add-pre-chain":
			data, _ := sctToJSON(testdata.TestPreCertProof)
			w.Write(data)
		default:
			t.Fatalf("Incorrect URL path: %s", r.URL.Path)
		}
	}))
	defer hs.Close()

	cert, err := x509util.CertificateFromPEM([]byte(testdata.TestCertPEM))
	if err != nil {
		t.Fatalf("Failed to parse certificate from PEM: %v", err)
	}
	certChain := []ct.ASN1Cert{{Data: cert.Raw}}
	precert, err := x509util.CertificateFromPEM([]byte(testdata.TestPreCertPEM))
	if x509.IsFatal(err) {
		t.Fatalf("Failed to parse pre-certificate from PEM: %v", err)
	}
	issuer, err := x509util.CertificateFromPEM([]byte(testdata.CACertPEM))
	if x509.IsFatal(err) {
		t.Fatalf("Failed to parse issuer certificate from PEM: %v", err)
	}
	precertChain := []ct.ASN1Cert{{Data: precert.Raw}, {Data: issuer.Raw}}
	// Both have Not After = Jun  1 00:00:00 2022 GMT
	ts1, _ := ptypes.TimestampProto(time.Date(2022, 5, 19, 11, 00, 00, 00, time.UTC))
	ts2, _ := ptypes.TimestampProto(time.Date(2022, 6, 19, 11, 00, 00, 00, time.UTC))
	p, _ := pem.Decode([]byte(testdata.LogPublicKeyPEM))
	if p == nil {
		t.Fatalf("Failed to parse public key from PEM: %v", err)
	}

	tests := []struct {
		cfg     configpb.TemporalLogConfig
		wantErr bool
	}{
		{
			cfg: configpb.TemporalLogConfig{
				Shard: []*configpb.LogShardConfig{
					{Uri: hs.URL, NotAfterStart: nil, NotAfterLimit: nil, PublicKeyDer: p.Bytes},
				},
			},
		},
		{
			cfg: configpb.TemporalLogConfig{
				Shard: []*configpb.LogShardConfig{
					{Uri: hs.URL, NotAfterStart: nil, NotAfterLimit: ts2, PublicKeyDer: p.Bytes},
				},
			},
		},
		{
			cfg: configpb.TemporalLogConfig{
				Shard: []*configpb.LogShardConfig{
					{Uri: hs.URL, NotAfterStart: ts1, NotAfterLimit: nil, PublicKeyDer: p.Bytes},
				},
			},
		},
		{
			cfg: configpb.TemporalLogConfig{
				Shard: []*configpb.LogShardConfig{
					{Uri: hs.URL, NotAfterStart: ts1, NotAfterLimit: ts2, PublicKeyDer: p.Bytes},
				},
			},
		},
		{
			cfg: configpb.TemporalLogConfig{
				Shard: []*configpb.LogShardConfig{
					{Uri: hs.URL, NotAfterStart: nil, NotAfterLimit: ts1, PublicKeyDer: p.Bytes},
				},
			},
			wantErr: true,
		},
		{
			cfg: configpb.TemporalLogConfig{
				Shard: []*configpb.LogShardConfig{
					{Uri: hs.URL, NotAfterStart: ts2, NotAfterLimit: nil, PublicKeyDer: p.Bytes},
				},
			},
			wantErr: true,
		},
	}

	ctx := context.Background()
	for _, test := range tests {
		tlc, err := client.NewTemporalLogClient(test.cfg, nil)
		if err != nil {
			t.Errorf("NewTemporalLogClient(%+v)=nil, %v; want _,nil", test.cfg, err)
			continue
		}

		_, err = tlc.AddChain(ctx, certChain)
		if err != nil {
			if !test.wantErr {
				t.Errorf("AddChain()=nil,%v; want sct,nil", err)
			}
		} else if test.wantErr {
			t.Errorf("AddChain()=sct,nil; want nil,_")
		}

		_, err = tlc.AddPreChain(ctx, precertChain)
		if err != nil {
			if !test.wantErr {
				t.Errorf("AddPreChain()=nil,%v; want sct,nil", err)
			}
		} else if test.wantErr {
			t.Errorf("AddPreChain()=sct,nil; want nil,_")
		}
	}
}

func TestTemporalAddChainErrors(t *testing.T) {
	hs := serveSCTAt(t, "/ct/v1/add-chain", testdata.TestCertProof)
	defer hs.Close()

	cfg := configpb.TemporalLogConfig{
		Shard: []*configpb.LogShardConfig{
			{
				Uri:           hs.URL,
				NotAfterStart: nil,
				NotAfterLimit: nil,
			},
		},
	}

	ctx := context.Background()
	tlc, err := client.NewTemporalLogClient(cfg, nil)
	if err != nil {
		t.Fatalf("NewTemporalLogClient(%+v)=nil, %v; want _,nil", cfg, err)
	}

	_, err = tlc.AddChain(ctx, nil)
	if err == nil {
		t.Errorf("AddChain(nil)=sct,nil; want nil, 'missing chain'")
	}
	_, err = tlc.AddChain(ctx, []ct.ASN1Cert{{Data: []byte{0x01, 0x02}}})
	if err == nil {
		t.Errorf("AddChain(nil)=sct,nil; want nil, 'failed to parse'")
	}

}
