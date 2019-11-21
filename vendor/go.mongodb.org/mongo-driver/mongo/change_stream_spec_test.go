// Copyright (C) MongoDB, Inc. 2017-present.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may
// not use this file except in compliance with the License. You may obtain
// a copy of the License at http://www.apache.org/licenses/LICENSE-2.0

package mongo

import (
	"encoding/json"
	"io/ioutil"
	"os"
	"testing"

	"path"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/internal/testutil/helpers"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.mongodb.org/mongo-driver/x/bsonx"
)

const csTestsDir = "../data/change-streams"

var topMap = map[string]string{
	"replica_set":     "replicaset",
	"sharded_cluster": "sharded",
	"server":          "single",
}

type csTestFile struct {
	CollectionName  string `json:"collection_name"`
	DatabaseName    string `json:"database_name"`
	CollectionName2 string `json:"collection2_name"`
	DatabaseName2   string `json:"database2_name"`
	Tests           []csTest
}

type csTest struct {
	Description      string                       `json:"description"`
	MinServerVersion string                       `json:"minServerVersion"`
	Target           string                       `json:"target"`
	Topology         []string                     `json:"topology"`
	Pipeline         []interface{}                `json:"changeStreamPipeline"`
	Options          map[string]interface{}       `json:"changeStreamOptions"`
	Operations       []csOperation                `json:"operations"`
	Expectations     []map[string]json.RawMessage `json:"expectations"`
	Result           csResult                     `json:"result"`
}

type csOperation struct {
	Database   string `json:"database"`
	Collection string `json:"collection"`
	Name       string `json:"name"`
	Arguments  map[string]interface{}
}

type csResult struct {
	Success []map[string]interface{} `json:"success"`
	Error   map[string]interface{}   `json:"error"`
}

func TestChangeStreamSpec(t *testing.T) {
	skipIfBelow36(t)
	globalClient := createTestClient(t)

	for _, file := range testhelpers.FindJSONFilesInDir(t, csTestsDir) {
		runCsTestFile(t, globalClient, path.Join(csTestsDir, file))
	}
}

func closeCursor(t *testing.T, stream *ChangeStream) {
	err := stream.Close(ctx)
	testhelpers.RequireNil(t, err, "error closing ChangeStream: %v", err)
}

func getStreamOptions(t *testing.T, test *csTest) *options.ChangeStreamOptions {
	opts := options.ChangeStream()
	for name, opt := range test.Options {
		switch name {
		case "batchSize":
			opts = opts.SetBatchSize(int32(opt.(float64)))
		default:
			t.Fatalf("unknown changeStream option: %s", name)
		}
	}

	// no options
	return opts
}

func changeStreamCompareErrors(t *testing.T, expected map[string]interface{}, actual error) {
	if cmdErr, ok := actual.(CommandError); ok {
		expectedCode := int32(expected["code"].(float64))
		if cmdErr.Code != expectedCode {
			t.Fatalf("error code mismatch. expected %d, got %d", expectedCode, cmdErr.Code)
		}

		if expected["errorLabels"] != nil {
			expectedLabels := expected["errorLabels"].([]interface{})
			var labelStrings []string
			for _, label := range expectedLabels {
				labelStrings = append(labelStrings, label.(string))
			}

			if len(labelStrings) != len(cmdErr.Labels) {
				t.Fatalf("error label length mismatch")
			}

			for _, exp := range labelStrings {
				var found bool
				for _, label := range cmdErr.Labels {
					if label == exp {
						found = true
						break
					}
				}
				if !found {
					t.Fatalf("error label %s not found", exp)
				}
			}
		}
	} else {
		t.Fatalf("error type mismatch; expected CommandError, got %T", actual)
	}
}

func compareCommands(t *testing.T, expectedraw, actualraw bson.Raw) {
	expected, err := bsonx.ReadDoc(expectedraw)
	if err != nil {
		t.Fatalf("could not parse document: %v", err)
	}
	actual, err := bsonx.ReadDoc(actualraw)
	if err != nil {
		t.Fatalf("could not parse document: %v", err)
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

func compareCsStartedEvent(t *testing.T, expected json.RawMessage) {
	if len(startedChan) == 0 {
		t.Fatalf("no started event waiting")
	}
	actual := <-startedChan

	expectedBytes, err := expected.MarshalJSON()
	testhelpers.RequireNil(t, err, "error marshalling json: %s", err)

	var expectedDoc bsonx.Doc
	err = bson.UnmarshalExtJSON(expectedBytes, true, &expectedDoc)
	testhelpers.RequireNil(t, err, "error converting command to BSON: %s", err)

	expectedCmdName := expectedDoc.Lookup("command_name").StringValue()
	if actual.CommandName != expectedCmdName {
		t.Fatalf("cmd name mismatch. expected %s got %s", expectedCmdName, actual.CommandName)
	}

	expectedDbName := expectedDoc.Lookup("database_name").StringValue()
	if actual.DatabaseName != expectedDbName {
		t.Fatalf("db name mismatch. expected %s got %s", expectedDbName, actual.DatabaseName)
	}

	expectedCmd, _ := expectedDoc.Lookup("command").Document().MarshalBSON()
	compareCommands(t, expectedCmd, actual.Command)
}

func compareCsExepectations(t *testing.T, test *csTest) {
	for _, expected := range test.Expectations {
		if event, ok := expected["command_started_event"]; ok {
			compareCsStartedEvent(t, event)
		} else {
			t.Fatalf("did not find started event for %s", t.Name())
		}
	}
}

func runCsTestFile(t *testing.T, globalClient *Client, path string) {
	content, err := ioutil.ReadFile(path)
	testhelpers.RequireNil(t, err, "error reading JSON file: %s", err)

	var testfile csTestFile
	err = json.Unmarshal(content, &testfile)
	testhelpers.RequireNil(t, err, "error creating structs: %s", err)

	for _, test := range testfile.Tests {
		t.Run(test.Description, func(t *testing.T) {
			currTop := topMap[os.Getenv("TOPOLOGY")]
			var foundTop bool
			for _, top := range test.Topology {
				if top == currTop {
					foundTop = true
					break
				}
			}

			if !foundTop {
				t.Skip("skipping topology")
			}

			db := globalClient.Database(testfile.DatabaseName)
			db2 := globalClient.Database(testfile.DatabaseName2)

			coll := db.Collection(testfile.CollectionName)
			coll2 := db2.Collection(testfile.CollectionName2)

			err = db.Drop(ctx)
			testhelpers.RequireNil(t, err, "error dropping db: %s", err)
			err = db2.Drop(ctx)
			testhelpers.RequireNil(t, err, "error dropping db2: %s", err)

			serverVersion, err := getServerVersion(db)
			testhelpers.RequireNil(t, err, "error getting server version: %s", err)

			if res := compareVersions(t, serverVersion, test.MinServerVersion); res < 0 {
				t.Skip("skipping server version")
			}

			client := createMonitoredClient(t, monitor)
			clientDb := client.Database(testfile.DatabaseName)
			err = clientDb.Drop(ctx)
			testhelpers.RequireNil(t, err, "err dropping client db: %s", err)
			clientColl := clientDb.Collection(testfile.CollectionName, options.Collection().SetWriteConcern(wcMajority))

			_, err = clientColl.InsertOne(ctx, doc1)
			testhelpers.RequireNil(t, err, "error inserting into client coll: %s", err)

			drainChannels()
			opts := getStreamOptions(t, &test)
			var cursor *ChangeStream
			switch test.Target {
			case "collection":
				cursor, err = clientColl.Watch(ctx, test.Pipeline, opts)
			case "database":
				cursor, err = clientDb.Watch(ctx, test.Pipeline, opts)
			case "client":
				cursor, err = client.Watch(ctx, test.Pipeline, opts)
			default:
				t.Fatalf("unknown target: %s", test.Target)
			}

			if err != nil {
				changeStreamCompareErrors(t, test.Result.Error, err)
				return
			}

			defer closeCursor(t, cursor) // end implicit session

			// run operations
			for _, op := range test.Operations {
				var opColl *Collection
				if op.Collection == coll.name {
					opColl = coll
				} else {
					opColl = coll2
				}

				var opErr error
				switch op.Name {
				case "insertOne":
					_, opErr = executeInsertOne(nil, opColl, op.Arguments)
				case "updateOne":
					_, opErr = executeUpdateOne(nil, opColl, op.Arguments)
				case "replaceOne":
					_, opErr = executeReplaceOne(nil, opColl, op.Arguments)
				case "deleteOne":
					_, opErr = executeDeleteOne(nil, opColl, op.Arguments)
				case "rename":
					opErr = executeRenameCollection(nil, opColl, op.Arguments).Err()
				case "drop":
					opErr = opColl.Drop(ctx)
				default:
					t.Fatalf("unknown operation for test %s: %s", t.Name(), op.Name)
				}

				if opErr != nil {
					changeStreamCompareErrors(t, test.Result.Error, opErr)
					return
				}
			}

			if len(test.Result.Success) == 0 && len(test.Result.Error) != 0 {
				if cursor.Next(ctx) {
					t.Fatalf("Next returned true instead of false")
				}
				changeStreamCompareErrors(t, test.Result.Error, cursor.Err())
			}

			for i := 0; i < len(test.Result.Success); i++ {
				if !cursor.Next(ctx) {
					t.Fatalf("Next returned false at iteration %d; expected %d changes", i, len(test.Result.Success))
				}
			}

			if len(test.Expectations) > 0 {
				compareCsExepectations(t, &test)
			}
		})
	}
}
