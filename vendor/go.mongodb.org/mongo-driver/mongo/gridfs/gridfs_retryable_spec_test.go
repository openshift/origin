package gridfs

import (
	"bytes"
	"context"
	"encoding/json"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/bsontype"
	"go.mongodb.org/mongo-driver/event"
	"go.mongodb.org/mongo-driver/internal/testutil"
	testhelpers "go.mongodb.org/mongo-driver/internal/testutil/helpers"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.mongodb.org/mongo-driver/mongo/readconcern"
	"go.mongodb.org/mongo-driver/mongo/writeconcern"
	"go.mongodb.org/mongo-driver/x/bsonx"
	"io/ioutil"
	"path"
	"strings"
	"testing"
)

const gridFSRetryDir = "../../data/retryable-reads/"

var gridFSStartedChan = make(chan *event.CommandStartedEvent, 100)

var gridFSMonitor = &event.CommandMonitor{
	Started: func(ctx context.Context, cse *event.CommandStartedEvent) {
		gridFSStartedChan <- cse
	},
}

type gridFSRetryTestFile struct {
	RunOns       []runOn `json:"runOn"`
	DatabaseName string  `json:"database_name"`
	BucketName   string  `json:"bucket_name"`
	Data         struct {
		Files  []json.RawMessage `json:"fs.files"`
		Chunks []json.RawMessage `json:"fs.chunks"`
	} `json:"data"`
	Tests []gridFSRetryTest `json:"tests"`
}

type gridFSRetryTest struct {
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

func TestGridFSRetryableReadSpec(t *testing.T) {
	for _, testFile := range testhelpers.FindJSONFilesInDir(t, gridFSRetryDir) {
		if !strings.HasPrefix(testFile, "gridfs") {
			continue
		}
		runGridFSRetryReadTestFile(t, testFile)
	}
}

func runGridFSRetryReadTestFile(t *testing.T, testFileName string) {
	content, err := ioutil.ReadFile(path.Join(gridFSRetryDir, testFileName))
	require.NoError(t, err)

	var testFile gridFSRetryTestFile
	err = json.Unmarshal(content, &testFile)
	require.NoError(t, err)

	t.Run(testFileName, func(t *testing.T) {
		for _, test := range testFile.Tests {
			t.Run(test.Description, func(t *testing.T) {

				for len(gridFSStartedChan) > 0 {
					<-gridFSStartedChan
				}

				cs := testutil.ConnString(t)
				hosts := cs.Hosts[0:1]
				if test.UseMultipleMongoses {
					hosts = cs.Hosts
				}

				clientOpts := options.Client().ApplyURI(cs.String()).SetMonitor(gridFSMonitor).SetWriteConcern(writeconcern.New(writeconcern.WMajority()))
				clientOpts.ReadConcern = readconcern.Majority()
				clientOpts.Hosts = hosts
				retryOnClient, err := mongo.NewClient(clientOpts)
				require.NoError(t, err, "unable to create client")
				err = retryOnClient.Connect(ctx)
				require.NoError(t, err, "unable to connect to client")

				f := false
				clientOpts.RetryReads = &f
				retryOffClient, err := mongo.NewClient(clientOpts)
				require.NoError(t, err, "unable to create client")
				err = retryOffClient.Connect(ctx)
				require.NoError(t, err, "unable to connect to client")

				dbAdmin := retryOnClient.Database("admin")
				skipIfNecessaryRunOnSlice(t, testFile.RunOns, dbAdmin)

				defer func() {
					_ = retryOnClient.Disconnect(ctx)
					_ = retryOffClient.Disconnect(ctx)
				}()
				runRetryGridFSTest(t, &testFile, &test, retryOnClient, retryOffClient)
			})
		}
	})
}

func runRetryGridFSTest(t *testing.T, testFile *gridFSRetryTestFile, test *gridFSRetryTest, retryOnClient, retryOffClient *mongo.Client) {
	bucket, cleanup := setupTest(t, testFile, test, retryOnClient, retryOffClient)
	defer cleanup()

Loop:
	for _, op := range test.Operations {
		if op.Object != "gridfsbucket" && op.Object != "" {
			t.Fatalf("unrecognized op.Object: %v", op.Object)
		}

		switch op.Name {
		case "download":
			downloadStream := bytes.NewBuffer(downloadBuffer)
			_, err := bucket.DownloadToStream(op.Arguments.Oid, downloadStream)
			if op.Error {
				require.Error(t, err, "download failed to error when expected")
				break Loop
			}
			require.NoError(t, err, "download errored unexpectedly")
		case "download_by_name":
			if op.Arguments.Filename == "" {
				t.Fatalf("unable to find filename in arguments")
			}
			_, err := bucket.OpenDownloadStreamByName(op.Arguments.Filename)
			if op.Error {
				require.Error(t, err, "download_by_name failed to error when expected")
				break Loop
			}
			require.NoError(t, err, "download_by_name errored unexpectedly")
		default:
			t.Fatalf("unrecognized operation name: %v", op.Name)
		}

		if op.Result != nil {
			t.Fatalf("unexpected result in GridFS test: %v", op.Result)
		}
	}
	checkExpectations(t, test.Expectations)
}

func checkExpectations(t *testing.T, expectations []expectation) {
	for _, expectation := range expectations {
		var evt *event.CommandStartedEvent
		select {
		case evt = <-gridFSStartedChan:
		default:
			require.Fail(t, "Expected command started event", expectation.CommandStartedEvent.CommandName)
		}
		if evt == nil {
			t.Fatalf("nil command started event occured")
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

func setupTest(t *testing.T, testFile *gridFSRetryTestFile, test *gridFSRetryTest, retryOnClient, retryOffClient *mongo.Client) (*Bucket, func()) {
	cleanup := func() {
		clearCollections(t)
		for len(gridFSStartedChan) > 0 {
			<-gridFSStartedChan
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
	db = client.Database(dbName)
	cleanup = func(oldCleanup func()) func() {
		return func() {
			_ = db.Drop(ctx)
			oldCleanup()
		}
	}(cleanup)

	skipIfNecessaryRunOnSlice(t, testFile.RunOns, db)

	testFiles := db.Collection("fs.files")
	testChunks := db.Collection("fs.chunks")
	chunkSize := loadInitialFiles(t, testFile.Data.Files, testFile.Data.Chunks, testFiles, testChunks, nil, nil)
	if chunkSize == 0 {
		chunkSize = DefaultChunkSize
	}

	opts := make([]*options.BucketOptions, 0)
	if testFile.BucketName != "" {
		opt := options.GridFSBucket()
		opt.Name = &testFile.BucketName
		opts = append(opts, opt)
	}
	opt := options.GridFSBucket()
	opt.SetChunkSizeBytes(chunkSize)
	opts = append(opts, opt)

	bucket, err := NewBucket(db, opts...)
	require.NoError(t, err, "unable to create bucket")
	err = bucket.SetReadDeadline(deadline)
	require.NoError(t, err, "unable to set ReadDeadline")
	err = bucket.SetWriteDeadline(deadline)
	require.NoError(t, err, "unable to set WriteDeadline")

	if test.FailPoint != nil {
		doc := createFailPointDoc(t, test.FailPoint)
		err := client.Database("admin").RunCommand(ctx, doc).Err()
		require.NoError(t, err)

		cleanup = func(oldCleanup func()) func() {
			return func() {
				// disable failpoint if specified
				res := client.Database("admin").RunCommand(ctx, bsonx.Doc{
					{"configureFailPoint", bsonx.String(test.FailPoint.ConfigureFailPoint)},
					{"mode", bsonx.String("off")},
				})
				require.NoError(t, res.Err(), "unable to deactivate failpoint")

				// do all the rest of the cleanup
				oldCleanup()
			}
		}(cleanup)
	}

	for len(gridFSStartedChan) > 0 {
		<-gridFSStartedChan
	}

	return bucket, cleanup
}
