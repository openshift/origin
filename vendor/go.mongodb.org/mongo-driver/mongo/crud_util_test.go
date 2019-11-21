// Copyright (C) MongoDB, Inc. 2017-present.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may
// not use this file except in compliance with the License. You may obtain
// a copy of the License at http://www.apache.org/licenses/LICENSE-2.0

package mongo

import (
	"bytes"
	"context"
	"encoding/json"
	"math"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/internal/testutil/helpers"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.mongodb.org/mongo-driver/mongo/readconcern"
	"go.mongodb.org/mongo-driver/mongo/writeconcern"
	"go.mongodb.org/mongo-driver/x/bsonx"
)

type cursor interface {
	Err() error
	Next(context.Context) bool
	Decode(interface{}) error
}

// Various helper functions for crud related operations

// Mutates the client to add options
func addClientOptions(c *Client, opts map[string]interface{}) {
	for name, opt := range opts {
		switch name {
		case "retryWrites":
			c.retryWrites = opt.(bool)
		case "w":
			switch opt.(type) {
			case float64:
				c.writeConcern = writeconcern.New(writeconcern.W(int(opt.(float64))))
			case string:
				c.writeConcern = writeconcern.New(writeconcern.WMajority())
			}
		case "readConcernLevel":
			c.readConcern = readconcern.New(readconcern.Level(opt.(string)))
		case "readPreference":
			c.readPreference = readPrefFromString(opt.(string))
		}
	}
}

// Mutates the collection to add options
func addCollectionOptions(c *Collection, opts map[string]interface{}) {
	for name, opt := range opts {
		switch name {
		case "readConcern":
			c.readConcern = getReadConcern(opt)
		case "writeConcern":
			c.writeConcern = getWriteConcern(opt)
		case "readPreference":
			c.readPreference = readPrefFromString(opt.(map[string]interface{})["mode"].(string))
		}
	}
}

func executeCount(sess *sessionImpl, coll *Collection, args map[string]interface{}) (int64, error) {
	var filter map[string]interface{}
	opts := options.Count()
	for name, opt := range args {
		switch name {
		case "filter":
			filter = opt.(map[string]interface{})
		case "skip":
			opts = opts.SetSkip(int64(opt.(float64)))
		case "limit":
			opts = opts.SetLimit(int64(opt.(float64)))
		case "collation":
			opts = opts.SetCollation(collationFromMap(opt.(map[string]interface{})))
		}
	}

	if sess != nil {
		// EXAMPLE:
		sessCtx := sessionContext{
			Context: context.WithValue(ctx, sessionKey{}, sess),
			Session: sess,
		}
		return coll.CountDocuments(sessCtx, filter, opts)
	}
	return coll.CountDocuments(ctx, filter, opts)
}

func executeCountDocuments(sess *sessionImpl, coll *Collection, args map[string]interface{}) (int64, error) {
	var filter map[string]interface{}
	opts := options.Count()
	for name, opt := range args {
		switch name {
		case "filter":
			filter = opt.(map[string]interface{})
		case "skip":
			opts = opts.SetSkip(int64(opt.(float64)))
		case "limit":
			opts = opts.SetLimit(int64(opt.(float64)))
		case "collation":
			opts = opts.SetCollation(collationFromMap(opt.(map[string]interface{})))
		}
	}

	if sess != nil {
		// EXAMPLE:
		sessCtx := sessionContext{
			Context: context.WithValue(ctx, sessionKey{}, sess),
			Session: sess,
		}
		return coll.CountDocuments(sessCtx, filter, opts)
	}
	return coll.CountDocuments(ctx, filter, opts)
}

func executeDistinct(sess *sessionImpl, coll *Collection, args map[string]interface{}) ([]interface{}, error) {
	var fieldName string
	var filter map[string]interface{}
	opts := options.Distinct()
	for name, opt := range args {
		switch name {
		case "filter":
			filter = opt.(map[string]interface{})
		case "fieldName":
			fieldName = opt.(string)
		case "collation":
			opts = opts.SetCollation(collationFromMap(opt.(map[string]interface{})))
		}
	}

	if sess != nil {
		sessCtx := sessionContext{
			Context: context.WithValue(ctx, sessionKey{}, sess),
			Session: sess,
		}
		return coll.Distinct(sessCtx, fieldName, filter, opts)
	}
	return coll.Distinct(ctx, fieldName, filter, opts)
}

func executeInsertOne(sess *sessionImpl, coll *Collection, args map[string]interface{}) (*InsertOneResult, error) {
	document := args["document"].(map[string]interface{})

	// For some reason, the insertion document is unmarshaled with a float rather than integer,
	// but the documents that are used to initially populate the collection are unmarshaled
	// correctly with integers. To ensure that the tests can correctly compare them, we iterate
	// through the insertion document and change any valid integers stored as floats to actual
	// integers.
	replaceFloatsWithInts(document)

	if sess != nil {
		sessCtx := sessionContext{
			Context: context.WithValue(ctx, sessionKey{}, sess),
			Session: sess,
		}
		return coll.InsertOne(sessCtx, document)
	}
	return coll.InsertOne(context.Background(), document)
}

func executeInsertMany(sess *sessionImpl, coll *Collection, args map[string]interface{}) (*InsertManyResult, error) {
	documents := args["documents"].([]interface{})

	// For some reason, the insertion documents are unmarshaled with a float rather than
	// integer, but the documents that are used to initially populate the collection are
	// unmarshaled correctly with integers. To ensure that the tests can correctly compare
	// them, we iterate through the insertion documents and change any valid integers stored
	// as floats to actual integers.
	for i, doc := range documents {
		docM := doc.(map[string]interface{})
		replaceFloatsWithInts(docM)

		documents[i] = docM
	}

	if sess != nil {
		sessCtx := sessionContext{
			Context: context.WithValue(ctx, sessionKey{}, sess),
			Session: sess,
		}
		return coll.InsertMany(sessCtx, documents)
	}
	return coll.InsertMany(context.Background(), documents)
}

func executeFind(sess *sessionImpl, coll *Collection, args map[string]interface{}) (*Cursor, error) {
	opts := options.Find()
	var filter map[string]interface{}
	for name, opt := range args {
		switch name {
		case "filter":
			filter = opt.(map[string]interface{})
		case "sort":
			opts = opts.SetSort(opt.(map[string]interface{}))
		case "skip":
			opts = opts.SetSkip(int64(opt.(float64)))
		case "limit":
			opts = opts.SetLimit(int64(opt.(float64)))
		case "batchSize":
			opts = opts.SetBatchSize(int32(opt.(float64)))
		case "collation":
			opts = opts.SetCollation(collationFromMap(opt.(map[string]interface{})))
		}
	}

	if sess != nil {
		sessCtx := sessionContext{
			Context: context.WithValue(ctx, sessionKey{}, sess),
			Session: sess,
		}
		return coll.Find(sessCtx, filter, opts)
	}
	return coll.Find(ctx, filter, opts)
}

func executeFindOneAndDelete(sess *sessionImpl, coll *Collection, args map[string]interface{}) *SingleResult {
	opts := options.FindOneAndDelete()
	var filter map[string]interface{}
	for name, opt := range args {
		switch name {
		case "filter":
			filter = opt.(map[string]interface{})
		case "sort":
			opts = opts.SetSort(opt.(map[string]interface{}))
		case "projection":
			opts = opts.SetProjection(opt.(map[string]interface{}))
		case "collation":
			opts = opts.SetCollation(collationFromMap(opt.(map[string]interface{})))
		}
	}

	if sess != nil {
		sessCtx := sessionContext{
			Context: context.WithValue(ctx, sessionKey{}, sess),
			Session: sess,
		}
		return coll.FindOneAndDelete(sessCtx, filter, opts)
	}
	return coll.FindOneAndDelete(ctx, filter, opts)
}

func executeFindOneAndUpdate(sess *sessionImpl, coll *Collection, args map[string]interface{}) *SingleResult {
	opts := options.FindOneAndUpdate()
	var filter map[string]interface{}
	var update map[string]interface{}
	var updatePipe []interface{}
	var ok bool
	for name, opt := range args {
		switch name {
		case "filter":
			filter = opt.(map[string]interface{})
		case "update":
			update, ok = opt.(map[string]interface{})
			if !ok {
				updatePipe = opt.([]interface{})
			}
		case "arrayFilters":
			opts = opts.SetArrayFilters(options.ArrayFilters{
				Filters: opt.([]interface{}),
			})
		case "sort":
			opts = opts.SetSort(opt.(map[string]interface{}))
		case "projection":
			opts = opts.SetProjection(opt.(map[string]interface{}))
		case "upsert":
			opts = opts.SetUpsert(opt.(bool))
		case "returnDocument":
			switch opt.(string) {
			case "After":
				opts = opts.SetReturnDocument(options.After)
			case "Before":
				opts = opts.SetReturnDocument(options.Before)
			}
		case "collation":
			opts = opts.SetCollation(collationFromMap(opt.(map[string]interface{})))
		}
	}

	// For some reason, the filter and update documents are unmarshaled with floats
	// rather than integers, but the documents that are used to initially populate the
	// collection are unmarshaled correctly with integers. To ensure that the tests can
	// correctly compare them, we iterate through the filter and replacement documents and
	// change any valid integers stored as floats to actual integers.
	replaceFloatsWithInts(filter)
	replaceFloatsWithInts(update)

	if sess != nil {
		sessCtx := sessionContext{
			Context: context.WithValue(ctx, sessionKey{}, sess),
			Session: sess,
		}
		if updatePipe != nil {
			return coll.FindOneAndUpdate(sessCtx, filter, updatePipe, opts)
		}
		return coll.FindOneAndUpdate(sessCtx, filter, update, opts)
	}
	if updatePipe != nil {
		return coll.FindOneAndUpdate(ctx, filter, updatePipe, opts)
	}
	return coll.FindOneAndUpdate(ctx, filter, update, opts)
}

func executeFindOneAndReplace(sess *sessionImpl, coll *Collection, args map[string]interface{}) *SingleResult {
	opts := options.FindOneAndReplace()
	var filter map[string]interface{}
	var replacement map[string]interface{}
	for name, opt := range args {
		switch name {
		case "filter":
			filter = opt.(map[string]interface{})
		case "replacement":
			replacement = opt.(map[string]interface{})
		case "sort":
			opts = opts.SetSort(opt.(map[string]interface{}))
		case "projection":
			opts = opts.SetProjection(opt.(map[string]interface{}))
		case "upsert":
			opts = opts.SetUpsert(opt.(bool))
		case "returnDocument":
			switch opt.(string) {
			case "After":
				opts = opts.SetReturnDocument(options.After)
			case "Before":
				opts = opts.SetReturnDocument(options.Before)
			}
		case "collation":
			opts = opts.SetCollation(collationFromMap(opt.(map[string]interface{})))
		}
	}

	// For some reason, the filter and replacement documents are unmarshaled with floats
	// rather than integers, but the documents that are used to initially populate the
	// collection are unmarshaled correctly with integers. To ensure that the tests can
	// correctly compare them, we iterate through the filter and replacement documents and
	// change any valid integers stored as floats to actual integers.
	replaceFloatsWithInts(filter)
	replaceFloatsWithInts(replacement)

	if sess != nil {
		sessCtx := sessionContext{
			Context: context.WithValue(ctx, sessionKey{}, sess),
			Session: sess,
		}
		return coll.FindOneAndReplace(sessCtx, filter, replacement, opts)
	}
	return coll.FindOneAndReplace(ctx, filter, replacement, opts)
}

func executeDeleteOne(sess *sessionImpl, coll *Collection, args map[string]interface{}) (*DeleteResult, error) {
	opts := options.Delete()
	var filter map[string]interface{}
	for name, opt := range args {
		switch name {
		case "filter":
			filter = opt.(map[string]interface{})
		case "collation":
			opts = opts.SetCollation(collationFromMap(opt.(map[string]interface{})))
		}
	}

	// For some reason, the filter document is unmarshaled with floats
	// rather than integers, but the documents that are used to initially populate the
	// collection are unmarshaled correctly with integers. To ensure that the tests can
	// correctly compare them, we iterate through the filter and replacement documents and
	// change any valid integers stored as floats to actual integers.
	replaceFloatsWithInts(filter)

	if sess != nil {
		sessCtx := sessionContext{
			Context: context.WithValue(ctx, sessionKey{}, sess),
			Session: sess,
		}
		return coll.DeleteOne(sessCtx, filter, opts)
	}
	return coll.DeleteOne(ctx, filter, opts)
}

func executeDeleteMany(sess *sessionImpl, coll *Collection, args map[string]interface{}) (*DeleteResult, error) {
	opts := options.Delete()
	var filter map[string]interface{}
	for name, opt := range args {
		switch name {
		case "filter":
			filter = opt.(map[string]interface{})
		case "collation":
			opts = opts.SetCollation(collationFromMap(opt.(map[string]interface{})))
		}
	}

	// For some reason, the filter document is unmarshaled with floats
	// rather than integers, but the documents that are used to initially populate the
	// collection are unmarshaled correctly with integers. To ensure that the tests can
	// correctly compare them, we iterate through the filter and replacement documents and
	// change any valid integers stored as floats to actual integers.
	replaceFloatsWithInts(filter)

	if sess != nil {
		sessCtx := sessionContext{
			Context: context.WithValue(ctx, sessionKey{}, sess),
			Session: sess,
		}
		return coll.DeleteMany(sessCtx, filter, opts)
	}
	return coll.DeleteMany(ctx, filter, opts)
}

func executeReplaceOne(sess *sessionImpl, coll *Collection, args map[string]interface{}) (*UpdateResult, error) {
	opts := options.Replace()
	var filter map[string]interface{}
	var replacement map[string]interface{}
	for name, opt := range args {
		switch name {
		case "filter":
			filter = opt.(map[string]interface{})
		case "replacement":
			replacement = opt.(map[string]interface{})
		case "upsert":
			opts = opts.SetUpsert(opt.(bool))
		case "collation":
			opts = opts.SetCollation(collationFromMap(opt.(map[string]interface{})))
		}
	}

	// For some reason, the filter and replacement documents are unmarshaled with floats
	// rather than integers, but the documents that are used to initially populate the
	// collection are unmarshaled correctly with integers. To ensure that the tests can
	// correctly compare them, we iterate through the filter and replacement documents and
	// change any valid integers stored as floats to actual integers.
	replaceFloatsWithInts(filter)
	replaceFloatsWithInts(replacement)

	// TODO temporarily default upsert to false explicitly to make test pass
	// because we do not send upsert=false by default
	//opts = opts.SetUpsert(false)
	if opts.Upsert == nil {
		opts = opts.SetUpsert(false)
	}
	if sess != nil {
		sessCtx := sessionContext{
			Context: context.WithValue(ctx, sessionKey{}, sess),
			Session: sess,
		}
		return coll.ReplaceOne(sessCtx, filter, replacement, opts)
	}
	return coll.ReplaceOne(ctx, filter, replacement, opts)
}

func executeUpdateOne(sess *sessionImpl, coll *Collection, args map[string]interface{}) (*UpdateResult, error) {
	opts := options.Update()
	var filter map[string]interface{}
	var update map[string]interface{}
	var updatePipe []interface{}
	var ok bool
	for name, opt := range args {
		switch name {
		case "filter":
			filter = opt.(map[string]interface{})
		case "update":
			update, ok = opt.(map[string]interface{})
			if !ok {
				updatePipe = opt.([]interface{})
			}
		case "arrayFilters":
			opts = opts.SetArrayFilters(options.ArrayFilters{Filters: opt.([]interface{})})
		case "upsert":
			opts = opts.SetUpsert(opt.(bool))
		case "collation":
			opts = opts.SetCollation(collationFromMap(opt.(map[string]interface{})))
		}
	}

	// For some reason, the filter and update documents are unmarshaled with floats
	// rather than integers, but the documents that are used to initially populate the
	// collection are unmarshaled correctly with integers. To ensure that the tests can
	// correctly compare them, we iterate through the filter and replacement documents and
	// change any valid integers stored as floats to actual integers.
	replaceFloatsWithInts(filter)
	replaceFloatsWithInts(update)

	if sess != nil {
		sessCtx := sessionContext{
			Context: context.WithValue(ctx, sessionKey{}, sess),
			Session: sess,
		}
		if updatePipe != nil {
			return coll.UpdateOne(sessCtx, filter, updatePipe, opts)
		}
		return coll.UpdateOne(sessCtx, filter, update, opts)
	}

	if updatePipe != nil {
		return coll.UpdateOne(ctx, filter, updatePipe, opts)
	}
	return coll.UpdateOne(ctx, filter, update, opts)
}

func executeUpdateMany(sess *sessionImpl, coll *Collection, args map[string]interface{}) (*UpdateResult, error) {
	opts := options.Update()
	var filter map[string]interface{}
	var update map[string]interface{}
	var updatePipe []interface{}
	var ok bool
	for name, opt := range args {
		switch name {
		case "filter":
			filter = opt.(map[string]interface{})
		case "update":
			update, ok = opt.(map[string]interface{})
			if !ok {
				updatePipe = opt.([]interface{})
			}
		case "arrayFilters":
			opts = opts.SetArrayFilters(options.ArrayFilters{Filters: opt.([]interface{})})
		case "upsert":
			opts = opts.SetUpsert(opt.(bool))
		case "collation":
			opts = opts.SetCollation(collationFromMap(opt.(map[string]interface{})))
		}
	}

	// For some reason, the filter and update documents are unmarshaled with floats
	// rather than integers, but the documents that are used to initially populate the
	// collection are unmarshaled correctly with integers. To ensure that the tests can
	// correctly compare them, we iterate through the filter and replacement documents and
	// change any valid integers stored as floats to actual integers.
	replaceFloatsWithInts(filter)
	replaceFloatsWithInts(update)

	if sess != nil {
		sessCtx := sessionContext{
			Context: context.WithValue(ctx, sessionKey{}, sess),
			Session: sess,
		}
		if updatePipe != nil {
			return coll.UpdateMany(sessCtx, filter, updatePipe, opts)
		}
		return coll.UpdateMany(sessCtx, filter, update, opts)
	}
	if updatePipe != nil {
		return coll.UpdateMany(ctx, filter, updatePipe, opts)
	}
	return coll.UpdateMany(ctx, filter, update, opts)
}

func executeAggregate(sess *sessionImpl, coll *Collection, args map[string]interface{}) (*Cursor, error) {
	var pipeline []interface{}
	opts := options.Aggregate()
	for name, opt := range args {
		switch name {
		case "pipeline":
			pipeline = opt.([]interface{})
		case "batchSize":
			opts = opts.SetBatchSize(int32(opt.(float64)))
		case "collation":
			opts = opts.SetCollation(collationFromMap(opt.(map[string]interface{})))
		case "maxTimeMS":
			opts = opts.SetMaxTime(time.Duration(opt.(float64)) * time.Millisecond)
		}
	}

	if sess != nil {
		sessCtx := sessionContext{
			Context: context.WithValue(ctx, sessionKey{}, sess),
			Session: sess,
		}
		return coll.Aggregate(sessCtx, pipeline, opts)
	}
	return coll.Aggregate(ctx, pipeline, opts)
}

func executeWithTransaction(t *testing.T, sess *sessionImpl, collName string, db *Database, args json.RawMessage) error {
	expectedBytes, err := args.MarshalJSON()
	if err != nil {
		return err
	}

	var testArgs withTransactionArgs
	err = json.Unmarshal(expectedBytes, &testArgs)
	if err != nil {
		return err
	}
	opts := getTransactionOptions(testArgs.Options)

	_, err = sess.WithTransaction(context.Background(), func(sessCtx SessionContext) (interface{}, error) {
		err := runWithTransactionOperations(t, testArgs.Callback.Operations, sess, collName, db)
		return nil, err
	}, opts)
	return err
}

func executeRenameCollection(sess Session, coll *Collection, argmap map[string]interface{}) *SingleResult {
	to := argmap["to"].(string)

	cmd := bson.D{
		{"renameCollection", strings.Join([]string{coll.db.name, coll.name}, ".")},
		{"to", strings.Join([]string{coll.db.name, to}, ".")},
	}

	admin := coll.db.client.Database("admin")
	if sess != nil {
		sessCtx := sessionContext{
			Context: context.WithValue(ctx, sessionKey{}, sess),
			Session: sess,
		}
		return admin.RunCommand(sessCtx, cmd)
	}

	return admin.RunCommand(ctx, cmd)
}

func executeRunCommand(sess Session, db *Database, argmap map[string]interface{}, args json.RawMessage) *SingleResult {
	var cmd bsonx.Doc
	opts := options.RunCmd()
	for name, opt := range argmap {
		switch name {
		case "command":
			argBytes, err := args.MarshalJSON()
			if err != nil {
				return &SingleResult{err: err}
			}

			var argCmdStruct struct {
				Cmd json.RawMessage `json:"command"`
			}
			err = json.NewDecoder(bytes.NewBuffer(argBytes)).Decode(&argCmdStruct)
			if err != nil {
				return &SingleResult{err: err}
			}

			err = bson.UnmarshalExtJSON(argCmdStruct.Cmd, true, &cmd)
			if err != nil {
				return &SingleResult{err: err}
			}
		case "readPreference":
			opts = opts.SetReadPreference(getReadPref(opt))
		}
	}

	if sess != nil {
		sessCtx := sessionContext{
			Context: context.WithValue(ctx, sessionKey{}, sess),
			Session: sess,
		}
		return db.RunCommand(sessCtx, cmd, opts)
	}
	return db.RunCommand(ctx, cmd, opts)
}

func verifyBulkWriteResult(t *testing.T, res *BulkWriteResult, result json.RawMessage) {
	expectedBytes, err := result.MarshalJSON()
	require.NoError(t, err)

	var expected BulkWriteResult
	err = json.NewDecoder(bytes.NewBuffer(expectedBytes)).Decode(&expected)
	require.NoError(t, err)

	require.Equal(t, expected.DeletedCount, res.DeletedCount)
	require.Equal(t, expected.InsertedCount, res.InsertedCount)
	require.Equal(t, expected.MatchedCount, res.MatchedCount)
	require.Equal(t, expected.ModifiedCount, res.ModifiedCount)
	require.Equal(t, expected.UpsertedCount, res.UpsertedCount)

	// replace floats with ints
	for opID, upsertID := range expected.UpsertedIDs {
		if floatID, ok := upsertID.(float64); ok {
			expected.UpsertedIDs[opID] = int32(floatID)
		}
	}

	for operationID, upsertID := range expected.UpsertedIDs {
		require.Equal(t, upsertID, res.UpsertedIDs[operationID])
	}
}

func verifyInsertOneResult(t *testing.T, res *InsertOneResult, result json.RawMessage) {
	expectedBytes, err := result.MarshalJSON()
	require.NoError(t, err)

	var expected InsertOneResult
	err = json.NewDecoder(bytes.NewBuffer(expectedBytes)).Decode(&expected)
	require.NoError(t, err)

	expectedID := expected.InsertedID
	if f, ok := expectedID.(float64); ok && f == math.Floor(f) {
		expectedID = int32(f)
	}

	if expectedID != nil {
		require.NotNil(t, res)
		require.Equal(t, expectedID, res.InsertedID)
	}
}

func verifyInsertManyResult(t *testing.T, res *InsertManyResult, result json.RawMessage) {
	expectedBytes, err := result.MarshalJSON()
	require.NoError(t, err)

	var expected struct{ InsertedIds map[string]interface{} }
	err = json.NewDecoder(bytes.NewBuffer(expectedBytes)).Decode(&expected)
	require.NoError(t, err)

	if expected.InsertedIds != nil {
		require.NotNil(t, res)
		replaceFloatsWithInts(expected.InsertedIds)

		for _, val := range expected.InsertedIds {
			require.Contains(t, res.InsertedIDs, val)
		}
	}
}

func verifyCursorResult(t *testing.T, cur cursor, result json.RawMessage) {
	for _, expected := range docSliceFromRaw(t, result) {
		require.NotNil(t, cur)
		require.True(t, cur.Next(context.Background()))

		var actual bsonx.Doc
		require.NoError(t, cur.Decode(&actual))

		compareDocs(t, expected, actual)
	}

	require.False(t, cur.Next(ctx))
	require.NoError(t, cur.Err())
}

func verifySingleResult(t *testing.T, res *SingleResult, result json.RawMessage) {
	jsonBytes, err := result.MarshalJSON()
	require.NoError(t, err)

	var actual bsonx.Doc
	err = res.Decode(&actual)
	if err == ErrNoDocuments {
		var expected map[string]interface{}
		err := json.NewDecoder(bytes.NewBuffer(jsonBytes)).Decode(&expected)
		require.NoError(t, err)

		require.Nil(t, expected)
		return
	}

	require.NoError(t, err)

	doc := bsonx.Doc{}
	err = bson.UnmarshalExtJSON(jsonBytes, true, &doc)
	require.NoError(t, err)

	require.True(t, doc.Equal(actual))
}

func verifyDistinctResult(t *testing.T, res []interface{}, result json.RawMessage) {
	resultBytes, err := result.MarshalJSON()
	require.NoError(t, err)

	var expected []interface{}
	require.NoError(t, json.NewDecoder(bytes.NewBuffer(resultBytes)).Decode(&expected))

	require.Equal(t, len(expected), len(res))

	for i := range expected {
		expectedElem := expected[i]
		actualElem := res[i]

		iExpected := testhelpers.GetIntFromInterface(expectedElem)
		iActual := testhelpers.GetIntFromInterface(actualElem)

		require.Equal(t, iExpected == nil, iActual == nil)
		if iExpected != nil {
			require.Equal(t, *iExpected, *iActual)
			continue
		}

		require.Equal(t, expected[i], res[i])
	}
}

func verifyDeleteResult(t *testing.T, res *DeleteResult, result json.RawMessage) {
	expectedBytes, err := result.MarshalJSON()
	require.NoError(t, err)

	var expected DeleteResult
	err = json.NewDecoder(bytes.NewBuffer(expectedBytes)).Decode(&expected)
	require.NoError(t, err)

	require.Equal(t, expected.DeletedCount, res.DeletedCount)
}

func verifyUpdateResult(t *testing.T, res *UpdateResult, result json.RawMessage) {
	expectedBytes, err := result.MarshalJSON()
	require.NoError(t, err)

	var expected struct {
		MatchedCount  int64
		ModifiedCount int64
		UpsertedCount int64
	}
	err = json.NewDecoder(bytes.NewBuffer(expectedBytes)).Decode(&expected)

	require.Equal(t, expected.MatchedCount, res.MatchedCount)
	require.Equal(t, expected.ModifiedCount, res.ModifiedCount)

	actualUpsertedCount := int64(0)
	if res.UpsertedID != nil {
		actualUpsertedCount = 1
	}

	require.Equal(t, expected.UpsertedCount, actualUpsertedCount)
}

func verifyRunCommandResult(t *testing.T, res bson.Raw, result json.RawMessage) {
	if len(result) == 0 {
		return
	}
	jsonBytes, err := result.MarshalJSON()
	require.NoError(t, err)

	expected := bsonx.Doc{}
	err = bson.UnmarshalExtJSON(jsonBytes, true, &expected)
	require.NoError(t, err)

	require.NotNil(t, res)
	actual, err := bsonx.ReadDoc(res)
	require.NoError(t, err)

	// All runcommand results in tests are for key "n" only
	compareElements(t, expected.LookupElement("n"), actual.LookupElement("n"))
}

func verifyCollectionContents(t *testing.T, coll *Collection, result json.RawMessage) {
	cursor, err := coll.Find(context.Background(), bsonx.Doc{})
	require.NoError(t, err)

	verifyCursorResult(t, cursor, result)
}

func sanitizeCollectionName(kind string, name string) string {
	// Collections can't have "$" in their names, so we substitute it with "%".
	name = strings.Replace(name, "$", "%", -1)

	// must have enough room for kind + "." + one character of name, can't have collection name end in .
	if len(kind) > 118 {
		kind = kind[:118]
	}

	// Namespaces can only have 120 bytes max.
	if len(kind+"."+name) >= 119 {
		name = name[:119-len(kind+".")]
	}

	return name
}

func compareElements(t *testing.T, expected bsonx.Elem, actual bsonx.Elem) {
	if expected.Value.IsNumber() {
		if expectedNum, ok := expected.Value.Int64OK(); ok {
			switch actual.Value.Type() {
			case bson.TypeInt32:
				require.Equal(t, expectedNum, int64(actual.Value.Int32()), "For key %v", expected.Key)
			case bson.TypeInt64:
				require.Equal(t, expectedNum, actual.Value.Int64(), "For key %v\n", expected.Key)
			case bson.TypeDouble:
				require.Equal(t, expectedNum, int64(actual.Value.Double()), "For key %v\n", expected.Key)
			}
		} else {
			expectedNum := expected.Value.Int32()
			switch actual.Value.Type() {
			case bson.TypeInt32:
				require.Equal(t, expectedNum, actual.Value.Int32(), "For key %v", expected.Key)
			case bson.TypeInt64:
				require.Equal(t, expectedNum, int32(actual.Value.Int64()), "For key %v\n", expected.Key)
			case bson.TypeDouble:
				require.Equal(t, expectedNum, int32(actual.Value.Double()), "For key %v\n", expected.Key)
			}
		}
	} else if conv, ok := expected.Value.DocumentOK(); ok {
		actualConv, actualOk := actual.Value.DocumentOK()
		require.True(t, actualOk)
		compareDocs(t, conv, actualConv)
	} else if conv, ok := expected.Value.ArrayOK(); ok {
		actualConv, actualOk := actual.Value.ArrayOK()
		require.True(t, actualOk)
		compareArrays(t, conv, actualConv)
	} else {
		require.True(t, actual.Equal(expected), "For key %s, expected %v\nactual: %v", expected.Key, expected, actual)
	}
}

func compareArrays(t *testing.T, expected bsonx.Arr, actual bsonx.Arr) {
	if len(expected) != len(actual) {
		t.Errorf("array length mismatch. expected %d got %d", len(expected), len(actual))
		t.FailNow()
	}

	for idx := range expected {
		expectedDoc := expected[idx].Document()
		actualDoc := actual[idx].Document()
		compareDocs(t, expectedDoc, actualDoc)
	}
}

func collationFromMap(m map[string]interface{}) *options.Collation {
	var collation options.Collation

	if locale, found := m["locale"]; found {
		collation.Locale = locale.(string)
	}

	if caseLevel, found := m["caseLevel"]; found {
		collation.CaseLevel = caseLevel.(bool)
	}

	if caseFirst, found := m["caseFirst"]; found {
		collation.CaseFirst = caseFirst.(string)
	}

	if strength, found := m["strength"]; found {
		collation.Strength = int(strength.(float64))
	}

	if numericOrdering, found := m["numericOrdering"]; found {
		collation.NumericOrdering = numericOrdering.(bool)
	}

	if alternate, found := m["alternate"]; found {
		collation.Alternate = alternate.(string)
	}

	if maxVariable, found := m["maxVariable"]; found {
		collation.MaxVariable = maxVariable.(string)
	}

	if normalization, found := m["normalization"]; found {
		collation.Normalization = normalization.(bool)
	}

	if backwards, found := m["backwards"]; found {
		collation.Backwards = backwards.(bool)
	}

	return &collation
}

func docSliceFromRaw(t *testing.T, raw json.RawMessage) []bsonx.Doc {
	jsonBytes, err := raw.MarshalJSON()
	require.NoError(t, err)

	array := bsonx.Arr{}
	err = bson.UnmarshalExtJSON(jsonBytes, true, &array)
	require.NoError(t, err)

	docs := make([]bsonx.Doc, 0)

	for _, val := range array {
		docs = append(docs, val.Document())
	}

	return docs
}

func docSliceToInterfaceSlice(docs []bsonx.Doc) []interface{} {
	out := make([]interface{}, 0, len(docs))

	for _, doc := range docs {
		out = append(out, doc)
	}

	return out
}

func replaceFloatsWithInts(m map[string]interface{}) {
	for key, val := range m {
		if f, ok := val.(float64); ok && f == math.Floor(f) {
			m[key] = int32(f)
			continue
		}

		if innerM, ok := val.(map[string]interface{}); ok {
			replaceFloatsWithInts(innerM)
			m[key] = innerM
		}
	}
}

func shouldSkip(t *testing.T, minVersion string, maxVersion string, db *Database) bool {
	versionStr, err := getServerVersion(db)
	require.NoError(t, err)

	if len(minVersion) > 0 && compareVersions(t, minVersion, versionStr) > 0 {
		return true
	}

	if len(maxVersion) > 0 && compareVersions(t, maxVersion, versionStr) < 0 {
		return true
	}

	return false
}

func canRunOn(t *testing.T, runOn runOn, dbAdmin *Database) bool {
	if shouldSkip(t, runOn.MinServerVersion, runOn.MaxServerVersion, dbAdmin) {
		return false
	}

	if runOn.Topology == nil || len(runOn.Topology) == 0 {
		return true
	}

	for _, top := range runOn.Topology {
		if os.Getenv("TOPOLOGY") == runOnTopologyToEnvTopology(t, top) {
			return true
		}
	}
	return false
}

func runOnTopologyToEnvTopology(t *testing.T, runOnTopology string) string {
	switch runOnTopology {
	case "single":
		return "server"
	case "replicaset":
		return "replica_set"
	case "sharded":
		return "sharded_cluster"
	default:
		t.Fatalf("unknown topology %v", runOnTopology)
	}
	return ""
}

func skipIfNecessaryRunOnSlice(t *testing.T, runOns []runOn, dbAdmin *Database) {
	for _, runOn := range runOns {
		if canRunOn(t, runOn, dbAdmin) {
			return
		}
	}
	versionStr, err := getServerVersion(dbAdmin)
	require.NoError(t, err, "unable to run on current server version, topology combination, error getting server version")
	t.Skipf("unable to run on %v %v", os.Getenv("TOPOLOGY"), versionStr)
}
