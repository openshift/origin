// Copyright (C) MongoDB, Inc. 2017-present.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may
// not use this file except in compliance with the License. You may obtain
// a copy of the License at http://www.apache.org/licenses/LICENSE-2.0

package mongo

import (
	"context"
	"encoding/json"
	"io/ioutil"
	"os"
	"path"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/event"
	"go.mongodb.org/mongo-driver/internal/testutil"
	testhelpers "go.mongodb.org/mongo-driver/internal/testutil/helpers"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.mongodb.org/mongo-driver/mongo/readpref"
	"go.mongodb.org/mongo-driver/mongo/writeconcern"
	"go.mongodb.org/mongo-driver/x/bsonx"
	"go.mongodb.org/mongo-driver/x/mongo/driver"
	"go.mongodb.org/mongo-driver/x/mongo/driver/connstring"
	"go.mongodb.org/mongo-driver/x/mongo/driver/session"
	"go.mongodb.org/mongo-driver/x/mongo/driver/topology"
)

const retryWritesDir = "../data/retryable-writes"

type retryWriteTestFile struct {
	Data             json.RawMessage  `json:"data"`
	MinServerVersion string           `json:"minServerVersion"`
	MaxServerVersion string           `json:"maxServerVersion"`
	Tests            []*retryTestCase `json:"tests"`
}

type retryTestCase struct {
	Description   string                 `json:"description"`
	FailPoint     *failPoint             `json:"failPoint"`
	ClientOptions map[string]interface{} `json:"clientOptions"`
	Operation     *retryWriteOperation   `json:"operation"`
	Outcome       *retryOutcome          `json:"outcome"`
}

type retryWriteOperation struct {
	Name      string                 `json:"name"`
	Arguments map[string]interface{} `json:"arguments"`
}

type retryOutcome struct {
	Error      bool            `json:"error"`
	Result     json.RawMessage `json:"result"`
	Collection struct {
		Name string          `json:"name"`
		Data json.RawMessage `json:"data"`
	} `json:"collection"`
}

var retryWritesMonitoredTopology *topology.Topology
var retryWritesMonitoredTopologyOnce sync.Once

var retryWritesStartedChan = make(chan *event.CommandStartedEvent, 100)

var retryWritesMonitor = &event.CommandMonitor{
	Started: func(ctx context.Context, cse *event.CommandStartedEvent) {
		retryWritesStartedChan <- cse
	},
}

func TestTxnNumberIncluded(t *testing.T) {
	client := createRetryMonitoredClient(t, retryWritesMonitor)
	client.retryWrites = true

	db := client.Database("retry-writes")

	version, err := getServerVersion(db)
	require.NoError(t, err)
	if shouldSkipRetryTest(t, version) {
		t.Skip()
	}

	doc1 := map[string]interface{}{"x": 1}
	doc2 := map[string]interface{}{"y": 2}
	update := map[string]interface{}{"$inc": 1}
	var cases = []struct {
		op          *retryWriteOperation
		includesTxn bool
	}{
		{&retryWriteOperation{Name: "deleteOne"}, true},
		{&retryWriteOperation{Name: "deleteMany"}, false},
		{&retryWriteOperation{Name: "updateOne", Arguments: map[string]interface{}{"update": update}}, true},
		{&retryWriteOperation{Name: "updateMany", Arguments: map[string]interface{}{"update": update}}, false},
		{&retryWriteOperation{Name: "replaceOne"}, true},
		{&retryWriteOperation{Name: "insertOne", Arguments: map[string]interface{}{"document": doc1}}, true},
		{&retryWriteOperation{Name: "insertMany", Arguments: map[string]interface{}{
			"ordered": true, "documents": []interface{}{doc1, doc2}}}, true},
		{&retryWriteOperation{Name: "insertMany", Arguments: map[string]interface{}{
			"ordered": false, "documents": []interface{}{doc1, doc2}}}, true},
		{&retryWriteOperation{Name: "findOneAndReplace"}, true},
		{&retryWriteOperation{Name: "findOneAndUpdate", Arguments: map[string]interface{}{"update": update}}, true},
		{&retryWriteOperation{Name: "findOneAndDelete"}, true},
	}

	err = db.Drop(ctx)
	require.NoError(t, err)

	for _, tc := range cases {
		t.Run(tc.op.Name, func(t *testing.T) {
			coll := db.Collection(tc.op.Name)
			err = coll.Drop(ctx)
			require.NoError(t, err)

			// insert sample data
			_, err = coll.InsertOne(ctx, doc1)
			require.NoError(t, err)
			_, err = coll.InsertOne(ctx, doc2)
			require.NoError(t, err)

			for len(retryWritesStartedChan) > 0 {
				<-retryWritesStartedChan
			}

			executeRetryOperation(t, tc.op, nil, coll)

			var evt *event.CommandStartedEvent
			select {
			case evt = <-retryWritesStartedChan:
			default:
				require.Fail(t, "Expected command started event")
			}

			if tc.includesTxn {
				require.NotNil(t, evt.Command.Lookup("txnNumber"))
			} else {
				require.Equal(t, evt.Command.Lookup("txnNumber"), bson.RawValue{})
			}
		})
	}
}

func TestRetryableWritesErrorOnMMAPV1(t *testing.T) {
	name := "test"
	version, err := getServerVersion(createTestDatabase(t, &name))
	require.NoError(t, err)

	if shouldSkipRetryTest(t, version) {
		t.Skip("only run on 3.6.x and not on standalone")
	}

	client := createTestClient(t)
	require.NoError(t, err)
	db := client.Database("test")
	defer func() { _ = db.Drop(context.Background()) }()
	coll := client.Database("test").Collection("test")
	defer func() { _ = coll.Drop(context.Background()) }()

	res := db.RunCommand(context.Background(), bson.D{
		{"serverStatus", 1},
	})
	noerr(t, res.Err())

	storageEngine, ok := res.rdr.Lookup("storageEngine", "name").StringValueOK()
	if !ok || storageEngine != "mmapv1" {
		t.Skip("only run on mmapv1")
	}

	_, err = coll.InsertOne(context.Background(), bson.D{
		{"_id", 1},
	})
	require.Equal(t, driver.ErrUnsupportedStorageEngine, err)
}

// test case for all RetryableWritesSpec tests
func TestRetryableWritesSpec(t *testing.T) {
	for _, file := range testhelpers.FindJSONFilesInDir(t, retryWritesDir) {
		runRetryWritesTestFile(t, path.Join(retryWritesDir, file))
	}
}

func runRetryWritesTestFile(t *testing.T, filepath string) {
	if strings.Contains(filepath, "bulk") {
		return
	}
	content, err := ioutil.ReadFile(filepath)
	require.NoError(t, err)

	var testfile retryWriteTestFile
	require.NoError(t, json.Unmarshal(content, &testfile))

	dbName := "admin"
	dbAdmin := createTestDatabase(t, &dbName)

	version, err := getServerVersion(dbAdmin)
	require.NoError(t, err)

	// check if we should skip all retry tests
	if shouldSkipRetryTest(t, version) || os.Getenv("TOPOLOGY") == "sharded_cluster" {
		t.Skip()
	}

	// check if we should skip individual test file
	if shouldSkip(t, testfile.MinServerVersion, testfile.MaxServerVersion, dbAdmin) {
		return
	}

	for _, test := range testfile.Tests {
		runRetryWriteTestCase(t, test, testfile.Data, dbAdmin)
	}

}

func runRetryWriteTestCase(t *testing.T, test *retryTestCase, data json.RawMessage, dbAdmin *Database) {
	t.Run(test.Description, func(t *testing.T) {
		client := createTestClient(t)

		db := client.Database("retry-writes")
		collName := sanitizeCollectionName("retry-writes", test.Description)

		err := db.Drop(ctx)
		require.NoError(t, err)

		// insert data if present
		coll := db.Collection(collName)
		docsToInsert := docSliceToInterfaceSlice(docSliceFromRaw(t, data))
		if len(docsToInsert) > 0 {
			coll2, err := coll.Clone(options.Collection().SetWriteConcern(writeconcern.New(writeconcern.WMajority())))
			require.NoError(t, err)
			_, err = coll2.InsertMany(ctx, docsToInsert)
			require.NoError(t, err)
		}

		// configure failpoint if needed
		if test.FailPoint != nil {
			doc := createFailPointDoc(t, test.FailPoint)
			err := dbAdmin.RunCommand(ctx, doc).Err()
			require.NoError(t, err)

			defer func() {
				// disable failpoint if specified
				_ = dbAdmin.RunCommand(ctx, bsonx.Doc{
					{"configureFailPoint", bsonx.String(test.FailPoint.ConfigureFailPoint)},
					{"mode", bsonx.String("off")},
				})
			}()
		}

		addClientOptions(client, test.ClientOptions)

		if test.Operation != nil && test.Outcome != nil {
			executeRetryOperation(t, test.Operation, test.Outcome, coll)
		}

		if test.Outcome != nil {
			verifyCollectionContents(t, coll, test.Outcome.Collection.Data)
		}
	})

}

func executeRetryOperation(t *testing.T, op *retryWriteOperation, outcome *retryOutcome, coll *Collection) {
	switch op.Name {
	case "deleteOne":
		res, err := executeDeleteOne(nil, coll, op.Arguments)
		if outcome == nil {
			return
		}
		if outcome.Error {
			require.Error(t, err)
		} else {
			require.NoError(t, err)
			verifyDeleteResult(t, res, outcome.Result)
		}
	case "deleteMany":
		_, _ = executeDeleteMany(nil, coll, op.Arguments)
		// no checking required for deleteMany
	case "updateOne":
		res, err := executeUpdateOne(nil, coll, op.Arguments)
		if outcome == nil {
			return
		}
		if outcome.Error {
			require.Error(t, err)
		} else {
			require.NoError(t, err)
			verifyUpdateResult(t, res, outcome.Result)
		}
	case "updateMany":
		_, _ = executeUpdateMany(nil, coll, op.Arguments)
		// no checking required for updateMany
	case "replaceOne":
		res, err := executeReplaceOne(nil, coll, op.Arguments)
		if outcome == nil {
			return
		}
		if outcome.Error {
			require.Error(t, err)
		} else {
			require.NoError(t, err)
			verifyUpdateResult(t, res, outcome.Result)
		}
	case "insertOne":
		res, err := executeInsertOne(nil, coll, op.Arguments)
		if outcome == nil {
			return
		}
		if outcome.Error {
			require.Error(t, err)
		} else {
			require.NoError(t, err)
			verifyInsertOneResult(t, res, outcome.Result)
		}
	case "insertMany":
		res, err := executeInsertMany(nil, coll, op.Arguments)
		if outcome == nil {
			return
		}
		if outcome.Error {
			require.Error(t, err)
		} else {
			require.NoError(t, err)
			verifyInsertManyResult(t, res, outcome.Result)
		}
	case "findOneAndUpdate":
		res := executeFindOneAndUpdate(nil, coll, op.Arguments)
		if outcome == nil {
			return
		}
		if outcome.Error {
			require.Error(t, res.err)
		} else {
			require.NoError(t, res.err)
			verifySingleResult(t, res, outcome.Result)
		}
	case "findOneAndDelete":
		res := executeFindOneAndDelete(nil, coll, op.Arguments)
		if outcome == nil {
			return
		}
		if outcome.Error {
			require.Error(t, res.err)
		} else {
			require.NoError(t, res.err)
			verifySingleResult(t, res, outcome.Result)
		}
	case "findOneAndReplace":
		res := executeFindOneAndReplace(nil, coll, op.Arguments)
		if outcome == nil {
			return
		}
		if outcome.Error {
			require.Error(t, res.err)
		} else {
			require.NoError(t, res.err)
			verifySingleResult(t, res, outcome.Result)
		}
	case "bulkWrite":
		// TODO reenable when bulk writes implemented
		t.Skip("Skipping until bulk writes implemented")
	}
}

func createRetryMonitoredClient(t *testing.T, monitor *event.CommandMonitor) *Client {
	clock := &session.ClusterClock{}

	c := &Client{
		topology:       createRetryMonitoredTopology(t, clock, monitor),
		connString:     testutil.ConnString(t),
		readPreference: readpref.Primary(),
		clock:          clock,
		registry:       bson.DefaultRegistry,
		monitor:        monitor,
	}

	subscription, err := c.topology.Subscribe()
	testhelpers.RequireNil(t, err, "error subscribing to topology: %s", err)
	c.topology.SessionPool = session.NewPool(subscription.C)

	return c
}

func createRetryMonitoredTopology(t *testing.T, clock *session.ClusterClock, monitor *event.CommandMonitor) *topology.Topology {
	cs := testutil.ConnString(t)
	cs.HeartbeatInterval = time.Minute
	cs.HeartbeatIntervalSet = true

	opts := []topology.Option{
		topology.WithConnString(func(connstring.ConnString) connstring.ConnString { return cs }),
		topology.WithServerOptions(func(opts ...topology.ServerOption) []topology.ServerOption {
			return append(
				opts,
				topology.WithConnectionOptions(func(opts ...topology.ConnectionOption) []topology.ConnectionOption {
					return append(
						opts,
						topology.WithMonitor(func(*event.CommandMonitor) *event.CommandMonitor {
							return monitor
						}),
					)
				}),
				topology.WithClock(func(c *session.ClusterClock) *session.ClusterClock {
					return clock
				}),
			)
		}),
	}

	retryWritesMonitoredTopologyOnce.Do(func() {
		retryMonitoredTopo, err := topology.New(opts...)
		if err != nil {
			t.Fatal(err)
		}
		err = retryMonitoredTopo.Connect()
		if err != nil {
			t.Fatal(err)
		}

		retryWritesMonitoredTopology = retryMonitoredTopo
	})

	return retryWritesMonitoredTopology
}

// skip entire test suite if server version less than 3.6 OR not a replica set
func shouldSkipRetryTest(t *testing.T, serverVersion string) bool {
	return compareVersions(t, serverVersion, "3.6") < 0 ||
		os.Getenv("TOPOLOGY") == "server"
}
