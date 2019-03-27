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

package ctfe

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"

	"github.com/golang/glog"
	ct "github.com/google/certificate-transparency-go"
	"github.com/google/trillian"
)

type contextKey string

// remoteQuotaCtxKey is the key used to attach a Trillian quota user to
// context.Context passed in to STH getters.
var remoteQuotaCtxKey = contextKey("quotaUser")

// MirrorSTHStorage provides STHs of a source log to be served from a mirror.
type MirrorSTHStorage interface {
	// GetMirrorSTH returns an STH of TreeSize <= maxTreeSize. It does best
	// effort to maximize the returned STH's TreeSize and/or Timestamp.
	GetMirrorSTH(ctx context.Context, maxTreeSize int64) (*ct.SignedTreeHead, error)
}

// STHGetter provides latest STHs for a log.
type STHGetter interface {
	// GetSTH returns the latest STH for the log, as required by the RFC-6962
	// get-sth endpoint: https://tools.ietf.org/html/rfc6962#section-4.3.
	GetSTH(ctx context.Context) (*ct.SignedTreeHead, error)
}

// LogSTHGetter is an STHGetter implementation for regular (non-mirror) logs,
// i.e. logs that have their own key and actively sign STHs.
type LogSTHGetter struct {
	li    *logInfo
	cache SignatureCache
}

// GetSTH retrieves and builds a tree head structure for the given log.
func (sg *LogSTHGetter) GetSTH(ctx context.Context) (*ct.SignedTreeHead, error) {
	slr, err := getSignedLogRoot(ctx, sg.li.rpcClient, sg.li.logID, sg.li.LogPrefix)
	if err != nil {
		return nil, err
	}

	// Build the CT STH object, except the signature.
	sth := &ct.SignedTreeHead{
		Version:   ct.V1,
		TreeSize:  uint64(slr.TreeSize),
		Timestamp: uint64(slr.TimestampNanos / 1000 / 1000),
	}
	// Note: The size was checked in getSignedLogRoot.
	copy(sth.SHA256RootHash[:], slr.RootHash)

	// Add the signature over the STH contents.
	err = signV1TreeHead(sg.li.signer, sth, &sg.cache)
	if err != nil || len(sth.TreeHeadSignature.Signature) == 0 {
		return nil, fmt.Errorf("failed to sign tree head: %v", err)
	}

	return sth, nil
}

// MirrorSTHGetter is an STHGetter implementation for mirror logs. It assumes
// no knowledge of the key, and returns STHs obtained from an external source
// represented by the MirrorSTHStorage interface.
type MirrorSTHGetter struct {
	li *logInfo
	st MirrorSTHStorage
}

// GetSTH returns a known source log's STH with as large TreeSize and/or
// timestamp as possible, but such that TreeSize <= Trillian log size. This is
// to ensure that the mirror doesn't expose a "future" state of the log before
// it is properly stored in Trillian.
func (sg *MirrorSTHGetter) GetSTH(ctx context.Context) (*ct.SignedTreeHead, error) {
	slr, err := getSignedLogRoot(ctx, sg.li.rpcClient, sg.li.logID, sg.li.LogPrefix)
	if err != nil {
		return nil, err
	}

	sth, err := sg.st.GetMirrorSTH(ctx, slr.TreeSize)
	if err != nil {
		return nil, err
	}
	// TODO(pavelkalinnikov): Check sth signature.
	// TODO(pavelkalinnikov): Check consistency between slr and sth.
	return sth, nil
}

// getSignedLogRoot obtains the latest SignedLogRoot from Trillian log.
func getSignedLogRoot(ctx context.Context, client trillian.TrillianLogClient, logID int64, prefix string) (*trillian.SignedLogRoot, error) {
	req := trillian.GetLatestSignedLogRootRequest{LogId: logID}
	if q := ctx.Value(remoteQuotaCtxKey); q != nil {
		quotaUser, ok := q.(string)
		if !ok {
			return nil, fmt.Errorf("incorrect quota value: %v, type %T", q, q)
		}
		req.ChargeTo = appendUserCharge(req.ChargeTo, quotaUser)
	}

	glog.V(2).Infof("%s: GetSTH => grpc.GetLatestSignedLogRoot %+v", prefix, req)
	rsp, err := client.GetLatestSignedLogRoot(ctx, &req)
	glog.V(2).Infof("%s: GetSTH <= grpc.GetLatestSignedLogRoot err=%v", prefix, err)
	if err != nil {
		return nil, err
	}

	// Check over the response.
	slr := rsp.SignedLogRoot
	if slr == nil {
		return nil, errors.New("no log root returned")
	}
	glog.V(3).Infof("%s: GetSTH <= slr=%+v", prefix, slr)
	if treeSize := slr.TreeSize; treeSize < 0 {
		return nil, fmt.Errorf("bad tree size from backend: %d", treeSize)
	}
	if hashSize := len(slr.RootHash); hashSize != sha256.Size {
		return nil, fmt.Errorf("bad hash size from backend expecting: %d got %d", sha256.Size, hashSize)
	}

	return slr, nil
}

// DefaultMirrorSTHFactory creates DefaultMirrorSTHStorage instances.
type DefaultMirrorSTHFactory struct{}

// NewStorage creates a dummy STH storage.
func (f DefaultMirrorSTHFactory) NewStorage(logID [sha256.Size]byte) (MirrorSTHStorage, error) {
	return DefaultMirrorSTHStorage{}, nil
}

// DefaultMirrorSTHStorage is a dummy STH storage that always returns an error.
type DefaultMirrorSTHStorage struct{}

// GetMirrorSTH returns an error.
func (st DefaultMirrorSTHStorage) GetMirrorSTH(ctx context.Context, maxTreeSize int64) (*ct.SignedTreeHead, error) {
	return nil, errors.New("not implemented")
}
