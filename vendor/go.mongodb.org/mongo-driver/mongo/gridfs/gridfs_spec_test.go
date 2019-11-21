// Copyright (C) MongoDB, Inc. 2017-present.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may
// not use this file except in compliance with the License. You may obtain
// a copy of the License at http://www.apache.org/licenses/LICENSE-2.0

package gridfs

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"io/ioutil"
	"path"
	"testing"

	"bytes"

	"fmt"

	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/internal/testutil"
	"go.mongodb.org/mongo-driver/internal/testutil/helpers"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.mongodb.org/mongo-driver/x/bsonx"
)

type testFile struct {
	Data  dataSection `json:"data"`
	Tests []test      `json:"tests"`
}

type dataSection struct {
	Files  []json.RawMessage `json:"files"`
	Chunks []json.RawMessage `json:"chunks"`
}

type test struct {
	Description string         `json:"description"`
	Arrange     arrangeSection `json:"arrange"`
	Act         actSection     `json:"act"`
	Assert      assertSection  `json:"assert"`
}

type arrangeSection struct {
	Data []json.RawMessage `json:"data"`
}

type actSection struct {
	Operation string          `json:"operation"`
	Arguments json.RawMessage `json:"arguments"`
}

type assertSection struct {
	Result json.RawMessage     `json:"result"`
	Error  string              `json:"error"`
	Data   []assertDataSection `json:"data"`
}

type assertDataSection struct {
	Insert    string            `json:"insert"`
	Documents []interface{}     `json:"documents"`
	Delete    string            `json:"delete"`
	Deletes   []json.RawMessage `json:"deletes"`
}

const gridFsTestDir = "../../data/gridfs"
const downloadBufferSize = 100

var ctx = context.Background()
var emptyDoc = bsonx.Doc{}
var client *mongo.Client
var db *mongo.Database
var chunks, files, expectedChunks, expectedFiles *mongo.Collection

var downloadBuffer = make([]byte, downloadBufferSize)
var deadline = time.Now().Add(time.Hour)

func TestGridFSSpec(t *testing.T) {
	var err error
	cs := testutil.ConnString(t)
	client, err = mongo.NewClient(options.Client().ApplyURI(cs.String()))
	testhelpers.RequireNil(t, err, "error creating client: %s", err)

	err = client.Connect(ctx)
	testhelpers.RequireNil(t, err, "error connecting client: %s", err)

	db = client.Database("gridFSTestDB")
	chunks = db.Collection("fs.chunks")
	files = db.Collection("fs.files")
	expectedChunks = db.Collection("expected.chunks")
	expectedFiles = db.Collection("expected.files")

	for _, file := range testhelpers.FindJSONFilesInDir(t, gridFsTestDir) {
		runGridFSTestFile(t, path.Join(gridFsTestDir, file), db)
	}
}

func runGridFSTestFile(t *testing.T, filepath string, db *mongo.Database) {
	content, err := ioutil.ReadFile(filepath)
	testhelpers.RequireNil(t, err, "error reading file %s: %s", filepath, err)

	var testfile testFile
	err = json.Unmarshal(content, &testfile)
	testhelpers.RequireNil(t, err, "error unmarshalling test file for %s: %s", filepath, err)

	clearCollections(t)
	chunkSize := loadInitialFiles(t, testfile.Data.Files, testfile.Data.Chunks, files, chunks, expectedFiles, expectedChunks)
	if chunkSize == 0 {
		chunkSize = DefaultChunkSize
	}

	bucket, err := NewBucket(db, options.GridFSBucket().SetChunkSizeBytes(chunkSize))
	testhelpers.RequireNil(t, err, "error creating bucket: %s", err)
	err = bucket.SetWriteDeadline(deadline)
	testhelpers.RequireNil(t, err, "error setting write deadline: %s", err)
	err = bucket.SetReadDeadline(deadline)
	testhelpers.RequireNil(t, err, "error setting read deadline: %s", err)

	for _, test := range testfile.Tests {
		t.Run(test.Description, func(t *testing.T) {
			switch test.Act.Operation {
			case "upload":
				runUploadTest(t, test, bucket)
				clearCollections(t)
				runUploadFromStreamTest(t, test, bucket)
			case "download":
				runDownloadTest(t, test, bucket)
				runDownloadToStreamTest(t, test, bucket)
			case "download_by_name":
				runDownloadByNameTest(t, test, bucket)
				runDownloadByNameToStreamTest(t, test, bucket)
			case "delete":
				runDeleteTest(t, test, bucket)
			}
		})

		if test.Arrange.Data != nil {
			clearCollections(t)
			loadInitialFiles(t, testfile.Data.Files, testfile.Data.Chunks, files, chunks, expectedFiles, expectedChunks)
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

func compareValues(expected bsonx.Val, actual bsonx.Val) bool {
	if expected.IsNumber() {
		if !actual.IsNumber() {
			return false
		}

		return getInt64(expected) == getInt64(actual)
	}

	switch expected.Type() {
	case bson.TypeString:
		return actual.StringValue() == expected.StringValue()
	case bson.TypeBinary:
		aSub, aBytes := actual.Binary()
		eSub, eBytes := expected.Binary()

		return aSub == eSub && bytes.Equal(aBytes, eBytes)
	case bson.TypeObjectID:
		eID := [12]byte(expected.ObjectID())
		aID := [12]byte(actual.ObjectID())

		return bytes.Equal(eID[:], aID[:])
	case bson.TypeEmbeddedDocument:
		return expected.Document().Equal(actual.Document())
	default:
		fmt.Printf("unknown type: %d\n", expected.Type())
	}

	return true // shouldn't get here
}

func compareGfsDoc(t *testing.T, expected bsonx.Doc, actual bsonx.Doc, filesID interface{}) {
	for _, elem := range expected {
		key := elem.Key

		// continue for deprecated fields
		if key == "md5" || key == "contentType" || key == "aliases" {
			continue
		}

		actualVal, err := actual.LookupErr(key)
		testhelpers.RequireNil(t, err, "key %s not found in actual for test %s", key, t.Name())

		// continue for fields with unknown values
		if key == "_id" || key == "uploadDate" {
			continue
		}

		if key == "files_id" {
			expectedBytes := make([]byte, 12)
			actualBytes := make([]byte, 12)

			var oid primitive.ObjectID
			err = (&oid).UnmarshalJSON(expectedBytes)
			testhelpers.RequireNil(t, err, "error unmarshalling expected bytes: %s", err)
			filesID = oid
			actualID := actualVal.ObjectID()
			err = (&actualID).UnmarshalJSON(actualBytes)
			testhelpers.RequireNil(t, err, "error unmarshalling actual bytes: %s", err)

			if !bytes.Equal(expectedBytes, actualBytes) {
				t.Fatalf("files_id mismatch for test %s", t.Name())
			}

			continue
		}

		if eDoc, ok := elem.Value.DocumentOK(); ok {
			compareGfsDoc(t, eDoc, actualVal.Document(), filesID)
			continue
		}

		if !compareValues(elem.Value, actualVal) {
			t.Fatalf("values for key %s not equal for test %s", key, t.Name())
		}
	}
}

// compare chunks and expectedChunks collections
func compareChunks(t *testing.T, filesID interface{}) {
	actualCursor, err := chunks.Find(ctx, emptyDoc)
	testhelpers.RequireNil(t, err, "error running Find for chunks: %s", err)
	expectedCursor, err := expectedChunks.Find(ctx, emptyDoc)
	testhelpers.RequireNil(t, err, "error running Find for expected chunks: %s", err)

	for expectedCursor.Next(ctx) {
		if !actualCursor.Next(ctx) {
			t.Fatalf("chunks has fewer documents than expectedChunks")
		}

		var actualChunk bsonx.Doc
		var expectedChunk bsonx.Doc

		err = actualCursor.Decode(&actualChunk)
		testhelpers.RequireNil(t, err, "error decoding actual chunk: %s", err)
		err = expectedCursor.Decode(&expectedChunk)
		testhelpers.RequireNil(t, err, "error decoding expected chunk: %s", err)

		compareGfsDoc(t, expectedChunk, actualChunk, filesID)
	}
}

// compare files and expectedFiles collections
func compareFiles(t *testing.T) {
	actualCursor, err := files.Find(ctx, emptyDoc)
	testhelpers.RequireNil(t, err, "error running Find for files: %s", err)
	expectedCursor, err := expectedFiles.Find(ctx, emptyDoc)
	testhelpers.RequireNil(t, err, "error running Find for expected files: %s", err)

	for expectedCursor.Next(ctx) {
		if !actualCursor.Next(ctx) {
			t.Fatalf("files has fewer documents than expectedFiles")
		}

		var actualFile bsonx.Doc
		var expectedFile bsonx.Doc

		err = actualCursor.Decode(&actualFile)
		testhelpers.RequireNil(t, err, "error decoding actual file: %s", err)
		err = expectedCursor.Decode(&expectedFile)
		testhelpers.RequireNil(t, err, "error decoding expected file: %s", err)

		compareGfsDoc(t, expectedFile, actualFile, primitive.ObjectID{})
	}
}

func convertHexToBytes(t *testing.T, hexStr string) []byte {
	hexBytes, err := hex.DecodeString(hexStr)
	testhelpers.RequireNil(t, err, "error decoding hex for %s: %s", t.Name(), err)
	return hexBytes
}

func msgToDoc(t *testing.T, msg json.RawMessage) bsonx.Doc {
	rawBytes, err := msg.MarshalJSON()
	testhelpers.RequireNil(t, err, "error marshalling message: %s", err)

	doc := bsonx.Doc{}
	err = bson.UnmarshalExtJSON(rawBytes, true, &doc)
	testhelpers.RequireNil(t, err, "error creating BSON doc: %s", err)

	return doc
}

func runUploadAssert(t *testing.T, test test, fileID interface{}) {
	assert := test.Assert

	for _, assertData := range assert.Data {
		// each assertData section is a single command that modifies an expected collection
		if assertData.Insert != "" {
			var err error
			docs := make([]interface{}, len(assertData.Documents))

			for i, docInterface := range assertData.Documents {
				rdr, err := bson.Marshal(docInterface)
				testhelpers.RequireNil(t, err, "error marshaling doc: %s", err)
				doc, err := bsonx.ReadDoc(rdr)
				testhelpers.RequireNil(t, err, "error reading doc: %s", err)

				if id, err := doc.LookupErr("_id"); err == nil {
					idStr := id.StringValue()
					if idStr == "*result" || idStr == "*actual" {
						// server will create _id
						doc = doc.Delete("_id")
					}
				}

				if data, err := doc.LookupErr("data"); err == nil {
					hexBytes := convertHexToBytes(t, data.Document().Lookup("$hex").StringValue())
					doc = doc.Delete("data")
					doc = append(doc, bsonx.Elem{"data", bsonx.Binary(0x00, hexBytes)})
				}

				docs[i] = doc
			}

			switch assertData.Insert {
			case "expected.files":
				_, err = expectedFiles.InsertMany(ctx, docs)
			case "expected.chunks":
				_, err = expectedChunks.InsertMany(ctx, docs)
			}

			testhelpers.RequireNil(t, err, "error modifying expected collections: %s", err)
		}

		compareFiles(t)
		compareChunks(t, fileID)
	}
}

func parseUploadOptions(args bsonx.Doc) *options.UploadOptions {
	opts := options.GridFSUpload()

	if optionsVal, err := args.LookupErr("options"); err == nil {
		for _, elem := range optionsVal.Document() {
			val := elem.Value

			switch elem.Key {
			case "chunkSizeBytes":
				size := val.Int32()
				opts = opts.SetChunkSizeBytes(size)
			case "metadata":
				opts = opts.SetMetadata(val.Document())
			}
		}
	}

	return opts
}

func runUploadFromStreamTest(t *testing.T, test test, bucket *Bucket) {
	args := msgToDoc(t, test.Act.Arguments)
	opts := parseUploadOptions(args)
	hexBytes := convertHexToBytes(t, args.Lookup("source", "$hex").StringValue())

	fileID, err := bucket.UploadFromStream(args.Lookup("filename").StringValue(), bytes.NewBuffer(hexBytes), opts)
	testhelpers.RequireNil(t, err, "error uploading from stream: %s", err)

	runUploadAssert(t, test, fileID)
}

func runUploadTest(t *testing.T, test test, bucket *Bucket) {
	// run operation from act section
	args := msgToDoc(t, test.Act.Arguments)

	opts := parseUploadOptions(args)
	hexBytes := convertHexToBytes(t, args.Lookup("source", "$hex").StringValue())
	stream, err := bucket.OpenUploadStream(args.Lookup("filename").StringValue(), opts)
	testhelpers.RequireNil(t, err, "error opening upload stream for %s: %s", t.Name(), err)

	err = stream.SetWriteDeadline(deadline)
	testhelpers.RequireNil(t, err, "error setting write deadline: %s", err)
	n, err := stream.Write(hexBytes)
	if n != len(hexBytes) {
		t.Fatalf("all bytes not written for %s. expected %d got %d", t.Name(), len(hexBytes), n)
	}

	err = stream.Close()
	testhelpers.RequireNil(t, err, "error closing upload stream for %s: %s", t.Name(), err)

	// assert section is laid out as a series of commands that modify expected.files and expected.chunks
	runUploadAssert(t, test, stream.FileID)
}

// run a series of delete operations that are already BSON documents
func runDeletes(t *testing.T, deletes bsonx.Arr, coll *mongo.Collection) {
	for _, val := range deletes {
		doc := val.Document() // has q and limit
		filter := doc.Lookup("q").Document()

		_, err := coll.DeleteOne(ctx, filter)
		testhelpers.RequireNil(t, err, "error running deleteOne for %s: %s", t.Name(), err)
	}
}

// run a series of updates that are already BSON documents
func runUpdates(t *testing.T, updates bsonx.Arr, coll *mongo.Collection) {
	for _, val := range updates {
		updateDoc := val.Document()
		filter := updateDoc.Lookup("q").Document()
		update := updateDoc.Lookup("u").Document()

		// update has $set -> data -> $hex
		if hexStr, err := update.LookupErr("$set", "data", "$hex"); err == nil {
			hexBytes := convertHexToBytes(t, hexStr.StringValue())
			update = update.Delete("$set")
			update = append(update, bsonx.Elem{"$set", bsonx.Document(bsonx.Doc{
				{"data", bsonx.Binary(0x00, hexBytes)},
			})})
			testhelpers.RequireNil(t, err, "error concatenating data bytes to update: %s", err)
		}

		_, err := coll.UpdateOne(ctx, filter, update)
		testhelpers.RequireNil(t, err, "error running updateOne for test %s: %s", t.Name(), err)
	}
}

func compareDownloadAssertResult(t *testing.T, assert assertSection, copied int64) {
	assertResult, err := assert.Result.MarshalJSON() // json.RawMessage
	testhelpers.RequireNil(t, err, "error marshalling assert result: %s", err)
	assertDoc := bsonx.Doc{}
	err = bson.UnmarshalExtJSON(assertResult, true, &assertDoc)
	testhelpers.RequireNil(t, err, "error constructing result doc: %s", err)

	if hexStr, err := assertDoc.LookupErr("$hex"); err == nil {
		hexBytes := convertHexToBytes(t, hexStr.StringValue())

		if copied != int64(len(hexBytes)) {
			t.Fatalf("bytes missing. expected %d bytes, got %d", len(hexBytes), copied)
		}

		if !bytes.Equal(hexBytes, downloadBuffer[:copied]) {
			t.Fatalf("downloaded bytes mismatch. expected %v, got %v", hexBytes, downloadBuffer[:copied])
		}
	} else {
		t.Fatalf("%v", err)
	}
}

func compareDownloadAssert(t *testing.T, assert assertSection, stream *DownloadStream, streamErr error) {
	var copied int
	var copiedErr error

	if streamErr == nil {
		// files are small enough to read into memory once
		err := stream.SetReadDeadline(deadline)
		testhelpers.RequireNil(t, err, "error setting read deadline: %s", err)
		copied, copiedErr = stream.Read(downloadBuffer)
		testhelpers.RequireNil(t, err, "error reading from stream: %s", err)
	}

	// assert section
	if assert.Result != nil {
		testhelpers.RequireNil(t, streamErr, "error downloading to stream: %s", streamErr)
		compareDownloadAssertResult(t, assert, int64(copied))
	} else if assert.Error != "" {
		var errToCompare error
		var expectedErr error

		switch assert.Error {
		case "FileNotFound":
			fallthrough
		case "RevisionNotFound":
			errToCompare = streamErr
			expectedErr = ErrFileNotFound
		case "ChunkIsMissing":
			errToCompare = copiedErr
			expectedErr = ErrWrongIndex
		case "ChunkIsWrongSize":
			errToCompare = copiedErr
			expectedErr = ErrWrongSize
		}

		testhelpers.RequireNotNil(t, errToCompare, "errToCompare is nil")
		if errToCompare != expectedErr {
			t.Fatalf("err mismatch. expected %s got %s", expectedErr, errToCompare)
		}
	}
}

func compareDownloadToStreamAssert(t *testing.T, assert assertSection, n int64, err error) {
	if assert.Result != nil {
		testhelpers.RequireNil(t, err, "error downloading to stream: %s", err)
		compareDownloadAssertResult(t, assert, n)
	} else if assert.Error != "" {
		var compareErr error

		switch assert.Error {
		case "FileNotFound":
			fallthrough
		case "RevisionNotFound":
			compareErr = ErrFileNotFound
		case "ChunkIsMissing":
			compareErr = ErrWrongIndex
		case "ChunkIsWrongSize":
			compareErr = ErrWrongSize
		}

		testhelpers.RequireNotNil(t, err, "no error when downloading to stream. expected %s", compareErr)
		if err != compareErr {
			t.Fatalf("download to stream error mismatch. expected %s got %s", compareErr, err)
		}
	}
}

func runArrangeSection(t *testing.T, test test, coll *mongo.Collection) {
	for _, msg := range test.Arrange.Data {
		msgBytes, err := msg.MarshalJSON()
		testhelpers.RequireNil(t, err, "error marshalling arrange data for test %s: %s", t.Name(), err)

		msgDoc := bsonx.Doc{}
		err = bson.UnmarshalExtJSON(msgBytes, true, &msgDoc)
		testhelpers.RequireNil(t, err, "error creating arrange data doc for test %s: %s", t.Name(), err)

		if _, err = msgDoc.LookupErr("delete"); err == nil {
			// all arrange sections in the current spec tests operate on the fs.chunks collection
			runDeletes(t, msgDoc.Lookup("deletes").Array(), coll)
		} else if _, err = msgDoc.LookupErr("update"); err == nil {
			runUpdates(t, msgDoc.Lookup("updates").Array(), coll)
		}
	}
}

func runDownloadTest(t *testing.T, test test, bucket *Bucket) {
	runArrangeSection(t, test, chunks)

	args := msgToDoc(t, test.Act.Arguments)
	stream, streamErr := bucket.OpenDownloadStream(args.Lookup("id").ObjectID())
	compareDownloadAssert(t, test.Assert, stream, streamErr)
}

func runDownloadToStreamTest(t *testing.T, test test, bucket *Bucket) {
	runArrangeSection(t, test, chunks)
	args := msgToDoc(t, test.Act.Arguments)

	downloadStream := bytes.NewBuffer(downloadBuffer)
	n, err := bucket.DownloadToStream(args.Lookup("id").ObjectID(), downloadStream)

	compareDownloadToStreamAssert(t, test.Assert, n, err)
}

func parseDownloadByNameOpts(t *testing.T, args bsonx.Doc) *options.NameOptions {
	opts := options.GridFSName()

	if optsVal, err := args.LookupErr("options"); err == nil {
		optsDoc := optsVal.Document()

		if revVal, err := optsDoc.LookupErr("revision"); err == nil {
			opts = opts.SetRevision(revVal.Int32())
		}
	}

	return opts
}

func runDownloadByNameTest(t *testing.T, test test, bucket *Bucket) {
	// act section
	args := msgToDoc(t, test.Act.Arguments)
	opts := parseDownloadByNameOpts(t, args)
	stream, streamErr := bucket.OpenDownloadStreamByName(args.Lookup("filename").StringValue(), opts)
	compareDownloadAssert(t, test.Assert, stream, streamErr)
}

func runDownloadByNameToStreamTest(t *testing.T, test test, bucket *Bucket) {
	args := msgToDoc(t, test.Act.Arguments)
	opts := parseDownloadByNameOpts(t, args)
	downloadStream := bytes.NewBuffer(downloadBuffer)
	n, err := bucket.DownloadToStreamByName(args.Lookup("filename").StringValue(), downloadStream, opts)

	compareDownloadToStreamAssert(t, test.Assert, n, err)
}

func runDeleteTest(t *testing.T, test test, bucket *Bucket) {
	runArrangeSection(t, test, files)
	args := msgToDoc(t, test.Act.Arguments)

	err := bucket.Delete(args.Lookup("id").ObjectID())
	if test.Assert.Error != "" {
		var errToCompare error
		switch test.Assert.Error {
		case "FileNotFound":
			errToCompare = ErrFileNotFound
		}

		if err != errToCompare {
			t.Fatalf("error mismatch for delete. expected %s got %s", errToCompare, err)
		}
	}

	if len(test.Assert.Data) != 0 {
		for _, data := range test.Assert.Data {
			deletes := bsonx.Arr{}

			for _, deleteMsg := range data.Deletes {
				deletes = append(deletes, bsonx.Document(msgToDoc(t, deleteMsg)))
			}

			runDeletes(t, deletes, expectedFiles)
			compareFiles(t)
		}
	}
}
