// Copyright (C) MongoDB, Inc. 2017-present.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may
// not use this file except in compliance with the License. You may obtain
// a copy of the License at http://www.apache.org/licenses/LICENSE-2.0

package mongo

import (
	"testing"

	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/x/bsonx"
)

func TestDeleteResult_unmarshalInto(t *testing.T) {
	t.Parallel()

	doc := bsonx.Doc{
		{"n", bsonx.Int64(2)},
		{"ok", bsonx.Int64(1)},
	}

	b, err := doc.MarshalBSON()
	require.Nil(t, err)

	var result DeleteResult
	err = bson.Unmarshal(b, &result)
	require.Nil(t, err)
	require.Equal(t, result.DeletedCount, int64(2))
}

func TestDeleteResult_marshalFrom(t *testing.T) {
	t.Parallel()

	result := DeleteResult{DeletedCount: 1}
	buf, err := bson.Marshal(result)
	require.Nil(t, err)

	doc, err := bsonx.ReadDoc(buf)
	require.Nil(t, err)

	require.Equal(t, len(doc), 1)
	e, err := doc.LookupErr("n")
	require.NoError(t, err)
	require.Equal(t, e.Type(), bson.TypeInt64)
	require.Equal(t, e.Int64(), int64(1))
}

func TestUpdateOneResult_unmarshalInto(t *testing.T) {
	t.Parallel()

	doc := bsonx.Doc{
		{"n", bsonx.Int32(1)},
		{"nModified", bsonx.Int32(2)},
		{"upserted", bsonx.Array(bsonx.Arr{
			bsonx.Document(bsonx.Doc{
				{"index", bsonx.Int32(0)},
				{"_id", bsonx.Int32(3)},
			}),
		}),
		}}

	b, err := doc.MarshalBSON()
	require.Nil(t, err)

	var result UpdateResult
	err = bson.Unmarshal(b, &result)
	require.Nil(t, err)
	require.Equal(t, result.MatchedCount, int64(1))
	require.Equal(t, result.ModifiedCount, int64(2))
	require.Equal(t, int(result.UpsertedID.(int32)), 3)
}
