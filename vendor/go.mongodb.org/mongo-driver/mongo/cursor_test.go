package mongo

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/x/bsonx/bsoncore"
	"go.mongodb.org/mongo-driver/x/mongo/driver"
)

type testBatchCursor struct {
	batches []*bsoncore.DocumentSequence
	batch   *bsoncore.DocumentSequence
	closed  bool
}

func newTestBatchCursor(numBatches, batchSize int) *testBatchCursor {
	batches := make([]*bsoncore.DocumentSequence, 0, numBatches)

	counter := 0
	for batch := 0; batch < numBatches; batch++ {
		var docSequence []byte

		for doc := 0; doc < batchSize; doc++ {
			var elem []byte
			elem = bsoncore.AppendInt32Element(elem, "foo", int32(counter))
			counter++

			var doc []byte
			doc = bsoncore.BuildDocumentFromElements(doc, elem)
			docSequence = append(docSequence, doc...)
		}

		batches = append(batches, &bsoncore.DocumentSequence{
			Style: bsoncore.SequenceStyle,
			Data:  docSequence,
		})
	}

	return &testBatchCursor{
		batches: batches,
	}
}

func (tbc *testBatchCursor) ID() int64 {
	if len(tbc.batches) == 0 {
		return 0 // cursor exhausted
	}

	return 10
}

func (tbc *testBatchCursor) Next(context.Context) bool {
	if len(tbc.batches) == 0 {
		return false
	}

	tbc.batch = tbc.batches[0]
	tbc.batches = tbc.batches[1:]
	return true
}

func (tbc *testBatchCursor) Batch() *bsoncore.DocumentSequence {
	return tbc.batch
}

func (tbc *testBatchCursor) Server() driver.Server {
	return nil
}

func (tbc *testBatchCursor) Err() error {
	return nil
}

func (tbc *testBatchCursor) Close(context.Context) error {
	tbc.closed = true
	return nil
}

func TestCursor(t *testing.T) {
	t.Run("loops until docs available", func(t *testing.T) {})
	t.Run("returns false on context cancellation", func(t *testing.T) {})
	t.Run("returns false if error occurred", func(t *testing.T) {})
	t.Run("returns false if ID is zero and no more docs", func(t *testing.T) {})

	t.Run("TestAll", func(t *testing.T) {
		t.Run("errors if argument is not pointer to slice", func(t *testing.T) {
			cursor, err := newCursor(newTestBatchCursor(1, 5), nil)
			require.Nil(t, err)
			err = cursor.All(context.Background(), []bson.D{})
			require.NotNil(t, err)
		})

		t.Run("fills slice with all documents", func(t *testing.T) {
			cursor, err := newCursor(newTestBatchCursor(1, 5), nil)
			require.Nil(t, err)

			var docs []bson.D
			err = cursor.All(context.Background(), &docs)
			require.Nil(t, err)
			require.Equal(t, 5, len(docs))

			for index, doc := range docs {
				require.Equal(t, doc, bson.D{{"foo", int32(index)}})
			}
		})

		t.Run("decodes each document into slice type", func(t *testing.T) {
			cursor, err := newCursor(newTestBatchCursor(1, 5), nil)
			require.Nil(t, err)

			type Document struct {
				Foo int32 `bson:"foo"`
			}
			var docs []Document
			err = cursor.All(context.Background(), &docs)
			require.Nil(t, err)
			require.Equal(t, 5, len(docs))

			for index, doc := range docs {
				require.Equal(t, doc, Document{Foo: int32(index)})
			}
		})

		t.Run("multiple batches are included", func(t *testing.T) {
			cursor, err := newCursor(newTestBatchCursor(2, 5), nil)
			var docs []bson.D
			err = cursor.All(context.Background(), &docs)
			require.Nil(t, err)
			require.Equal(t, 10, len(docs))

			for index, doc := range docs {
				require.Equal(t, doc, bson.D{{"foo", int32(index)}})
			}
		})

		t.Run("cursor is closed after All is called", func(t *testing.T) {
			var docs []bson.D

			tbc := newTestBatchCursor(1, 5)
			cursor, err := newCursor(tbc, nil)
			err = cursor.All(context.Background(), &docs)

			require.Nil(t, err)
			require.True(t, tbc.closed)
		})
	})
}

// func TestTailableCursorLoopsUntilDocsAvailable(t *testing.T) {
// 	server, err := testutil.Topology(t).SelectServerLegacy(context.Background(), description.WriteSelector())
// 	noerr(t, err)
//
// 	// create capped collection
// 	createCmd := bsonx.Doc{
// 		{"create", bsonx.String(testutil.ColName(t))},
// 		{"capped", bsonx.Boolean(true)},
// 		{"size", bsonx.Int32(1000)}}
// 	_, err = testutil.RunCommand(t, server.Server, dbName, createCmd)
//
// 	// Insert a document
// 	d := bsonx.Doc{{"_id", bsonx.Int32(1)}, {"ts", bsonx.Timestamp(5, 0)}}
// 	wc := writeconcern.New(writeconcern.WMajority())
// 	testutil.AutoInsertDocs(t, wc, d)
//
// 	rdr, err := d.MarshalBSON()
// 	noerr(t, err)
//
// 	clientID, err := uuid.New()
// 	noerr(t, err)
//
// 	cursor, err := driverlegacy.Find(
// 		context.Background(),
// 		command.Find{
// 			NS:     command.Namespace{DB: dbName, Collection: testutil.ColName(t)},
// 			Filter: bsonx.Doc{{"ts", bsonx.Document(bsonx.Doc{{"$gte", bsonx.Timestamp(5, 0)}})}},
// 		},
// 		testutil.Topology(t),
// 		description.WriteSelector(),
// 		clientID,
// 		&session.Pool{},
// 		bson.DefaultRegistry,
// 		options.Find().SetCursorType(options.TailableAwait),
// 	)
// 	noerr(t, err)
//
// 	// assert that there is a document returned
// 	assert.True(t, cursor.Next(context.Background()), "Cursor should have a next result")
//
// 	// make sure it's the right document
// 	var next bsoncore.Document
// 	next, err = cursor.Batch().Next()
// 	noerr(t, err)
//
// 	if !bytes.Equal(next, rdr) {
// 		t.Errorf("Did not get expected document. got %v; want %v", bson.Raw(next), bson.Raw(rdr))
// 	}
//
// 	// insert another document in 500 MS
// 	d = bsonx.Doc{{"_id", bsonx.Int32(2)}, {"ts", bsonx.Timestamp(6, 0)}}
//
// 	rdr, err = d.MarshalBSON()
// 	noerr(t, err)
//
// 	go func() {
// 		time.Sleep(time.Millisecond * 500)
// 		testutil.AutoInsertDocs(t, wc, d)
// 	}()
//
// 	// context with timeout so test fails if loop does not work as expected
// 	ctx, cancel := context.WithTimeout(context.Background(), time.Second*2)
// 	defer cancel()
//
// 	// assert that there is another document returned
// 	// cursor.Next should loop calling getMore until a document becomes available (in 500 ms)
// 	assert.True(t, cursor.Next(ctx), "Cursor should have a next result")
//
// 	noerr(t, cursor.Err())
//
// 	// make sure it's the right document the second time
// 	next, err = cursor.Batch().Next()
// 	noerr(t, err)
//
// 	if !bytes.Equal(next, rdr) {
// 		t.Errorf("Did not get expected document. got %v; want %v", bson.Raw(next), bson.Raw(rdr))
// 	}
// }
