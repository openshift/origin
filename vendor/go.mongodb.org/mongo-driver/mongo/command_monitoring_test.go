// Copyright (C) MongoDB, Inc. 2017-present.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may
// not use this file except in compliance with the License. You may obtain
// a copy of the License at http://www.apache.org/licenses/LICENSE-2.0

package mongo

import (
	"context"
	"io/ioutil"
	"path"
	"testing"

	"go.mongodb.org/mongo-driver/mongo/options"
	"go.mongodb.org/mongo-driver/x/bsonx"

	"bytes"
	"fmt"
	"time"

	"os"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/event"
	"go.mongodb.org/mongo-driver/internal/testutil"
	testhelpers "go.mongodb.org/mongo-driver/internal/testutil/helpers"
	"go.mongodb.org/mongo-driver/mongo/readpref"
	"go.mongodb.org/mongo-driver/mongo/writeconcern"
	"go.mongodb.org/mongo-driver/x/mongo/driver/session"
)

const cmTestsDir = "../data/command-monitoring"

var startedChan = make(chan *event.CommandStartedEvent, 100)
var succeededChan = make(chan *event.CommandSucceededEvent, 100)
var failedChan = make(chan *event.CommandFailedEvent, 100)
var cursorID int64

var monitor = &event.CommandMonitor{
	Started: func(ctx context.Context, cse *event.CommandStartedEvent) {
		startedChan <- cse
	},
	Succeeded: func(ctx context.Context, cse *event.CommandSucceededEvent) {
		succeededChan <- cse
	},
	Failed: func(ctx context.Context, cfe *event.CommandFailedEvent) {
		failedChan <- cfe
	},
}

func createMonitoredClient(t *testing.T, monitor *event.CommandMonitor) *Client {
	client, err := NewClient()
	testhelpers.RequireNil(t, err, "unable to create client")
	client.topology = testutil.GlobalMonitoredTopology(t, monitor)
	client.connString = testutil.ConnString(t)
	client.readPreference = readpref.Primary()
	client.clock = &session.ClusterClock{}
	client.registry = bson.DefaultRegistry
	client.monitor = monitor
	return client
}

func skipCmTest(t *testing.T, testCase bsonx.Doc, serverVersion string) bool {
	minVersionVal, err := testCase.LookupErr("ignore_if_server_version_less_than")
	if err == nil {
		if compareVersions(t, minVersionVal.StringValue(), serverVersion) > 0 {
			return true
		}
	}

	maxVersionVal, err := testCase.LookupErr("ignore_if_server_version_greater_than")
	if err == nil {
		if compareVersions(t, maxVersionVal.StringValue(), serverVersion) < 0 {
			return true
		}
	}

	return false
}

func drainChannels() {
	for len(startedChan) > 0 {
		<-startedChan
	}

	for len(succeededChan) > 0 {
		<-succeededChan
	}

	for len(failedChan) > 0 {
		<-failedChan
	}
}

func insertDocuments(docsArray bsonx.Arr, coll *Collection, opts ...*options.InsertManyOptions) error {
	docs := make([]interface{}, 0, len(docsArray))
	for _, val := range docsArray {
		docs = append(docs, val.Document())
	}

	_, err := coll.InsertMany(context.Background(), docs, opts...)
	return err
}

func TestCommandMonitoring(t *testing.T) {
	for _, file := range testhelpers.FindJSONFilesInDir(t, cmTestsDir) {
		runCmTestFile(t, path.Join(cmTestsDir, file))
	}
}

func runCmTestFile(t *testing.T, filepath string) {
	drainChannels() // remove any wire messages from prev test file
	content, err := ioutil.ReadFile(filepath)
	testhelpers.RequireNil(t, err, "error reading JSON file: %s", err)

	doc := bsonx.Doc{}
	err = bson.UnmarshalExtJSON(content, true, &doc)
	testhelpers.RequireNil(t, err, "error converting JSON to BSON: %s", err)

	client := createMonitoredClient(t, monitor)
	db := client.Database(doc.Lookup("database_name").StringValue())

	serverVersionStr, err := getServerVersion(db)
	testhelpers.RequireNil(t, err, "error getting server version: %s", err)

	collName := doc.Lookup("collection_name").StringValue()

	for _, val := range doc.Lookup("tests").Array() {
		testDoc := val.Document()

		if skipCmTest(t, testDoc, serverVersionStr) {
			continue
		}

		skippedTopos, err := testDoc.LookupErr("ignore_if_topology_type")
		if err == nil {
			var skipTop bool
			currentTop := os.Getenv("topology")

			for _, val := range skippedTopos.Array() {
				top := val.StringValue()
				// the only use of ignore_if_topology_type in the tests is in find.json and has "sharded" which actually maps
				// to topology type "sharded_cluster". This option isn't documented in the command monitoring testing README
				// so there's no way of knowing if future CM tests will use this option for other topologies.
				if top == "sharded" && currentTop == "sharded_cluster" {
					skipTop = true
					break
				}
			}

			if skipTop {
				continue
			}
		}

		err = db.RunCommand(
			context.Background(),
			bsonx.Doc{{"drop", bsonx.String(collName)}},
		).Err()

		coll := db.Collection(collName)
		err = insertDocuments(doc.Lookup("data").Array(), coll)
		testhelpers.RequireNil(t, err, "error inserting starting data: %s", err)

		operationDoc := testDoc.Lookup("operation").Document()

		drainChannels()

		t.Run(testDoc.Lookup("description").StringValue(), func(t *testing.T) {
			switch operationDoc.Lookup("name").StringValue() {
			case "insertMany":
				cmInsertManyTest(t, testDoc, operationDoc, coll)
			case "find":
				cmFindTest(t, testDoc, operationDoc, coll)
			case "deleteMany":
				cmDeleteManyTest(t, testDoc, operationDoc, coll)
			case "deleteOne":
				cmDeleteOneTest(t, testDoc, operationDoc, coll)
			case "insertOne":
				cmInsertOneTest(t, testDoc, operationDoc, coll)
			case "updateOne":
				cmUpdateOneTest(t, testDoc, operationDoc, coll)
			case "updateMany":
				cmUpdateManyTest(t, testDoc, operationDoc, coll)
			case "count":
				// count has been deprecated
			case "bulkWrite":
				cmBulkWriteTest(t, testDoc, operationDoc, coll)
			}
		})
	}
}

func checkActualHelper(t *testing.T, expected bsonx.Doc, actual bsonx.Doc, nested bool) {
	for _, elem := range actual {
		key := elem.Key

		// TODO: see comments in compareStartedEvent about ordered and batchSize
		if key == "ordered" || key == "batchSize" {
			continue
		}

		val := elem.Value

		if nested {
			expectedVal, err := expected.LookupErr(key)
			testhelpers.RequireNil(t, err, "nested field %s not found in expected cmd", key)

			if nestedDoc, ok := val.DocumentOK(); ok {
				checkActualHelper(t, expectedVal.Document(), nestedDoc, true)
			} else if !compareValues(expectedVal, val) {
				t.Errorf("nested field %s has different value", key)
			}
		}
	}
}

func checkActualFields(t *testing.T, expected bsonx.Doc, actual bsonx.Doc) {
	// check that the command sent has no extra fields in nested subdocuments
	checkActualHelper(t, expected, actual, false)
}

func compareWriteError(t *testing.T, expected bsonx.Doc, actual bsonx.Doc) {
	expectedIndex := expected.Lookup("index").Int32()
	actualIndex := actual.Lookup("index").Int32()

	if expectedIndex != actualIndex {
		t.Errorf("index mismatch in writeError. expected %d got %d", expectedIndex, actualIndex)
		t.FailNow()
	}

	expectedCode := expected.Lookup("code").Int32()
	actualCode := actual.Lookup("code").Int32()
	if expectedCode == 42 {
		if actualCode <= 0 {
			t.Errorf("expected error code > 0 in writeError. got %d", actualCode)
			t.FailNow()
		}
	} else if expectedCode != actualCode {
		t.Errorf("error code mismatch in writeError. expected %d got %d", expectedCode, actualCode)
		t.FailNow()
	}

	expectedErrMsg := expected.Lookup("errmsg").StringValue()
	actualErrMsg := actual.Lookup("errmsg").StringValue()
	if expectedErrMsg == "" {
		if len(actualErrMsg) == 0 {
			t.Errorf("expected non-empty error msg in writeError")
			t.FailNow()
		}
	} else if expectedErrMsg != actualErrMsg {
		t.Errorf("error message mismatch in writeError. expected %s got %s", expectedErrMsg, actualErrMsg)
		t.FailNow()
	}
}

func compareWriteErrors(t *testing.T, expected bsonx.Arr, actual bsonx.Arr) {
	if len(expected) != len(actual) {
		t.Errorf("writeErrors length mismatch. expected %d got %d", len(expected), len(actual))
		t.FailNow()
	}

	for idx := range expected {
		expectedErr := expected[idx].Document()
		actualErr := actual[idx].Document()
		compareWriteError(t, expectedErr, actualErr)
	}
}

func compareBatches(t *testing.T, expected bsonx.Arr, actual bsonx.Arr) {
	if len(expected) != len(actual) {
		t.Errorf("batch length mismatch. expected %d got %d", len(expected), len(actual))
		t.FailNow()
	}

	for idx := range expected {
		expectedDoc := expected[idx].Document()
		actualDoc := actual[idx].Document()

		if !expectedDoc.Equal(actualDoc) {
			t.Errorf("document mismatch in cursor batch")
			t.FailNow()
		}
	}
}

func compareCursors(t *testing.T, expected bsonx.Doc, actual bsonx.Doc) {
	expectedID := expected.Lookup("id").Int64()
	actualID := actual.Lookup("id").Int64()

	// this means we're getting a new cursor ID from the server in response to a query
	// future cursor IDs in getMore commands for this test case should match this ID
	if expectedID == 42 {
		cursorID = actualID
	} else {
		if expectedID != actualID {
			t.Errorf("cursor ID mismatch. expected %d got %d", expectedID, actualID)
			t.FailNow()
		}
	}

	expectedNS := expected.Lookup("ns").StringValue()
	actualNS := actual.Lookup("ns").StringValue()
	if expectedNS != actualNS {
		t.Errorf("cursor NS mismatch. expected %s got %s", expectedNS, actualNS)
		t.FailNow()
	}

	batchID := "firstBatch"
	batchVal, err := expected.LookupErr(batchID)
	if err != nil {
		batchID = "nextBatch"
		batchVal = expected.Lookup(batchID)
	}

	actualBatchVal, err := actual.LookupErr(batchID)
	testhelpers.RequireNil(t, err, "could not find batch with ID %s in actual cursor", batchID)

	expectedBatch := batchVal.Array()
	actualBatch := actualBatchVal.Array()
	compareBatches(t, expectedBatch, actualBatch)
}

func compareUpserted(t *testing.T, expected bsonx.Arr, actual bsonx.Arr) {
	if len(expected) != len(actual) {
		t.Errorf("length mismatch. expected %d got %d", len(expected), len(actual))
	}

	for idx := range expected {
		compareDocs(t, expected[idx].Document(), actual[idx].Document())
	}
}

func compareReply(t *testing.T, succeeded *event.CommandSucceededEvent, reply bsonx.Doc) {
	eventReply, err := bsonx.ReadDoc(succeeded.Reply)
	if err != nil {
		t.Fatalf("could not read reply doc: %v", err)
	}
	for _, elem := range reply {
		switch elem.Key {
		case "ok":
			var actualOk int32

			actualOkVal, err := succeeded.Reply.LookupErr("ok")
			testhelpers.RequireNil(t, err, "could not find key ok in reply")

			switch actualOkVal.Type {
			case bson.TypeInt32:
				actualOk = actualOkVal.Int32()
			case bson.TypeInt64:
				actualOk = int32(actualOkVal.Int64())
			case bson.TypeDouble:
				actualOk = int32(actualOkVal.Double())
			}

			if actualOk != elem.Value.Int32() {
				t.Errorf("ok value in reply does not match. expected %d got %d", elem.Value.Int32(), actualOk)
				t.FailNow()
			}
		case "n":
			actualNVal, err := eventReply.LookupErr("n")
			testhelpers.RequireNil(t, err, "could not find key n in reply")

			if !compareValues(elem.Value, actualNVal) {
				t.Errorf("n values do not match")
			}
		case "writeErrors":
			actualArr, err := eventReply.LookupErr("writeErrors")
			testhelpers.RequireNil(t, err, "could not find key writeErrors in reply")

			compareWriteErrors(t, elem.Value.Array(), actualArr.Array())
		case "cursor":
			actualDoc, err := eventReply.LookupErr("cursor")
			testhelpers.RequireNil(t, err, "could not find key cursor in reply")

			compareCursors(t, elem.Value.Document(), actualDoc.Document())
		case "upserted":
			actualDoc, err := eventReply.LookupErr("upserted")
			testhelpers.RequireNil(t, err, "could not find key upserted in reply")

			compareUpserted(t, elem.Value.Array(), actualDoc.Array())
		default:
			fmt.Printf("key %s does not match existing case\n", elem.Key)
		}
	}
}

func getInt64(val bsonx.Val) int64 {
	switch val.Type() {
	case bson.TypeInt32:
		return int64(val.Int32())
	case bson.TypeInt64:
		return val.Int64()
	case bson.TypeDouble:
		return int64(val.Double())
	}

	return 0
}

func compareRawValues(expected, actual bson.RawValue) bool {
	var e bsonx.Val
	var a bsonx.Val
	if err := e.UnmarshalBSONValue(expected.Type, expected.Value); err != nil {
		return false
	}
	if err := a.UnmarshalBSONValue(actual.Type, actual.Value); err != nil {
		return false
	}
	return compareValues(e, a)
}

func compareValues(expected bsonx.Val, actual bsonx.Val) bool {
	if expected.IsNumber() {
		if !actual.IsNumber() {
			return false
		}

		return getInt64(expected) == getInt64(actual)
	}

	switch expected.Type() {
	case bson.TypeString:
		if aStr, ok := actual.StringValueOK(); !(ok && aStr == expected.StringValue()) {
			return false
		}
	case bson.TypeBinary:
		aSub, aBytes := actual.Binary()
		eSub, eBytes := expected.Binary()

		if (aSub != eSub) || (!bytes.Equal(aBytes, eBytes)) {
			return false
		}
	}

	return true
}

func compareDocs(t *testing.T, expected bsonx.Doc, actual bsonx.Doc) {
	// this is necessary even though Equal() exists for documents because types not match between commands and the BSON
	// documents given in test cases. for example, all numbers in the test case JSON are parsed as int64, but many nubmers
	// sent over the wire are type int32
	if len(expected) != len(actual) {
		t.Errorf("doc length mismatch. expected %d got %d", len(expected), len(actual))
		t.FailNow()
	}

	for _, expectedElem := range expected {

		aVal, err := actual.LookupErr(expectedElem.Key)
		testhelpers.RequireNil(t, err, "docs not equal. key %s not found in actual", expectedElem.Key)

		eVal := expectedElem.Value

		if doc, ok := eVal.DocumentOK(); ok {
			// nested doc
			compareDocs(t, doc, aVal.Document())

			// nested docs were equal
			continue
		}

		if !compareValues(eVal, aVal) {
			t.Errorf("docs not equal because value mismatch for key %s", expectedElem.Key)
		}
	}
}

func compareStartedEvent(t *testing.T, expected bsonx.Doc) {
	// rules for command comparison (from spec):
	// 1. the actual command can have extra fields not specified in the test case, but only as top level fields
	// these fields cannot be in nested subdocuments
	// 2. the actual command must have all fields specified in the test case

	// this function only checks that everything in the test case cmd is also in the monitored cmd
	// checkActualFields() checks that the actual cmd has no extra fields in nested subdocuments
	if len(startedChan) == 0 {
		t.Errorf("no started event waiting")
		t.FailNow()
	}

	started := <-startedChan

	if started.CommandName == "isMaster" {
		return
	}

	expectedCmdName := expected.Lookup("command_name").StringValue()
	if expectedCmdName != started.CommandName {
		t.Errorf("command name mismatch. expected %s, got %s", expectedCmdName, started.CommandName)
		t.FailNow()
	}
	expectedDbName := expected.Lookup("database_name").StringValue()
	if expectedDbName != started.DatabaseName {
		t.Errorf("database name mismatch. expected %s, got %s", expectedDbName, started.DatabaseName)
		t.FailNow()
	}

	expectedCmd := expected.Lookup("command").Document()
	var identifier string
	var expectedPayload bsonx.Arr

	for _, elem := range expectedCmd {
		key := elem.Key
		val := elem.Value

		if key == "getMore" && expectedCmdName == "getMore" {
			expectedCursorID := val.Int64()
			actualCursorID := started.Command.Lookup("getMore").Int64()

			if expectedCursorID == 42 {
				if actualCursorID != cursorID {
					t.Errorf("cursor ID mismatch in getMore. expected %d got %d", cursorID, actualCursorID)
					t.FailNow()
				}
			} else if expectedCursorID != actualCursorID {
				t.Errorf("cursor ID mismatch in getMore. expected %d got %d", expectedCursorID, actualCursorID)
				t.FailNow()
			}
			continue
		}

		// type 1 payload
		if key == "documents" || key == "updates" || key == "deletes" {
			expectedPayload = val.Array()
			identifier = key
			continue
		}

		// TODO: some tests specify that "ordered" must be a key in the event but ordered isn't a valid option for some of these cases (e.g. insertOne)
		if key == "ordered" {
			continue
		}

		actualRawVal, err := started.Command.LookupErr(key)
		testhelpers.RequireNil(t, err, "key %s not found in started event %s", key, started.CommandName)

		// TODO: tests in find.json expect different batch sizes for find/getMore commands based on an optional limit
		// e.g. if limit = 4 and find is run with batchSize = 3, the getMore should have batchSize = 1
		// skipping because the driver doesn't have this logic
		if key == "batchSize" {
			continue
		}

		var actualVal bsonx.Val
		err = actualVal.UnmarshalBSONValue(actualRawVal.Type, actualRawVal.Value)
		if err != nil {
			t.Errorf("could not unmarshal actual value: %v", err)
		}
		if !compareValues(val, actualVal) {
			t.Errorf("values for key %s do not match. cmd: %s", key, started.CommandName)
			t.FailNow()
		}
	}

	if expectedPayload == nil {
		// no type 1 payload
		return
	}

	actualArrayVal, err := started.Command.LookupErr(identifier)
	testhelpers.RequireNil(t, err, "array with id %s not found in started command", identifier)
	var actualPayload bsonx.Arr
	err = actualPayload.UnmarshalBSONValue(actualArrayVal.Type, actualArrayVal.Value)

	if len(expectedPayload) != len(actualPayload) {
		t.Errorf("payload length mismatch. expected %d, got %d", len(expectedPayload), len(actualPayload))
		t.FailNow()
	}

	for idx := range expectedPayload {
		expectedDoc := expectedPayload[idx].Document()
		actualDoc := actualPayload[idx].Document()

		compareDocs(t, expectedDoc, actualDoc)
	}

	startedCommand, err := bsonx.ReadDoc(started.Command)
	if err != nil {
		t.Fatalf("cannot read command: %v", err)
	}
	checkActualFields(t, expectedCmd, startedCommand)
}

func compareSuccessEvent(t *testing.T, expected bsonx.Doc) {
	if len(succeededChan) == 0 {
		t.Errorf("no success event waiting")
		t.FailNow()
	}

	succeeded := <-succeededChan

	if succeeded.CommandName == "isMaster" {
		return
	}

	cmdName := expected.Lookup("command_name").StringValue()
	if cmdName != succeeded.CommandName {
		t.Errorf("command name mismatch. expected %s got %s", cmdName, succeeded.CommandName)
		t.FailNow()
	}

	compareReply(t, succeeded, expected.Lookup("reply").Document())
}

func compareFailureEvent(t *testing.T, expected bsonx.Doc) {
	if len(failedChan) == 0 {
		t.Errorf("no failure event waiting")
		t.FailNow()
	}

	failed := <-failedChan
	expectedName := expected.Lookup("command_name").StringValue()

	if expectedName != failed.CommandName {
		t.Errorf("command name mismatch for failed event. expected %s got %s", expectedName, failed.CommandName)
		t.FailNow()
	}
}

func compareExpectations(t *testing.T, testCase bsonx.Doc) {
	expectations := testCase.Lookup("expectations").Array()

	for _, val := range expectations {
		expectedDoc := val.Document()

		if startDoc, err := expectedDoc.LookupErr("command_started_event"); err == nil {
			compareStartedEvent(t, startDoc.Document())
			continue
		}

		if successDoc, err := expectedDoc.LookupErr("command_succeeded_event"); err == nil {
			compareSuccessEvent(t, successDoc.Document())
			continue
		}

		compareFailureEvent(t, expectedDoc.Lookup("command_failed_event").Document())
	}
}

func cmFindOptions(arguments bsonx.Doc) *options.FindOptions {
	opts := options.Find()

	if sort, err := arguments.LookupErr("sort"); err == nil {
		opts = opts.SetSort(sort.Document())
	}

	if skip, err := arguments.LookupErr("skip"); err == nil {
		opts = opts.SetSkip(skip.Int64())
	}

	if limit, err := arguments.LookupErr("limit"); err == nil {
		opts = opts.SetLimit(limit.Int64())
	}

	if batchSize, err := arguments.LookupErr("batchSize"); err == nil {
		opts = opts.SetBatchSize(int32(batchSize.Int64()))
	}

	if collation, err := arguments.LookupErr("collation"); err == nil {
		collMap := collation.Interface().(map[string]interface{})
		opts = opts.SetCollation(collationFromMap(collMap))
	}

	modifiersVal, err := arguments.LookupErr("modifiers")
	if err != nil {
		return opts
	}

	for _, elem := range modifiersVal.Document() {
		val := elem.Value

		switch elem.Key {
		case "$comment":
			opts = opts.SetComment(val.StringValue())
		case "$hint":
			opts = opts.SetHint(val.Document())
		case "$max":
			opts = opts.SetMax(val.Document())
		case "$maxTimeMS":
			ns := time.Duration(val.Int32()) * time.Millisecond
			opts = opts.SetMaxTime(ns)
		case "$min":
			opts = opts.SetMin(val.Document())
		case "$returnKey":
			opts = opts.SetReturnKey(val.Boolean())
		case "$showDiskLoc":
			opts = opts.SetShowRecordID(val.Boolean())
		}
	}

	return opts
}

func getRp(rpDoc bsonx.Doc) *readpref.ReadPref {
	rpMode := rpDoc.Lookup("mode").StringValue()
	switch rpMode {
	case "primary":
		return readpref.Primary()
	case "secondary":
		return readpref.Secondary()
	case "primaryPreferred":
		return readpref.PrimaryPreferred()
	case "secondaryPreferred":
		return readpref.SecondaryPreferred()
	}

	return nil
}

func cmInsertManyTest(t *testing.T, testCase bsonx.Doc, operation bsonx.Doc, coll *Collection) {
	t.Logf("RUNNING %s\n", testCase.Lookup("description").StringValue())
	args := operation.Lookup("arguments").Document()
	opts := options.InsertMany()
	var orderedGiven bool

	if optionsDoc, err := args.LookupErr("options"); err == nil {
		if ordered, err := optionsDoc.Document().LookupErr("ordered"); err == nil {
			orderedGiven = true
			opts = opts.SetOrdered(ordered.Boolean())
		}
	}

	// TODO: ordered?
	if !orderedGiven {
		opts = opts.SetOrdered(true)
	}
	// ignore errors because write errors constitute a successful command
	_ = insertDocuments(args.Lookup("documents").Array(), coll, opts)
	compareExpectations(t, testCase)
}

func cmFindTest(t *testing.T, testCase bsonx.Doc, operation bsonx.Doc, coll *Collection) {
	arguments := operation.Lookup("arguments").Document()
	filter := arguments.Lookup("filter").Document()
	opts := cmFindOptions(arguments)

	oldRp := coll.readPreference
	if rpVal, err := operation.LookupErr("read_preference"); err == nil {
		coll.readPreference = getRp(rpVal.Document())
	}

	cursor, _ := coll.Find(context.Background(), filter, opts) // ignore errors at this stage

	if cursor != nil {
		for cursor.Next(context.Background()) {
		}
	}

	coll.readPreference = oldRp
	compareExpectations(t, testCase)
}

func cmDeleteManyTest(t *testing.T, testCase bsonx.Doc, operation bsonx.Doc, coll *Collection) {
	args := operation.Lookup("arguments").Document()
	filter := args.Lookup("filter").Document()

	_, _ = coll.DeleteMany(context.Background(), filter)
	compareExpectations(t, testCase)
}

func cmDeleteOneTest(t *testing.T, testCase bsonx.Doc, operation bsonx.Doc, coll *Collection) {
	args := operation.Lookup("arguments").Document()
	filter := args.Lookup("filter").Document()

	_, _ = coll.DeleteOne(context.Background(), filter)
	compareExpectations(t, testCase)
}

func cmInsertOneTest(t *testing.T, testCase bsonx.Doc, operation bsonx.Doc, coll *Collection) {
	// ignore errors because write errors constitute a successful command
	_, _ = coll.InsertOne(context.Background(),
		operation.Lookup("arguments").Document().Lookup("document").Document())
	compareExpectations(t, testCase)
}

func getUpdateParams(args bsonx.Doc) (bsonx.Doc, bsonx.Doc, []*options.UpdateOptions) {
	filter := args.Lookup("filter").Document()
	update := args.Lookup("update").Document()

	opts := []*options.UpdateOptions{options.Update()}
	if upsert, err := args.LookupErr("upsert"); err == nil {
		opts = append(opts, options.Update().SetUpsert(upsert.Boolean()))
	}

	return filter, update, opts
}

func runUpdateOne(args bsonx.Doc, coll *Collection, updateOpts ...*options.UpdateOptions) {
	filter, update, opts := getUpdateParams(args)
	opts = append(opts, updateOpts...)
	_, _ = coll.UpdateOne(context.Background(), filter, update, opts...)
}

func cmUpdateOneTest(t *testing.T, testCase bsonx.Doc, operation bsonx.Doc, coll *Collection) {
	args := operation.Lookup("arguments").Document()
	runUpdateOne(args, coll)
	compareExpectations(t, testCase)
}

func cmUpdateManyTest(t *testing.T, testCase bsonx.Doc, operation bsonx.Doc, coll *Collection) {
	args := operation.Lookup("arguments").Document()
	filter, update, opts := getUpdateParams(args)
	_, _ = coll.UpdateMany(context.Background(), filter, update, opts...)
	compareExpectations(t, testCase)
}

func cmBulkWriteTest(t *testing.T, testCase bsonx.Doc, operation bsonx.Doc, coll *Collection) {
	outerArguments := operation.Lookup("arguments").Document()

	var wc *writeconcern.WriteConcern
	if collOpts, err := operation.LookupErr("collectionOptions"); err == nil {
		wcDoc := collOpts.Document().Lookup("writeConcern").Document()
		wc = writeconcern.New(writeconcern.W(int(wcDoc.Lookup("w").Int32())))
	}

	oldWc := coll.writeConcern
	if wc != nil {
		coll.writeConcern = wc
	}

	for _, val := range outerArguments.Lookup("requests").Array() {
		requestDoc := val.Document()
		args := requestDoc.Lookup("arguments").Document()

		switch requestDoc.Lookup("name").StringValue() {
		case "insertOne":
			_, _ = coll.InsertOne(context.Background(),
				args.Lookup("document").Document())
		case "updateOne":
			runUpdateOne(args, coll)
		}
	}

	coll.writeConcern = oldWc

	if !writeconcern.AckWrite(wc) {
		time.Sleep(time.Second) // sleep to allow event to be written to channel before checking expectations
	}
	compareExpectations(t, testCase)
}
