// Copyright (C) MongoDB, Inc. 2017-present.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may
// not use this file except in compliance with the License. You may obtain
// a copy of the License at http://www.apache.org/licenses/LICENSE-2.0

package mongo

import (
	"encoding/json"
	"io/ioutil"
	"testing"

	"context"

	"strings"
	"time"

	"bytes"
	"os"
	"path"

	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/bsontype"
	"go.mongodb.org/mongo-driver/event"
	"go.mongodb.org/mongo-driver/internal/testutil"
	testhelpers "go.mongodb.org/mongo-driver/internal/testutil/helpers"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.mongodb.org/mongo-driver/mongo/readconcern"
	"go.mongodb.org/mongo-driver/mongo/readpref"
	"go.mongodb.org/mongo-driver/mongo/writeconcern"
	"go.mongodb.org/mongo-driver/x/bsonx"
	"go.mongodb.org/mongo-driver/x/bsonx/bsoncore"
	"go.mongodb.org/mongo-driver/x/mongo/driver/description"
	"go.mongodb.org/mongo-driver/x/mongo/driver/operation"
	"go.mongodb.org/mongo-driver/x/mongo/driver/session"
)

const transactionTestsDir = "../data/transactions"

type transTestFile struct {
	RunOn          []*runOn         `json:"runOn"`
	DatabaseName   string           `json:"database_name"`
	CollectionName string           `json:"collection_name"`
	Data           json.RawMessage  `json:"data"`
	Tests          []*transTestCase `json:"tests"`
}

type runOn struct {
	MinServerVersion string   `json:"minServerVersion"`
	MaxServerVersion string   `json:"maxServerVersion"`
	Topology         []string `json:"topology"`
}

type transTestCase struct {
	Description         string                 `json:"description"`
	SkipReason          string                 `json:"skipReason"`
	FailPoint           *failPoint             `json:"failPoint"`
	ClientOptions       map[string]interface{} `json:"clientOptions"`
	SessionOptions      map[string]interface{} `json:"sessionOptions"`
	Operations          []*transOperation      `json:"operations"`
	Outcome             *transOutcome          `json:"outcome"`
	Expectations        []*expectation         `json:"expectations"`
	UseMultipleMongoses bool                   `json:"useMultipleMongoses"`
}

type failPoint struct {
	ConfigureFailPoint string          `json:"configureFailPoint"`
	Mode               json.RawMessage `json:"mode"`
	Data               *failPointData  `json:"data"`
}

type failPointData struct {
	FailCommands                  []string `json:"failCommands"`
	CloseConnection               bool     `json:"closeConnection"`
	ErrorCode                     int32    `json:"errorCode"`
	FailBeforeCommitExceptionCode int32    `json:"failBeforeCommitExceptionCode"`
	WriteConcernError             *struct {
		Code   int32  `json:"code"`
		Name   string `json:"codeName"`
		Errmsg string `json:"errmsg"`
	} `json:"writeConcernError"`
}

type transOperation struct {
	Name              string                 `json:"name"`
	Object            string                 `json:"object"`
	CollectionOptions map[string]interface{} `json:"collectionOptions"`
	Result            json.RawMessage        `json:"result"`
	Arguments         json.RawMessage        `json:"arguments"`
	ArgMap            map[string]interface{}
	Error             bool `json:"error"`
}

type transOutcome struct {
	Collection struct {
		Data json.RawMessage `json:"data"`
	} `json:"collection"`
}

type expectation struct {
	CommandStartedEvent struct {
		CommandName  string          `json:"command_name"`
		DatabaseName string          `json:"database_name"`
		Command      json.RawMessage `json:"command"`
	} `json:"command_started_event"`
}

type transError struct {
	ErrorContains      string   `bson:"errorContains"`
	ErrorCodeName      string   `bson:"errorCodeName"`
	ErrorLabelsContain []string `bson:"errorLabelsContain"`
	ErrorLabelsOmit    []string `bson:"errorLabelsOmit"`
}

var commandStarted []*event.CommandStartedEvent

var transMonitor = &event.CommandMonitor{
	Started: func(ctx context.Context, cse *event.CommandStartedEvent) {
		commandStarted = append(commandStarted, cse)
	},
}

// test case for all TransactionSpec tests
func TestTransactionSpec(t *testing.T) {
	for _, file := range testhelpers.FindJSONFilesInDir(t, transactionTestsDir) {
		t.Run(file, func(t *testing.T) {
			runTransactionTestFile(t, path.Join(transactionTestsDir, file))
		})
	}
}

func runTransactionTestFile(t *testing.T, filepath string) {
	content, err := ioutil.ReadFile(filepath)
	require.NoError(t, err)

	var testfile transTestFile
	require.NoError(t, json.Unmarshal(content, &testfile))

	dbName := "admin"
	dbAdmin := createTestDatabase(t, &dbName)

	version, err := getServerVersion(dbAdmin)
	require.NoError(t, err)
	runTest := len(testfile.RunOn) == 0
	for _, reqs := range testfile.RunOn {
		if shouldExecuteTest(t, version, reqs) {
			runTest = true
			break
		}
	}

	if !runTest {
		t.Skip()
	}

	if os.Getenv("TOPOLOGY") == "replica_set" {
		err := dbAdmin.RunCommand(ctx, bson.D{
			{"setParameter", 1},
			{"transactionLifetimeLimitSeconds", 3},
		}).Err()
		require.NoError(t, err)
	}

	for _, test := range testfile.Tests {
		runTransactionsTestCase(t, test, testfile, dbAdmin)
	}

}

func runTransactionsTestCase(t *testing.T, test *transTestCase, testfile transTestFile, dbAdmin *Database) {
	t.Run(test.Description, func(t *testing.T) {
		if len(test.SkipReason) > 0 {
			t.Skip(test.SkipReason)
		}

		// kill sessions from previously failed tests
		killSessions(t, dbAdmin.client)

		if testfile.CollectionName == "" {
			testfile.CollectionName = "collection_name"
		}
		collName := sanitizeCollectionName(testfile.DatabaseName, testfile.CollectionName)

		var shardedHost string
		var failPointNames []string

		defer disableFailpoints(t, &failPointNames)

		if os.Getenv("TOPOLOGY") == "sharded_cluster" {
			mongodbURI := testutil.ConnString(t)
			opts := options.Client().ApplyURI(mongodbURI.String())
			hosts := opts.Hosts
			for _, host := range hosts {
				shardClient, err := NewClient(opts.SetHosts([]string{host}))
				require.NoError(t, err)
				addClientOptions(shardClient, test.ClientOptions)
				err = shardClient.Connect(context.Background())
				require.NoError(t, err)
				killSessions(t, shardClient)
				// Workaround for SERVER-39704
				if test.Description == "distinct" {
					shardDatabase := shardClient.Database(testfile.DatabaseName)
					_, err = shardDatabase.Collection(collName).Distinct(context.Background(), "x", bsonx.Doc{})
					require.NoError(t, err)
				}
				if !test.UseMultipleMongoses {
					shardedHost = host
					break
				}
				_ = shardClient.Disconnect(ctx)
			}
		}

		client := createTransactionsMonitoredClient(t, transMonitor, test.ClientOptions, shardedHost)
		addClientOptions(client, test.ClientOptions)

		db := client.Database(testfile.DatabaseName)

		_ = db.Collection(collName, options.Collection().SetWriteConcern(writeconcern.New(writeconcern.WMajority()))).Drop(context.Background())

		err := db.RunCommand(
			context.Background(),
			bsonx.Doc{{"create", bsonx.String(collName)}},
		).Err()
		require.NoError(t, err)

		// client for setup data
		var i map[string]interface{}
		setupClient := createTransactionsMonitoredClient(t, transMonitor, i, shardedHost)
		setupDb := setupClient.Database(testfile.DatabaseName)

		// insert data if present
		coll := setupDb.Collection(collName)
		docsToInsert := docSliceToInterfaceSlice(docSliceFromRaw(t, testfile.Data))
		if len(docsToInsert) > 0 {
			coll2, err := coll.Clone(options.Collection().SetWriteConcern(writeconcern.New(writeconcern.WMajority())))
			require.NoError(t, err)
			_, err = coll2.InsertMany(context.Background(), docsToInsert)
			require.NoError(t, err)
		}

		if test.FailPoint != nil {
			doc := createFailPointDoc(t, test.FailPoint)
			mongodbURI := testutil.ConnString(t)
			opts := options.Client().ApplyURI(mongodbURI.String())
			if len(shardedHost) > 0 {
				opts.SetHosts([]string{shardedHost})
			}
			fpClient, err := NewClient(opts)
			require.NoError(t, err)
			addClientOptions(fpClient, test.ClientOptions)
			err = fpClient.Connect(context.Background())
			require.NoError(t, err)
			fpDatabase := fpClient.Database("admin")
			err = fpDatabase.RunCommand(ctx, doc).Err()
			require.NoError(t, err)
			_ = fpClient.Disconnect(context.Background())
			failPointNames = append(failPointNames, test.FailPoint.ConfigureFailPoint)
		}

		var sess0Opts *options.SessionOptions
		var sess1Opts *options.SessionOptions
		if test.SessionOptions != nil {
			if test.SessionOptions["session0"] != nil {
				sess0Opts = getSessionOptions(test.SessionOptions["session0"].(map[string]interface{}))
			} else if test.SessionOptions["session1"] != nil {
				sess1Opts = getSessionOptions(test.SessionOptions["session1"].(map[string]interface{}))
			}
		}

		commandStarted = commandStarted[:0]

		session0, err := client.StartSession(sess0Opts)
		require.NoError(t, err)
		session1, err := client.StartSession(sess1Opts)
		require.NoError(t, err)

		sess0 := session0.(*sessionImpl)
		sess1 := session1.(*sessionImpl)

		lsid0 := sess0.clientSession.SessionID
		lsid1 := sess1.clientSession.SessionID

		defer func() {
			sess0.EndSession(ctx)
			sess1.EndSession(ctx)
		}()

		for _, op := range test.Operations {
			if op.Name == "count" {
				t.Skip("count has been deprecated")
			}

			// Arguments aren't marshaled directly into a map because runcommand
			// needs to convert them into BSON docs.  We convert them to a map here
			// for getting the session and for all other collection operations
			op.ArgMap = getArgMap(t, op.Arguments)

			// Get the session if specified in arguments
			var sess *sessionImpl
			if sessStr, ok := op.ArgMap["session"]; ok {
				switch sessStr.(string) {
				case "session0":
					sess = sess0
				case "session1":
					sess = sess1
				}
			}

			if op.Object == "testRunner" {
				fpName, err := executeTestRunnerOperation(t, op, sess)
				require.NoError(t, err)
				if len(fpName) > 0 {
					failPointNames = append(failPointNames, fpName)
				}
				continue
			}

			// create collection with default read preference Primary (needed to prevent server selection fail)
			coll := db.Collection(collName, options.Collection().SetReadPreference(readpref.Primary()).SetReadConcern(readconcern.Local()))
			addCollectionOptions(coll, op.CollectionOptions)

			// execute the command on given object
			var err error
			switch op.Object {
			case "session0":
				err = executeSessionOperation(t, op, sess0, collName, db)
			case "session1":
				err = executeSessionOperation(t, op, sess1, collName, db)
			case "collection":
				err = executeCollectionOperation(t, op, sess, coll)
			case "database":
				err = executeDatabaseOperation(t, op, sess, db)
			}

			// ensure error is what we expect
			verifyError(t, err, op.Result)

			if op.Error {
				require.Error(t, err)
			}
		}

		// Needs to be done here (in spite of defer) because some tests
		// require end session to be called before we check expectation
		sess0.EndSession(ctx)
		sess1.EndSession(ctx)

		checkExpectations(t, test.Expectations, lsid0, lsid1)

		disableFailpoints(t, &failPointNames)

		if test.Outcome != nil {
			// Verify with primary read pref
			coll2, err := coll.Clone(options.Collection().SetReadPreference(readpref.Primary()).SetReadConcern(readconcern.Local()))
			require.NoError(t, err)
			verifyCollectionContents(t, coll2, test.Outcome.Collection.Data)
		}

	})
}

func killSessions(t *testing.T, client *Client) {
	err := operation.NewCommand(bsoncore.BuildDocument(nil, bsoncore.AppendArrayElement(nil, "killAllSessions", bsoncore.BuildArray(nil)))).
		Database("admin").ServerSelector(description.WriteSelector()).Deployment(client.topology).Execute(context.Background())
	require.NoError(t, err)
}

func disableFailpoints(t *testing.T, failPointNames *[]string) {
	mongodbURI := testutil.ConnString(t)
	opts := options.Client().ApplyURI(mongodbURI.String())
	hosts := opts.Hosts
	for _, host := range hosts {
		shardClient, err := NewClient(opts.SetHosts([]string{host}))
		require.NoError(t, err)
		require.NoError(t, shardClient.Connect(ctx))
		// disable failpoint if specified
		for _, failpt := range *failPointNames {
			require.NoError(t, shardClient.Database("admin").RunCommand(ctx, bson.D{
				{"configureFailPoint", failpt},
				{"mode", "off"},
			}).Err())
		}
		_ = shardClient.Disconnect(ctx)
	}
}

func createTransactionsMonitoredClient(t *testing.T, monitor *event.CommandMonitor, opts map[string]interface{}, host string) *Client {
	clock := &session.ClusterClock{}

	cs := testutil.ConnString(t)
	if len(host) > 0 {
		cs.Hosts = []string{host}
	}
	c := &Client{
		topology:       createMonitoredTopology(t, clock, monitor, &cs),
		connString:     cs,
		readPreference: readpref.Primary(),
		clock:          clock,
		registry:       bson.NewRegistryBuilder().Build(),
		monitor:        monitor,
	}
	addClientOptions(c, opts)

	subscription, err := c.topology.Subscribe()
	testhelpers.RequireNil(t, err, "error subscribing to topology: %s", err)
	c.topology.SessionPool = session.NewPool(subscription.C)

	return c
}

func createFailPointDoc(t *testing.T, failPoint *failPoint) bsonx.Doc {
	failDoc := bsonx.Doc{{"configureFailPoint", bsonx.String(failPoint.ConfigureFailPoint)}}

	modeBytes, err := failPoint.Mode.MarshalJSON()
	require.NoError(t, err)

	var modeStruct struct {
		Times int32 `json:"times"`
		Skip  int32 `json:"skip"`
	}
	err = json.Unmarshal(modeBytes, &modeStruct)
	if err != nil {
		failDoc = append(failDoc, bsonx.Elem{"mode", bsonx.String("alwaysOn")})
	} else {
		modeDoc := bsonx.Doc{}
		if modeStruct.Times != 0 {
			modeDoc = append(modeDoc, bsonx.Elem{"times", bsonx.Int32(modeStruct.Times)})
		}
		if modeStruct.Skip != 0 {
			modeDoc = append(modeDoc, bsonx.Elem{"skip", bsonx.Int32(modeStruct.Skip)})
		}
		failDoc = append(failDoc, bsonx.Elem{"mode", bsonx.Document(modeDoc)})
	}

	if failPoint.Data != nil {
		dataDoc := bsonx.Doc{}

		if failPoint.Data.FailCommands != nil {
			failCommandElems := make(bsonx.Arr, len(failPoint.Data.FailCommands))
			for i, str := range failPoint.Data.FailCommands {
				failCommandElems[i] = bsonx.String(str)
			}
			dataDoc = append(dataDoc, bsonx.Elem{"failCommands", bsonx.Array(failCommandElems)})
		}

		if failPoint.Data.CloseConnection {
			dataDoc = append(dataDoc, bsonx.Elem{"closeConnection", bsonx.Boolean(failPoint.Data.CloseConnection)})
		}

		if failPoint.Data.ErrorCode != 0 {
			dataDoc = append(dataDoc, bsonx.Elem{"errorCode", bsonx.Int32(failPoint.Data.ErrorCode)})
		}

		if failPoint.Data.WriteConcernError != nil {
			dataDoc = append(dataDoc,
				bsonx.Elem{"writeConcernError", bsonx.Document(bsonx.Doc{
					{"code", bsonx.Int32(failPoint.Data.WriteConcernError.Code)},
					{"codeName", bsonx.String(failPoint.Data.WriteConcernError.Name)},
					{"errmsg", bsonx.String(failPoint.Data.WriteConcernError.Errmsg)},
				})},
			)
		}

		if failPoint.Data.FailBeforeCommitExceptionCode != 0 {
			dataDoc = append(dataDoc, bsonx.Elem{"failBeforeCommitExceptionCode", bsonx.Int32(failPoint.Data.FailBeforeCommitExceptionCode)})
		}

		failDoc = append(failDoc, bsonx.Elem{"data", bsonx.Document(dataDoc)})
	}

	return failDoc
}

func executeSessionOperation(t *testing.T, op *transOperation, sess *sessionImpl, collName string, db *Database) error {
	switch op.Name {
	case "startTransaction":
		// options are only argument
		var transOpts *options.TransactionOptions
		if op.ArgMap["options"] != nil {
			transOpts = getTransactionOptions(op.ArgMap["options"].(map[string]interface{}))
		}
		return sess.StartTransaction(transOpts)
	case "commitTransaction":
		return sess.CommitTransaction(ctx)
	case "abortTransaction":
		return sess.AbortTransaction(ctx)
	case "withTransaction":
		return executeWithTransaction(t, sess, collName, db, op.Arguments)
	case "endSession":
		sess.EndSession(ctx)
		return nil
	default:
		require.Fail(t, "unknown operation", op.Name)
	}
	return nil
}

func executeCollectionOperation(t *testing.T, op *transOperation, sess *sessionImpl, coll *Collection) error {
	switch op.Name {
	case "countDocuments":
		_, err := executeCountDocuments(sess, coll, op.ArgMap)
		// no results to verify with count
		return err
	case "distinct":
		res, err := executeDistinct(sess, coll, op.ArgMap)
		if !resultHasError(t, op.Result) && err == nil {
			verifyDistinctResult(t, res, op.Result)
		}
		return err
	case "insertOne":
		res, err := executeInsertOne(sess, coll, op.ArgMap)
		if !resultHasError(t, op.Result) && err == nil {
			verifyInsertOneResult(t, res, op.Result)
		}
		return err
	case "insertMany":
		res, err := executeInsertMany(sess, coll, op.ArgMap)
		if !resultHasError(t, op.Result) && err == nil {
			verifyInsertManyResult(t, res, op.Result)
		}
		return err
	case "find":
		res, err := executeFind(sess, coll, op.ArgMap)
		if !resultHasError(t, op.Result) && err == nil {
			verifyCursorResult(t, res, op.Result)
		}
		return err
	case "findOneAndDelete":
		res := executeFindOneAndDelete(sess, coll, op.ArgMap)
		if !resultHasError(t, op.Result) && res.err == nil {
			verifySingleResult(t, res, op.Result)
		}
		return res.err
	case "findOneAndUpdate":
		res := executeFindOneAndUpdate(sess, coll, op.ArgMap)
		if !resultHasError(t, op.Result) && res.err == nil {
			verifySingleResult(t, res, op.Result)
		}
		return res.err
	case "findOneAndReplace":
		res := executeFindOneAndReplace(sess, coll, op.ArgMap)
		if !resultHasError(t, op.Result) && res.err == nil {
			verifySingleResult(t, res, op.Result)
		}
		return res.err
	case "deleteOne":
		res, err := executeDeleteOne(sess, coll, op.ArgMap)
		if !resultHasError(t, op.Result) && err == nil {
			verifyDeleteResult(t, res, op.Result)
		}
		return err
	case "deleteMany":
		res, err := executeDeleteMany(sess, coll, op.ArgMap)
		if !resultHasError(t, op.Result) && err == nil {
			verifyDeleteResult(t, res, op.Result)
		}
		return err
	case "updateOne":
		res, err := executeUpdateOne(sess, coll, op.ArgMap)
		if !resultHasError(t, op.Result) && err == nil {
			verifyUpdateResult(t, res, op.Result)
		}
		return err
	case "updateMany":
		res, err := executeUpdateMany(sess, coll, op.ArgMap)
		if !resultHasError(t, op.Result) && err == nil {
			verifyUpdateResult(t, res, op.Result)
		}
		return err
	case "replaceOne":
		res, err := executeReplaceOne(sess, coll, op.ArgMap)
		if !resultHasError(t, op.Result) && err == nil {
			verifyUpdateResult(t, res, op.Result)
		}
		return err
	case "aggregate":
		res, err := executeAggregate(sess, coll, op.ArgMap)
		if !resultHasError(t, op.Result) && err == nil {
			verifyCursorResult(t, res, op.Result)
		}
		return err
	case "bulkWrite":
		// TODO reenable when bulk writes implemented
		t.Skip("Skipping until bulk writes implemented")
	}
	return nil
}

func executeDatabaseOperation(t *testing.T, op *transOperation, sess *sessionImpl, db *Database) error {
	switch op.Name {
	case "runCommand":
		var result bsonx.Doc
		err := executeRunCommand(sess, db, op.ArgMap, op.Arguments).Decode(&result)
		if !resultHasError(t, op.Result) {
			res, err := result.MarshalBSON()
			if err != nil {
				return err
			}
			verifyRunCommandResult(t, res, op.Result)
		}
		return err
	}
	return nil
}

func executeTestRunnerOperation(t *testing.T, op *transOperation, sess *sessionImpl) (string, error) {
	switch op.Name {
	case "targetedFailPoint":
		failPtStr, ok := op.ArgMap["failPoint"]
		require.True(t, ok)
		var fp failPoint
		marshaled, err := json.Marshal(failPtStr.(map[string]interface{}))
		require.NoError(t, err)
		require.NoError(t, json.Unmarshal(marshaled, &fp))

		doc := createFailPointDoc(t, &fp)
		mongodbURI := testutil.ConnString(t)
		opts := options.Client().ApplyURI(mongodbURI.String())
		client, err := NewClient(opts.SetHosts([]string{sess.clientSession.PinnedServer.Addr.String()}))
		require.NoError(t, err)
		require.NoError(t, client.Connect(ctx))
		require.NoError(t, client.Database("admin").RunCommand(ctx, doc).Err())

		_ = client.Disconnect(ctx)

		return fp.ConfigureFailPoint, nil
	case "assertSessionPinned":
		require.NotNil(t, sess.clientSession.PinnedServer)
	case "assertSessionUnpinned":
		require.Nil(t, sess.clientSession.PinnedServer)
	case "assertSessionDirty":
		require.NotNil(t, sess.clientSession.Server)
		require.True(t, sess.clientSession.Server.Dirty)
	case "assertSessionNotDirty":
		require.NotNil(t, sess.clientSession.Server)
		require.False(t, sess.clientSession.Server.Dirty)
	case "assertSameLsidOnLastTwoCommands":
		require.True(t, sameLsidOnLastTwoCommandEvents(t))
	case "assertDifferentLsidOnLastTwoCommands":
		require.False(t, sameLsidOnLastTwoCommandEvents(t))
	default:
		require.Fail(t, "unknown operation", op.Name)
	}
	return "", nil
}

func sameLsidOnLastTwoCommandEvents(t *testing.T) bool {
	res := commandStarted[len(commandStarted)-2:]
	require.Equal(t, len(res), 2)
	if cmp.Equal(res[0].Command.Lookup("lsid"), res[1].Command.Lookup("lsid")) {
		return true
	}
	return false
}

func verifyError(t *testing.T, e error, result json.RawMessage) {
	expected := getErrorFromResult(t, result)
	if expected == nil {
		return
	}

	if cerr, ok := e.(CommandError); ok {
		if expected.ErrorCodeName != "" {
			require.NotNil(t, cerr)
			require.Equal(t, expected.ErrorCodeName, cerr.Name)
		}
		if expected.ErrorContains != "" {
			require.NotNil(t, cerr, "Expected error %v", expected.ErrorContains)
			require.Contains(t, strings.ToLower(cerr.Message), strings.ToLower(expected.ErrorContains))
		}
		if expected.ErrorLabelsContain != nil {
			require.NotNil(t, cerr)
			for _, l := range expected.ErrorLabelsContain {
				require.True(t, cerr.HasErrorLabel(l), "Error missing error label %s", l)
			}
		}
		if expected.ErrorLabelsOmit != nil {
			require.NotNil(t, cerr)
			for _, l := range expected.ErrorLabelsOmit {
				require.False(t, cerr.HasErrorLabel(l))
			}
		}
	} else {
		require.Equal(t, expected.ErrorCodeName, "")
		require.Equal(t, len(expected.ErrorLabelsContain), 0)
		// ErrorLabelsOmit can contain anything, since they are all omitted for e not type CommandError
		// so we do not check that here

		if expected.ErrorContains != "" {
			require.NotNil(t, e, "Expected error %v", expected.ErrorContains)
			require.Contains(t, strings.ToLower(e.Error()), strings.ToLower(expected.ErrorContains))
		}
	}
}

func resultHasError(t *testing.T, result json.RawMessage) bool {
	if result == nil {
		return false
	}
	res := getErrorFromResult(t, result)
	if res == nil {
		return false
	}
	return res.ErrorLabelsOmit != nil ||
		res.ErrorLabelsContain != nil ||
		res.ErrorCodeName != "" ||
		res.ErrorContains != ""
}

func getErrorFromResult(t *testing.T, result json.RawMessage) *transError {
	expectedBytes, err := result.MarshalJSON()
	require.NoError(t, err)

	var expected transError
	err = json.NewDecoder(bytes.NewBuffer(expectedBytes)).Decode(&expected)
	if err != nil {
		return nil
	}
	return &expected
}

func checkExpectations(t *testing.T, expectations []*expectation, id0 bsonx.Doc, id1 bsonx.Doc) {
	for i, expectation := range expectations {
		if i == len(commandStarted) {
			require.Fail(t, "Expected command started event", expectation.CommandStartedEvent.CommandName)
		}
		evt := commandStarted[i]

		require.Equal(t, expectation.CommandStartedEvent.CommandName, evt.CommandName)
		require.Equal(t, expectation.CommandStartedEvent.DatabaseName, evt.DatabaseName)

		jsonBytes, err := expectation.CommandStartedEvent.Command.MarshalJSON()
		require.NoError(t, err)

		expected := bsonx.Doc{}
		err = bson.UnmarshalExtJSON(jsonBytes, true, &expected)
		require.NoError(t, err)

		actual := evt.Command
		for _, elem := range expected {
			key := elem.Key
			val := elem.Value

			actualVal := actual.Lookup(key)

			// Keys that may be nil
			if val.Type() == bson.TypeNull {
				require.Equal(t, actual.Lookup(key), bson.RawValue{}, "Expected %s to be nil", key)
				continue
			} else if key == "ordered" {
				// TODO: some tests specify that "ordered" must be a key in the event but ordered isn't a valid option for some of these cases (e.g. insertOne)
				continue
			}

			// Keys that should not be nil
			require.NotEqual(t, actualVal.Type, bsontype.Null, "Expected %v, got nil for key: %s", elem, key)
			require.NoError(t, actualVal.Validate(), "Expected %v, couldn't validate", elem)
			if key == "lsid" {
				if val.StringValue() == "session0" {
					doc, err := bsonx.ReadDoc(actualVal.Document())
					require.NoError(t, err)
					require.True(t, id0.Equal(doc), "Session ID mismatch")
				}
				if val.StringValue() == "session1" {
					doc, err := bsonx.ReadDoc(actualVal.Document())
					require.NoError(t, err)
					require.True(t, id1.Equal(doc), "Session ID mismatch")
				}
			} else if key == "getMore" {
				require.NotNil(t, actualVal, "Expected %v, got nil for key: %s", elem, key)
				expectedCursorID := val.Int64()
				// ignore if equal to 42
				if expectedCursorID != 42 {
					require.Equal(t, expectedCursorID, actualVal.Int64())
				}
			} else if key == "readConcern" {
				rcExpectDoc := val.Document()
				rcActualDoc := actualVal.Document()
				clusterTime := rcExpectDoc.Lookup("afterClusterTime")
				level := rcExpectDoc.Lookup("level")
				if clusterTime.Type() != bsontype.Null {
					require.NotNil(t, rcActualDoc.Lookup("afterClusterTime"))
				}
				if level.Type() != bsontype.Null {
					doc, err := bsonx.ReadDoc(rcActualDoc)
					require.NoError(t, err)
					compareElements(t, rcExpectDoc.LookupElement("level"), doc.LookupElement("level"))
				}
			} else {
				doc, err := bsonx.ReadDoc(actual)
				require.NoError(t, err)
				compareElements(t, elem, doc.LookupElement(key))
			}

		}
	}
}

// convert operation arguments from raw message into map
func getArgMap(t *testing.T, args json.RawMessage) map[string]interface{} {
	if args == nil {
		return nil
	}
	var argmap map[string]interface{}
	err := json.Unmarshal(args, &argmap)
	require.NoError(t, err)
	return argmap
}

func getSessionOptions(opts map[string]interface{}) *options.SessionOptions {
	sessOpts := options.Session()
	for name, opt := range opts {
		switch name {
		case "causalConsistency":
			sessOpts = sessOpts.SetCausalConsistency(opt.(bool))
		case "defaultTransactionOptions":
			transOpts := opt.(map[string]interface{})
			if transOpts["readConcern"] != nil {
				sessOpts = sessOpts.SetDefaultReadConcern(getReadConcern(transOpts["readConcern"]))
			}
			if transOpts["writeConcern"] != nil {
				sessOpts = sessOpts.SetDefaultWriteConcern(getWriteConcern(transOpts["writeConcern"]))
			}
			if transOpts["readPreference"] != nil {
				sessOpts = sessOpts.SetDefaultReadPreference(getReadPref(transOpts["readPreference"]))
			}
			if transOpts["maxCommitTimeMS"] != nil {
				sessOpts = sessOpts.SetDefaultMaxCommitTime(getMaxCommitTime(transOpts["maxCommitTimeMS"]))
			}
		}
	}

	return sessOpts
}

func getTransactionOptions(opts map[string]interface{}) *options.TransactionOptions {
	transOpts := options.Transaction()
	for name, opt := range opts {
		switch name {
		case "writeConcern":
			transOpts = transOpts.SetWriteConcern(getWriteConcern(opt))
		case "readPreference":
			transOpts = transOpts.SetReadPreference(getReadPref(opt))
		case "readConcern":
			transOpts = transOpts.SetReadConcern(getReadConcern(opt))
		case "maxCommitTimeMS":
			transOpts = transOpts.SetMaxCommitTime(getMaxCommitTime(opt))
		}
	}
	return transOpts
}

func getWriteConcern(opt interface{}) *writeconcern.WriteConcern {
	if w, ok := opt.(map[string]interface{}); ok {
		var newTimeout time.Duration
		if conv, ok := w["wtimeout"].(float64); ok {
			newTimeout = time.Duration(int(conv)) * time.Millisecond
		}
		var newJ bool
		if conv, ok := w["j"].(bool); ok {
			newJ = conv
		}
		if conv, ok := w["w"].(string); ok && conv == "majority" {
			return writeconcern.New(writeconcern.WMajority(), writeconcern.J(newJ), writeconcern.WTimeout(newTimeout))
		} else if conv, ok := w["w"].(float64); ok {
			return writeconcern.New(writeconcern.W(int(conv)), writeconcern.J(newJ), writeconcern.WTimeout(newTimeout))
		}
	}
	return nil
}

func getReadConcern(opt interface{}) *readconcern.ReadConcern {
	return readconcern.New(readconcern.Level(opt.(map[string]interface{})["level"].(string)))
}

func getReadPref(opt interface{}) *readpref.ReadPref {
	if conv, ok := opt.(map[string]interface{}); ok {
		return readPrefFromString(conv["mode"].(string))
	}
	return nil
}

func getMaxCommitTime(opt interface{}) *time.Duration {
	if max, ok := opt.(float64); ok {
		res := time.Duration(max) * time.Millisecond
		return &res
	}
	return nil
}

func readPrefFromString(s string) *readpref.ReadPref {
	switch strings.ToLower(s) {
	case "primary":
		return readpref.Primary()
	case "primarypreferred":
		return readpref.PrimaryPreferred()
	case "secondary":
		return readpref.Secondary()
	case "secondarypreferred":
		return readpref.SecondaryPreferred()
	case "nearest":
		return readpref.Nearest()
	}
	return readpref.Primary()
}

func shouldExecuteTest(t *testing.T, serverVersion string, reqs *runOn) bool {
	if len(reqs.MinServerVersion) > 0 && compareVersions(t, serverVersion, reqs.MinServerVersion) < 0 {
		return false
	}
	if len(reqs.MaxServerVersion) > 0 && compareVersions(t, serverVersion, reqs.MaxServerVersion) > 0 {
		return false
	}
	if len(reqs.Topology) == 0 {
		return true
	}
	for _, top := range reqs.Topology {
		envTop := os.Getenv("TOPOLOGY")
		switch envTop {
		case "server":
			if top == "single" {
				return true
			}
		case "replica_set":
			if top == "replicaset" {
				return true
			}
		case "sharded_cluster":
			if top == "sharded" {
				return true
			}
		default:
			t.Fatalf("unrecognized TOPOLOGY: %v", envTop)
		}
	}
	return false
}
