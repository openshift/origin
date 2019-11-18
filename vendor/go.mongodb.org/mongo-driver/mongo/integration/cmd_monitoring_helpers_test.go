// Copyright (C) MongoDB, Inc. 2017-present.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may
// not use this file except in compliance with the License. You may obtain
// a copy of the License at http://www.apache.org/licenses/LICENSE-2.0

package integration

import (
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/bsontype"
	"go.mongodb.org/mongo-driver/internal/testutil/assert"
	"go.mongodb.org/mongo-driver/mongo/integration/mtest"
	"go.mongodb.org/mongo-driver/x/bsonx"
)

// Helper functions to compare BSON values and command monitoring expectations.

func numberFromValue(mt *mtest.T, val bson.RawValue) int64 {
	switch val.Type {
	case bson.TypeInt32:
		return int64(val.Int32())
	case bson.TypeInt64:
		return val.Int64()
	case bson.TypeDouble:
		return int64(val.Double())
	default:
		mt.Fatalf("unexpected type for number: %v", val.Type)
	}

	return 0
}

func compareNumberValues(mt *mtest.T, key string, expected, actual bson.RawValue) {
	eInt := numberFromValue(mt, expected)
	if eInt == 42 {
		assert.NotEqual(mt, bson.TypeNull, actual.Type, "expected non-null value for key %v, got null", key)
		return
	}

	aInt := numberFromValue(mt, actual)
	assert.Equal(mt, eInt, aInt, "value mismatch for key %s; expected %v, got %v", key, expected, actual)
}

// compare BSON values and fail if they are not equal. the key parameter is used for error strings.
// if the expected value is a numeric type (int32, int64, or double) and the value is 42, the function only asserts that
// the actual value is non-null.
func compareValues(mt *mtest.T, key string, expected, actual bson.RawValue) {
	mt.Helper()

	switch expected.Type {
	case bson.TypeInt32, bson.TypeInt64, bson.TypeDouble:
		compareNumberValues(mt, key, expected, actual)
	case bson.TypeString:
		val := expected.StringValue()
		if val == "42" {
			assert.NotEqual(mt, bson.TypeNull, actual.Type, "expected non-null value for key %v, got null", key)
			return
		}
		assert.Equal(mt, expected.Value, actual.Value,
			"value mismatch for key %v; expected %v, got %v", key, expected.Value, actual.Value)
	case bson.TypeEmbeddedDocument:
		e := expected.Document()
		if typeVal, err := e.LookupErr("$$type"); err == nil {
			// $$type represents a type assertion
			// for example {field: {$$type: "binData"}} should assert that "field" is an element with a binary value
			assertType(mt, actual.Type, typeVal.StringValue())
			return
		}

		a := actual.Document()
		compareDocs(mt, e, a)
	case bson.TypeArray:
		e := expected.Array()
		a := actual.Array()
		compareDocs(mt, e, a)
	default:
		assert.Equal(mt, expected.Value, actual.Value,
			"value mismatch for key %v; expected %v, got %v", key, expected.Value, actual.Value)
	}
}

// helper for $$type assertions
func assertType(mt *mtest.T, actual bsontype.Type, typeStr string) {
	mt.Helper()

	var expected bsontype.Type
	switch typeStr {
	case "double":
		expected = bsontype.Double
	case "string":
		expected = bsontype.String
	case "object":
		expected = bsontype.EmbeddedDocument
	case "array":
		expected = bsontype.Array
	case "binData":
		expected = bsontype.Binary
	case "undefined":
		expected = bsontype.Undefined
	case "objectId":
		expected = bsontype.ObjectID
	case "boolean":
		expected = bsontype.Boolean
	case "date":
		expected = bsontype.DateTime
	case "null":
		expected = bsontype.Null
	case "regex":
		expected = bsontype.Regex
	case "dbPointer":
		expected = bsontype.DBPointer
	case "javascript":
		expected = bsontype.JavaScript
	case "symbol":
		expected = bsontype.Symbol
	case "javascriptWithScope":
		expected = bsontype.CodeWithScope
	case "int":
		expected = bsontype.Int32
	case "timestamp":
		expected = bsontype.Timestamp
	case "long":
		expected = bsontype.Int64
	case "decimal":
		expected = bsontype.Decimal128
	case "minKey":
		expected = bsontype.MinKey
	case "maxKey":
		expected = bsontype.MaxKey
	default:
		mt.Fatalf("unrecognized type string: %v", typeStr)
	}

	assert.Equal(mt, expected, actual, "BSON type mismatch; expected %v, got %v", expected, actual)
}

// compare expected and actual BSON documents. comparison succeeds if actual contains each element in expected.
func compareDocs(mt *mtest.T, expected, actual bson.Raw) {
	mt.Helper()

	eElems, err := expected.Elements()
	assert.Nil(mt, err, "error getting expected elements: %v", err)

	for _, e := range eElems {
		eKey := e.Key()
		aVal, err := actual.LookupErr(eKey)
		assert.Nil(mt, err, "key %s not found in result", e.Key())

		eVal := e.Value()
		if doc, ok := eVal.DocumentOK(); ok {
			// special $$type assertion
			if typeVal, err := doc.LookupErr("$$type"); err == nil {
				assertType(mt, aVal.Type, typeVal.StringValue())
				continue
			}

			// nested doc
			compareDocs(mt, doc, aVal.Document())
			continue
		}

		compareValues(mt, eKey, eVal, aVal)
	}
}

func checkExpectations(mt *mtest.T, expectations []*expectation, id0, id1 bsonx.Doc) {
	mt.Helper()

	for _, expectation := range expectations {
		if expectation.CommandStartedEvent != nil {
			compareStartedEvent(mt, expectation, id0, id1)
		}
		if expectation.CommandSucceededEvent != nil {
			compareSucceededEvent(mt, expectation)
		}
		if expectation.CommandFailedEvent != nil {
			compareFailedEvent(mt, expectation)
		}
	}
}

func compareStartedEvent(mt *mtest.T, expectation *expectation, id0, id1 bsonx.Doc) {
	mt.Helper()

	expected := expectation.CommandStartedEvent
	evt := mt.GetStartedEvent()
	assert.NotNil(mt, evt, "expected CommandStartedEvent, got nil")

	if expected.CommandName != "" {
		assert.Equal(mt, expected.CommandName, evt.CommandName,
			"cmd name mismatch; expected %s, got %s", expected.CommandName, evt.CommandName)
	}
	if expected.DatabaseName != "" {
		assert.Equal(mt, expected.DatabaseName, evt.DatabaseName,
			"db name mismatch; expected %s, got %s", expected.DatabaseName, evt.DatabaseName)
	}

	eElems, err := expected.Command.Elements()
	assert.Nil(mt, err, "error getting expected elements: %v", err)

	for _, elem := range eElems {
		key := elem.Key()
		val := elem.Value()

		actualVal := evt.Command.Lookup(key)

		// Keys that may be nil
		if val.Type == bson.TypeNull {
			assert.Equal(mt, bson.RawValue{}, actualVal, "expected value for key %s to be nil but got %v", key, actualVal)
			continue
		}
		if key == "ordered" || key == "cursor" || key == "batchSize" {
			// TODO: some tests specify that "ordered" must be a key in the event but ordered isn't a valid option for some of these cases (e.g. insertOne)
			// TODO: some FLE tests specify "cursor" subdocument for listCollections
			// TODO: find.json cmd monitoring tests expect different batch sizes for find/getMore commands based on an optional limit
			continue
		}

		// keys that should not be nil
		assert.NotEqual(mt, bson.TypeNull, actualVal.Type, "expected value %v for key %s but got nil", val, key)
		err = actualVal.Validate()
		assert.Nil(mt, err, "error validating value for key %s: %v", key, err)

		switch key {
		case "lsid":
			sessName := val.StringValue()
			var expectedID bson.Raw
			actualID := actualVal.Document()

			switch sessName {
			case "session0":
				expectedID, err = id0.MarshalBSON()
			case "session1":
				expectedID, err = id1.MarshalBSON()
			default:
				mt.Fatalf("unrecognized session identifier: %v", sessName)
			}
			assert.Nil(mt, err, "error getting expected session ID bytes: %v", err)

			assert.Equal(mt, expectedID, actualID,
				"session ID mismatch for session %v; expected %v, got %v", sessName, expectedID, actualID)
		default:
			compareValues(mt, key, val, actualVal)
		}
	}
}

func compareWriteErrors(mt *mtest.T, expected, actual bson.Raw) {
	mt.Helper()

	expectedErrors, _ := expected.Values()
	actualErrors, _ := actual.Values()

	for i, expectedErrVal := range expectedErrors {
		expectedErr := expectedErrVal.Document()
		actualErr := actualErrors[i].Document()

		eIdx := expectedErr.Lookup("index").Int32()
		aIdx := actualErr.Lookup("index").Int32()
		assert.Equal(mt, eIdx, aIdx, "expected error index %v, got %v", eIdx, aIdx)

		eCode := expectedErr.Lookup("code").Int32()
		aCode := actualErr.Lookup("code").Int32()
		if eCode != 42 {
			assert.Equal(mt, eCode, aCode, "expected error code %v, got %v", eCode, aCode)
		}

		eMsg := expectedErr.Lookup("errmsg").StringValue()
		aMsg := actualErr.Lookup("errmsg").StringValue()
		if eMsg == "" {
			assert.NotEqual(mt, aMsg, "", "expected non-empty error message, got empty")
			return
		}
		assert.Equal(mt, eMsg, aMsg, "expected error message %v, got %v", eMsg, aMsg)
	}
}

func compareSucceededEvent(mt *mtest.T, expectation *expectation) {
	mt.Helper()

	expected := expectation.CommandSucceededEvent
	evt := mt.GetSucceededEvent()
	assert.NotNil(mt, evt, "expected CommandSucceededEvent, got nil")

	if expected.CommandName != "" {
		assert.Equal(mt, expected.CommandName, evt.CommandName,
			"cmd name mismatch; expected %s, got %s", expected.CommandName, evt.CommandName)
	}

	eElems, err := expected.Reply.Elements()
	assert.Nil(mt, err, "error getting expected elements: %v", err)

	for _, elem := range eElems {
		key := elem.Key()
		val := elem.Value()
		actualVal := evt.Reply.Lookup(key)

		switch key {
		case "writeErrors":
			compareWriteErrors(mt, val.Array(), actualVal.Array())
		default:
			compareValues(mt, key, val, actualVal)
		}
	}
}

func compareFailedEvent(mt *mtest.T, expectation *expectation) {
	mt.Helper()

	expected := expectation.CommandFailedEvent
	evt := mt.GetFailedEvent()
	assert.NotNil(mt, evt, "expected CommandFailedEvent, got nil")

	if expected.CommandName != "" {
		assert.Equal(mt, expected.CommandName, evt.CommandName,
			"cmd name mismatch; expected %s, got %s", expected.CommandName, evt.CommandName)
	}
}
