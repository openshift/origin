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
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"time"

	"github.com/golang/glog"
	"github.com/golang/protobuf/proto"
	"github.com/golang/protobuf/ptypes"
	"github.com/google/certificate-transparency-go/client"
	"github.com/google/certificate-transparency-go/gossip/minimal/configpb"
	"github.com/google/certificate-transparency-go/jsonclient"
	"github.com/google/certificate-transparency-go/x509util"
	"github.com/google/trillian/crypto/keys"
)

// NewGossiperFromFile creates a gossiper from the given filename, which should
// contain text-protobuf encoded configuration data, together with an optional
// http Client.
func NewGossiperFromFile(ctx context.Context, filename string, hc *http.Client) (*Gossiper, error) {
	cfgText, err := ioutil.ReadFile(filename)
	if err != nil {
		return nil, err
	}

	var cfgProto configpb.GossipConfig
	if err := proto.UnmarshalText(string(cfgText), &cfgProto); err != nil {
		return nil, fmt.Errorf("%s: failed to parse gossip config: %v", filename, err)
	}
	cfg, err := NewGossiper(ctx, &cfgProto, hc)
	if err != nil {
		return nil, fmt.Errorf("%s: config error: %v", filename, err)
	}
	return cfg, nil
}

// NewGossiper creates a gossiper from the given configuration protobuf and optional
// http client.
func NewGossiper(ctx context.Context, cfg *configpb.GossipConfig, hc *http.Client) (*Gossiper, error) {
	if cfg.DestLog == nil {
		return nil, errors.New("no dest log config found")
	}
	if cfg.SourceLog == nil || len(cfg.SourceLog) == 0 {
		return nil, errors.New("no source log config found")
	}
	if cfg.PrivateKey == nil {
		return nil, errors.New("no private key found")
	}

	var keyProto ptypes.DynamicAny
	if err := ptypes.UnmarshalAny(cfg.PrivateKey, &keyProto); err != nil {
		return nil, fmt.Errorf("failed to unmarshal cfg.PrivateKey: %v", err)
	}
	signer, err := keys.NewSigner(ctx, keyProto.Message)
	if err != nil {
		return nil, fmt.Errorf("failed to load private key: %v", err)
	}

	root, err := x509util.CertificateFromPEM([]byte(cfg.RootCert))
	if err != nil {
		return nil, fmt.Errorf("failed to parse root cert: %v", err)
	}

	dests := make(map[string]*destLog)
	for _, lc := range cfg.DestLog {
		base, err := logConfigFromProto(lc, hc)
		if err != nil {
			return nil, fmt.Errorf("failed to parse dest log config: %v", err)
		}
		if _, ok := dests[base.Name]; ok {
			return nil, fmt.Errorf("duplicate dest logs for name %s", base.Name)
		}
		dests[base.Name] = &destLog{logConfig: *base, lastLogSubmission: make(map[string]time.Time)}
	}
	srcs := make(map[string]*sourceLog)
	for _, lc := range cfg.SourceLog {
		base, err := logConfigFromProto(lc, hc)
		if err != nil {
			return nil, fmt.Errorf("failed to parse source log config: %v", err)
		}
		if _, ok := srcs[base.Name]; ok {
			return nil, fmt.Errorf("duplicate source logs for name %s", base.Name)
		}
		srcs[base.Name] = &sourceLog{logConfig: *base}
	}

	return &Gossiper{
		signer:     signer,
		root:       root,
		dests:      dests,
		srcs:       srcs,
		bufferSize: int(cfg.BufferSize),
	}, nil
}

func logConfigFromProto(lc *configpb.LogConfig, hc *http.Client) (*logConfig, error) {
	if lc.Name == "" {
		return nil, errors.New("no log name provided")
	}
	interval, err := ptypes.Duration(lc.MinReqInterval)
	if err != nil {
		return nil, fmt.Errorf("failed to parse MinReqInterval: %v", err)
	}
	opts := jsonclient.Options{PublicKeyDER: lc.PublicKey.GetDer()}
	client, err := client.New(lc.Url, hc, opts)
	if err != nil {
		return nil, fmt.Errorf("failed to create log client for %q: %v", lc.Name, err)
	}
	if client.Verifier == nil {
		glog.Warningf("No public key provided for log %s, signature checks will be skipped", lc.Name)
	}
	return &logConfig{
		Name:        lc.Name,
		URL:         lc.Url,
		Log:         client,
		MinInterval: interval,
	}, nil
}
