package mongo

import (
	"context"
	"encoding/json"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/internal/testutil"
	testhelpers "go.mongodb.org/mongo-driver/internal/testutil/helpers"
	"go.mongodb.org/mongo-driver/mongo/readconcern"
	"io/ioutil"
	"os"
	"path"
	"strings"
	"testing"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/bsontype"
	"go.mongodb.org/mongo-driver/event"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.mongodb.org/mongo-driver/mongo/writeconcern"
	"go.mongodb.org/mongo-driver/x/bsonx"
)

const retryReadsDir = "../data/retryable-reads"

type retryReadTest struct {
	Description   string `json:"description"`
	ClientOptions *struct {
		RetryReads bool `json:"retryReads"`
	} `json:"clientOptions"`
	UseMultipleMongoses bool          `json:"useMultipleMongoses"`
	SkipReason          string        `json:"skipReason"`
	FailPoint           *failPoint    `json:"failPoint"`
	Operations          []op          `json:"operations"`
	Expectations        []expectation `json:"expectations"`
}

type retryReadTestFile struct {
	RunOns         []runOn         `json:"runOn"`
	DatabaseName   string          `json:"database_name"`
	CollectionName string          `json:"collection_name"`
	BucketName     string          `json:"bucket_name"`
	Data           json.RawMessage `json:"data"`
	Tests          []retryReadTest `json:"tests"`
}

type watcher interface {
	Watch(ctx context.Context, pipeline interface{}, opts ...*options.ChangeStreamOptions) (*ChangeStream, error)
}

var retryReadsStartedChan = make(chan *event.CommandStartedEvent, 100)

var retryReadsMonitor = &event.CommandMonitor{
	Started: func(ctx context.Context, cse *event.CommandStartedEvent) {
		retryReadsStartedChan <- cse
	},
}

func TestRetryableReadsSpec(t *testing.T) {
	for _, testFile := range testhelpers.FindJSONFilesInDir(t, retryReadsDir) {
		if strings.HasPrefix(testFile, "gridfs") {
			continue
		}
		runRetryReadsTestFile(t, testFile)
	}
}

func runRetryReadsTestFile(t *testing.T, testFileName string) {
	content, err := ioutil.ReadFile(path.Join(retryReadsDir, testFileName))
	require.NoError(t, err)

	var testFile retryReadTestFile
	err = json.Unmarshal(content, &testFile)
	require.NoError(t, err)

	t.Run(testFileName, func(t *testing.T) {
		for _, test := range testFile.Tests {
			t.Run(test.Description, func(t *testing.T) {

				for len(retryReadsStartedChan) > 0 {
					<-retryReadsStartedChan
				}

				cs := testutil.ConnString(t)
				hosts := cs.Hosts[0:1]
				if test.UseMultipleMongoses {
					hosts = cs.Hosts
				}

				clientOpts := options.Client().ApplyURI(cs.String()).SetMonitor(retryReadsMonitor).SetWriteConcern(writeconcern.New(writeconcern.WMajority()))
				clientOpts.ReadConcern = readconcern.Majority()
				clientOpts.Hosts = hosts
				retryOnClient, err := NewClient(clientOpts)
				require.NoError(t, err, "unable to create client")
				err = retryOnClient.Connect(ctx)
				require.NoError(t, err, "unable to connect to client")

				f := false
				clientOpts.RetryReads = &f
				retryOffClient, err := NewClient(clientOpts)
				require.NoError(t, err, "unable to create client")
				err = retryOffClient.Connect(ctx)
				require.NoError(t, err, "unable to connect to client")

				dbAdmin := retryOnClient.Database("admin")
				serverVersion, err := getServerVersion(dbAdmin)
				require.NoError(t, err)
				runTest := len(testFile.RunOns) == 0
				for _, reqs := range testFile.RunOns {
					if len(reqs.MinServerVersion) > 0 && compareVersions(t, serverVersion, reqs.MinServerVersion) < 0 {
						continue
					}
					if len(reqs.MaxServerVersion) > 0 && compareVersions(t, serverVersion, reqs.MaxServerVersion) > 0 {
						continue
					}
					if len(reqs.Topology) == 0 {
						runTest = true
						break
					}
					for _, top := range reqs.Topology {
						envTop := os.Getenv("TOPOLOGY")
						switch envTop {
						case "server":
							if top == "single" {
								runTest = true
								break
							}
						case "replica_set":
							if top == "replicaset" {
								runTest = true
								break
							}
						case "sharded_cluster":
							if top == "sharded" {
								runTest = true
								break
							}
						default:
							t.Fatalf("unrecognized TOPOLOGY: %v", envTop)
						}
					}
				}

				if !runTest {
					t.Skip()
				}

				defer func() {
					_ = retryOnClient.Disconnect(ctx)
					_ = retryOffClient.Disconnect(ctx)
				}()

				runRetryReadsTest(t, &test, &testFile, retryOnClient, retryOffClient)
			})
		}
	})
}

func runRetryReadsTest(t *testing.T, test *retryReadTest, testFile *retryReadTestFile, retryOnClient, retryOffClient *Client) {
	coll, cleanup := setUpTest(t, test, testFile, retryOnClient, retryOffClient)
	defer cleanup()

	for _, op := range test.Operations {
		runRetryReadOperation(t, test, &op, coll)
	}

	checkRetryExpectations(t, test.Expectations)
}

func setUpTest(t *testing.T, test *retryReadTest, testFile *retryReadTestFile, retryOnClient, retryOffClient *Client) (*Collection, func()) {
	cleanup := func() {
		for len(retryReadsStartedChan) > 0 {
			<-retryReadsStartedChan
		}
	}

	client := retryOnClient
	if test.ClientOptions != nil && !test.ClientOptions.RetryReads {
		client = retryOffClient
	}

	dbName := testFile.DatabaseName
	if dbName == "" {
		dbName = "retryable-reads-tests"
	}
	db := client.Database(dbName)

	err := db.Drop(ctx)
	require.NoError(t, err, "unable to drop database")

	collName := testFile.CollectionName
	if collName == "" {
		collName = "retryable-reads-tests"
	}
	collName = sanitizeCollectionName(dbName, collName)
	coll := db.Collection(collName)

	docsToInsert := docSliceToInterfaceSlice(docSliceFromRaw(t, testFile.Data))
	if len(docsToInsert) > 0 {
		wMajColl, err := coll.Clone(options.Collection().SetWriteConcern(writeconcern.New(writeconcern.WMajority())))
		require.NoError(t, err, "unable to setup w-majority collection")

		res, err := wMajColl.InsertMany(ctx, docsToInsert)
		require.NoError(t, err, "unable to insert test data")
		require.Equal(t, len(docsToInsert), len(res.InsertedIDs), "incorrect number of documents inserted")
	}

	if test.FailPoint != nil {
		doc := createFailPointDoc(t, test.FailPoint)
		err := client.Database("admin").RunCommand(ctx, doc).Err()
		require.NoError(t, err)

		//oldCleanup := cleanup
		cleanup = func(oldCleanup func()) func() {
			return func() {
				// disable failpoint if specified
				_ = client.Database("admin").RunCommand(ctx, bsonx.Doc{
					{"configureFailPoint", bsonx.String(test.FailPoint.ConfigureFailPoint)},
					{"mode", bsonx.String("off")},
				})

				// do all the rest of the cleanup
				oldCleanup()
			}
		}(cleanup)
	}

	for _, op := range test.Operations {
		replaceFloatsWithInts(op.Arguments)
	}
	for len(retryReadsStartedChan) > 0 {
		<-retryReadsStartedChan
	}

	return coll, cleanup
}

func runRetryReadOperation(t *testing.T, test *retryReadTest, op *op, coll *Collection) {
	switch op.Name {
	case "aggregate":
		var a aggregator
		opts := options.Collection().SetReadConcern(readconcern.Majority()).SetWriteConcern(writeconcern.New(writeconcern.WMajority()))
		switch op.Object {
		case "database":
			c, err := coll.Clone(opts)
			require.NoError(t, err, "unable to setup aggregator")
			a = c.Database()
		case "collection":
			c, err := coll.Clone(opts)
			require.NoError(t, err, "unable to setup aggregator")
			a = c
		default:
			t.Fatalf("unable to aggregate on object: %v", op.Object)
		}

		pipeline, ok := op.Arguments["pipeline"]
		require.True(t, ok, "unable to find pipeline argument in operation arguments")

		cur, err := a.Aggregate(ctx, pipeline)
		if op.Error {
			require.Error(t, err, "no error occurred in aggregation when one was expected")
			return
		}
		require.NoError(t, err, "aggregation errored unexpectedly")
		if op.Result == nil {
			return
		}
		//cur, err = a.Aggregate(ctx, pipeline) // this is very wierd but if you run it twice it will pass on sharded
		//require.NoError(t, err)
		verifyCursorResult(t, cur, op.Result)
	case "watch":
		var w watcher
		switch op.Object {
		case "database":
			w = coll.Database()
		case "collection":
			w = coll
		case "client":
			w = coll.Database().Client()
		default:
			t.Fatalf("unable to aggregate on object: %v", op.Object)
		}

		pipeline, ok := op.Arguments["pipeline"]
		if !ok {
			pipeline = Pipeline{}
		}
		cur, err := w.Watch(ctx, pipeline)

		if op.Error {
			require.Error(t, err, "no error occurred in aggregation when one was expected")
			return
		}

		require.NoError(t, err, "aggregation errored unexpectedly")
		if op.Result == nil {
			return
		}
		verifyCursorResult(t, cur, op.Result)
	case "count":
		t.Skip("count is not supported")
	case "distinct":
		if op.Object != "collection" && op.Object != "" {
			t.Fatalf("cannot distinct on object: %v", op.Object)
		}

		fieldName, ok := op.Arguments["fieldName"]
		if !ok {
			t.Fatalf("test missing fieldName")
		}
		filter, ok := op.Arguments["filter"]
		if !ok {
			filter = bson.D{}
		}

		distincts, err := coll.Distinct(ctx, fieldName.(string), filter)
		if op.Error {
			require.Error(t, err, "distinct failed to error when expected")
			return
		}

		if op.Result == nil {
			return
		}
		var arr []interface{}
		err = bson.UnmarshalExtJSON(op.Result, true, &arr)
		require.NoError(t, err, "unable to unmarshal operation result")
		require.Equal(t, len(arr), len(distincts), "length of distinct values differed from expected")
	case "countDocuments":
		if op.Object != "collection" && op.Object != "" {
			t.Fatalf("cannot countDocuments on object: %v", op.Object)
		}

		filter, ok := op.Arguments["filter"]
		if !ok {
			filter = bson.D{}
		}

		count, err := coll.CountDocuments(ctx, filter)
		if op.Error {
			require.Error(t, err, "CountDocuments failed to error when expected")
			return
		}
		require.NoError(t, err, "CountDocuments errored unexpectedly")

		if op.Result == nil {
			return
		}
		var res int64
		err = bson.UnmarshalExtJSON(op.Result, true, &res)
		require.Equal(t, res, count, "expected result differed from supplied CountDocuments")
	case "estimatedDocumentCount":
		if op.Object != "collection" && op.Object != "" {
			t.Fatalf("cannot EstimatedDocumentCount on object: %v", op.Object)
		}

		count, err := coll.EstimatedDocumentCount(ctx)
		if op.Error {
			require.Error(t, err, "EstimatedDocumentCount failed to error when expected")
			return
		}
		require.NoError(t, err, "EstimatedDocumentCount errored unexpectedly")

		if op.Result == nil {
			return
		}
		var res int64
		err = bson.UnmarshalExtJSON(op.Result, true, &res)
		require.Equal(t, res, count, "expected result differed from supplied EstimatedDocumentCount")
	case "find":
		if op.Object != "collection" && op.Object != "" {
			t.Fatalf("cannot Find on object: %v", op.Object)
		}

		filter, ok := op.Arguments["filter"]
		if !ok {
			filter = bson.D{}
		}

		opts := options.Find()

		limit, ok := op.Arguments["limit"]
		if ok {
			lim := int64(limit.(int32))
			opts.Limit = &lim
		}

		sort, ok := op.Arguments["sort"]
		if ok {
			opts.Sort = sort
		}

		cur, err := coll.Find(ctx, filter, opts)
		if op.Error {
			require.Error(t, err, "Find failed to error when expected")
			return
		}
		require.NoError(t, err, "Find errored unexpectedly")

		if op.Result == nil {
			return
		}
		verifyCursorResult(t, cur, op.Result)
	case "findOne":
		if op.Object != "collection" && op.Object != "" {
			t.Fatalf("cannot Find on object: %v", op.Object)
		}

		filter, ok := op.Arguments["filter"]
		if !ok {
			filter = bson.D{}
		}

		res := coll.FindOne(ctx, filter)
		if op.Error {
			require.Error(t, res.Err(), "EstimatedDocumentCount failed to error when expected")
			return
		}
		require.NoError(t, res.Err(), "EstimatedDocumentCount errored unexpectedly")

		if op.Result == nil {
			return
		}
		verifySingleResult(t, res, op.Result)
	case "listCollections":
		if op.Object != "database" && op.Object != "" {
			t.Fatalf("cannot Find on object: %v", op.Object)
		}

		filter, ok := op.Arguments["filter"]
		if !ok {
			filter = bson.D{}
		}

		cur, err := coll.Database().ListCollections(ctx, filter)
		if op.Error {
			require.Error(t, err, "EstimatedDocumentCount failed to error when expected")
			return
		}
		require.NoError(t, err, "EstimatedDocumentCount errored unexpectedly")

		if op.Result == nil {
			return
		}
		verifyCursorResult(t, cur, op.Result)
	case "listCollectionNames":
		if op.Object != "database" && op.Object != "" {
			t.Fatalf("cannot Find on object: %v", op.Object)
		}

		filter, ok := op.Arguments["filter"]
		if !ok {
			filter = bson.D{}
		}

		names, err := coll.Database().ListCollectionNames(ctx, filter)
		if op.Error {
			require.Error(t, err, "EstimatedDocumentCount failed to error when expected")
			return
		}
		require.NoError(t, err, "EstimatedDocumentCount errored unexpectedly")

		if op.Result == nil {
			return
		}
		var expectedNames []string
		err = bson.UnmarshalExtJSON(op.Result, true, &expectedNames)
		require.NoError(t, err, "unable to unmarshal op.Result")
		require.Equal(t, len(expectedNames), len(names), "number of collection names differed from expected")
	case "listCollectionObjects":
		t.Skipf("not testing listCollectionObjects, skipping")
	case "listDatabaseNames":
		if op.Object != "client" && op.Object != "" {
			t.Fatalf("cannot Find on object: %v", op.Object)
		}

		filter, ok := op.Arguments["filter"]
		if !ok {
			filter = bson.D{}
		}

		names, err := coll.Database().Client().ListDatabaseNames(ctx, filter)
		if op.Error {
			require.Error(t, err, "EstimatedDocumentCount failed to error when expected")
			return
		}
		require.NoError(t, err, "EstimatedDocumentCount errored unexpectedly")

		if op.Result == nil {
			return
		}
		var expectedNames []string
		err = bson.UnmarshalExtJSON(op.Result, true, &expectedNames)
		require.NoError(t, err, "unable to unmarshal op.Result")
		require.Equal(t, len(expectedNames), len(names), "number of collection names differed from expected")
	case "listDatabaseObjects":
		t.Skipf("not testing listDatabaseObjects, skipping")
	case "listDatabases":
		if op.Object != "client" && op.Object != "" {
			t.Fatalf("cannot Find on object: %v", op.Object)
		}

		filter, ok := op.Arguments["filter"]
		if !ok {
			filter = bson.D{}
		}

		_, err := coll.Database().Client().ListDatabases(ctx, filter)
		if op.Error {
			require.Error(t, err, "EstimatedDocumentCount failed to error when expected")
			return
		}
		require.NoError(t, err, "EstimatedDocumentCount errored unexpectedly")

		if op.Result != nil {
			t.Fatalf("there should not be any result in any operation for listDatabases")
		}
	case "listIndexes":
		if op.Object != "collection" && op.Object != "" {
			t.Fatalf("cannot Find on object: %v", op.Object)
		}

		cur, err := coll.Indexes().List(ctx)
		if op.Error {
			require.Error(t, err, "EstimatedDocumentCount failed to error when expected")
			return
		}
		require.NoError(t, err, "EstimatedDocumentCount errored unexpectedly")

		if op.Result == nil {
			return
		}
		verifyCursorResult(t, cur, op.Result)
	case "listIndexNames":
		t.Skipf("not testing listIndexNames, skipping")
	case "mapReduce":
		t.Skip("not testing mapReduce, skipping")
	default:
		t.Fatalf("unknown operation type, operation name: %v", op.Name)
	}
}

func checkRetryExpectations(t *testing.T, expectations []expectation) {
	for _, expectation := range expectations {
		var evt *event.CommandStartedEvent
		select {
		case evt = <-retryReadsStartedChan:
		default:
			require.Fail(t, "Expected command started event", expectation.CommandStartedEvent.CommandName)
		}

		if expectation.CommandStartedEvent.CommandName != "" {
			require.Equal(t, expectation.CommandStartedEvent.CommandName, evt.CommandName)
		}

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
			if key == "getMore" {
				require.NotNil(t, actualVal, "Expected %v, got nil for key: %s", elem, key)
				expectedCursorID := val.Int64()
				// ignore if equal to 42
				if expectedCursorID != 42 {
					require.Equal(t, expectedCursorID, actualVal.Int64())
				}
				continue
			}
			if key == "readConcern" {
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
				continue
			}
			doc, err := bsonx.ReadDoc(actual)
			require.NoError(t, err)
			compareElements(t, elem, doc.LookupElement(key))

		}
	}
}
