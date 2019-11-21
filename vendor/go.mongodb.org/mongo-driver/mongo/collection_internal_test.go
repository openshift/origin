// Copyright (C) MongoDB, Inc. 2017-present.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may
// not use this file except in compliance with the License. You may obtain
// a copy of the License at http://www.apache.org/licenses/LICENSE-2.0

package mongo

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/event"
	"go.mongodb.org/mongo-driver/internal/testutil"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.mongodb.org/mongo-driver/mongo/readconcern"
	"go.mongodb.org/mongo-driver/mongo/readpref"
	"go.mongodb.org/mongo-driver/mongo/writeconcern"
	"go.mongodb.org/mongo-driver/x/bsonx"
	"go.mongodb.org/mongo-driver/x/bsonx/bsoncore"
)

var impossibleWriteConcern = writeconcern.New(writeconcern.W(50), writeconcern.WTimeout(time.Second))

func createTestCollection(t *testing.T, dbName *string, collName *string, opts ...*options.CollectionOptions) *Collection {
	if collName == nil {
		coll := testutil.ColName(t)
		collName = &coll
	}

	db := createTestDatabase(t, dbName)
	db.RunCommand(
		context.Background(),
		bsonx.Doc{{"create", bsonx.String(*collName)}},
	)

	collOpts := []*options.CollectionOptions{options.Collection().SetWriteConcern(writeconcern.New(writeconcern.WMajority()))}
	collOpts = append(collOpts, opts...)
	return db.Collection(*collName, collOpts...)
}

func skipIfBelow34(t *testing.T, db *Database) {
	versionStr, err := getServerVersion(db)
	if err != nil {
		t.Fatalf("error getting server version: %s", err)
	}
	if compareVersions(t, versionStr, "3.4") < 0 {
		t.Skip("skipping collation test for server version < 3.4")
	}
}

func initCollection(t *testing.T, coll *Collection) {
	docs := []interface{}{}
	var i int32
	for i = 1; i <= 5; i++ {
		docs = append(docs, bsonx.Doc{{"x", bsonx.Int32(i)}})
	}

	_, err := coll.InsertMany(ctx, docs)
	require.Nil(t, err)
}

func create16MBDocument(t *testing.T) bsoncore.Document {
	// 4 bytes = document length
	// 1 byte = element type (ObjectID = \x07)
	// 4 bytes = key name ("_id" + \x00)
	// 12 bytes = ObjectID value
	// 1 byte = element type (string = \x02)
	// 4 bytes = key name ("key" + \x00)
	// 4 bytes = string length
	// X bytes = string of length X bytes
	// 1 byte = \x00
	// 1 byte = \x00
	//
	// Therefore the string length should be: 1024*1024*16 - 32

	targetDocSize := 1024 * 1024 * 16
	strSize := targetDocSize - 32
	var b strings.Builder
	b.Grow(strSize)
	for i := 0; i < strSize; i++ {
		b.WriteByte('A')
	}

	idx, doc := bsoncore.AppendDocumentStart(nil)
	doc = bsoncore.AppendObjectIDElement(doc, "_id", primitive.NewObjectID())
	doc = bsoncore.AppendStringElement(doc, "key", b.String())
	doc, _ = bsoncore.AppendDocumentEnd(doc, idx)
	require.Equal(t, targetDocSize, len(doc), "expected document length %v, got %v", targetDocSize, len(doc))
	return doc
}

func TestCollection_initialize(t *testing.T) {
	dbName := "foo"
	collName := "bar"

	coll := createTestCollection(t, &dbName, &collName)
	require.Equal(t, coll.name, collName)
	require.NotNil(t, coll.db)

}

func compareColls(t *testing.T, expected *Collection, got *Collection) {
	switch {
	case expected.readPreference != got.readPreference:
		t.Errorf("expected read preference %#v. got %#v", expected.readPreference, got.readPreference)
	case expected.readConcern != got.readConcern:
		t.Errorf("expected read concern %#v. got %#v", expected.readConcern, got.readConcern)
	case expected.writeConcern != got.writeConcern:
		t.Errorf("expected write concern %#v. got %#v", expected.writeConcern, got.writeConcern)
	}
}

func TestCollection_Options(t *testing.T) {
	name := "testDb_options"
	rpPrimary := readpref.Primary()
	rpSecondary := readpref.Secondary()
	wc1 := writeconcern.New(writeconcern.W(5))
	wc2 := writeconcern.New(writeconcern.W(10))
	rcLocal := readconcern.Local()
	rcMajority := readconcern.Majority()

	opts := options.Collection().SetReadPreference(rpPrimary).SetReadConcern(rcLocal).SetWriteConcern(wc1).
		SetReadPreference(rpSecondary).SetReadConcern(rcMajority).SetWriteConcern(wc2)

	dbName := "collection_internal_test_db1"

	expectedColl := &Collection{
		readConcern:    rcMajority,
		readPreference: rpSecondary,
		writeConcern:   wc2,
	}

	t.Run("IndividualOptions", func(t *testing.T) {
		// if options specified multiple times, last instance should take precedence
		coll := createTestCollection(t, &dbName, &name, opts)
		compareColls(t, expectedColl, coll)

	})

	t.Run("Bundle", func(t *testing.T) {
		coll := createTestCollection(t, &dbName, &name, opts)
		compareColls(t, expectedColl, coll)
	})
}

func TestCollection_InheritOptions(t *testing.T) {
	name := "testDb_options_inherit"
	client := createTestClient(t)

	rpPrimary := readpref.Primary()
	rcLocal := readconcern.Local()
	wc1 := writeconcern.New(writeconcern.W(10))

	db := client.Database("collection_internal_test_db2")
	db.readPreference = rpPrimary
	db.readConcern = rcLocal
	coll := db.Collection(name, options.Collection().SetWriteConcern(wc1))

	// coll should inherit read preference and read concern from client
	switch {
	case coll.readPreference != rpPrimary:
		t.Errorf("expected read preference primary. got %#v", coll.readPreference)
	case coll.readConcern != rcLocal:
		t.Errorf("expected read concern local. got %#v", coll.readConcern)
	case coll.writeConcern != wc1:
		t.Errorf("expected write concern %#v. got %#v", wc1, coll.writeConcern)
	}
}

func TestCollection_ReplaceTopologyError(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}

	cs := testutil.ConnString(t)
	c, err := NewClient(options.Client().ApplyURI(cs.String()))
	require.NoError(t, err)
	require.NotNil(t, c)

	db := c.Database("TestCollection")
	coll := db.Collection("ReplaceTopologyError")

	doc1 := bsonx.Doc{{"x", bsonx.Int32(1)}}
	doc2 := bsonx.Doc{{"x", bsonx.Int32(6)}}
	docs := []interface{}{doc1, doc2}
	update := bsonx.Doc{
		{"$inc", bsonx.Document(bsonx.Doc{{"x", bsonx.Int32(1)}})},
	}

	_, err = coll.InsertOne(context.Background(), doc1)
	require.Equal(t, err, ErrClientDisconnected)

	_, err = coll.InsertMany(context.Background(), docs)
	require.Equal(t, err, ErrClientDisconnected)

	_, err = coll.DeleteOne(context.Background(), doc1)
	require.Equal(t, err, ErrClientDisconnected)

	_, err = coll.DeleteMany(context.Background(), doc1)
	require.Equal(t, err, ErrClientDisconnected)

	_, err = coll.UpdateOne(context.Background(), doc1, update)
	require.Equal(t, err, ErrClientDisconnected)

	_, err = coll.UpdateMany(context.Background(), doc1, update)
	require.Equal(t, err, ErrClientDisconnected)

	_, err = coll.ReplaceOne(context.Background(), doc1, doc2)
	require.Equal(t, err, ErrClientDisconnected)

	pipeline := bsonx.Arr{
		bsonx.Document(
			bsonx.Doc{{"$match", bsonx.Document(bsonx.Doc{{"x", bsonx.Document(bsonx.Doc{{"$gte", bsonx.Int32(2)}})}})}},
		),
		bsonx.Document(
			bsonx.Doc{{
				"$project",
				bsonx.Document(bsonx.Doc{
					{"_id", bsonx.Int32(0)},
					{"x", bsonx.Int32(1)},
				}),
			}},
		)}

	_, err = coll.Aggregate(context.Background(), pipeline, options.Aggregate())
	require.Equal(t, err, ErrClientDisconnected)

	_, err = coll.EstimatedDocumentCount(context.Background())
	require.Equal(t, err, ErrClientDisconnected)

	_, err = coll.CountDocuments(context.Background(), bsonx.Doc{})
	require.Equal(t, err, ErrClientDisconnected)

	_, err = coll.EstimatedDocumentCount(context.Background())
	require.Equal(t, err, ErrClientDisconnected)

	_, err = coll.Distinct(context.Background(), "x", bsonx.Doc{})
	require.Equal(t, err, ErrClientDisconnected)

	_, err = coll.Find(context.Background(), doc1)
	require.Equal(t, err, ErrClientDisconnected)

	result := coll.FindOne(context.Background(), doc1)
	require.Equal(t, result.err, ErrClientDisconnected)

	result = coll.FindOneAndDelete(context.Background(), doc1)
	require.Equal(t, result.err, ErrClientDisconnected)

	result = coll.FindOneAndReplace(context.Background(), doc1, doc2)
	require.Equal(t, result.err, ErrClientDisconnected)

	result = coll.FindOneAndUpdate(context.Background(), doc1, update)
	require.Equal(t, result.err, ErrClientDisconnected)
}

func TestCollection_database_accessor(t *testing.T) {
	dbName := "foo"
	collName := "bar"

	coll := createTestCollection(t, &dbName, &collName)
	require.Equal(t, coll.Database().Name(), dbName)
}

func TestCollection_BulkWrite_writeErrors_Insert(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	want := WriteError{Code: 11000}
	doc1 := NewInsertOneModel().SetDocument(bson.D{{"_id", "x"}})
	doc2 := NewInsertOneModel().SetDocument(bson.D{{"_id", "y"}})
	models := []WriteModel{doc1, doc1, doc2, doc2}

	t.Run("unordered", func(t *testing.T) {
		coll := createTestCollection(t, nil, nil)
		res, err := coll.BulkWrite(context.Background(), models, options.BulkWrite().SetOrdered(false))
		require.Equal(t, res.InsertedCount, int64(2))

		got, ok := err.(BulkWriteException)
		if !ok {
			t.Errorf("Did not receive correct type of error. got %T; want %T", err, WriteErrors{})
			t.FailNow()
		}
		if len(got.WriteErrors) != 2 {
			t.Errorf("Incorrect number of errors receieved. got %d; want %d", len(got.WriteErrors), 2)
			t.FailNow()
		}
		if got.WriteErrors[0].Code != want.Code {
			t.Errorf("Did not receive the correct error code. got %d; want %d", got.WriteErrors[0].Code, want.Code)
		}
	})

	t.Run("ordered", func(t *testing.T) {
		coll := createTestCollection(t, nil, nil)
		res, err := coll.BulkWrite(context.Background(), models, options.BulkWrite())
		require.Equal(t, res.InsertedCount, int64(1))

		got, ok := err.(BulkWriteException)
		if !ok {
			t.Errorf("Did not receive correct type of error. got %T; want %T", err, WriteErrors{})
			t.FailNow()
		}
		if len(got.WriteErrors) != 1 {
			t.Errorf("Incorrect number of errors receieved. got %d; want %d", len(got.WriteErrors), 1)
			t.FailNow()
		}
		if got.WriteErrors[0].Code != want.Code {
			t.Errorf("Did not receive the correct error code. got %d; want %d", got.WriteErrors[0].Code, want.Code)
		}
	})
}

func TestCollection_BulkWrite_writeErrors_Delete(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	doc := NewDeleteOneModel().SetFilter(bson.D{{"x", 1}})
	models := []WriteModel{doc, doc}

	db := createTestDatabase(t, nil)
	collName := testutil.ColName(t)
	err := db.RunCommand(
		context.Background(),
		bsonx.Doc{
			{"create", bsonx.String(collName)},
			{"capped", bsonx.Boolean(true)},
			{"size", bsonx.Int32(64 * 1024)},
		},
	).Err()
	require.NoError(t, err)

	t.Run("unordered", func(t *testing.T) {
		coll := db.Collection(collName)
		_, err = coll.BulkWrite(context.Background(), models, options.BulkWrite().SetOrdered(false))

		got, ok := err.(BulkWriteException)
		if !ok {
			t.Errorf("Did not receive correct type of error. got %T; want %T", err, WriteErrors{})
			t.FailNow()
		}
		if len(got.WriteErrors) != 2 {
			t.Errorf("Incorrect number of errors receieved. got %d; want %d", len(got.WriteErrors), 2)
			t.FailNow()
		}
		if got.WriteErrors[0].Code != 20 && got.WriteErrors[0].Code != 10101 {
			t.Errorf("Did not receive the correct error code. got %d; want 20 or 10101", got.WriteErrors[0].Code)
		}
	})

	t.Run("ordered", func(t *testing.T) {
		coll := db.Collection(collName)
		_, err = coll.BulkWrite(context.Background(), models, options.BulkWrite())

		got, ok := err.(BulkWriteException)
		if !ok {
			t.Errorf("Did not receive correct type of error. got %T; want %T", err, WriteErrors{})
			t.FailNow()
		}
		if len(got.WriteErrors) != 1 {
			t.Errorf("Incorrect number of errors receieved. got %d; want %d", len(got.WriteErrors), 1)
			t.FailNow()
		}
		if got.WriteErrors[0].Code != 20 && got.WriteErrors[0].Code != 10101 {
			t.Errorf("Did not receive the correct error code. got %d; want 20 or 10101", got.WriteErrors[0].Code)
		}
	})
}

func TestCollection_BulkWrite_writeErrors_Update(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	doc1 := NewUpdateOneModel().SetFilter(bson.D{{"_id", "foo"}}).SetUpdate(bson.D{{"$set", bson.D{{"_id", 3.14159}}}})
	doc2 := NewUpdateOneModel().SetFilter(bson.D{{"_id", "foo"}}).SetUpdate(bson.D{{"$set", bson.D{{"x", "fa"}}}})
	models := []WriteModel{doc1, doc1, doc2}
	want := WriteError{Code: 66}

	t.Run("unordered", func(t *testing.T) {
		coll := createTestCollection(t, nil, nil)
		_, err := coll.InsertOne(context.Background(), bsonx.Doc{{"_id", bsonx.String("foo")}})
		require.NoError(t, err)

		res, err := coll.BulkWrite(context.Background(), models, options.BulkWrite().SetOrdered(false))
		require.Equal(t, res.ModifiedCount, int64(1))

		got, ok := err.(BulkWriteException)
		if !ok {
			t.Errorf("Did not receive correct type of error. got %T; want %T", err, WriteErrors{})
			t.FailNow()
		}
		if len(got.WriteErrors) != 2 {
			t.Errorf("Incorrect number of errors receieved. got %d; want %d", len(got.WriteErrors), 2)
			t.FailNow()
		}
		if got.WriteErrors[0].Code != want.Code {
			t.Errorf("Did not receive the correct error code. got %d; want %d", got.WriteErrors[0].Code, want.Code)
		}
	})

	t.Run("ordered", func(t *testing.T) {
		coll := createTestCollection(t, nil, nil)
		_, err := coll.InsertOne(context.Background(), bsonx.Doc{{"_id", bsonx.String("foo")}})
		require.NoError(t, err)

		res, err := coll.BulkWrite(context.Background(), models, options.BulkWrite())
		require.Equal(t, res.ModifiedCount, int64(0))

		got, ok := err.(BulkWriteException)
		if !ok {
			t.Errorf("Did not receive correct type of error. got %T; want %T", err, WriteErrors{})
			t.FailNow()
		}
		if len(got.WriteErrors) != 1 {
			t.Errorf("Incorrect number of errors receieved. got %d; want %d", len(got.WriteErrors), 1)
			t.FailNow()
		}
		if got.WriteErrors[0].Code != want.Code {
			t.Errorf("Did not receive the correct error code. got %d; want %d", got.WriteErrors[0].Code, want.Code)
		}
	})
}

func TestCollection_InsertOne(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	id := primitive.NewObjectID()
	want := id
	doc := bsonx.Doc{bsonx.Elem{"_id", bsonx.ObjectID(id)}, {"x", bsonx.Int32(1)}}
	coll := createTestCollection(t, nil, nil)

	result, err := coll.InsertOne(context.Background(), doc)
	require.Nil(t, err)
	if !cmp.Equal(result.InsertedID, want) {
		t.Errorf("Result documents do not match. got %v; want %v", result.InsertedID, want)
	}

}

func TestCollection_InsertOne_WriteError(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	want := WriteError{Code: 11000}
	doc := bsonx.Doc{{"_id", bsonx.ObjectID(primitive.NewObjectID())}}
	coll := createTestCollection(t, nil, nil)

	_, err := coll.InsertOne(context.Background(), doc)
	require.NoError(t, err)
	_, err = coll.InsertOne(context.Background(), doc)
	got, ok := err.(WriteException)
	if !ok {
		t.Errorf("Did not receive correct type of error. got %T; want %T", err, WriteException{})
	}
	if len(got.WriteErrors) != 1 {
		t.Errorf("Incorrect number of errors receieved. got %d; want %d", len(got.WriteErrors), 1)
		t.FailNow()
	}
	if got.WriteErrors[0].Code != want.Code {
		t.Errorf("Did not receive the correct error code. got %d; want %d", got.WriteErrors[0].Code, want.Code)
	}

}

func TestCollection_InsertOne_WriteConcernError(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	if os.Getenv("TOPOLOGY") != "replica_set" {
		t.Skip()
	}

	doc := bsonx.Doc{{"_id", bsonx.ObjectID(primitive.NewObjectID())}}
	coll := createTestCollection(t, nil, nil, options.Collection().SetWriteConcern(impossibleWriteConcern))

	_, err := coll.InsertOne(context.Background(), doc)
	writeErr, ok := err.(WriteException)
	if !ok {
		t.Errorf("incorrect error type returned: %T", writeErr)
	}
	if writeErr.WriteConcernError == nil {
		t.Errorf("write concern error is nil: %+v", writeErr)
	}
}

func TestCollection_NilDocumentError(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	coll := createTestCollection(t, nil, nil)

	_, err := coll.InsertOne(context.Background(), nil)
	require.Equal(t, err, ErrNilDocument)

	_, err = coll.InsertMany(context.Background(), nil)
	require.Equal(t, err, ErrEmptySlice)

	_, err = coll.InsertMany(context.Background(), []interface{}{})
	require.Equal(t, err, ErrEmptySlice)

	_, err = coll.InsertMany(context.Background(), []interface{}{bsonx.Doc{bsonx.Elem{"_id", bsonx.Int32(1)}}, nil})
	require.Equal(t, err, ErrNilDocument)

	_, err = coll.DeleteOne(context.Background(), nil)
	require.Equal(t, err, ErrNilDocument)

	_, err = coll.DeleteMany(context.Background(), nil)
	require.Equal(t, err, ErrNilDocument)

	_, err = coll.UpdateOne(context.Background(), nil, bsonx.Doc{{"$set", bsonx.Document(bsonx.Doc{{"_id", bsonx.Double(3.14159)}})}})
	require.Equal(t, err, ErrNilDocument)

	_, err = coll.UpdateOne(context.Background(), bsonx.Doc{{"_id", bsonx.Double(3.14159)}}, nil)
	require.Equal(t, err, ErrNilDocument)

	_, err = coll.UpdateMany(context.Background(), nil, bsonx.Doc{{"$set", bsonx.Document(bsonx.Doc{{"_id", bsonx.Double(3.14159)}})}})
	require.Equal(t, err, ErrNilDocument)

	_, err = coll.UpdateMany(context.Background(), bsonx.Doc{{"_id", bsonx.Double(3.14159)}}, nil)
	require.Equal(t, err, ErrNilDocument)

	_, err = coll.ReplaceOne(context.Background(), bsonx.Doc{{"_id", bsonx.Double(3.14159)}}, nil)
	require.Equal(t, err, ErrNilDocument)

	_, err = coll.ReplaceOne(context.Background(), nil, bsonx.Doc{{"_id", bsonx.Double(3.14159)}})
	require.Equal(t, err, ErrNilDocument)

	_, err = coll.CountDocuments(context.Background(), nil)
	require.Equal(t, err, ErrNilDocument)

	_, err = coll.Distinct(context.Background(), "field", nil)
	require.Equal(t, err, ErrNilDocument)

	_, err = coll.Find(context.Background(), nil)
	require.Equal(t, err, ErrNilDocument)

	res := coll.FindOne(context.Background(), nil)
	require.Equal(t, res.err, ErrNilDocument)

	res = coll.FindOneAndDelete(context.Background(), nil)
	require.Equal(t, res.err, ErrNilDocument)

	res = coll.FindOneAndReplace(context.Background(), bsonx.Doc{{"_id", bsonx.Double(3.14159)}}, nil)
	require.Equal(t, res.err, ErrNilDocument)

	res = coll.FindOneAndReplace(context.Background(), nil, bsonx.Doc{{"_id", bsonx.Double(3.14159)}})
	require.Equal(t, res.err, ErrNilDocument)

	res = coll.FindOneAndUpdate(context.Background(), bsonx.Doc{{"_id", bsonx.Double(3.14159)}}, nil)
	require.Equal(t, res.err, ErrNilDocument)

	res = coll.FindOneAndUpdate(context.Background(), nil, bsonx.Doc{{"_id", bsonx.Double(3.14159)}})
	require.Equal(t, res.err, ErrNilDocument)

	_, err = coll.BulkWrite(context.Background(), nil)
	require.Equal(t, err, ErrEmptySlice)

	_, err = coll.BulkWrite(context.Background(), []WriteModel{})
	require.Equal(t, err, ErrEmptySlice)

	_, err = coll.BulkWrite(context.Background(), []WriteModel{nil})
	require.Equal(t, err, ErrNilDocument)

	_, err = coll.Aggregate(context.Background(), nil)
	require.Equal(t, err, errors.New("can only transform slices and arrays into aggregation pipelines, but got invalid"))

	_, err = coll.Watch(context.Background(), nil)
	require.Equal(t, err, errors.New("can only transform slices and arrays into aggregation pipelines, but got invalid"))
}

func TestCollection_InsertMany(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	want1 := int32(11)
	want2 := int32(12)
	docs := []interface{}{
		bsonx.Doc{bsonx.Elem{"_id", bsonx.Int32(11)}},
		bsonx.Doc{{"x", bsonx.Int32(6)}},
		bsonx.Doc{bsonx.Elem{"_id", bsonx.Int32(12)}},
	}
	coll := createTestCollection(t, nil, nil)

	result, err := coll.InsertMany(context.Background(), docs)
	require.Nil(t, err)

	require.Len(t, result.InsertedIDs, 3)
	if !cmp.Equal(result.InsertedIDs[0], want1) {
		t.Errorf("Result documents do not match. got %v; want %v", result.InsertedIDs[0], want1)
	}
	require.NotNil(t, result.InsertedIDs[1])
	if !cmp.Equal(result.InsertedIDs[2], want2) {
		t.Errorf("Result documents do not match. got %v; want %v", result.InsertedIDs[2], want2)
	}

}

func TestCollection_InsertMany_Batches(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	// TODO(GODRIVER-425): remove this as part a larger project to
	// refactor integration and other longrunning tasks.
	if os.Getenv("EVR_TASK_ID") == "" {
		t.Skip("skipping long running integration test outside of evergreen")
	}

	////t.Parallel()

	const (
		megabyte = 10 * 10 * 10 * 10 * 10 * 10
		numDocs  = 700000
	)

	docs := []interface{}{}
	total := uint32(0)
	expectedDocSize := uint32(26)
	for i := 0; i < numDocs; i++ {
		d := bsonx.Doc{
			{"a", bsonx.Int32(int32(i))},
			{"b", bsonx.Int32(int32(i * 2))},
			{"c", bsonx.Int32(int32(i * 3))},
		}
		b, _ := d.MarshalBSON()
		require.Equal(t, int(expectedDocSize), len(b), "len=%d expected=%d", len(b), expectedDocSize)
		docs = append(docs, d)
		total += uint32(len(b))
	}
	assert.True(t, total > 16*megabyte)
	dbName := "InsertManyBatchesDB"
	collName := "InsertManyBatchesColl"
	coll := createTestCollection(t, &dbName, &collName)

	result, err := coll.InsertMany(context.Background(), docs)
	require.Nil(t, err)
	require.Len(t, result.InsertedIDs, numDocs)

}

func TestCollection_InsertMany_LargeDocumentBatches(t *testing.T) {
	// TODO(GODRIVER-425): remove this as part a larger project to
	// refactor integration and other longrunning tasks.
	if os.Getenv("EVR_TASK_ID") == "" {
		t.Skip("skipping long running integration test outside of evergreen")
	}

	client := createMonitoredClient(t, monitor)
	db := client.Database("insertmany_largebatches_db")
	coll := db.Collection("insertmany_largebatches_coll")
	err := coll.Drop(ctx)
	require.NoError(t, err, "Drop error: %v", err)
	docs := []interface{}{create16MBDocument(t), create16MBDocument(t)}

	drainChannels()
	_, err = coll.InsertMany(ctx, docs)
	require.NoError(t, err, "InsertMany error: %v", err)

	for i := 0; i < 2; i++ {
		var evt *event.CommandStartedEvent
		select {
		case evt = <-startedChan:
		default:
			t.Fatalf("no insert event found for iteration %v", i)
		}

		require.Equal(t, "insert", evt.CommandName, "expected 'insert' event, got '%v'", evt.CommandName)
	}
}

func TestCollection_InsertMany_ErrorCases(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	want := WriteError{Code: 11000}
	docs := []interface{}{
		bsonx.Doc{{"_id", bsonx.ObjectID(primitive.NewObjectID())}},
		bsonx.Doc{{"_id", bsonx.ObjectID(primitive.NewObjectID())}},
		bsonx.Doc{{"_id", bsonx.ObjectID(primitive.NewObjectID())}},
	}
	coll := createTestCollection(t, nil, nil)

	t.Run("insert_batch_unordered", func(t *testing.T) {
		_, err := coll.InsertMany(context.Background(), docs)
		require.NoError(t, err)

		// without option ordered
		_, err = coll.InsertMany(context.Background(), docs, options.InsertMany().SetOrdered(false))
		got, ok := err.(BulkWriteException)
		if !ok {
			t.Errorf("Did not receive correct type of error. got %T; want %T", err, WriteErrors{})
			t.FailNow()
		}
		if len(got.WriteErrors) != 3 {
			t.Errorf("Incorrect number of errors receieved. got %d; want %d", len(got.WriteErrors), 3)
			t.FailNow()
		}
		if got.WriteErrors[0].Code != want.Code {
			t.Errorf("Did not receive the correct error code. got %d; want %d", got.WriteErrors[0].Code, want.Code)
		}
	})
	t.Run("insert_batch_ordered_write_error", func(t *testing.T) {
		// run the insert again to ensure that the documents
		// are there in cases when this case is run
		// independently of the previous test
		_, _ = coll.InsertMany(context.Background(), docs)

		// with the ordered option (default, we should only get one write error)
		_, err := coll.InsertMany(context.Background(), docs)
		got, ok := err.(BulkWriteException)
		if !ok {
			t.Errorf("Did not receive correct type of error. got %T; want %T", err, WriteErrors{})
			t.FailNow()
		}
		if len(got.WriteErrors) != 1 {
			t.Errorf("Incorrect number of errors receieved. got %d; want %d", len(got.WriteErrors), 1)
			t.FailNow()
		}
		if got.WriteErrors[0].Code != want.Code {
			t.Errorf("Did not receive the correct error code. got %d; want %d", got.WriteErrors[0].Code, want.Code)
		}

	})
	t.Run("insert_batch_write_concern_error", func(t *testing.T) {
		if os.Getenv("TOPOLOGY") != "replica_set" {
			t.Skip()
		}

		docs = []interface{}{
			bsonx.Doc{{"_id", bsonx.ObjectID(primitive.NewObjectID())}},
			bsonx.Doc{{"_id", bsonx.ObjectID(primitive.NewObjectID())}},
			bsonx.Doc{{"_id", bsonx.ObjectID(primitive.NewObjectID())}},
		}

		copyColl, err := coll.Clone(options.Collection().SetWriteConcern(impossibleWriteConcern))
		if err != nil {
			t.Errorf("err copying collection: %s", err)
		}

		_, err = copyColl.InsertMany(context.Background(), docs)
		if err == nil {
			t.Errorf("write concern error not propagated from command: %+v", err)
		}
		bulkErr, ok := err.(BulkWriteException)
		if !ok {
			t.Errorf("incorrect error type returned: %T", err)
		}
		if bulkErr.WriteConcernError == nil {
			t.Errorf("write concern error is nil: %+v", bulkErr)
		}
	})

}

func TestCollection_InsertMany_WriteConcernError(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	if os.Getenv("TOPOLOGY") != "replica_set" {
		t.Skip()
	}

	docs := []interface{}{
		bsonx.Doc{{"_id", bsonx.ObjectID(primitive.NewObjectID())}},
		bsonx.Doc{{"_id", bsonx.ObjectID(primitive.NewObjectID())}},
		bsonx.Doc{{"_id", bsonx.ObjectID(primitive.NewObjectID())}},
	}
	coll := createTestCollection(t, nil, nil,
		options.Collection().SetWriteConcern(impossibleWriteConcern))

	_, err := coll.InsertMany(context.Background(), docs)
	got, ok := err.(BulkWriteException)
	if !ok {
		t.Errorf("Did not receive correct type of error. got %T; want %T\nError message: %s", err, BulkWriteException{}, err)
		t.Errorf("got error message %v", err)
	}
	if got.WriteConcernError == nil {
		t.Errorf("write concern error was nil")
	}
}

func TestCollection_DeleteOne_found(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	coll := createTestCollection(t, nil, nil)
	initCollection(t, coll)

	filter := bsonx.Doc{{"x", bsonx.Int32(1)}}
	result, err := coll.DeleteOne(context.Background(), filter)
	require.Nil(t, err)
	require.NotNil(t, result)
	require.Equal(t, result.DeletedCount, int64(1))

}

func TestCollection_DeleteOne_notFound(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	coll := createTestCollection(t, nil, nil)
	initCollection(t, coll)

	filter := bsonx.Doc{{"x", bsonx.Int32(0)}}
	result, err := coll.DeleteOne(context.Background(), filter)
	require.Nil(t, err)
	require.Equal(t, result.DeletedCount, int64(0))

}

func TestCollection_DeleteOne_notFound_withOption(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	coll := createTestCollection(t, nil, nil)
	skipIfBelow34(t, coll.db)
	initCollection(t, coll)

	filter := bsonx.Doc{{"x", bsonx.Int32(0)}}

	result, err := coll.DeleteOne(context.Background(), filter,
		options.Delete().SetCollation(&options.Collation{Locale: "en_US"}))
	require.Nil(t, err)
	require.Equal(t, result.DeletedCount, int64(0))

}

func TestCollection_DeleteOne_WriteError(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	filter := bsonx.Doc{{"x", bsonx.Int32(1)}}
	db := createTestDatabase(t, nil)
	err := db.RunCommand(
		context.Background(),
		bsonx.Doc{
			{"create", bsonx.String(testutil.ColName(t))},
			{"capped", bsonx.Boolean(true)},
			{"size", bsonx.Int32(64 * 1024)},
		},
	).Err()
	require.NoError(t, err)
	coll := db.Collection(testutil.ColName(t))

	_, err = coll.DeleteOne(context.Background(), filter)
	got, ok := err.(WriteException)
	if !ok {
		t.Errorf("Did not receive correct type of error. got %T; want %T", err, WriteException{})
	}
	if len(got.WriteErrors) != 1 {
		t.Errorf("Incorrect number of errors receieved. got %d; want %d", len(got.WriteErrors), 1)
		t.FailNow()
	}
	// 2.6 returns 10101 instead of 20
	if got.WriteErrors[0].Code != 20 && got.WriteErrors[0].Code != 10101 {
		t.Errorf("Did not receive the correct error code. got %d; want 20 or 10101", got.WriteErrors[0].Code)
	}
}

func TestCollection_DeleteMany_WriteConcernError(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	if os.Getenv("TOPOLOGY") != "replica_set" {
		t.Skip()
	}

	filter := bsonx.Doc{{"x", bsonx.Int32(1)}}
	coll := createTestCollection(t, nil, nil)

	// 2.6 server returns right away if the document doesn't exist
	_, err := coll.InsertOne(ctx, filter)
	if err != nil {
		t.Fatalf("error inserting doc: %s", err)
	}

	cloned, err := coll.Clone(options.Collection().SetWriteConcern(impossibleWriteConcern))
	if err != nil {
		t.Fatalf("error cloning collection: %s", err)
	}
	_, err = cloned.DeleteOne(context.Background(), filter)
	writeErr, ok := err.(WriteException)
	if !ok {
		t.Errorf("incorrect error type returned: %T", writeErr)
	}
	if writeErr.WriteConcernError == nil {
		t.Errorf("write concern error is nil: %+v", writeErr)
	}
}

func TestCollection_DeleteMany_found(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	coll := createTestCollection(t, nil, nil)
	initCollection(t, coll)

	filter := bsonx.Doc{{"x", bsonx.Document(bsonx.Doc{{"$gte", bsonx.Int32(3)}})}}

	result, err := coll.DeleteMany(context.Background(), filter)
	require.Nil(t, err)
	require.Equal(t, result.DeletedCount, int64(3))

}

func TestCollection_DeleteMany_notFound(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	coll := createTestCollection(t, nil, nil)
	initCollection(t, coll)

	filter := bsonx.Doc{{"x", bsonx.Document(bsonx.Doc{{"$lt", bsonx.Int32(1)}})}}

	result, err := coll.DeleteMany(context.Background(), filter)
	require.Nil(t, err)
	require.Equal(t, result.DeletedCount, int64(0))

}

func TestCollection_DeleteMany_notFound_withOption(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	coll := createTestCollection(t, nil, nil)
	skipIfBelow34(t, coll.db)
	initCollection(t, coll)

	filter := bsonx.Doc{{"x", bsonx.Document(bsonx.Doc{{"$lt", bsonx.Int32(1)}})}}

	result, err := coll.DeleteMany(context.Background(), filter,
		options.Delete().SetCollation(&options.Collation{Locale: "en_US"}))
	require.Nil(t, err)
	require.Equal(t, result.DeletedCount, int64(0))

}

func TestCollection_DeleteMany_WriteError(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	filter := bsonx.Doc{{"x", bsonx.Int32(1)}}
	db := createTestDatabase(t, nil)
	err := db.RunCommand(
		context.Background(),
		bsonx.Doc{
			{"create", bsonx.String(testutil.ColName(t))},
			{"capped", bsonx.Boolean(true)},
			{"size", bsonx.Int32(64 * 1024)},
		},
	).Err()
	require.NoError(t, err)
	coll := db.Collection(testutil.ColName(t))

	_, err = coll.DeleteMany(context.Background(), filter)
	got, ok := err.(WriteException)
	if !ok {
		t.Errorf("Did not receive correct type of error. got %T; want %T", err, WriteException{})
	}
	if len(got.WriteErrors) != 1 {
		t.Errorf("Incorrect number of errors receieved. got %d; want %d", len(got.WriteErrors), 1)
		t.FailNow()
	}
	// 2.6 returns 10101 instead of 20
	if got.WriteErrors[0].Code != 20 && got.WriteErrors[0].Code != 10101 {
		t.Errorf("Did not receive the correct error code. got %d; want 20 or 10101", got.WriteErrors[0].Code)
	}
}

func TestCollection_DeleteOne_WriteConcernError(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	if os.Getenv("TOPOLOGY") != "replica_set" {
		t.Skip()
	}

	filter := bsonx.Doc{{"x", bsonx.Int32(1)}}
	coll := createTestCollection(t, nil, nil)

	// 2.6 server returns right away if the document doesn't exist
	_, err := coll.InsertOne(ctx, filter)
	if err != nil {
		t.Fatalf("error inserting document: %s", err)
	}

	cloned, err := coll.Clone(options.Collection().SetWriteConcern(impossibleWriteConcern))
	if err != nil {
		t.Fatalf("error cloning collection: %s", err)
	}

	_, err = cloned.DeleteMany(context.Background(), filter)
	writeErr, ok := err.(WriteException)
	if !ok {
		t.Errorf("incorrect error type returned: %T", writeErr)
	}
	if writeErr.WriteConcernError == nil {
		t.Errorf("write concern error is nil: %+v", writeErr)
	}
}

func TestCollection_UpdateOne_EmptyUpdate(t *testing.T) {
	coll := createTestCollection(t, nil, nil)
	_, err := coll.UpdateOne(ctx, bsonx.Doc{}, bsonx.Doc{})
	require.NotNil(t, err)
}

func TestCollection_UpdateOne_found(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	coll := createTestCollection(t, nil, nil)
	initCollection(t, coll)

	filter := bsonx.Doc{{"x", bsonx.Int32(1)}}
	update := bsonx.Doc{{"$inc", bsonx.Document(bsonx.Doc{{"x", bsonx.Int32(1)}})}}

	result, err := coll.UpdateOne(context.Background(), filter, update)
	require.Nil(t, err)
	require.NotNil(t, result)
	require.Equal(t, result.MatchedCount, int64(1))
	require.Equal(t, result.ModifiedCount, int64(1))
	require.Nil(t, result.UpsertedID)

}

func TestCollection_UpdateOne_notFound(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	coll := createTestCollection(t, nil, nil)
	initCollection(t, coll)

	filter := bsonx.Doc{{"x", bsonx.Int32(0)}}
	update := bsonx.Doc{{"$inc", bsonx.Document(bsonx.Doc{{"x", bsonx.Int32(1)}})}}

	result, err := coll.UpdateOne(context.Background(), filter, update)
	require.Nil(t, err)
	require.Equal(t, result.MatchedCount, int64(0))
	require.Equal(t, result.ModifiedCount, int64(0))
	require.Nil(t, result.UpsertedID)

}

func TestCollection_UpdateOne_upsert(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	coll := createTestCollection(t, nil, nil)
	initCollection(t, coll)

	filter := bsonx.Doc{{"x", bsonx.Int32(0)}}
	update := bsonx.Doc{{"$inc", bsonx.Document(bsonx.Doc{{"x", bsonx.Int32(1)}})}}

	result, err := coll.UpdateOne(context.Background(), filter, update, options.Update().SetUpsert(true))
	require.Nil(t, err)
	require.Equal(t, result.MatchedCount, int64(0))
	require.Equal(t, result.ModifiedCount, int64(0))
	require.NotNil(t, result.UpsertedID)

}

func TestCollection_UpdateOne_WriteError(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	want := WriteError{Code: 66}
	filter := bsonx.Doc{{"_id", bsonx.String("foo")}}
	update := bsonx.Doc{{"$set", bsonx.Document(bsonx.Doc{{"_id", bsonx.Double(3.14159)}})}}
	coll := createTestCollection(t, nil, nil)

	_, err := coll.InsertOne(context.Background(), bsonx.Doc{{"_id", bsonx.String("foo")}})
	require.NoError(t, err)

	_, err = coll.UpdateOne(context.Background(), filter, update)
	got, ok := err.(WriteException)
	if !ok {
		t.Errorf("Did not receive correct type of error. got %T; want %T", err, WriteException{})
	}
	if len(got.WriteErrors) != 1 {
		t.Errorf("Incorrect number of errors receieved. got %d; want %d", len(got.WriteErrors), 1)
		t.FailNow()
	}
	if got.WriteErrors[0].Code != want.Code {
		t.Errorf("Did not receive the correct error code. got %d; want %d", got.WriteErrors[0].Code, want.Code)
	}

}

func TestCollection_UpdateOne_WriteConcernError(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	if os.Getenv("TOPOLOGY") != "replica_set" {
		t.Skip()
	}

	filter := bsonx.Doc{{"_id", bsonx.String("foo")}}
	update := bsonx.Doc{{"$set", bsonx.Document(bsonx.Doc{{"pi", bsonx.Double(3.14159)}})}}
	coll := createTestCollection(t, nil, nil)

	// 2.6 server returns right away if the document doesn't exist
	_, err := coll.InsertOne(ctx, filter)
	if err != nil {
		t.Fatalf("error inserting document: %s", err)
	}

	cloned, err := coll.Clone(options.Collection().SetWriteConcern(impossibleWriteConcern))
	if err != nil {
		t.Fatalf("error cloning collection: %s", err)
	}
	_, err = cloned.UpdateOne(context.Background(), filter, update)
	writeErr, ok := err.(WriteException)
	if !ok {
		t.Errorf("incorrect error type returned: %T", writeErr)
	}
	if writeErr.WriteConcernError == nil {
		t.Errorf("write concern error is nil: %+v", writeErr)
	}
}

func TestCollection_UpdateMany_EmptyUpdate(t *testing.T) {
	coll := createTestCollection(t, nil, nil)
	_, err := coll.UpdateMany(ctx, bsonx.Doc{}, bsonx.Doc{})
	require.NotNil(t, err)
}

func TestCollection_UpdateMany_found(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	coll := createTestCollection(t, nil, nil)
	initCollection(t, coll)

	filter := bsonx.Doc{{"x", bsonx.Document(bsonx.Doc{{"$gte", bsonx.Int32(3)}})}}

	update := bsonx.Doc{{"$inc", bsonx.Document(bsonx.Doc{{"x", bsonx.Int32(1)}})}}

	result, err := coll.UpdateMany(context.Background(), filter, update)
	require.Nil(t, err)
	require.Equal(t, result.MatchedCount, int64(3))
	require.Equal(t, result.ModifiedCount, int64(3))
	require.Nil(t, result.UpsertedID)

}

func TestCollection_UpdateMany_notFound(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	coll := createTestCollection(t, nil, nil)
	initCollection(t, coll)

	filter := bsonx.Doc{{"x", bsonx.Document(bsonx.Doc{{"$lt", bsonx.Int32(1)}})}}

	update := bsonx.Doc{{"$inc", bsonx.Document(bsonx.Doc{{"x", bsonx.Int32(1)}})}}

	result, err := coll.UpdateMany(context.Background(), filter, update)
	require.Nil(t, err)
	require.Equal(t, result.MatchedCount, int64(0))
	require.Equal(t, result.ModifiedCount, int64(0))
	require.Nil(t, result.UpsertedID)

}

func TestCollection_UpdateMany_upsert(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	coll := createTestCollection(t, nil, nil)
	initCollection(t, coll)

	filter := bsonx.Doc{{"x", bsonx.Document(bsonx.Doc{{"$lt", bsonx.Int32(1)}})}}

	update := bsonx.Doc{{"$inc", bsonx.Document(bsonx.Doc{{"x", bsonx.Int32(1)}})}}

	result, err := coll.UpdateMany(context.Background(), filter, update, options.Update().SetUpsert(true))
	require.Nil(t, err)
	require.Equal(t, result.MatchedCount, int64(0))
	require.Equal(t, result.ModifiedCount, int64(0))
	require.NotNil(t, result.UpsertedID)

}

func TestCollection_UpdateMany_WriteError(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	want := WriteError{Code: 66}
	filter := bsonx.Doc{{"_id", bsonx.String("foo")}}
	update := bsonx.Doc{{"$set", bsonx.Document(bsonx.Doc{{"_id", bsonx.Double(3.14159)}})}}
	coll := createTestCollection(t, nil, nil)

	_, err := coll.InsertOne(context.Background(), bsonx.Doc{{"_id", bsonx.String("foo")}})
	require.NoError(t, err)

	_, err = coll.UpdateMany(context.Background(), filter, update)
	got, ok := err.(WriteException)
	if !ok {
		t.Errorf("Did not receive correct type of error. got %T; want %T", err, WriteException{})
	}
	if len(got.WriteErrors) != 1 {
		t.Errorf("Incorrect number of errors receieved. got %d; want %d", len(got.WriteErrors), 1)
		t.FailNow()
	}
	if got.WriteErrors[0].Code != want.Code {
		t.Errorf("Did not receive the correct error code. got %d; want %d", got.WriteErrors[0].Code, want.Code)
	}

}

func TestCollection_UpdateMany_WriteConcernError(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	if os.Getenv("TOPOLOGY") != "replica_set" {
		t.Skip()
	}

	filter := bsonx.Doc{{"_id", bsonx.String("foo")}}
	update := bsonx.Doc{{"$set", bsonx.Document(bsonx.Doc{{"pi", bsonx.Double(3.14159)}})}}
	coll := createTestCollection(t, nil, nil)

	// 2.6 server returns right away if the document doesn't exist
	_, err := coll.InsertOne(ctx, filter)
	if err != nil {
		t.Fatalf("error inserting document: %s", err)
	}

	cloned, err := coll.Clone(options.Collection().SetWriteConcern(impossibleWriteConcern))
	if err != nil {
		t.Fatalf("error cloning collection: %s", err)
	}
	_, err = cloned.UpdateMany(context.Background(), filter, update)
	writeErr, ok := err.(WriteException)
	if !ok {
		t.Errorf("incorrect error type returned: %T", writeErr)
	}
	if writeErr.WriteConcernError == nil {
		t.Errorf("write concern error is nil: %+v", writeErr)
	}
}

func TestCollection_ReplaceOne_found(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	coll := createTestCollection(t, nil, nil)
	initCollection(t, coll)

	filter := bsonx.Doc{{"x", bsonx.Int32(1)}}
	replacement := bsonx.Doc{{"y", bsonx.Int32(1)}}

	result, err := coll.ReplaceOne(context.Background(), filter, replacement)
	require.Nil(t, err)
	require.NotNil(t, result)
	require.Equal(t, result.MatchedCount, int64(1))
	require.Equal(t, result.ModifiedCount, int64(1))
	require.Nil(t, result.UpsertedID)

}

func TestCollection_ReplaceOne_notFound(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	coll := createTestCollection(t, nil, nil)
	initCollection(t, coll)

	filter := bsonx.Doc{{"x", bsonx.Int32(0)}}
	replacement := bsonx.Doc{{"y", bsonx.Int32(1)}}

	result, err := coll.ReplaceOne(context.Background(), filter, replacement)
	require.Nil(t, err)
	require.Equal(t, result.MatchedCount, int64(0))
	require.Equal(t, result.ModifiedCount, int64(0))
	require.Nil(t, result.UpsertedID)

}

func TestCollection_ReplaceOne_upsert(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	coll := createTestCollection(t, nil, nil)
	initCollection(t, coll)

	filter := bsonx.Doc{{"x", bsonx.Int32(0)}}
	replacement := bsonx.Doc{{"y", bsonx.Int32(1)}}

	result, err := coll.ReplaceOne(context.Background(), filter, replacement, options.Replace().SetUpsert(true))
	require.Nil(t, err)
	require.Equal(t, result.MatchedCount, int64(0))
	require.Equal(t, result.ModifiedCount, int64(0))
	require.NotNil(t, result.UpsertedID)

}

func TestCollection_ReplaceOne_WriteError(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	filter := bsonx.Doc{{"_id", bsonx.String("foo")}}
	replacement := bsonx.Doc{{"_id", bsonx.Double(3.14159)}}
	coll := createTestCollection(t, nil, nil)

	_, err := coll.InsertOne(context.Background(), bsonx.Doc{{"_id", bsonx.String("foo")}})
	require.NoError(t, err)

	_, err = coll.ReplaceOne(context.Background(), filter, replacement)
	got, ok := err.(WriteException)
	if !ok {
		t.Errorf("Did not receive correct type of error. got %T; want %T", err, WriteException{})
	}
	if len(got.WriteErrors) != 1 {
		t.Errorf("Incorrect number of errors receieved. got %d; want %d", len(got.WriteErrors), 1)
		t.FailNow()
	}
	switch got.WriteErrors[0].Code {
	case 66: // mongod v3.6
	case 16837: //mongod v3.4, mongod v3.2
	default:
		t.Errorf("Did not receive the correct error code. got %d; want (one of) %d", got.WriteErrors[0].Code, []int{66, 16837})
		fmt.Printf("%#v\n", got)
	}

}

func TestCollection_ReplaceOne_WriteConcernError(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	if os.Getenv("TOPOLOGY") != "replica_set" {
		t.Skip()
	}

	filter := bsonx.Doc{{"_id", bsonx.String("foo")}}
	update := bsonx.Doc{{"pi", bsonx.Double(3.14159)}}
	coll := createTestCollection(t, nil, nil)

	// 2.6 server returns right away if the document doesn't exist
	_, err := coll.InsertOne(ctx, filter)
	if err != nil {
		t.Fatalf("error inserting document: %s", err)
	}

	cloned, err := coll.Clone(options.Collection().SetWriteConcern(impossibleWriteConcern))
	if err != nil {
		t.Fatalf("error cloning collection: %s", err)
	}
	_, err = cloned.ReplaceOne(context.Background(), filter, update)
	writeErr, ok := err.(WriteException)
	if !ok {
		t.Errorf("incorrect error type returned: %T", writeErr)
	}
	if writeErr.WriteConcernError == nil {
		t.Errorf("write concern error is nil: %+v", writeErr)
	}
}

func TestCollection_Aggregate(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	coll := createTestCollection(t, nil, nil)
	initCollection(t, coll)

	pipeline := bsonx.Arr{
		bsonx.Document(
			bsonx.Doc{{"$match", bsonx.Document(bsonx.Doc{{"x", bsonx.Document(bsonx.Doc{{"$gte", bsonx.Int32(2)}})}})}},
		),
		bsonx.Document(
			bsonx.Doc{{
				"$project",
				bsonx.Document(bsonx.Doc{
					{"_id", bsonx.Int32(0)},
					{"x", bsonx.Int32(1)},
				}),
			}},
		),
		bsonx.Document(
			bsonx.Doc{{"$sort", bsonx.Document(bsonx.Doc{{"x", bsonx.Int32(1)}})}},
		)}

	//cursor, err := coll.Aggregate(context.Background(), pipeline, aggregateopt.BundleAggregate())
	cursor, err := coll.Aggregate(context.Background(), pipeline, options.Aggregate())
	require.Nil(t, err)

	for i := 2; i < 5; i++ {
		var doc bsonx.Doc
		require.True(t, cursor.Next(context.Background()))
		err = cursor.Decode(&doc)
		require.NoError(t, err)

		require.Equal(t, len(doc), 1)
		num, err := doc.LookupErr("x")
		require.NoError(t, err)
		if num.Type() != bson.TypeInt32 {
			t.Errorf("Incorrect type for x. Got %s, but wanted Int32", num.Type())
			t.FailNow()
		}
		require.Equal(t, int(num.Int32()), i)
	}

}

func testAggregateWithOptions(t *testing.T, createIndex bool, opts *options.AggregateOptions) error {
	coll := createTestCollection(t, nil, nil)
	initCollection(t, coll)

	if createIndex {
		indexView := coll.Indexes()
		_, err := indexView.CreateOne(context.Background(), IndexModel{
			Keys: bsonx.Doc{{"x", bsonx.Int32(1)}},
		})

		if err != nil {
			return err
		}
	}

	pipeline := Pipeline{
		{{"$match", bson.D{{"x", bson.D{{"$gte", 2}}}}}},
		{{"$project", bson.D{{"_id", 0}, {"x", 1}}}},
		{{"$sort", bson.D{{"x", 1}}}},
	}

	cursor, err := coll.Aggregate(context.Background(), pipeline, opts)
	if err != nil {
		return err
	}

	for i := 2; i < 5; i++ {
		var doc bsonx.Doc
		cursor.Next(context.Background())
		err = cursor.Decode(&doc)
		if err != nil {
			return err
		}

		if len(doc) != 1 {
			return fmt.Errorf("got doc len %d, expected 1", len(doc))
		}

		num, err := doc.LookupErr("x")
		if err != nil {
			return err
		}

		if num.Type() != bson.TypeInt32 {
			return fmt.Errorf("incorrect type for x. got %s, wanted Int32", num.Type())
		}

		if int(num.Int32()) != i {
			return fmt.Errorf("unexpected value returned. got %d, expected %d", int(num.Int32()), i)
		}
	}

	return nil
}

func TestCollection_Aggregate_IndexHint(t *testing.T) {
	skipIfBelow36(t)

	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	//hint := aggregateopt.Hint(bson.NewDocument(bson.EC.Int32("x", 1)))
	aggOpts := options.Aggregate().SetHint(bsonx.Doc{{"x", bsonx.Int32(1)}})

	err := testAggregateWithOptions(t, true, aggOpts)
	require.NoError(t, err)
}

func TestCollection_Aggregate_withOptions(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	aggOpts := options.Aggregate().SetAllowDiskUse(true)

	err := testAggregateWithOptions(t, false, aggOpts)
	require.NoError(t, err)
}

func TestCollection_Aggregate_WriteConcernError(t *testing.T) {
	skipIfBelow36(t)

	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	if os.Getenv("TOPOLOGY") != "replica_set" {
		t.Skip()
	}

	coll := createTestCollection(t, nil, nil, options.Collection().SetWriteConcern(impossibleWriteConcern))

	pipeline := Pipeline{
		{{"$out", testutil.ColName(t)}},
	}

	cursor, err := coll.Aggregate(context.Background(), pipeline)
	require.Nil(t, cursor)
	require.Error(t, err)
	_, ok := err.(WriteConcernError)
	if !ok {
		t.Errorf("incorrect error type returned: %T", err)
	}
}

func TestCollection_CountDocuments(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	col1 := createTestCollection(t, nil, nil)
	initCollection(t, col1)

	count, err := col1.CountDocuments(context.Background(), bsonx.Doc{})
	require.Nil(t, err)
	require.Equal(t, count, int64(5))
}

func TestCollection_CountDocuments_withFilter(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	coll := createTestCollection(t, nil, nil)
	initCollection(t, coll)

	filter := bsonx.Doc{{"x", bsonx.Document(bsonx.Doc{{"$gt", bsonx.Int32(2)}})}}

	count, err := coll.CountDocuments(context.Background(), filter)
	require.Nil(t, err)
	require.Equal(t, count, int64(3))

}

func TestCollection_CountDocuments_withLimitOptions(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	coll := createTestCollection(t, nil, nil)
	initCollection(t, coll)

	count, err := coll.CountDocuments(context.Background(), bsonx.Doc{}, options.Count().SetLimit(3))
	require.Nil(t, err)
	require.Equal(t, count, int64(3))
}

func TestCollection_CountDocuments_withSkipOptions(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	coll := createTestCollection(t, nil, nil)
	initCollection(t, coll)

	count, err := coll.CountDocuments(context.Background(), bsonx.Doc{}, options.Count().SetSkip(3))
	require.Nil(t, err)
	require.Equal(t, count, int64(2))
}

func TestCollection_EstimatedDocumentCount(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	coll := createTestCollection(t, nil, nil)
	initCollection(t, coll)

	count, err := coll.EstimatedDocumentCount(context.Background())
	require.Nil(t, err)
	require.Equal(t, count, int64(5))

}

func TestCollection_EstimatedDocumentCount_withOption(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	coll := createTestCollection(t, nil, nil)
	initCollection(t, coll)

	count, err := coll.EstimatedDocumentCount(context.Background(), options.EstimatedDocumentCount().SetMaxTime(100))
	require.Nil(t, err)
	require.Equal(t, count, int64(5))
}

func TestCollection_Distinct(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	coll := createTestCollection(t, nil, nil)
	initCollection(t, coll)

	results, err := coll.Distinct(context.Background(), "x", bsonx.Doc{})
	require.Nil(t, err)
	require.Equal(t, results, []interface{}{int32(1), int32(2), int32(3), int32(4), int32(5)})
}

func TestCollection_Distinct_withFilter(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	coll := createTestCollection(t, nil, nil)
	initCollection(t, coll)

	filter := bsonx.Doc{{"x", bsonx.Document(bsonx.Doc{{"$gt", bsonx.Int32(2)}})}}

	results, err := coll.Distinct(context.Background(), "x", filter)
	require.Nil(t, err)
	require.Equal(t, results, []interface{}{int32(3), int32(4), int32(5)})
}

func TestCollection_Distinct_withOption(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	coll := createTestCollection(t, nil, nil)
	initCollection(t, coll)

	results, err := coll.Distinct(context.Background(), "x", bsonx.Doc{},
		options.Distinct().SetMaxTime(5000000000))
	require.Nil(t, err)
	require.Equal(t, results, []interface{}{int32(1), int32(2), int32(3), int32(4), int32(5)})
}

func TestCollection_Find_found(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	coll := createTestCollection(t, nil, nil)
	initCollection(t, coll)

	cursor, err := coll.Find(context.Background(),
		bsonx.Doc{},
		options.Find().SetSort(bsonx.Doc{{"x", bsonx.Int32(1)}}),
	)
	require.Nil(t, err)

	results := make([]int, 0, 5)
	var doc bson.Raw
	for cursor.Next(context.Background()) {
		err = cursor.Decode(&doc)
		require.NoError(t, err)

		_, err = doc.LookupErr("_id")
		require.NoError(t, err)

		i, err := doc.LookupErr("x")
		require.NoError(t, err)
		if i.Type != bson.TypeInt32 {
			t.Errorf("Incorrect type for x. Got %s, but wanted Int32", i.Type)
			t.FailNow()
		}
		results = append(results, int(i.Int32()))
	}

	require.Len(t, results, 5)
	require.Equal(t, results, []int{1, 2, 3, 4, 5})
}

func TestCollection_Find_With_Limit_And_BatchSize(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	coll := createTestCollection(t, nil, nil)
	initCollection(t, coll)

	batchSizes := []int32{2, 3, 4}
	for _, b := range batchSizes {
		name := fmt.Sprintf("Limit: 3, BatchSize: %v", b)
		t.Run(name, func(t *testing.T) {
			cursor, err := coll.Find(context.Background(), bson.D{}, options.Find().SetLimit(3).SetBatchSize(b))

			numRecieved := 0
			var doc bson.Raw
			for cursor.Next(context.Background()) {
				err = cursor.Decode(&doc)
				require.NoError(t, err)

				_, err = doc.LookupErr("_id")
				require.NoError(t, err)

				numRecieved++
			}

			require.NoError(t, cursor.Err())
			require.Equal(t, 3, numRecieved)
		})
	}
}

func TestCollection_Find_notFound(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	coll := createTestCollection(t, nil, nil)
	initCollection(t, coll)

	cursor, err := coll.Find(context.Background(), bsonx.Doc{{"x", bsonx.Int32(6)}})
	require.Nil(t, err)

	require.False(t, cursor.Next(context.Background()))
}

func TestCollection_Find_Error(t *testing.T) {
	t.Run("TestInvalidIdentifier", func(t *testing.T) {
		coll := createTestCollection(t, nil, nil)
		cursor, err := coll.Find(context.Background(), bsonx.Doc{{"$foo", bsonx.Int32(1)}})
		require.NotNil(t, err, "expected error for invalid identifier, got nil")
		require.Nil(t, cursor, "expected nil cursor for invalid identifier, got non-nil")
	})

	t.Run("Test killCursor is killed server side", func(t *testing.T) {

		coll := createTestCollection(t, nil, nil)
		version, err := getServerVersion(coll.db)
		if err != nil {
			t.Fatalf("getServerVersion failed %v", err)
		}

		if compareVersions(t, version, "3.0") <= 0 {
			t.Skip("skipping because less than 3.0 server version")
		}

		initCollection(t, coll)
		c, err := coll.Find(context.Background(), bsonx.Doc{}, options.Find().SetBatchSize(2))
		require.Nil(t, err, "error running find: %s", err)

		id := c.ID()
		require.True(t, c.Next(context.Background()))
		err = c.Close(context.Background())
		require.NoError(t, err)

		sr := coll.Database().RunCommand(context.Background(), bson.D{
			{"getMore", id},
			{"collection", coll.Name()},
		})

		ce := sr.Err().(CommandError)
		require.Equal(t, int(ce.Code), int(43)) // CursorNotFound
	})
}

func TestCollection_Find_NegativeLimit(t *testing.T) {
	coll := createTestCollection(t, nil, nil)
	initCollection(t, coll)

	c, err := coll.Find(ctx, bson.D{}, options.Find().SetLimit(-2))
	require.NoError(t, err)
	require.Equal(t, int64(0), c.ID()) // single batch returned so cursor should not have valid ID

	var numDocs int
	for c.Next(ctx) {
		numDocs++
	}
	require.Equal(t, 2, numDocs)
}

func TestCollection_Find_ExhaustCursor(t *testing.T) {
	coll := createTestCollection(t, nil, nil)
	initCollection(t, coll)
	c, err := coll.Find(ctx, bson.D{})
	require.NoError(t, err)

	var numDocs int
	for c.Next(ctx) {
		numDocs++
	}
	require.Equal(t, 5, numDocs)

	err = c.Close(ctx)
	require.NoError(t, err)
}

func TestCollection_Find_Hint(t *testing.T) {
	coll := createTestCollection(t, nil, nil)
	initCollection(t, coll)

	t.Run("string", func(t *testing.T) {
		c, err := coll.Find(ctx, bson.D{}, options.Find().SetHint("_id_"))
		if err != nil {
			t.Fatalf("find error: %v", err)
		}
		_ = c.Close(ctx)
	})
	t.Run("document", func(t *testing.T) {
		c, err := coll.Find(ctx, bson.D{}, options.Find().SetHint(bson.D{{"_id", 1}}))
		if err != nil {
			t.Fatalf("find error: %v", err)
		}
		_ = c.Close(ctx)
	})
	t.Run("error", func(t *testing.T) {
		c, err := coll.Find(ctx, bson.D{}, options.Find().SetHint("foobar"))
		if err == nil {
			_ = c.Close(ctx)
			t.Fatal("expected bad hint error but got nil")
		}
		_, ok := err.(CommandError)
		if !ok {
			t.Fatalf("err type mismatch; expected CommandError, got %T", err)
		}
	})
}

func TestCollection_FindOne_LimitSet(t *testing.T) {
	client := createMonitoredClient(t, monitor)
	coll := client.Database("FindOneLimitDB").Collection("FindOneLimitColl")
	defer func() {
		_ = coll.Drop(ctx)
	}()
	drainChannels()

	res := coll.FindOne(ctx, bson.D{})
	if err := res.Err(); err != ErrNoDocuments {
		t.Fatalf("FindOne error: %v", err)
	}

	var started *event.CommandStartedEvent
	select {
	case started = <-startedChan:
	default:
		t.Fatalf("expected a CommandStartedEvent but none found")
	}

	limitVal, err := started.Command.LookupErr("limit")
	if err != nil {
		t.Fatal("no limit sent")
	}
	if limit := limitVal.Int64(); limit != 1 {
		t.Fatalf("limit mismatch; expected %d, got %d", 1, limit)
	}
}

func TestCollection_FindOne_found(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	coll := createTestCollection(t, nil, nil)
	initCollection(t, coll)

	filter := bsonx.Doc{{"x", bsonx.Int32(1)}}
	var result bsonx.Doc
	err := coll.FindOne(context.Background(),
		filter,
	).Decode(&result)

	require.Nil(t, err)
	require.Equal(t, len(result), 2)

	_, err = result.LookupErr("_id")
	require.NoError(t, err)

	num, err := result.LookupErr("x")
	require.NoError(t, err)
	if num.Type() != bson.TypeInt32 {
		t.Errorf("Incorrect type for x. Got %s, but wanted Int32", num.Type())
		t.FailNow()
	}
	require.Equal(t, int(num.Int32()), 1)
}

func TestCollection_FindOne_found_withOption(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	coll := createTestCollection(t, nil, nil)
	initCollection(t, coll)

	filter := bsonx.Doc{{"x", bsonx.Int32(1)}}
	var result bsonx.Doc
	err := coll.FindOne(context.Background(),
		filter,
		options.FindOne().SetComment("here's a query for ya"),
	).Decode(&result)
	require.Nil(t, err)
	require.Equal(t, len(result), 2)

	_, err = result.LookupErr("_id")
	require.NoError(t, err)

	num, err := result.LookupErr("x")
	require.NoError(t, err)
	if num.Type() != bson.TypeInt32 {
		t.Errorf("Incorrect type for x. Got %s, but wanted Int32", num.Type())
		t.FailNow()
	}
	require.Equal(t, int(num.Int32()), 1)
}

func TestCollection_FindOne_notFound(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	coll := createTestCollection(t, nil, nil)
	initCollection(t, coll)

	filter := bsonx.Doc{{"x", bsonx.Int32(6)}}
	err := coll.FindOne(context.Background(), filter).Err()
	require.Equal(t, err, ErrNoDocuments)
}

func TestCollection_FindOneAndDelete_found(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	coll := createTestCollection(t, nil, nil)
	initCollection(t, coll)

	filter := bsonx.Doc{{"x", bsonx.Int32(3)}}

	var result bsonx.Doc
	err := coll.FindOneAndDelete(context.Background(), filter).Decode(&result)
	require.NoError(t, err)

	elem, err := result.LookupErr("x")
	require.NoError(t, err)
	require.Equal(t, elem.Type(), bson.TypeInt32, "Incorrect BSON Element type")
	require.Equal(t, int(elem.Int32()), 3)
}

func TestCollection_FindOneAndDelete_found_ignoreResult(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	coll := createTestCollection(t, nil, nil)
	initCollection(t, coll)

	filter := bsonx.Doc{{"x", bsonx.Int32(3)}}

	err := coll.FindOneAndDelete(context.Background(), filter).Err()
	require.NoError(t, err)
}

func TestCollection_FindOneAndDelete_notFound(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	coll := createTestCollection(t, nil, nil)
	initCollection(t, coll)

	filter := bsonx.Doc{{"x", bsonx.Int32(6)}}

	err := coll.FindOneAndDelete(context.Background(), filter).Err()
	require.Equal(t, err, ErrNoDocuments)
}

func TestCollection_FindOneAndDelete_notFound_ignoreResult(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	coll := createTestCollection(t, nil, nil)
	initCollection(t, coll)

	filter := bsonx.Doc{{"x", bsonx.Int32(6)}}

	err := coll.FindOneAndDelete(context.Background(), filter).Err()
	require.Equal(t, ErrNoDocuments, err)
}

func TestCollection_FindOneAndDelete_WriteConcernError(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	if os.Getenv("TOPOLOGY") != "replica_set" {
		t.Skip()
	}

	skipIfBelow32(t)

	coll := createTestCollection(t, nil, nil, options.Collection().SetWriteConcern(impossibleWriteConcern))

	filter := bsonx.Doc{{"x", bsonx.Int32(3)}}

	var result bsonx.Doc
	err := coll.FindOneAndDelete(context.Background(), filter).Decode(&result)
	require.Error(t, err)
	we, ok := err.(WriteException)
	if !ok {
		t.Fatalf("incorrect error type returned: %T", err)
	}
	if we.WriteConcernError == nil {
		t.Fatal("write concern error is nil")
	}
}

func TestCollection_FindOneAndReplace_found(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	coll := createTestCollection(t, nil, nil)
	initCollection(t, coll)

	filter := bsonx.Doc{{"x", bsonx.Int32(3)}}
	replacement := bsonx.Doc{{"y", bsonx.Int32(3)}}

	var result bsonx.Doc
	err := coll.FindOneAndReplace(context.Background(), filter, replacement).Decode(&result)
	require.NoError(t, err)

	elem, err := result.LookupErr("x")
	require.NoError(t, err)
	require.Equal(t, elem.Type(), bson.TypeInt32, "Incorrect BSON Element type")
	require.Equal(t, int(elem.Int32()), 3)
}

func TestCollection_FindOneAndReplace_found_ignoreResult(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	coll := createTestCollection(t, nil, nil)
	initCollection(t, coll)

	filter := bsonx.Doc{{"x", bsonx.Int32(3)}}
	replacement := bsonx.Doc{{"y", bsonx.Int32(3)}}

	err := coll.FindOneAndReplace(context.Background(), filter, replacement).Err()
	require.NoError(t, err)
}

func TestCollection_FindOneAndReplace_notFound(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	coll := createTestCollection(t, nil, nil)
	initCollection(t, coll)

	filter := bsonx.Doc{{"x", bsonx.Int32(6)}}
	replacement := bsonx.Doc{{"y", bsonx.Int32(6)}}

	err := coll.FindOneAndReplace(context.Background(), filter, replacement).Err()
	require.Equal(t, err, ErrNoDocuments)
}

func TestCollection_FindOneAndReplace_notFound_ignoreResult(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	coll := createTestCollection(t, nil, nil)
	initCollection(t, coll)

	filter := bsonx.Doc{{"x", bsonx.Int32(6)}}
	replacement := bsonx.Doc{{"y", bsonx.Int32(6)}}

	err := coll.FindOneAndReplace(context.Background(), filter, replacement).Err()
	require.Equal(t, err, ErrNoDocuments)
}

func TestCollection_FindOneAndReplace_WriteConcernError(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	if os.Getenv("TOPOLOGY") != "replica_set" {
		t.Skip()
	}

	skipIfBelow32(t)

	coll := createTestCollection(t, nil, nil, options.Collection().SetWriteConcern(impossibleWriteConcern))

	filter := bsonx.Doc{{"x", bsonx.Int32(3)}}
	replacement := bsonx.Doc{{"y", bsonx.Int32(3)}}

	var result bsonx.Doc
	err := coll.FindOneAndReplace(context.Background(), filter, replacement).Decode(&result)
	require.Error(t, err)
	we, ok := err.(WriteException)
	if !ok {
		t.Fatalf("incorrect error type returned: %T", we)
	}
	if we.WriteConcernError == nil {
		t.Fatal("expected write concern error but got nil")
	}
}

func TestCollection_FindOneAndUpdate_found(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	coll := createTestCollection(t, nil, nil)
	initCollection(t, coll)

	filter := bsonx.Doc{{"x", bsonx.Int32(3)}}
	update := bsonx.Doc{{"$set", bsonx.Document(bsonx.Doc{{"x", bsonx.Int32(6)}})}}

	var result bsonx.Doc
	err := coll.FindOneAndUpdate(context.Background(), filter, update).Decode(&result)
	require.NoError(t, err)

	elem, err := result.LookupErr("x")
	require.NoError(t, err)
	require.Equal(t, elem.Type(), bson.TypeInt32, "Incorrect BSON Element type")
	require.Equal(t, int(elem.Int32()), 3)
}

func TestCollection_FindOneAndUpdate_EmptyUpdate(t *testing.T) {
	coll := createTestCollection(t, nil, nil)
	res := coll.FindOneAndUpdate(context.Background(), bsonx.Doc{}, bsonx.Doc{})
	require.NotNil(t, res.Err())
}

func TestCollection_FindOneAndUpdate_found_ignoreResult(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	coll := createTestCollection(t, nil, nil)
	initCollection(t, coll)

	filter := bsonx.Doc{{"x", bsonx.Int32(3)}}
	update := bsonx.Doc{{"$set", bsonx.Document(bsonx.Doc{{"x", bsonx.Int32(6)}})}}

	err := coll.FindOneAndUpdate(context.Background(), filter, update).Err()
	require.NoError(t, err)
}

func TestCollection_FindOneAndUpdate_notFound(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	coll := createTestCollection(t, nil, nil)
	initCollection(t, coll)

	filter := bsonx.Doc{{"x", bsonx.Int32(6)}}
	update := bsonx.Doc{{"$set", bsonx.Document(bsonx.Doc{{"x", bsonx.Int32(6)}})}}

	err := coll.FindOneAndUpdate(context.Background(), filter, update).Err()
	require.Equal(t, err, ErrNoDocuments)
}

func TestCollection_FindOneAndUpdate_notFound_ignoreResult(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	coll := createTestCollection(t, nil, nil)
	initCollection(t, coll)

	filter := bsonx.Doc{{"x", bsonx.Int32(6)}}
	update := bsonx.Doc{{"$set", bsonx.Document(bsonx.Doc{{"x", bsonx.Int32(6)}})}}

	err := coll.FindOneAndUpdate(context.Background(), filter, update).Err()
	require.Equal(t, err, ErrNoDocuments)
}

func TestCollection_FindOneAndUpdate_WriteConcernError(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	if os.Getenv("TOPOLOGY") != "replica_set" {
		t.Skip()
	}

	skipIfBelow32(t)

	coll := createTestCollection(t, nil, nil, options.Collection().SetWriteConcern(impossibleWriteConcern))

	filter := bsonx.Doc{{"x", bsonx.Int32(3)}}
	update := bsonx.Doc{{"$set", bsonx.Document(bsonx.Doc{{"x", bsonx.Int32(6)}})}}

	var result bsonx.Doc
	err := coll.FindOneAndUpdate(context.Background(), filter, update).Decode(&result)
	require.Error(t, err)

	we, ok := err.(WriteException)
	if !ok {
		t.Fatalf("incorrect error type returned: %T", we)
	}
	if we.WriteConcernError == nil {
		t.Fatal("write concern error expected but got nil")
	}
}

func TestCollection_BulkWrite(t *testing.T) {
	t.Run("TestWriteConcernError", func(t *testing.T) {
		if os.Getenv("TOPOLOGY") != "replica_set" {
			t.Skip()
		}

		filter := bson.D{{"foo", "bar"}}
		update := bson.D{{"$set", bson.D{{"foo", 10}}}}

		testCases := []struct {
			name   string
			models []WriteModel
		}{
			{"insert", []WriteModel{NewInsertOneModel().SetDocument(bson.D{{"foo", 1}})}},
			{"update", []WriteModel{NewUpdateOneModel().SetFilter(filter).SetUpdate(update)}},
			{"delete", []WriteModel{NewDeleteOneModel().SetFilter(filter)}},
		}

		coll := createTestCollection(t, nil, nil, options.Collection().SetWriteConcern(impossibleWriteConcern))
		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				_, err := coll.BulkWrite(ctx, tc.models)
				require.Error(t, err)

				bwException, ok := err.(BulkWriteException)
				require.True(t, ok)

				require.Equal(t, 0, len(bwException.WriteErrors),
					"expected no write errors, got %v", bwException.WriteErrors)
				require.NotNil(t, bwException.WriteConcernError)
			})
		}
	})
}

// test special types that should be converted to a document for updates even though the underlying type is a
// slice/array
func TestCollection_Update_SpecialSliceTypes(t *testing.T) {
	doc := bson.D{{"$set", bson.D{{"x", 2}}}}
	docBytes, err := bson.Marshal(doc)
	require.NoError(t, err, "error getting document bytes: %v", err)
	xUpdate := bsonx.Doc{{"x", bsonx.Int32(2)}}
	xDoc := bsonx.Doc{{"$set", bsonx.Document(xUpdate)}}

	testCases := []struct {
		name   string
		update interface{}
	}{
		{"bsoncore Document", bsoncore.Document(docBytes)},
		{"bson Raw", bson.Raw(docBytes)},
		{"bson D", doc},
		{"bsonx Document", xDoc},
		{"byte slice", docBytes},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			coll := createTestCollection(t, nil, &tc.name)
			defer func() {
				_ = coll.Drop(ctx)
			}()

			insertedDoc := bson.D{{"x", 1}}
			_, err = coll.InsertOne(ctx, insertedDoc)
			require.NoError(t, err, "error inserting document: %v", err)

			res, err := coll.UpdateOne(ctx, insertedDoc, tc.update)
			require.NoError(t, err, "error updating document: %v", err)
			require.Equal(t, int64(1), res.MatchedCount,
				"matched count mismatch; expected %d, got %d", 1, res.MatchedCount)
			require.Equal(t, int64(1), res.ModifiedCount,
				"modified count mismatch; expected %d, got %d", 1, res.ModifiedCount)
		})
	}
}
