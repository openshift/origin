package mongo

import (
	"bytes"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/x/bsonx/bsoncore"
	"go.mongodb.org/mongo-driver/x/mongo/driver"
	"go.mongodb.org/mongo-driver/x/mongo/driver/description"
	"go.mongodb.org/mongo-driver/x/mongo/driver/drivertest"
	"go.mongodb.org/mongo-driver/x/mongo/driver/wiremessage"
)

type opQuery struct {
	flags                       wiremessage.QueryFlag
	fullCollectionName          string
	numToSkip, numToReturn      int32
	query, returnFieldsSelector bson.D
}

var legacyTestDb = "LegacyOperationDb"

func fullCollName(coll string) string {
	return legacyTestDb + "." + coll
}

// create collections foo and bar with 5 documents each
// create indexes on fields x and y
func initLegacyDb(t *testing.T, db *Database) {
	var docs []interface{}
	for i := 0; i < 5; i++ {
		docs = append(docs, bson.D{{"x", i}, {"y", i}})
	}

	collNames := []string{"foo", "bar"}
	for _, name := range collNames {
		coll := db.Collection(name)
		if _, err := coll.DeleteMany(ctx, bson.D{}); err != nil {
			t.Fatalf("error deleting docs for collection %s: %v\n", name, err)
		}

		if _, err := coll.InsertMany(ctx, docs); err != nil {
			t.Fatalf("error inserting docs into collection %s: %v", name, err)
		}

		iv := coll.Indexes()
		indexModels := []IndexModel{
			{Keys: bson.D{{"x", 1}}},
			{Keys: bson.D{{"y", 1}}},
		}
		if _, err := iv.CreateMany(ctx, indexModels); err != nil {
			t.Fatalf("erorr creating indexes on collection %s: %v", name, err)
		}
	}
}

func validateHeader(t *testing.T, wm []byte, expectedOpcode wiremessage.OpCode) []byte {
	actualLen := len(wm)
	var readLen int32
	var opcode wiremessage.OpCode
	var ok bool

	readLen, _, _, opcode, wm, ok = wiremessage.ReadHeader(wm)
	if !ok {
		t.Fatalf("could not read header")
	}
	if readLen != int32(actualLen) {
		t.Fatalf("length mismatch; expected %d, got %d", actualLen, readLen)
	}
	if opcode != expectedOpcode {
		t.Fatalf("opcode mismatch; expected %d, got %d", expectedOpcode, opcode)
	}

	return wm
}

func validateQueryWiremessage(t *testing.T, wm []byte, expected opQuery) {
	var numToSkip, numToReturn int32
	var flags wiremessage.QueryFlag
	var fullCollName string
	var query, returnFieldsSelector bsoncore.Document
	var ok bool

	wm = validateHeader(t, wm, wiremessage.OpQuery)

	flags, wm, ok = wiremessage.ReadQueryFlags(wm)
	if !ok {
		t.Fatalf("could not read flags")
	}
	if flags != expected.flags {
		t.Fatalf("flags mismatch; expected %d, got %d", expected.flags, flags)
	}

	fullCollName, wm, ok = wiremessage.ReadQueryFullCollectionName(wm)
	if !ok {
		t.Fatalf("could not read fullCollectionName")
	}
	if fullCollName != expected.fullCollectionName {
		t.Fatalf("fullCollectionName mismatch; expected %s, got %s", expected.fullCollectionName, fullCollName)
	}

	numToSkip, wm, ok = wiremessage.ReadQueryNumberToSkip(wm)
	if !ok {
		t.Fatalf("could not read numberToSkip")
	}
	if numToSkip != expected.numToSkip {
		t.Fatalf("numberToSkip mismatch; expected %d, got %d", expected.numToSkip, numToSkip)
	}

	numToReturn, wm, ok = wiremessage.ReadQueryNumberToReturn(wm)
	if !ok {
		t.Fatalf("could not read numberToReturn")
	}
	if numToSkip != expected.numToSkip {
		t.Fatalf("numberToReturn mismatch; expected %d, got %d", expected.numToReturn, numToReturn)
	}

	query, wm, ok = wiremessage.ReadQueryQuery(wm)
	if !ok {
		t.Fatalf("could not read query")
	}
	expectedQueryBytes, err := bson.Marshal(expected.query)
	if err != nil {
		t.Fatalf("unexpected error marshaling query: %v", err)
	}
	if !bytes.Equal(query, expectedQueryBytes) {
		t.Fatalf("query mismatch; expected %v, got %v", bsoncore.Document(expectedQueryBytes), query)
	}

	if len(expected.returnFieldsSelector) == 0 {
		if len(wm) != 0 {
			t.Fatalf("wiremessage had extraneous bytes")
		}
		return
	}

	returnFieldsSelector, wm, ok = wiremessage.ReadQueryReturnFieldsSelector(wm)
	if !ok {
		t.Fatalf("could not read returnFieldsSelector")
	}
	if len(wm) != 0 {
		t.Fatalf("wiremessage had extraneous bytes")
	}
	expectedRfsBytes, err := bson.Marshal(expected.returnFieldsSelector)
	if err != nil {
		t.Fatalf("error marshaling returnFieldsSelector: %v", err)
	}
	if !bytes.Equal(returnFieldsSelector, expectedRfsBytes) {
		t.Fatalf("returnFieldsSelector mismatch; expected %v, got %v", bsoncore.Document(expectedRfsBytes), returnFieldsSelector)
	}
}

// runs an operation given a BSON command and a deployment. Returns the response document and the Server the command
// was run against.
func runOperationWithDeployment(t *testing.T, cmd bson.D, deployment driver.Deployment, legacy driver.LegacyOperationKind) (bsoncore.Document, driver.Server) {
	cmdBytes, err := bson.Marshal(cmd)
	if err != nil {
		t.Fatalf("error marshalling command: %v\n", err)
	}
	commandFn := func(dst []byte, _ description.SelectedServer) ([]byte, error) {
		return append(dst, cmdBytes[4:len(cmdBytes)-1]...), nil
	}

	var response bsoncore.Document
	var server driver.Server
	processFn := func(res bsoncore.Document, srvr driver.Server, _ description.Server) error {
		response = res
		server = srvr
		return nil
	}

	op := driver.Operation{
		Database:          legacyTestDb,
		Deployment:        deployment,
		CommandFn:         commandFn,
		ProcessResponseFn: processFn,
		Legacy:            legacy,
	}
	if err = op.Execute(ctx, nil); err != nil {
		t.Fatalf("error during operation execution: %v", err)
	}

	return response, server
}

func parseAndIterateCursor(t *testing.T, res bsoncore.Document, server driver.Server, batchSize int32) []bson.Raw {
	cursorDoc := res.Lookup("cursor").Document()
	cursorID := cursorDoc.Lookup("id").Int64()
	ns := cursorDoc.Lookup("ns").StringValue()
	nsParts := strings.Split(ns, ".")
	collName := nsParts[len(nsParts)-1]
	batch := cursorDoc.Lookup("firstBatch").Array()

	var docs []bson.Raw
	for {
		// add the documents from the current batch
		batchValues, err := batch.Values()
		if err != nil {
			t.Fatalf("error getting batch values: %v", err)
		}
		for _, rawDoc := range batchValues {
			var doc bson.Raw
			if err := bson.Unmarshal(rawDoc.Data, &doc); err != nil {
				t.Fatalf("error unmarshaling document: %v", err)
			}

			docs = append(docs, doc)
		}

		// run getMore to get the next batch if the cursor ID is still valid
		if cursorID == 0 {
			break
		}

		getMoreCmd := bson.D{
			{"getMore", cursorID},
			{"collection", collName},
			{"batchSize", batchSize},
		}
		getMoreResponse, _ := runOperationWithDeployment(t, getMoreCmd, driver.SingleServerDeployment{server}, driver.LegacyGetMore)
		cursorDoc = getMoreResponse.Lookup("cursor").Document()
		cursorID = cursorDoc.Lookup("id").Int64()
		batch = cursorDoc.Lookup("nextBatch").Array()
	}

	return docs
}

func TestOperationLegacy(t *testing.T) {

	// Tests run against a mock server to verify wire message correctness.
	t.Run("VerifyWiremessage", func(t *testing.T) {
		res := bson.D{{"ok", 1}}
		resBytes, err := bson.Marshal(res)
		if err != nil {
			t.Fatalf("error marshalling response: %v", err)
		}
		fakeOpReply := drivertest.MakeReply(resBytes)

		maxDoc := bson.D{{"indexBounds", bson.D{{"x", 50}}}}
		minDoc := bson.D{{"indexBounds", bson.D{{"x", 50}}}}
		projection := bson.D{{"y", 0}}
		sort := bson.D{{"x", 1}}
		filter := bson.D{{"x", 1}}

		// commands with all options
		// commands with all options
		findCmd := bson.D{
			{"find", "foo"},
			{"filter", filter},
			{"allowPartialResults", true},
			{"batchSize", 2},
			{"comment", "hello"},
			{"tailable", true},
			{"hint", "hintFoo"},
			{"limit", int64(5)},
			{"max", maxDoc},
			{"maxTimeMS", int64(10000)},
			{"min", minDoc},
			{"noCursorTimeout", true},
			{"oplogReplay", true},
			{"projection", projection},
			{"returnKey", false},
			{"showRecordId", false},
			{"skip", int64(1)},
			{"snapshot", false},
			{"sort", sort},
		}
		listCollCmd := bson.D{
			{"listCollections", 1},
			{"filter", bson.D{{"name", "foo"}}},
		}
		listIndexesCmd := bson.D{
			{"listIndexes", "foo"},
			{"batchSize", 2},
			{"maxTimeMS", int64(10000)},
		}

		// find expectations
		findQueryDoc := bson.D{
			{"$query", filter},
			{"$comment", "hello"},
			{"$hint", "hintFoo"},
			{"$max", maxDoc},
			{"$maxTimeMS", int64(10000)},
			{"$min", minDoc},
			{"$returnKey", false},
			{"$showDiskLoc", false},
			{"$snapshot", false},
			{"$orderby", sort},
		}
		findQuery := opQuery{
			flags:                wiremessage.QueryFlag(wiremessage.Partial | wiremessage.TailableCursor | wiremessage.NoCursorTimeout | wiremessage.OplogReplay | wiremessage.SlaveOK),
			fullCollectionName:   fullCollName("foo"),
			numToSkip:            1,
			numToReturn:          2,
			query:                findQueryDoc,
			returnFieldsSelector: projection,
		}

		// list collections expectations
		regexDoc := bson.D{{"name", primitive.Regex{Pattern: "^[^$]*$"}}}
		modifiedFilterDoc := bson.D{{"name", fullCollName("foo")}}
		listCollDoc := bson.D{
			{"$and", bson.A{regexDoc, modifiedFilterDoc}},
		}
		listCollQuery := opQuery{
			flags:              wiremessage.SlaveOK,
			fullCollectionName: fullCollName("system.namespaces"),
			query:              listCollDoc,
		}

		// list indexes expectations
		listIndexesDoc := bson.D{
			{"$query", bson.D{{"ns", fullCollName("foo")}}},
			{"$maxTimeMS", int64(10000)},
		}
		listIndexesQuery := opQuery{
			flags:              wiremessage.SlaveOK,
			fullCollectionName: fullCollName("system.indexes"),
			numToReturn:        2,
			query:              listIndexesDoc,
		}

		// mock connection
		testConn := &drivertest.ChannelConn{
			Written:  make(chan []byte, 1),
			ReadResp: make(chan []byte, 1),
			Desc: description.Server{
				WireVersion: &description.VersionRange{
					Max: 2,
				},
			},
		}
		defer func() {
			close(testConn.Written)
			close(testConn.ReadResp)
		}()

		// test cases for commands that will generate an OP_QUERY
		cases := []struct {
			name          string
			cmd           bson.D
			expectedQuery opQuery
			legacy        driver.LegacyOperationKind
		}{
			{"find", findCmd, findQuery, driver.LegacyFind},
			{"listCollections", listCollCmd, listCollQuery, driver.LegacyListCollections},
			{"listIndexes", listIndexesCmd, listIndexesQuery, driver.LegacyListIndexes},
		}
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				testConn.ReadResp <- fakeOpReply

				_, _ = runOperationWithDeployment(t, tc.cmd, driver.SingleConnectionDeployment{C: testConn}, tc.legacy)
				if len(testConn.Written) == 0 {
					t.Fatalf("no message written to connection")
				}

				validateQueryWiremessage(t, <-testConn.Written, tc.expectedQuery)
			})
		}

		// test for OP_KILL_CURSORS
		t.Run("killCursors", func(t *testing.T) {
			// no response expected so skip filling testConn.ReadResp
			cursors := []interface{}{int64(1), int64(2)}
			cmd := bson.D{
				{"killCursors", "foo"},
				{"cursors", bson.A(cursors)},
			}

			_, _ = runOperationWithDeployment(t, cmd, driver.SingleConnectionDeployment{C: testConn}, driver.LegacyKillCursors)
			if len(testConn.Written) == 0 {
				t.Fatalf("no message written to connection")
			}

			wm := <-testConn.Written

			var zero, numCursorIDs int32
			var cursorIDs []int64
			var ok bool

			wm = validateHeader(t, wm, wiremessage.OpKillCursors)
			zero, wm, ok = wiremessage.ReadKillCursorsZero(wm)
			if !ok {
				t.Fatalf("could not read zero field")
			}
			if zero != 0 {
				t.Fatalf("zero mismatch; expected 0, got %d", zero)
			}
			numCursorIDs, wm, ok = wiremessage.ReadKillCursorsNumberIDs(wm)
			if !ok {
				t.Fatalf("could not read numberOfCursorIDs field")
			}
			if numCursorIDs != int32(len(cursors)) {
				t.Fatalf("numberOfCursorIDs mismatch; expected %d, got %d", len(cursors), numCursorIDs)
			}
			cursorIDs, wm, ok = wiremessage.ReadKillCursorsCursorIDs(wm, numCursorIDs)
			if !ok {
				t.Fatalf("could not read cursorIDs field")
			}
			for i, got := range cursorIDs {
				expected := cursors[i].(int64)
				if expected != got {
					t.Fatalf("cursor ID mismatch; expected %d, got %d", expected, got)
				}
			}
		})
	})

	t.Run("VerifyResults", func(t *testing.T) {
		db := createTestDatabase(t, &legacyTestDb)
		versionStr, err := getServerVersion(db)
		if err != nil {
			t.Fatalf("error getting server version: %v", err)
		}
		initLegacyDb(t, db)

		t.Run("find", func(t *testing.T) {
			if compareVersions(t, versionStr, "3.0") > 0 {
				t.Skip("skipping for server version > 3.0")
			}

			cmd := bson.D{
				{"find", "foo"},
				{"filter", bson.D{{"x", bson.D{{"$gte", 0}}}}},
				{"batchSize", 2}, // force multiple batches
				{"projection", bson.D{{"_id", 0}}},
				{"sort", bson.D{{"x", 1}}},
			}
			var expected []bson.Raw
			for i := 0; i < 5; i++ {
				doc := bson.D{{"x", int32(i)}, {"y", int32(i)}}
				docBytes, _ := bson.Marshal(doc)
				expected = append(expected, docBytes)
			}

			res, srvr := runOperationWithDeployment(t, cmd, db.client.topology, driver.LegacyFind)
			docs := parseAndIterateCursor(t, res, srvr, 2)
			if len(docs) != len(expected) {
				t.Fatalf("documents length match; expected %d, got %d", len(expected), len(docs))
			}
			for i, doc := range docs {
				if !cmp.Equal(doc, expected[i]) {
					t.Fatalf("document mismatch; expected %v, got %v", expected[i], doc)
				}
			}
		})
		t.Run("listCollections", func(t *testing.T) {
			if compareVersions(t, versionStr, "2.7.6") >= 0 {
				t.Skip("skipping for server version >= 2.7.6")
			}

			cmd := bson.D{{"listCollections", 1}}
			res, srvr := runOperationWithDeployment(t, cmd, db.client.topology, driver.LegacyListCollections)
			docs := parseAndIterateCursor(t, res, srvr, 2)
			if len(docs) != 3 {
				t.Fatalf("documents length mismatch; expected 3, got %d", len(docs))
			}

			for _, doc := range docs {
				collName := doc.Lookup("name").StringValue()
				if collName != fullCollName("foo") && collName != fullCollName("bar") && collName != fullCollName("system.indexes") {
					t.Fatalf("unexpected collection %s", collName)
				}
			}
		})
		t.Run("listIndexes", func(t *testing.T) {
			if compareVersions(t, versionStr, "2.7.6") >= 0 {
				t.Skip("skipping for server version >= 2.7.6")
			}

			// should get indexes x_1 and y_1
			cmd := bson.D{
				{"listIndexes", "foo"},
			}
			res, srvr := runOperationWithDeployment(t, cmd, db.client.topology, driver.LegacyListIndexes)
			docs := parseAndIterateCursor(t, res, srvr, 2)
			if len(docs) != 3 {
				t.Fatalf("documents length mismatch; expected 3, got %d", len(docs))
			}

			expectedNs := fullCollName("foo")
			for _, doc := range docs {
				ns := doc.Lookup("ns").StringValue()
				if ns != expectedNs {
					t.Fatalf("ns mismatch; expected %s, got %s", expectedNs, ns)
				}

				indexName := doc.Lookup("name").StringValue()
				if indexName != "x_1" && indexName != "y_1" && indexName != "_id_" {
					t.Fatalf("unexpected index %s", indexName)
				}
			}
		})
		t.Run("killCursors", func(t *testing.T) {
			if compareVersions(t, versionStr, "3.0") > 0 {
				t.Skip("skipping for server version > 3.0")
			}

			cmd := bson.D{
				{"killCursors", "foo"},
				{"cursors", bson.A{int64(1), int64(2)}},
			}
			res, _ := runOperationWithDeployment(t, cmd, db.client.topology, driver.LegacyKillCursors)
			if len(res) != 0 {
				t.Fatalf("got non-empty response %v", res)
			}
		})
	})
}
