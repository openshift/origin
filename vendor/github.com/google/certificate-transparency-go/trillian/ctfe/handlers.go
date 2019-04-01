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
	"context"
	"crypto"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io/ioutil"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/golang/glog"
	"github.com/google/certificate-transparency-go/tls"
	"github.com/google/certificate-transparency-go/trillian/util"
	"github.com/google/certificate-transparency-go/x509"
	"github.com/google/trillian"
	"github.com/google/trillian/monitoring"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	ct "github.com/google/certificate-transparency-go"
)

// TODO(drysdale): remove this flag once everything has migrated to ByRange
var getByRange = flag.Bool("by_range", false, "Use trillian.GetEntriesByRange for get-entries processing")

const (
	// HTTP content type header
	contentTypeHeader string = "Content-Type"
	// MIME content type for JSON
	contentTypeJSON string = "application/json"
	// The name of the JSON response map key in get-roots responses
	jsonMapKeyCertificates string = "certificates"
	// The name of the get-entries start parameter
	getEntriesParamStart = "start"
	// The name of the get-entries end parameter
	getEntriesParamEnd = "end"
	// The name of the get-proof-by-hash parameter
	getProofParamHash = "hash"
	// The name of the get-proof-by-hash tree size parameter
	getProofParamTreeSize = "tree_size"
	// The name of the get-sth-consistency first snapshot param
	getSTHConsistencyParamFirst = "first"
	// The name of the get-sth-consistency second snapshot param
	getSTHConsistencyParamSecond = "second"
	// The name of the get-entry-and-proof index parameter
	getEntryAndProofParamLeafIndex = "leaf_index"
	// The name of the get-entry-and-proof tree size parameter
	getEntryAndProofParamTreeSize = "tree_size"
)

var (
	// MaxGetEntriesAllowed is the number of entries we allow in a get-entries request
	MaxGetEntriesAllowed int64 = 1000

	// Use an explicitly empty slice for empty proofs so it gets JSON-encoded as
	// '[]' rather than 'null'.
	emptyProof = make([][]byte, 0)
)

// EntrypointName identifies a CT entrypoint as defined in section 4 of RFC 6962.
type EntrypointName string

// Constants for entrypoint names, as exposed in statistics/logging.
const (
	AddChainName          = EntrypointName("AddChain")
	AddPreChainName       = EntrypointName("AddPreChain")
	GetSTHName            = EntrypointName("GetSTH")
	GetSTHConsistencyName = EntrypointName("GetSTHConsistency")
	GetProofByHashName    = EntrypointName("GetProofByHash")
	GetEntriesName        = EntrypointName("GetEntries")
	GetRootsName          = EntrypointName("GetRoots")
	GetEntryAndProofName  = EntrypointName("GetEntryAndProof")
)

var (
	// Metrics are all per-log (label "logid"), but may also be
	// per-entrypoint (label "ep") or per-return-code (label "rc").
	once             sync.Once
	knownLogs        monitoring.Gauge     // logid => value (always 1.0)
	maxMergeDelay    monitoring.Gauge     // logid => value
	expMergeDelay    monitoring.Gauge     // logid => value
	lastSCTTimestamp monitoring.Gauge     // logid => value
	lastSTHTimestamp monitoring.Gauge     // logid => value
	lastSTHTreeSize  monitoring.Gauge     // logid => value
	reqsCounter      monitoring.Counter   // logid, ep => value
	rspsCounter      monitoring.Counter   // logid, ep, rc => value
	rspLatency       monitoring.Histogram // logid, ep, rc => value
)

// setupMetrics initializes all the exported metrics.
func setupMetrics(mf monitoring.MetricFactory) {
	knownLogs = mf.NewGauge("known_logs", "Set to 1 for known logs", "logid")
	maxMergeDelay = mf.NewGauge("max_merge_delay", "Maximum Merge Delay in seconds", "logid")
	expMergeDelay = mf.NewGauge("expected_merge_delay", "Expected Merge Delay in seconds", "logid")
	lastSCTTimestamp = mf.NewGauge("last_sct_timestamp", "Time of last SCT in ms since epoch", "logid")
	lastSTHTimestamp = mf.NewGauge("last_sth_timestamp", "Time of last STH in ms since epoch", "logid")
	lastSTHTreeSize = mf.NewGauge("last_sth_treesize", "Size of tree at last STH", "logid")
	reqsCounter = mf.NewCounter("http_reqs", "Number of requests", "logid", "ep")
	rspsCounter = mf.NewCounter("http_rsps", "Number of responses", "logid", "ep", "rc")
	rspLatency = mf.NewHistogram("http_latency", "Latency of responses in seconds", "logid", "ep", "rc")
}

// Entrypoints is a list of entrypoint names as exposed in statistics/logging.
var Entrypoints = []EntrypointName{AddChainName, AddPreChainName, GetSTHName, GetSTHConsistencyName, GetProofByHashName, GetEntriesName, GetRootsName, GetEntryAndProofName}

// PathHandlers maps from a path to the relevant AppHandler instance.
type PathHandlers map[string]AppHandler

// AppHandler holds a logInfo and a handler function that uses it, and is
// an implementation of the http.Handler interface.
type AppHandler struct {
	Info    *logInfo
	Handler func(context.Context, *logInfo, http.ResponseWriter, *http.Request) (int, error)
	Name    EntrypointName
	Method  string // http.MethodGet or http.MethodPost
}

// ServeHTTP for an AppHandler invokes the underlying handler function but
// does additional common error and stats processing.
func (a AppHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	var status int
	label0 := strconv.FormatInt(a.Info.logID, 10)
	label1 := string(a.Name)
	reqsCounter.Inc(label0, label1)
	startTime := a.Info.TimeSource.Now()
	logCtx := a.Info.RequestLog.Start(r.Context())
	a.Info.RequestLog.LogPrefix(logCtx, a.Info.LogPrefix)
	defer func() {
		latency := a.Info.TimeSource.Now().Sub(startTime).Seconds()
		rspLatency.Observe(latency, label0, label1, strconv.Itoa(status))
	}()
	glog.V(2).Infof("%s: request %v %q => %s", a.Info.LogPrefix, r.Method, r.URL, a.Name)
	if r.Method != a.Method {
		glog.Warningf("%s: %s wrong HTTP method: %v", a.Info.LogPrefix, a.Name, r.Method)
		sendHTTPError(w, http.StatusMethodNotAllowed, fmt.Errorf("method not allowed: %s", r.Method))
		a.Info.RequestLog.Status(logCtx, http.StatusMethodNotAllowed)
		return
	}

	// For GET requests all params come as form encoded so we might as well parse them now.
	// POSTs will decode the raw request body as JSON later.
	if r.Method == http.MethodGet {
		if err := r.ParseForm(); err != nil {
			sendHTTPError(w, http.StatusBadRequest, fmt.Errorf("failed to parse form data: %s", err))
			a.Info.RequestLog.Status(logCtx, http.StatusBadRequest)
			return
		}
	}

	// Many/most of the handlers forward the request on to the Log RPC server; impose a deadline
	// on this onward request.
	ctx, cancel := context.WithDeadline(logCtx, getRPCDeadlineTime(a.Info))
	defer cancel()

	status, err := a.Handler(ctx, a.Info, w, r)
	a.Info.RequestLog.Status(ctx, status)
	glog.V(2).Infof("%s: %s <= status=%d", a.Info.LogPrefix, a.Name, status)
	rspsCounter.Inc(label0, label1, strconv.Itoa(status))
	if err != nil {
		glog.Warningf("%s: %s handler error: %v", a.Info.LogPrefix, a.Name, err)
		sendHTTPError(w, status, err)
		return
	}

	// Additional check, for consistency the handler must return an error for non-200 status
	if status != http.StatusOK {
		glog.Warningf("%s: %s handler non 200 without error: %d %v", a.Info.LogPrefix, a.Name, status, err)
		sendHTTPError(w, http.StatusInternalServerError, fmt.Errorf("http handler misbehaved, status: %d", status))
		return
	}
}

// CertValidationOpts contains various parameters for certificate chain validation
type CertValidationOpts struct {
	// trustedRoots is a pool of certificates that defines the roots the CT log will accept
	trustedRoots *PEMCertPool
	// rejectExpired indicates whether certificate validity period should be used during chain verification
	rejectExpired bool
	// notAfterStart is the earliest notAfter date which will be accepted.
	// nil means no lower bound on the accepted range.
	notAfterStart *time.Time
	// notAfterLimit defines the cut off point of notAfter dates - only notAfter
	// dates strictly *before* notAfterLimit will be accepted.
	// nil means no upper bound on the accepted range.
	notAfterLimit *time.Time
	// acceptOnlyCA will reject any certificate without the CA bit set.
	acceptOnlyCA bool
	// extKeyUsages contains the list of EKUs to use during chain verification
	extKeyUsages []x509.ExtKeyUsage
}

// logInfo holds information for a specific log instance.
type logInfo struct {
	// LogPrefix is a pre-formatted string identifying the log for diagnostics
	LogPrefix string
	// TimeSource is a util.TimeSource that can be injected for testing
	TimeSource util.TimeSource
	// RequestLog is a logger for various request / processing / response debug
	// information.
	RequestLog RequestLog

	// Instance-wide options
	instanceOpts InstanceOptions
	// logID is the tree ID that identifies this log in node storage
	logID int64
	// validationOpts contains the certificate chain validation parameters
	validationOpts CertValidationOpts
	// rpcClient is the client used to communicate with the Trillian backend
	rpcClient trillian.TrillianLogClient
	// signer signs objects (e.g. STHs, SCTs) for regular logs
	signer crypto.Signer
	// sthGetter provides STHs for the log
	sthGetter STHGetter
}

// newLogInfo creates a new instance of logInfo.
func newLogInfo(
	instanceOpts InstanceOptions,
	validationOpts CertValidationOpts,
	signer crypto.Signer,
	timeSource util.TimeSource,
) *logInfo {
	logID, prefix := instanceOpts.Config.LogId, instanceOpts.Config.Prefix
	li := &logInfo{
		logID:          logID,
		LogPrefix:      fmt.Sprintf("%s{%d}", prefix, logID),
		rpcClient:      instanceOpts.Client,
		signer:         signer,
		TimeSource:     timeSource,
		instanceOpts:   instanceOpts,
		validationOpts: validationOpts,
		RequestLog:     instanceOpts.RequestLog,
	}

	if instanceOpts.Config.IsMirror {
		li.sthGetter = &MirrorSTHGetter{li: li, st: DefaultMirrorSTHStorage{}}
	} else {
		li.sthGetter = &LogSTHGetter{li: li}
	}

	once.Do(func() { setupMetrics(instanceOpts.MetricFactory) })
	label := strconv.FormatInt(logID, 10)
	knownLogs.Set(1.0, label)
	maxMergeDelay.Set(float64(instanceOpts.Config.MaxMergeDelaySec), label)
	expMergeDelay.Set(float64(instanceOpts.Config.ExpectedMergeDelaySec), label)

	return li
}

// Handlers returns a map from URL paths (with the given prefix) and AppHandler instances
// to handle those entrypoints.
func (li *logInfo) Handlers(prefix string) PathHandlers {
	if !strings.HasPrefix(prefix, "/") {
		prefix = "/" + prefix
	}
	prefix = strings.TrimRight(prefix, "/")

	// Bind the logInfo instance to give an AppHandler instance for each endpoint.
	ph := PathHandlers{
		prefix + ct.AddChainPath:          AppHandler{Info: li, Handler: addChain, Name: AddChainName, Method: http.MethodPost},
		prefix + ct.AddPreChainPath:       AppHandler{Info: li, Handler: addPreChain, Name: AddPreChainName, Method: http.MethodPost},
		prefix + ct.GetSTHPath:            AppHandler{Info: li, Handler: getSTH, Name: GetSTHName, Method: http.MethodGet},
		prefix + ct.GetSTHConsistencyPath: AppHandler{Info: li, Handler: getSTHConsistency, Name: GetSTHConsistencyName, Method: http.MethodGet},
		prefix + ct.GetProofByHashPath:    AppHandler{Info: li, Handler: getProofByHash, Name: GetProofByHashName, Method: http.MethodGet},
		prefix + ct.GetEntriesPath:        AppHandler{Info: li, Handler: getEntries, Name: GetEntriesName, Method: http.MethodGet},
		prefix + ct.GetRootsPath:          AppHandler{Info: li, Handler: getRoots, Name: GetRootsName, Method: http.MethodGet},
		prefix + ct.GetEntryAndProofPath:  AppHandler{Info: li, Handler: getEntryAndProof, Name: GetEntryAndProofName, Method: http.MethodGet},
	}
	// Remove endpoints not provided by mirrors.
	if li.instanceOpts.Config.IsMirror {
		delete(ph, prefix+ct.AddChainPath)
		delete(ph, prefix+ct.AddPreChainPath)
	}

	return ph
}

func parseBodyAsJSONChain(li *logInfo, r *http.Request) (ct.AddChainRequest, error) {
	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		glog.V(1).Infof("%s: Failed to read request body: %v", li.LogPrefix, err)
		return ct.AddChainRequest{}, err
	}

	var req ct.AddChainRequest
	if err := json.Unmarshal(body, &req); err != nil {
		glog.V(1).Infof("%s: Failed to parse request body: %v", li.LogPrefix, err)
		return ct.AddChainRequest{}, err
	}

	// The cert chain is not allowed to be empty. We'll defer other validation for later
	if len(req.Chain) == 0 {
		glog.V(1).Infof("%s: Request chain is empty: %s", li.LogPrefix, body)
		return ct.AddChainRequest{}, errors.New("cert chain was empty")
	}

	return req, nil
}

// appendUserCharge adds the specified user to the passed in ChargeTo and
// and returns the result.
// If the passed-in ChargeTo is nil, then a new one is created with the passed
// in user and returned.
func appendUserCharge(a *trillian.ChargeTo, user string) *trillian.ChargeTo {
	if a == nil {
		a = &trillian.ChargeTo{}
	}
	a.User = append(a.User, user)
	return a
}

// chargeUser returns a trillian.ChargeTo containing an ID for the remote User,
// or nil if instanceOpts does not have a RemoteQuotaUser function set.
func (li *logInfo) chargeUser(r *http.Request) *trillian.ChargeTo {
	if li.instanceOpts.RemoteQuotaUser != nil {
		return &trillian.ChargeTo{User: []string{li.instanceOpts.RemoteQuotaUser(r)}}
	}
	return nil
}

// addChainInternal is called by add-chain and add-pre-chain as the logic involved in
// processing these requests is almost identical
func addChainInternal(ctx context.Context, li *logInfo, w http.ResponseWriter, r *http.Request, isPrecert bool) (int, error) {
	var method EntrypointName
	var etype ct.LogEntryType
	if isPrecert {
		method = AddPreChainName
		etype = ct.PrecertLogEntryType
	} else {
		method = AddChainName
		etype = ct.X509LogEntryType
	}

	// Check the contents of the request and convert to slice of certificates.
	addChainReq, err := parseBodyAsJSONChain(li, r)
	if err != nil {
		return http.StatusBadRequest, fmt.Errorf("failed to parse add-chain body: %s", err)
	}
	// Log the DERs now because they might not parse as valid X.509.
	for _, der := range addChainReq.Chain {
		li.RequestLog.AddDERToChain(ctx, der)
	}
	chain, err := verifyAddChain(li, addChainReq, isPrecert)
	if err != nil {
		return http.StatusBadRequest, fmt.Errorf("failed to verify add-chain contents: %s", err)
	}
	for _, cert := range chain {
		li.RequestLog.AddCertToChain(ctx, cert)
	}
	// Get the current time in the form used throughout RFC6962, namely milliseconds since Unix
	// epoch, and use this throughout.
	timeMillis := uint64(li.TimeSource.Now().UnixNano() / millisPerNano)

	// Build the MerkleTreeLeaf that gets sent to the backend, and make a trillian.LogLeaf for it.
	merkleLeaf, err := ct.MerkleTreeLeafFromChain(chain, etype, timeMillis)
	if err != nil {
		return http.StatusBadRequest, fmt.Errorf("failed to build MerkleTreeLeaf: %s", err)
	}
	leaf, err := buildLogLeafForAddChain(li, *merkleLeaf, chain, isPrecert)
	if err != nil {
		return http.StatusInternalServerError, fmt.Errorf("failed to build LogLeaf: %s", err)
	}

	// Send the Merkle tree leaf on to the Log server.
	req := trillian.QueueLeavesRequest{
		LogId:    li.logID,
		Leaves:   []*trillian.LogLeaf{&leaf},
		ChargeTo: li.chargeUser(r),
	}
	if li.instanceOpts.CertificateQuotaUser != nil {
		// TODO(al): ignore pre-issuers? Probably doesn't matter
		for _, cert := range chain[1:] {
			req.ChargeTo = appendUserCharge(req.ChargeTo, li.instanceOpts.CertificateQuotaUser(cert))
		}
	}

	glog.V(2).Infof("%s: %s => grpc.QueueLeaves", li.LogPrefix, method)
	rsp, err := li.rpcClient.QueueLeaves(ctx, &req)
	glog.V(2).Infof("%s: %s <= grpc.QueueLeaves err=%v", li.LogPrefix, method, err)
	if err != nil {
		return li.toHTTPStatus(err), fmt.Errorf("backend QueueLeaves request failed: %s", err)
	}
	if rsp == nil {
		return http.StatusInternalServerError, errors.New("missing QueueLeaves response")
	}
	if len(rsp.QueuedLeaves) != 1 {
		return http.StatusInternalServerError, fmt.Errorf("unexpected QueueLeaves response leaf count: %d", len(rsp.QueuedLeaves))
	}
	queuedLeaf := rsp.QueuedLeaves[0]

	// Always use the returned leaf as the basis for an SCT.
	var loggedLeaf ct.MerkleTreeLeaf
	if rest, err := tls.Unmarshal(queuedLeaf.Leaf.LeafValue, &loggedLeaf); err != nil {
		return http.StatusInternalServerError, fmt.Errorf("failed to reconstruct MerkleTreeLeaf: %s", err)
	} else if len(rest) > 0 {
		return http.StatusInternalServerError, fmt.Errorf("extra data (%d bytes) on reconstructing MerkleTreeLeaf", len(rest))
	}

	// As the Log server has definitely got the Merkle tree leaf, we can
	// generate an SCT and respond with it.
	sct, err := buildV1SCT(li.signer, &loggedLeaf)
	if err != nil {
		return http.StatusInternalServerError, fmt.Errorf("failed to generate SCT: %s", err)
	}
	sctBytes, err := tls.Marshal(*sct)
	if err != nil {
		return http.StatusInternalServerError, fmt.Errorf("failed to marshall SCT: %s", err)
	}
	// We could possibly fail to issue the SCT after this but it's v. unlikely.
	li.RequestLog.IssueSCT(ctx, sctBytes)
	err = marshalAndWriteAddChainResponse(sct, li.signer, w)
	if err != nil {
		// reason is logged and http status is already set
		return http.StatusInternalServerError, fmt.Errorf("failed to write response: %s", err)
	}
	glog.V(3).Infof("%s: %s <= SCT", li.LogPrefix, method)
	if sct.Timestamp == timeMillis {
		lastSCTTimestamp.Set(float64(sct.Timestamp), strconv.FormatInt(li.logID, 10))
	}

	return http.StatusOK, nil
}

func addChain(ctx context.Context, li *logInfo, w http.ResponseWriter, r *http.Request) (int, error) {
	return addChainInternal(ctx, li, w, r, false)
}

func addPreChain(ctx context.Context, li *logInfo, w http.ResponseWriter, r *http.Request) (int, error) {
	return addChainInternal(ctx, li, w, r, true)
}

// PingTreeHead retrieves a tree head for the given log, and updates the STH
// timestamp metrics correspondingly.
// TODO(pavelkalinnikov): Should we cache the resulting STH?
func PingTreeHead(ctx context.Context, client trillian.TrillianLogClient, logID int64, prefix string) error {
	slr, err := getSignedLogRoot(ctx, client, logID, prefix)
	if err != nil {
		return err
	}
	lastSTHTimestamp.Set(float64(slr.TimestampNanos/1000/1000), strconv.FormatInt(logID, 10))
	lastSTHTreeSize.Set(float64(slr.TreeSize), strconv.FormatInt(logID, 10))
	return nil
}

func getSTH(ctx context.Context, li *logInfo, w http.ResponseWriter, r *http.Request) (int, error) {
	qctx := ctx
	if li.instanceOpts.RemoteQuotaUser != nil {
		rqu := li.instanceOpts.RemoteQuotaUser(r)
		qctx = context.WithValue(qctx, remoteQuotaCtxKey, rqu)
	}

	sth, err := li.sthGetter.GetSTH(qctx)
	if err != nil {
		return li.toHTTPStatus(err), err
	}
	lastSTHTimestamp.Set(float64(sth.Timestamp), strconv.FormatInt(li.logID, 10))
	lastSTHTreeSize.Set(float64(sth.TreeSize), strconv.FormatInt(li.logID, 10))

	if err := writeSTH(sth, w); err != nil {
		return http.StatusInternalServerError, err
	}
	return http.StatusOK, nil
}

// writeSTH marshals the STH to JSON and writes it to HTTP response.
func writeSTH(sth *ct.SignedTreeHead, w http.ResponseWriter) error {
	jsonRsp := ct.GetSTHResponse{
		TreeSize:       sth.TreeSize,
		SHA256RootHash: sth.SHA256RootHash[:],
		Timestamp:      sth.Timestamp,
	}
	var err error
	jsonRsp.TreeHeadSignature, err = tls.Marshal(sth.TreeHeadSignature)
	if err != nil {
		return fmt.Errorf("failed to tls.Marshal signature: %s", err)
	}

	w.Header().Set(contentTypeHeader, contentTypeJSON)
	jsonData, err := json.Marshal(&jsonRsp)
	if err != nil {
		return fmt.Errorf("failed to marshal response: %s", err)
	}

	_, err = w.Write(jsonData)
	if err != nil {
		// Probably too late for this as headers might have been written but we
		// don't know for sure.
		return fmt.Errorf("failed to write response data: %s", err)
	}

	return nil
}

func getSTHConsistency(ctx context.Context, li *logInfo, w http.ResponseWriter, r *http.Request) (int, error) {
	first, second, err := parseGetSTHConsistencyRange(r)
	if err != nil {
		return http.StatusBadRequest, fmt.Errorf("failed to parse consistency range: %s", err)
	}
	li.RequestLog.FirstAndSecond(ctx, first, second)
	var jsonRsp ct.GetSTHConsistencyResponse
	if first != 0 {
		req := trillian.GetConsistencyProofRequest{
			LogId:          li.logID,
			FirstTreeSize:  first,
			SecondTreeSize: second,
			ChargeTo:       li.chargeUser(r),
		}

		glog.V(2).Infof("%s: GetSTHConsistency(%d, %d) => grpc.GetConsistencyProof %+v", li.LogPrefix, first, second, req)
		rsp, err := li.rpcClient.GetConsistencyProof(ctx, &req)
		glog.V(2).Infof("%s: GetSTHConsistency <= grpc.GetConsistencyProof err=%v", li.LogPrefix, err)
		if err != nil {
			return li.toHTTPStatus(err), fmt.Errorf("backend GetConsistencyProof request failed: %s", err)
		}

		// We can get here with a tree size too small to satisfy the proof.
		if rsp.SignedLogRoot != nil && rsp.SignedLogRoot.TreeSize < second {
			return http.StatusBadRequest, fmt.Errorf("need tree size: %d for proof but only got: %d", second, rsp.SignedLogRoot.TreeSize)
		}

		// Additional sanity checks, none of the hashes in the returned path should be empty
		if !checkAuditPath(rsp.Proof.Hashes) {
			return http.StatusInternalServerError, fmt.Errorf("backend returned invalid proof: %v", rsp.Proof)
		}

		// We got a valid response from the server. Marshal it as JSON and return it to the client
		jsonRsp.Consistency = rsp.Proof.Hashes
		if jsonRsp.Consistency == nil {
			jsonRsp.Consistency = emptyProof
		}
	} else {
		glog.V(2).Infof("%s: GetSTHConsistency(%d, %d) starts from 0 so return empty proof", li.LogPrefix, first, second)
		jsonRsp.Consistency = emptyProof
	}

	w.Header().Set(contentTypeHeader, contentTypeJSON)
	jsonData, err := json.Marshal(&jsonRsp)
	if err != nil {
		return http.StatusInternalServerError, fmt.Errorf("failed to marshal get-sth-consistency resp: %s", err)
	}

	_, err = w.Write(jsonData)
	if err != nil {
		// Probably too late for this as headers might have been written but we don't know for sure
		return http.StatusInternalServerError, fmt.Errorf("failed to write get-sth-consistency resp: %s", err)
	}

	return http.StatusOK, nil
}

func getProofByHash(ctx context.Context, li *logInfo, w http.ResponseWriter, r *http.Request) (int, error) {
	// Accept any non empty hash that decodes from base64 and let the backend validate it further
	hash := r.FormValue(getProofParamHash)
	if len(hash) == 0 {
		return http.StatusBadRequest, errors.New("get-proof-by-hash: missing / empty hash param for get-proof-by-hash")
	}
	leafHash, err := base64.StdEncoding.DecodeString(hash)
	if err != nil {
		return http.StatusBadRequest, fmt.Errorf("get-proof-by-hash: invalid base64 hash: %s", err)
	}

	treeSize, err := strconv.ParseInt(r.FormValue(getProofParamTreeSize), 10, 64)
	if err != nil || treeSize < 1 {
		return http.StatusBadRequest, fmt.Errorf("get-proof-by-hash: missing or invalid tree_size: %v", r.FormValue(getProofParamTreeSize))
	}
	li.RequestLog.LeafHash(ctx, leafHash)
	li.RequestLog.TreeSize(ctx, treeSize)

	// Per RFC 6962 section 4.5 the API returns a single proof. This should be the lowest leaf index
	// Because we request order by sequence and we only passed one hash then the first result is
	// the correct proof to return
	req := trillian.GetInclusionProofByHashRequest{
		LogId:           li.logID,
		LeafHash:        leafHash,
		TreeSize:        treeSize,
		OrderBySequence: true,
		ChargeTo:        li.chargeUser(r),
	}
	rsp, err := li.rpcClient.GetInclusionProofByHash(ctx, &req)
	if err != nil {
		return li.toHTTPStatus(err), fmt.Errorf("backend GetInclusionProofByHash request failed: %s", err)
	}

	// We could fail to get the proof because the tree size that the server has
	// is not large enough.
	if rsp.SignedLogRoot != nil && rsp.SignedLogRoot.TreeSize < treeSize {
		return http.StatusNotFound, fmt.Errorf("log returned tree size: %d but we expected: %d", rsp.SignedLogRoot.TreeSize, treeSize)
	}

	// Additional sanity checks on the response.
	if len(rsp.Proof) == 0 {
		// The backend returns the STH even when there is no proof, so explicitly
		// map this to 4xx.
		return http.StatusNotFound, errors.New("get-proof-by-hash: backend did not return a proof")
	}
	if !checkAuditPath(rsp.Proof[0].Hashes) {
		return http.StatusInternalServerError, fmt.Errorf("get-proof-by-hash: backend returned invalid proof: %v", rsp.Proof[0])
	}

	// All checks complete, marshal and return the response
	proofRsp := ct.GetProofByHashResponse{
		LeafIndex: rsp.Proof[0].LeafIndex,
		AuditPath: rsp.Proof[0].Hashes,
	}
	if proofRsp.AuditPath == nil {
		proofRsp.AuditPath = emptyProof
	}

	w.Header().Set(contentTypeHeader, contentTypeJSON)
	jsonData, err := json.Marshal(&proofRsp)
	if err != nil {
		glog.Warningf("%s: Failed to marshal get-proof-by-hash resp: %v", li.LogPrefix, proofRsp)
		return http.StatusInternalServerError, fmt.Errorf("failed to marshal get-proof-by-hash resp: %s", err)
	}

	_, err = w.Write(jsonData)
	if err != nil {
		// Probably too late for this as headers might have been written but we don't know for sure
		return http.StatusInternalServerError, fmt.Errorf("failed to write get-proof-by-hash resp: %s", err)
	}

	return http.StatusOK, nil
}

func getEntries(ctx context.Context, li *logInfo, w http.ResponseWriter, r *http.Request) (int, error) {
	// The first job is to parse the params and make sure they're sensible. We just make
	// sure the range is valid. We don't do an extra roundtrip to get the current tree
	// size and prefer to let the backend handle this case
	start, end, err := parseGetEntriesRange(r, MaxGetEntriesAllowed)
	if err != nil {
		return http.StatusBadRequest, fmt.Errorf("bad range on get-entries request: %s", err)
	}
	li.RequestLog.StartAndEnd(ctx, start, end)

	// Now make a request to the backend to get the relevant leaves
	var leaves []*trillian.LogLeaf
	if *getByRange {
		count := end + 1 - start
		req := trillian.GetLeavesByRangeRequest{
			LogId:      li.logID,
			StartIndex: start,
			Count:      count,
			ChargeTo:   li.chargeUser(r),
		}
		rsp, err := li.rpcClient.GetLeavesByRange(ctx, &req)
		if err != nil {
			return li.toHTTPStatus(err), fmt.Errorf("backend GetLeavesByRange request failed: %s", err)
		}
		if rsp.SignedLogRoot != nil && rsp.SignedLogRoot.TreeSize <= start {
			// If the returned tree is too small to contain any leaves return the 4xx
			// explicitly here.
			return http.StatusBadRequest, fmt.Errorf("need tree size: %d to get leaves but only got: %d", rsp.SignedLogRoot.TreeSize, start)
		}
		// Do some sanity checks on the result.
		if len(rsp.Leaves) > int(count) {
			return http.StatusInternalServerError, fmt.Errorf("backend returned too many leaves: %d vs [%d,%d]", len(rsp.Leaves), start, end)
		}
		for i, leaf := range rsp.Leaves {
			if leaf.LeafIndex != start+int64(i) {
				return http.StatusInternalServerError, fmt.Errorf("backend returned unexpected leaf index: rsp.Leaves[%d].LeafIndex=%d for range [%d,%d]", i, leaf.LeafIndex, start, end)
			}
		}
		leaves = rsp.Leaves
	} else {
		req := trillian.GetLeavesByIndexRequest{
			LogId:     li.logID,
			LeafIndex: buildIndicesForRange(start, end),
			ChargeTo:  li.chargeUser(r),
		}
		rsp, err := li.rpcClient.GetLeavesByIndex(ctx, &req)
		if err != nil {
			return li.toHTTPStatus(err), fmt.Errorf("backend GetLeavesByIndex request failed: %s", err)
		}

		if rsp.SignedLogRoot != nil && rsp.SignedLogRoot.TreeSize <= start {
			// If the returned tree is too small to contain any leaves return the 4xx
			// explicitly here. It was previously returned via the error status
			// mapping above.
			return http.StatusBadRequest, fmt.Errorf("need tree size: %d to get leaves but only got: %d", rsp.SignedLogRoot.TreeSize, start)
		}

		// Trillian doesn't guarantee the returned leaves are in order (they don't need to be
		// because each leaf comes with an index).  CT doesn't expose an index field and so
		// needs to return leaves in order.  Therefore, sort the results (and check for missing
		// or duplicate indices along the way).
		if err := sortLeafRange(rsp, start, end); err != nil {
			return http.StatusInternalServerError, fmt.Errorf("backend get-entries range invalid: %s", err)
		}
		leaves = rsp.Leaves
	}

	// Now we've checked the RPC response and it seems to be valid we need
	// to serialize the leaves in JSON format for the HTTP response. Doing a
	// round trip via the leaf deserializer gives us another chance to
	// prevent bad / corrupt data from reaching the client.
	jsonRsp, err := marshalGetEntriesResponse(li, leaves)
	if err != nil {
		return http.StatusInternalServerError, fmt.Errorf("failed to process leaves returned from backend: %s", err)
	}

	w.Header().Set(contentTypeHeader, contentTypeJSON)
	jsonData, err := json.Marshal(&jsonRsp)
	if err != nil {
		return http.StatusInternalServerError, fmt.Errorf("failed to marshal get-entries resp: %s", err)
	}

	_, err = w.Write(jsonData)
	if err != nil {
		// Probably too late for this as headers might have been written but we don't know for sure
		return http.StatusInternalServerError, fmt.Errorf("failed to write get-entries resp: %s", err)
	}

	return http.StatusOK, nil
}

func getRoots(ctx context.Context, li *logInfo, w http.ResponseWriter, r *http.Request) (int, error) {
	// Pull out the raw certificates from the parsed versions
	rawCerts := make([][]byte, 0, len(li.validationOpts.trustedRoots.RawCertificates()))
	for _, cert := range li.validationOpts.trustedRoots.RawCertificates() {
		rawCerts = append(rawCerts, cert.Raw)
	}

	jsonMap := make(map[string]interface{})
	jsonMap[jsonMapKeyCertificates] = rawCerts
	enc := json.NewEncoder(w)
	err := enc.Encode(jsonMap)
	if err != nil {
		glog.Warningf("%s: get_roots failed: %v", li.LogPrefix, err)
		return http.StatusInternalServerError, fmt.Errorf("get-roots failed with: %s", err)
	}

	return http.StatusOK, nil
}

// See RFC 6962 Section 4.8.
func getEntryAndProof(ctx context.Context, li *logInfo, w http.ResponseWriter, r *http.Request) (int, error) {
	// Ensure both numeric params are present and look reasonable.
	leafIndex, treeSize, err := parseGetEntryAndProofParams(r)
	if err != nil {
		return http.StatusBadRequest, fmt.Errorf("failed to parse get-entry-and-proof params: %s", err)
	}
	li.RequestLog.LeafIndex(ctx, leafIndex)
	li.RequestLog.TreeSize(ctx, treeSize)

	req := trillian.GetEntryAndProofRequest{
		LogId:     li.logID,
		LeafIndex: leafIndex,
		TreeSize:  treeSize,
		ChargeTo:  li.chargeUser(r),
	}
	rsp, err := li.rpcClient.GetEntryAndProof(ctx, &req)
	if err != nil {
		return li.toHTTPStatus(err), fmt.Errorf("backend GetEntryAndProof request failed: %s", err)
	}

	if rsp.SignedLogRoot != nil && rsp.SignedLogRoot.TreeSize < treeSize {
		// If tree size is not large enough return the 4xx here, would previously
		// have come from the error status mapping above.
		return http.StatusBadRequest, fmt.Errorf("need tree size: %d for proof but only got: %d", req.TreeSize, rsp.SignedLogRoot.TreeSize)
	}

	// Apply some checks that we got reasonable data from the backend
	if rsp.Leaf == nil || len(rsp.Leaf.LeafValue) == 0 || rsp.Proof == nil {
		return http.StatusInternalServerError, fmt.Errorf("got RPC bad response, possible extra info: %v", rsp)
	}
	if treeSize > 1 && len(rsp.Proof.Hashes) == 0 {
		return http.StatusInternalServerError, fmt.Errorf("got RPC bad response (missing proof), possible extra info: %v", rsp)
	}

	// Build and marshal the response to the client
	jsonRsp := ct.GetEntryAndProofResponse{
		LeafInput: rsp.Leaf.LeafValue,
		ExtraData: rsp.Leaf.ExtraData,
		AuditPath: rsp.Proof.Hashes,
	}

	w.Header().Set(contentTypeHeader, contentTypeJSON)
	jsonData, err := json.Marshal(&jsonRsp)
	if err != nil {
		return http.StatusInternalServerError, fmt.Errorf("failed to marshal get-entry-and-proof resp: %s", err)
	}

	_, err = w.Write(jsonData)
	if err != nil {

		// Probably too late for this as headers might have been written but we don't know for sure
		return http.StatusInternalServerError, fmt.Errorf("failed to write get-entry-and-proof resp: %s", err)
	}

	return http.StatusOK, nil
}

// Generates a custom error page to give more information on why something didn't work
// TODO(Martin2112): Not sure if we want to expose any detail or not
func sendHTTPError(w http.ResponseWriter, statusCode int, err error) {
	http.Error(w, fmt.Sprintf("%s\n%v", http.StatusText(statusCode), err), statusCode)
}

// getRPCDeadlineTime calculates the future time an RPC should expire based on our config
func getRPCDeadlineTime(li *logInfo) time.Time {
	return li.TimeSource.Now().Add(li.instanceOpts.Deadline)
}

// verifyAddChain is used by add-chain and add-pre-chain. It does the checks that the supplied
// cert is of the correct type and chains to a trusted root.
// TODO(Martin2112): This may not implement all the RFC requirements. Check what is provided
// by fixchain (called by this code) plus the ones here to make sure that it is compliant.
func verifyAddChain(li *logInfo, req ct.AddChainRequest, expectingPrecert bool) ([]*x509.Certificate, error) {
	// We already checked that the chain is not empty so can move on to verification
	validPath, err := ValidateChain(req.Chain, li.validationOpts)
	if err != nil {
		// We rejected it because the cert failed checks or we could not find a path to a root etc.
		// Lots of possible causes for errors
		return nil, fmt.Errorf("chain failed to verify: %s", err)
	}

	isPrecert, err := IsPrecertificate(validPath[0])
	if err != nil {
		return nil, fmt.Errorf("precert test failed: %s", err)
	}

	// The type of the leaf must match the one the handler expects
	if isPrecert != expectingPrecert {
		if expectingPrecert {
			glog.Warningf("%s: Cert (or precert with invalid CT ext) submitted as precert chain: %x", li.LogPrefix, req.Chain)
		} else {
			glog.Warningf("%s: Precert (or cert with invalid CT ext) submitted as cert chain: %x", li.LogPrefix, req.Chain)
		}
		return nil, fmt.Errorf("cert / precert mismatch: %T", expectingPrecert)
	}

	return validPath, nil
}

func extractRawCerts(chain []*x509.Certificate) []ct.ASN1Cert {
	raw := make([]ct.ASN1Cert, len(chain))
	for i, cert := range chain {
		raw[i] = ct.ASN1Cert{Data: cert.Raw}
	}
	return raw
}

// buildLogLeafForAddChain does the hashing to build a LogLeaf that will be
// sent to the backend by add-chain and add-pre-chain endpoints.
func buildLogLeafForAddChain(li *logInfo,
	merkleLeaf ct.MerkleTreeLeaf, chain []*x509.Certificate, isPrecert bool,
) (trillian.LogLeaf, error) {
	raw := extractRawCerts(chain)
	return util.BuildLogLeaf(li.LogPrefix, merkleLeaf, 0, raw[0], raw[1:], isPrecert)
}

// marshalAndWriteAddChainResponse is used by add-chain and add-pre-chain to create and write
// the JSON response to the client
func marshalAndWriteAddChainResponse(sct *ct.SignedCertificateTimestamp, signer crypto.Signer, w http.ResponseWriter) error {
	logID, err := GetCTLogID(signer.Public())
	if err != nil {
		return fmt.Errorf("failed to marshal logID: %s", err)
	}
	sig, err := tls.Marshal(sct.Signature)
	if err != nil {
		return fmt.Errorf("failed to marshal signature: %s", err)
	}

	rsp := ct.AddChainResponse{
		SCTVersion: sct.SCTVersion,
		Timestamp:  sct.Timestamp,
		ID:         logID[:],
		Extensions: base64.StdEncoding.EncodeToString(sct.Extensions),
		Signature:  sig,
	}

	w.Header().Set(contentTypeHeader, contentTypeJSON)
	jsonData, err := json.Marshal(&rsp)
	if err != nil {
		return fmt.Errorf("failed to marshal add-chain: %s", err)
	}

	_, err = w.Write(jsonData)
	if err != nil {
		return fmt.Errorf("failed to write add-chain resp: %s", err)
	}

	return nil
}

func parseGetEntriesRange(r *http.Request, maxRange int64) (int64, int64, error) {
	start, err := strconv.ParseInt(r.FormValue(getEntriesParamStart), 10, 64)
	if err != nil {
		return 0, 0, err
	}

	end, err := strconv.ParseInt(r.FormValue(getEntriesParamEnd), 10, 64)
	if err != nil {
		return 0, 0, err
	}

	if start < 0 || end < 0 {
		return 0, 0, fmt.Errorf("start (%d) and end (%d) parameters must be >= 0", start, end)
	}
	if start > end {
		return 0, 0, fmt.Errorf("start (%d) and end (%d) is not a valid range", start, end)
	}

	count := end - start + 1
	if count > maxRange {
		end = start + maxRange - 1
	}

	return start, end, nil
}

func parseGetEntryAndProofParams(r *http.Request) (int64, int64, error) {
	leafIndex, err := strconv.ParseInt(r.FormValue(getEntryAndProofParamLeafIndex), 10, 64)
	if err != nil {
		return 0, 0, err
	}

	treeSize, err := strconv.ParseInt(r.FormValue(getEntryAndProofParamTreeSize), 10, 64)
	if err != nil {
		return 0, 0, err
	}

	if treeSize <= 0 {
		return 0, 0, fmt.Errorf("tree_size must be > 0, got: %d", treeSize)
	}
	if leafIndex < 0 {
		return 0, 0, fmt.Errorf("leaf_index must be >= 0, got: %d", treeSize)
	}
	if leafIndex >= treeSize {
		return 0, 0, fmt.Errorf("leaf_index %d out of range for tree of size %d", leafIndex, treeSize)
	}

	return leafIndex, treeSize, nil
}

func parseGetSTHConsistencyRange(r *http.Request) (int64, int64, error) {
	firstVal := r.FormValue(getSTHConsistencyParamFirst)
	secondVal := r.FormValue(getSTHConsistencyParamSecond)
	if firstVal == "" {
		return 0, 0, errors.New("parameter 'first' is required")
	}
	if secondVal == "" {
		return 0, 0, errors.New("parameter 'second' is required")
	}

	first, err := strconv.ParseInt(firstVal, 10, 64)
	if err != nil {
		return 0, 0, errors.New("parameter 'first' is malformed")
	}

	second, err := strconv.ParseInt(secondVal, 10, 64)
	if err != nil {
		return 0, 0, errors.New("parameter 'second' is malformed")
	}

	if first < 0 || second < 0 {
		return 0, 0, fmt.Errorf("first and second params cannot be <0: %d %d", first, second)
	}
	if second < first {
		return 0, 0, fmt.Errorf("invalid first, second params: %d %d", first, second)
	}

	return first, second, nil
}

// buildIndicesForRange expands the range out, the backend allows for non contiguous leaf fetches
// but the CT spec doesn't. The input values should have been checked for consistency before calling
// this.
func buildIndicesForRange(start, end int64) []int64 {
	indices := make([]int64, 0, end-start+1)
	for i := start; i <= end; i++ {
		indices = append(indices, i)
	}
	return indices
}

type byLeafIndex []*trillian.LogLeaf

func (ll byLeafIndex) Len() int {
	return len(ll)
}
func (ll byLeafIndex) Swap(i, j int) {
	ll[i], ll[j] = ll[j], ll[i]
}
func (ll byLeafIndex) Less(i, j int) bool {
	return ll[i].LeafIndex < ll[j].LeafIndex
}

// sortLeafRange re-orders the leaves in rsp to be in ascending order by LeafIndex.  It also
// checks that the resulting range of leaves in rsp is valid, starting at start and finishing
// at end (or before) without duplicates.
func sortLeafRange(rsp *trillian.GetLeavesByIndexResponse, start, end int64) error {
	if got := int64(len(rsp.Leaves)); got > (end + 1 - start) {
		return fmt.Errorf("backend returned too many leaves: %d v [%d,%d]", got, start, end)
	}
	sort.Sort(byLeafIndex(rsp.Leaves))
	for i, leaf := range rsp.Leaves {
		if leaf.LeafIndex != (start + int64(i)) {
			return fmt.Errorf("backend returned unexpected leaf index: rsp.Leaves[%d].LeafIndex=%d for range [%d,%d]", i, leaf.LeafIndex, start, end)
		}
	}

	return nil
}

// marshalGetEntriesResponse does the conversion from the backend response to the one we need for
// an RFC compliant JSON response to the client.
func marshalGetEntriesResponse(li *logInfo, leaves []*trillian.LogLeaf) (ct.GetEntriesResponse, error) {
	jsonRsp := ct.GetEntriesResponse{}

	for _, leaf := range leaves {
		// We're only deserializing it to ensure it's valid, don't need the result. We still
		// return the data if it fails to deserialize as otherwise the root hash could not
		// be verified. However this indicates a potentially serious failure in log operation
		// or data storage that should be investigated.
		var treeLeaf ct.MerkleTreeLeaf
		if rest, err := tls.Unmarshal(leaf.LeafValue, &treeLeaf); err != nil {
			glog.Errorf("%s: Failed to deserialize Merkle leaf from backend: %d", li.LogPrefix, leaf.LeafIndex)
		} else if len(rest) > 0 {
			glog.Errorf("%s: Trailing data after Merkle leaf from backend: %d", li.LogPrefix, leaf.LeafIndex)
		}

		extraData := leaf.ExtraData
		if len(extraData) == 0 {
			glog.Errorf("%s: Missing ExtraData for leaf %d", li.LogPrefix, leaf.LeafIndex)
		}
		jsonRsp.Entries = append(jsonRsp.Entries, ct.LeafEntry{
			LeafInput: leaf.LeafValue,
			ExtraData: extraData,
		})
	}

	return jsonRsp, nil
}

// checkAuditPath does a quick scan of the proof we got from the backend for consistency.
// All the hashes should be non zero length.
func checkAuditPath(path [][]byte) bool {
	for _, node := range path {
		if len(node) != sha256.Size {
			return false
		}
	}
	return true
}

func (li *logInfo) toHTTPStatus(err error) int {
	if li.instanceOpts.ErrorMapper != nil {
		if status, ok := li.instanceOpts.ErrorMapper(err); ok {
			return status
		}
	}

	rpcStatus, ok := status.FromError(err)
	if !ok {
		return http.StatusInternalServerError
	}

	switch rpcStatus.Code() {
	case codes.OK:
		return http.StatusOK
	case codes.Canceled, codes.DeadlineExceeded:
		return http.StatusRequestTimeout
	case codes.InvalidArgument, codes.OutOfRange, codes.AlreadyExists:
		return http.StatusBadRequest
	case codes.NotFound:
		return http.StatusNotFound
	case codes.PermissionDenied, codes.ResourceExhausted:
		return http.StatusForbidden
	case codes.Unauthenticated:
		return http.StatusUnauthorized
	case codes.FailedPrecondition:
		return http.StatusPreconditionFailed
	case codes.Aborted:
		return http.StatusConflict
	case codes.Unimplemented:
		return http.StatusNotImplemented
	case codes.Unavailable:
		return http.StatusServiceUnavailable
	default:
		return http.StatusInternalServerError
	}
}
