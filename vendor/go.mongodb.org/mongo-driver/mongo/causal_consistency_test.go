// Copyright (C) MongoDB, Inc. 2017-present.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may
// not use this file except in compliance with the License. You may obtain
// a copy of the License at http://www.apache.org/licenses/LICENSE-2.0

package mongo

import (
	"context"
	"os"
	"testing"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/event"
	"go.mongodb.org/mongo-driver/internal/testutil/helpers"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.mongodb.org/mongo-driver/mongo/readconcern"
	"go.mongodb.org/mongo-driver/mongo/writeconcern"
	"go.mongodb.org/mongo-driver/x/bsonx"
)

var ccStarted *event.CommandStartedEvent
var ccSucceeded *event.CommandSucceededEvent

var ccMonitor = &event.CommandMonitor{
	Started: func(ctx context.Context, cse *event.CommandStartedEvent) {
		ccStarted = cse
	},
	Succeeded: func(ctx context.Context, cse *event.CommandSucceededEvent) {
		ccSucceeded = cse
	},
}

var startingDoc = bsonx.Doc{{"hello", bsonx.Int32(5)}}

func compareOperationTimes(t *testing.T, expected *primitive.Timestamp, actual *primitive.Timestamp) {
	if expected.T != actual.T {
		t.Fatalf("T value mismatch; expected %d got %d", expected.T, actual.T)
	}

	if expected.I != actual.I {
		t.Fatalf("I value mismatch; expected %d got %d", expected.I, actual.I)
	}
}

func checkOperationTime(t *testing.T, cmd bson.Raw, shouldInclude bool) {
	rc, err := cmd.LookupErr("readConcern")
	testhelpers.RequireNil(t, err, "key read concern not found")

	_, err = bson.Raw(rc.Value).LookupErr("afterClusterTime")
	if shouldInclude {
		testhelpers.RequireNil(t, err, "afterClusterTime not found")
	} else {
		testhelpers.RequireNotNil(t, err, "afterClusterTime found")
	}
}

func getOperationTime(t *testing.T, cmd bson.Raw) *primitive.Timestamp {
	rc, err := cmd.LookupErr("readConcern")
	testhelpers.RequireNil(t, err, "key read concern not found")

	ct, err := bson.Raw(rc.Value).LookupErr("afterClusterTime")
	testhelpers.RequireNil(t, err, "key afterClusterTime not found")

	timeT, timeI := ct.Timestamp()
	return &primitive.Timestamp{
		T: timeT,
		I: timeI,
	}
}

func createReadFuncMap(t *testing.T, dbName string, collName string) (*Client, *Database, *Collection, []CollFunction) {
	client := createSessionsMonitoredClient(t, ccMonitor)
	db := client.Database(dbName)
	err := db.Drop(ctx)
	testhelpers.RequireNil(t, err, "error dropping database after creation: %s", err)

	coll := db.Collection(collName)
	coll.writeConcern = writeconcern.New(writeconcern.WMajority())

	functions := []CollFunction{
		{"Aggregate", coll, nil, func(mctx SessionContext) error { _, err := coll.Aggregate(mctx, emptyArr); return err }},
		{"EstimatedDocumentCount", coll, nil, func(mctx SessionContext) error { _, err := coll.EstimatedDocumentCount(mctx); return err }},
		{"Distinct", coll, nil, func(mctx SessionContext) error { _, err := coll.Distinct(mctx, "field", emptyDoc); return err }},
		{"Find", coll, nil, func(mctx SessionContext) error { _, err := coll.Find(mctx, emptyDoc); return err }},
		{"FindOne", coll, nil, func(mctx SessionContext) error { res := coll.FindOne(mctx, emptyDoc); return res.err }},
	}

	_, err = coll.InsertOne(ctx, startingDoc)
	testhelpers.RequireNil(t, err, "error inserting starting doc: %s", err)
	coll.writeConcern = nil
	return client, db, coll, functions
}

func checkReadConcern(t *testing.T, cmd bson.Raw, levelIncluded bool, expectedLevel string, optimeIncluded bool, expectedTime *primitive.Timestamp) {
	rc, err := cmd.LookupErr("readConcern")
	testhelpers.RequireNil(t, err, "key readConcern not found")

	rcDoc := bson.Raw(rc.Value)
	levelVal, err := rcDoc.LookupErr("level")
	if levelIncluded {
		testhelpers.RequireNil(t, err, "key level not found")
		if levelVal.StringValue() != expectedLevel {
			t.Fatalf("level mismatch. expected %s got %s", expectedLevel, levelVal.StringValue())
		}
	} else {
		testhelpers.RequireNotNil(t, err, "key level found")
	}

	ct, err := rcDoc.LookupErr("afterClusterTime")
	if optimeIncluded {
		testhelpers.RequireNil(t, err, "key afterClusterTime not found")
		ctT, ctI := ct.Timestamp()
		compareOperationTimes(t, expectedTime, &primitive.Timestamp{ctT, ctI})
	} else {
		testhelpers.RequireNotNil(t, err, "key afterClusterTime found")
	}
}

func createWriteFuncMap(t *testing.T, dbName string, collName string) (*Client, *Database, *Collection, []CollFunction) {
	client := createSessionsMonitoredClient(t, ccMonitor)
	db := client.Database(dbName)
	err := db.Drop(ctx)
	testhelpers.RequireNil(t, err, "error dropping database after creation: %s", err)

	coll := db.Collection(collName)
	coll.writeConcern = writeconcern.New(writeconcern.WMajority())
	_, err = coll.InsertOne(ctx, startingDoc)
	testhelpers.RequireNil(t, err, "error inserting starting doc: %s", err)
	coll.writeConcern = nil

	iv := coll.Indexes()

	manyIndexes := []IndexModel{barIndex, bazIndex}

	functions := []CollFunction{
		{"InsertOne", coll, nil, func(mctx SessionContext) error { _, err := coll.InsertOne(mctx, doc); return err }},
		{"InsertMany", coll, nil, func(mctx SessionContext) error { _, err := coll.InsertMany(mctx, []interface{}{doc2}); return err }},
		{"DeleteOne", coll, nil, func(mctx SessionContext) error { _, err := coll.DeleteOne(mctx, emptyDoc); return err }},
		{"DeleteMany", coll, nil, func(mctx SessionContext) error { _, err := coll.DeleteMany(mctx, emptyDoc); return err }},
		{"UpdateOne", coll, nil, func(mctx SessionContext) error { _, err := coll.UpdateOne(mctx, emptyDoc, updateDoc); return err }},
		{"UpdateMany", coll, nil, func(mctx SessionContext) error { _, err := coll.UpdateMany(mctx, emptyDoc, updateDoc); return err }},
		{"ReplaceOne", coll, nil, func(mctx SessionContext) error { _, err := coll.ReplaceOne(mctx, emptyDoc, emptyDoc); return err }},
		{"FindOneAndDelete", coll, nil, func(mctx SessionContext) error { res := coll.FindOneAndDelete(mctx, emptyDoc); return res.err }},
		{"FindOneAndReplace", coll, nil, func(mctx SessionContext) error {
			res := coll.FindOneAndReplace(mctx, emptyDoc, emptyDoc)
			return res.err
		}},
		{"FindOneAndUpdate", coll, nil, func(mctx SessionContext) error {
			res := coll.FindOneAndUpdate(mctx, emptyDoc, updateDoc)
			return res.err
		}},
		{"DropCollection", coll, nil, func(mctx SessionContext) error { err := coll.Drop(mctx); return err }},
		{"DropDatabase", coll, nil, func(mctx SessionContext) error { err := db.Drop(mctx); return err }},
		{"ListCollections", coll, nil, func(mctx SessionContext) error { _, err := db.ListCollections(mctx, emptyDoc); return err }},
		{"ListDatabases", coll, nil, func(mctx SessionContext) error { _, err := client.ListDatabases(mctx, emptyDoc); return err }},
		{"CreateOneIndex", coll, nil, func(mctx SessionContext) error { _, err := iv.CreateOne(mctx, fooIndex); return err }},
		{"CreateManyIndexes", coll, nil, func(mctx SessionContext) error { _, err := iv.CreateMany(mctx, manyIndexes); return err }},
		{"DropOneIndex", coll, nil, func(mctx SessionContext) error { _, err := iv.DropOne(mctx, "barIndex"); return err }},
		{"DropAllIndexes", coll, nil, func(mctx SessionContext) error { _, err := iv.DropAll(mctx); return err }},
		{"ListIndexes", coll, nil, func(mctx SessionContext) error { _, err := iv.List(mctx); return err }},
	}

	return client, db, coll, functions
}

func skipIfSessionsSupported(t *testing.T, db *Database) {
	if os.Getenv("TOPOLOGY") != "server" {
		t.Skip("skipping topology")
	}

	serverVersion, err := getServerVersion(db)
	testhelpers.RequireNil(t, err, "error getting server version: %s", err)

	if compareVersions(t, serverVersion, "3.6") >= 0 {
		t.Skip("skipping server version")
	}
}

func TestCausalConsistency(t *testing.T) {
	skipIfBelow36(t)

	t.Run("TestOperationTimeNil", func(t *testing.T) {
		// When a ClientSession is first created the operationTime has no value

		client := createSessionsMonitoredClient(t, ccMonitor)
		sess, err := client.StartSession()
		testhelpers.RequireNil(t, err, "error creating session: %s", err)
		defer sess.EndSession(ctx)

		if sess.OperationTime() != nil {
			t.Fatal("operation time is not nil")
		}
	})

	t.Run("TestNoTimeOnFirstCommand", func(t *testing.T) {
		// First read in causally consistent session must not send afterClusterTime to the server

		client := createSessionsMonitoredClient(t, ccMonitor)

		db := client.Database("FirstCommandDB")
		err := db.Drop(ctx)
		testhelpers.RequireNil(t, err, "error dropping db: %s", err)

		coll := db.Collection("FirstCommandColl")
		err = client.UseSessionWithOptions(ctx, options.Session().SetCausalConsistency(true),
			func(mctx SessionContext) error {
				_, err := coll.Find(mctx, emptyDoc)
				return err
			})
		testhelpers.RequireNil(t, err, "error running find: %s", err)

		testhelpers.RequireNotNil(t, ccStarted, "no started command found")
		if ccStarted.CommandName != "find" {
			t.Fatalf("started command %s is not a find command", ccStarted.CommandName)
		}

		checkOperationTime(t, ccStarted.Command, false)
	})

	t.Run("TestOperationTimeUpdated", func(t *testing.T) {
		//The first read or write on a ClientSession should update the operationTime of the ClientSession, even if there is an error

		skipInvalidTopology(t)
		skipIfBelow36(t)

		client := createSessionsMonitoredClient(t, ccMonitor)
		sess, err := client.StartSession()
		testhelpers.RequireNil(t, err, "error starting session: %s", err)
		defer sess.EndSession(ctx)

		db := client.Database("OptimeUpdateDB")
		err = db.Drop(ctx)
		testhelpers.RequireNil(t, err, "error dropping db: %s", err)

		coll := db.Collection("OptimeUpdateColl")
		_ = WithSession(ctx, sess, func(mctx SessionContext) error {
			_, _ = coll.Find(mctx, emptyDoc)
			return nil
		})

		testhelpers.RequireNotNil(t, ccSucceeded, "no succeeded command")
		serverT, serverI := ccSucceeded.Reply.Lookup("operationTime").Timestamp()

		testhelpers.RequireNotNil(t, sess.OperationTime(), "operation time nil after first command")
		compareOperationTimes(t, &primitive.Timestamp{serverT, serverI}, sess.OperationTime())
	})

	t.Run("TestOperationTimeSent", func(t *testing.T) {
		// findOne followed by another read operation should include operationTime returned by the server for the first
		// operation in the afterClusterTime parameter of the second operation

		skipInvalidTopology(t)
		skipIfBelow36(t)

		client, _, coll, readMap := createReadFuncMap(t, "OptimeSentDB", "OptimeSentColl")

		for _, tc := range readMap {
			t.Run(tc.name, func(t *testing.T) {
				sess, err := client.StartSession(options.Session().SetCausalConsistency(true))
				testhelpers.RequireNil(t, err, "error creating session for %s: %s", tc.name, err)
				defer sess.EndSession(ctx)

				err = WithSession(ctx, sess, func(mctx SessionContext) error {
					docRes := coll.FindOne(mctx, emptyDoc)
					return docRes.err
				})
				testhelpers.RequireNil(t, err, "find one error for %s: %s", tc.name, err)

				currOptime := sess.OperationTime()

				err = WithSession(ctx, sess, tc.f)
				testhelpers.RequireNil(t, err, "error running %s: %s", tc.name, err)

				testhelpers.RequireNotNil(t, ccStarted, "no started command")
				sentOptime := getOperationTime(t, ccStarted.Command)

				compareOperationTimes(t, currOptime, sentOptime)
			})
		}
	})

	t.Run("TestWriteThenRead", func(t *testing.T) {
		// Any write operation followed by findOne should include operationTime of first op in afterClusterTime parameter of
		// second op

		skipInvalidTopology(t)
		skipIfBelow36(t)

		client, _, coll, writeMap := createWriteFuncMap(t, "WriteThenReadDB", "WriteThenReadColl")

		for _, tc := range writeMap {
			t.Run(tc.name, func(t *testing.T) {
				sess, err := client.StartSession(options.Session().SetCausalConsistency(true))
				testhelpers.RequireNil(t, err, "error starting session: %s", err)
				defer sess.EndSession(ctx)

				err = WithSession(ctx, sess, tc.f)
				testhelpers.RequireNil(t, err, "error running %s: %s", tc.name, err)

				currentOptime := sess.OperationTime()

				_ = WithSession(ctx, sess, func(mctx SessionContext) error {
					_ = coll.FindOne(mctx, emptyDoc)
					return nil
				})

				testhelpers.RequireNotNil(t, ccStarted, "no started command")
				sentOptime := getOperationTime(t, ccStarted.Command)

				compareOperationTimes(t, currentOptime, sentOptime)
			})
		}
	})

	t.Run("TestNonConsistentRead", func(t *testing.T) {
		// Read op in a non causally-consistent session should not include afterClusterTime in cmd sent to server

		skipInvalidTopology(t)
		skipIfBelow36(t)

		client := createSessionsMonitoredClient(t, ccMonitor)

		db := client.Database("NonConsistentReadDB")
		err := db.Drop(ctx)
		testhelpers.RequireNil(t, err, "error dropping db: %s", err)

		coll := db.Collection("NonConsistentReadColl")
		_ = client.UseSessionWithOptions(ctx, options.Session().SetCausalConsistency(false),
			func(mctx SessionContext) error {
				_, _ = coll.Find(mctx, emptyDoc)
				return nil
			})

		testhelpers.RequireNotNil(t, ccStarted, "no started command")
		if ccStarted.CommandName != "find" {
			t.Fatalf("started command %s was not a find command", ccStarted.CommandName)
		}

		checkOperationTime(t, ccStarted.Command, false)
	})

	t.Run("TestInvalidTopology", func(t *testing.T) {
		// A read op in a causally consistent session does not include afterClusterTime in a deployment that does not
		// support cluster times

		client := createSessionsMonitoredClient(t, ccMonitor)
		db := client.Database("InvalidTopologyDB")
		err := db.Drop(ctx)
		testhelpers.RequireNil(t, err, "error dropping db: %s", err)

		skipIfSessionsSupported(t, db)

		coll := db.Collection("InvalidTopologyColl")
		_ = client.UseSessionWithOptions(ctx, options.Session().SetCausalConsistency(true),
			func(mctx SessionContext) error {
				_, _ = coll.Find(mctx, emptyDoc)
				return nil
			})

		testhelpers.RequireNotNil(t, ccStarted, "no started command found")
		if ccStarted.CommandName != "find" {
			t.Fatalf("started command %s was not a find command", ccStarted.CommandName)
		}

		checkOperationTime(t, ccStarted.Command, false)
	})

	t.Run("TestDefaultReadConcern", func(t *testing.T) {
		// When using the default server read concern, the readConcern parameter in the command sent to the server should
		// not include a level field

		skipInvalidTopology(t)
		skipIfBelow36(t)

		client := createSessionsMonitoredClient(t, ccMonitor)
		sess, err := client.StartSession(options.Session().SetCausalConsistency(true))
		testhelpers.RequireNil(t, err, "error starting session: %s", err)

		db := client.Database("DefaultReadConcernDB")
		err = db.Drop(ctx)
		testhelpers.RequireNil(t, err, "error dropping db: %s", err)

		coll := db.Collection("DefaultReadConcernColl")
		coll.readConcern = readconcern.New()
		_ = WithSession(ctx, sess, func(mctx SessionContext) error {
			_ = coll.FindOne(mctx, emptyDoc)
			return nil
		})

		currOptime := sess.OperationTime()
		_ = WithSession(ctx, sess, func(mctx SessionContext) error {
			_ = coll.FindOne(mctx, emptyDoc)
			return nil
		})

		testhelpers.RequireNotNil(t, ccStarted, "no started command found")
		if ccStarted.CommandName != "find" {
			t.Fatalf("started command %s was not a find command", ccStarted.CommandName)
		}

		checkReadConcern(t, ccStarted.Command, false, "", true, currOptime)
	})

	t.Run("TestCustomReadConcern", func(t *testing.T) {
		// When using a custom read concern, the readConcern field in commands sent to the server should have level
		// and afterClusterTime

		skipInvalidTopology(t)
		skipIfBelow36(t)

		client := createSessionsMonitoredClient(t, ccMonitor)
		sess, err := client.StartSession(options.Session().SetCausalConsistency(true))
		testhelpers.RequireNil(t, err, "error starting session: %s", err)
		defer sess.EndSession(ctx)

		db := client.Database("CustomReadConcernDB")
		err = db.Drop(ctx)
		testhelpers.RequireNil(t, err, "error dropping db: %s", err)

		coll := db.Collection("CustomReadConcernColl")
		coll.readConcern = readconcern.Majority()

		_ = WithSession(ctx, sess, func(mctx SessionContext) error {
			_ = coll.FindOne(mctx, emptyDoc)
			return nil
		})

		currOptime := sess.OperationTime()

		_ = WithSession(ctx, sess, func(mctx SessionContext) error {
			_ = coll.FindOne(mctx, emptyDoc)
			return nil
		})

		testhelpers.RequireNotNil(t, ccStarted, "no started command found")
		if ccStarted.CommandName != "find" {
			t.Fatalf("started command %s was not a find command", ccStarted.CommandName)
		}

		checkReadConcern(t, ccStarted.Command, true, "majority", true, currOptime)
	})

	t.Run("TestUnacknowledgedWrite", func(t *testing.T) {
		// Unacknowledged write should not update the operationTime property of a session

		client := createSessionsMonitoredClient(t, nil)
		sess, err := client.StartSession(options.Session().SetCausalConsistency(true))
		testhelpers.RequireNil(t, err, "error starting session: %s", err)
		defer sess.EndSession(ctx)

		db := client.Database("UnackWriteDB")
		err = db.Drop(ctx)
		testhelpers.RequireNil(t, err, "error dropping db: %s", err)

		coll := db.Collection("UnackWriteColl")
		coll.writeConcern = writeconcern.New(writeconcern.W(0))
		_, _ = coll.InsertOne(ctx, doc)

		if sess.OperationTime() != nil {
			t.Fatal("operation time updated for unacknowledged write")
		}
	})

	t.Run("TestInvalidTopologyClusterTime", func(t *testing.T) {
		// $clusterTime should not be included in commands if the deployment does not support cluster times

		client := createSessionsMonitoredClient(t, ccMonitor)
		db := client.Database("InvalidTopCTDB")
		err := db.Drop(ctx)
		testhelpers.RequireNil(t, err, "error dropping db: %s", err)

		skipIfSessionsSupported(t, db)

		coll := db.Collection("InvalidTopCTColl")
		_ = coll.FindOne(ctx, emptyDoc)

		testhelpers.RequireNotNil(t, ccStarted, "no started command found")
		if ccStarted.CommandName != "find" {
			t.Fatalf("started command %s was not a find command", ccStarted.CommandName)
		}

		_, err = ccStarted.Command.LookupErr("$clusterTime")
		testhelpers.RequireNotNil(t, err, "$clusterTime found for invalid topology")
	})

	t.Run("TestValidTopologyClusterTime", func(t *testing.T) {
		// $clusterTime should be included in commands if the deployment supports cluster times

		skipInvalidTopology(t)
		skipIfBelow36(t)

		client := createSessionsMonitoredClient(t, ccMonitor)
		db := client.Database("ValidTopCTDB")
		err := db.Drop(ctx)
		testhelpers.RequireNil(t, err, "error dropping db: %s", err)

		coll := db.Collection("ValidTopCTColl")
		_ = coll.FindOne(ctx, emptyDoc)

		testhelpers.RequireNotNil(t, ccStarted, "no started command found")
		if ccStarted.CommandName != "find" {
			t.Fatalf("started command %s was not a find command", ccStarted.CommandName)
		}

		_, err = ccStarted.Command.LookupErr("$clusterTime")
		testhelpers.RequireNil(t, err, "$clusterTime found for invalid topology")
	})
}
