// Copyright (C) MongoDB, Inc. 2017-present.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may
// not use this file except in compliance with the License. You may obtain
// a copy of the License at http://www.apache.org/licenses/LICENSE-2.0

package gridfs

import (
	"bytes"
	"math"
	"math/rand"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/internal/testutil"
	"go.mongodb.org/mongo-driver/internal/testutil/helpers"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.mongodb.org/mongo-driver/x/bsonx"

	"go.mongodb.org/mongo-driver/internal/testutil/israce"
	"golang.org/x/net/context"
)

var chunkSizeTests = []struct {
	testName   string
	bucketOpts *options.BucketOptions
	uploadOpts *options.UploadOptions
}{
	{"Default values", nil, nil},
	{"Options provided without chunk size", options.GridFSBucket(), options.GridFSUpload()},
	{"Bucket chunk size set", options.GridFSBucket().SetChunkSizeBytes(27), nil},
	{"Upload stream chunk size set", nil, options.GridFSUpload().SetChunkSizeBytes(27)},
	{"Bucket and upload set to different values", options.GridFSBucket().SetChunkSizeBytes(27), options.GridFSUpload().SetChunkSizeBytes(31)},
}

func findIndex(ctx context.Context, t *testing.T, coll *mongo.Collection, unique bool, keys ...string) {
	cur, err := coll.Indexes().List(ctx)
	if err != nil {
		t.Fatalf("Couldn't establish a cursor on the collection %v: %v", coll.Name(), err)
	}
	foundIndex := false
	for cur.Next(ctx) {
		if _, err := cur.Current.LookupErr(keys...); err == nil {
			if uVal, err := cur.Current.LookupErr("unique"); (unique && err == nil && uVal.Boolean() == true) || (!unique && (err != nil || uVal.Boolean() == false)) {
				foundIndex = true
			}
		}
	}
	if !foundIndex {
		t.Errorf("Expected index on %v, but did not find one.", keys)
	}
}

// TODO: remove this check once BACKPORT-3804 is completed and Mongo 4.1.7 is released.
func canRunRoundTripTest(t *testing.T, db *mongo.Database) bool {
	if runtime.GOOS != "darwin" {
		return true
	}

	var serverStatus bsonx.Doc
	err := db.RunCommand(
		context.Background(),
		bsonx.Doc{{"serverStatus", bsonx.Int32(1)}},
	).Decode(&serverStatus)
	if err != nil {
		t.Fatalf("Problem getting server status: %v", err)
	}

	version, err := serverStatus.LookupErr("version")
	if err != nil {
		t.Fatalf("Couldn't find server version: %v", err)
	}

	if compareVersions(t, version.StringValue(), "4.0") < 0 {
		return true
	}

	_, err = serverStatus.LookupErr("sharding")
	if err != nil {
		return true
	}

	_, err = serverStatus.LookupErr("security")
	if err != nil {
		return true
	}

	return false
}

func compareVersions(t *testing.T, v1 string, v2 string) int {
	n1 := strings.Split(v1, ".")
	n2 := strings.Split(v2, ".")

	for i := 0; i < int(math.Min(float64(len(n1)), float64(len(n2)))); i++ {
		i1, err := strconv.Atoi(n1[i])
		require.NoError(t, err)

		i2, err := strconv.Atoi(n2[i])
		require.NoError(t, err)

		difference := i1 - i2
		if difference != 0 {
			return difference
		}
	}

	return 0
}

func TestGridFS(t *testing.T) {
	cs := testutil.ConnString(t)
	client, err := mongo.NewClient(options.Client().ApplyURI(cs.String()))
	testhelpers.RequireNil(t, err, "error creating client: %s", err)

	ctx := context.Background()
	err = client.Connect(ctx)
	testhelpers.RequireNil(t, err, "error connecting client: %s", err)

	db := client.Database("gridFSTestDB")

	// Unit tests showing the chunk size is set correctly on the bucket and upload stream objects.
	t.Run("ChunkSize", func(t *testing.T) {
		for _, tt := range chunkSizeTests {
			t.Run(tt.testName, func(t *testing.T) {
				bucket, err := NewBucket(db, tt.bucketOpts)
				if err != nil {
					t.Fatalf("Failed to create bucket: %v", err)
				}

				us, err := bucket.OpenUploadStream("filename", tt.uploadOpts)
				if err != nil {
					t.Fatalf("Failed to open upload stream: %v", err)
				}

				expectedBucketChunkSize := DefaultChunkSize
				if tt.bucketOpts != nil && tt.bucketOpts.ChunkSizeBytes != nil {
					expectedBucketChunkSize = *tt.bucketOpts.ChunkSizeBytes
				}
				if bucket.chunkSize != expectedBucketChunkSize {
					t.Errorf("Bucket had wrong chunkSize. Want %v, got %v.", expectedBucketChunkSize, bucket.chunkSize)
				}

				expectedUploadChunkSize := expectedBucketChunkSize
				if tt.uploadOpts != nil && tt.uploadOpts.ChunkSizeBytes != nil {
					expectedUploadChunkSize = *tt.uploadOpts.ChunkSizeBytes
				}
				if us.chunkSize != expectedUploadChunkSize {
					t.Errorf("Upload stream had wrong chunkSize. Want %v, got %v.", expectedUploadChunkSize, us.chunkSize)
				}

			})
		}
	})

	// Unit tests showing that UploadFromStream creates indexes on the chunks and files collections.
	t.Run("IndexCreation", func(t *testing.T) {
		bucket, err := NewBucket(db, nil)
		if err != nil {
			t.Fatalf("Failed to create bucket: %v", err)
		}
		err = bucket.SetWriteDeadline(time.Now().Add(5 * time.Second))
		if err != nil {
			t.Fatalf("Failed to set write deadline: %v", err)
		}
		err = bucket.Drop()
		if err != nil {
			t.Fatalf("Drop failed: %v", err)
		}

		byteData := []byte("Hello, world!")
		r := bytes.NewReader(byteData)

		_, err = bucket.UploadFromStream("filename", r)
		if err != nil {
			t.Fatalf("Failed to open upload stream: %v", err)
		}

		findCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()

		findIndex(findCtx, t, bucket.filesColl, false, "key", "filename")
		findIndex(findCtx, t, bucket.chunksColl, true, "key", "files_id")
	})

	t.Run("RoundTrip", func(t *testing.T) {

		oneK := 1024
		smallBuffSize := 100

		tests := []struct {
			name      string
			chunkSize int // make -1 for no capacity for no chunkSize
			fileSize  int
			bufSize   int // make -1 for no capacity for no bufSize
		}{
			{"RoundTrip: original", -1, oneK, -1},
			{"RoundTrip: chunk size multiple of file", oneK, oneK * 16, -1},
			{"RoundTrip: chunk size is file size", oneK, oneK, -1},
			{"RoundTrip: chunk size multiple of file size and with strict buffer size", oneK, oneK * 16, smallBuffSize},
			{"RoundTrip: chunk size multiple of file size and buffer size", oneK, oneK * 16, oneK * 16},
			{"RoundTrip: chunk size, file size, buffer size all the same", oneK, oneK, oneK},
		}

		for _, test := range tests {

			t.Run(test.name, func(t *testing.T) {

				if !canRunRoundTripTest(t, db) {
					t.Skip()
				}

				var chunkSize *int32
				var temp int32
				if test.chunkSize != -1 {
					temp = int32(test.chunkSize)
					chunkSize = &temp
				}

				bucket, err := NewBucket(db, &options.BucketOptions{
					ChunkSizeBytes: chunkSize,
				})
				if err != nil {
					t.Fatalf("Failed to create bucket: %v", err)
				}

				timeout := 5 * time.Second
				if israce.Enabled {
					timeout = 20 * time.Second // race detector causes 2-20x slowdown
				}

				err = bucket.SetWriteDeadline(time.Now().Add(timeout))
				if err != nil {
					t.Fatalf("Failed to set write deadline: %v", err)
				}
				err = bucket.Drop()
				if err != nil {
					t.Fatalf("Drop failed: %v", err)
				}

				// Test that Upload works when the buffer to write is longer than the upload stream's internal buffer.
				// This requires multiple calls to uploadChunks.
				size := test.fileSize
				p := make([]byte, size)
				for i := 0; i < size; i++ {
					p[i] = byte(rand.Intn(100))
				}

				_, err = bucket.UploadFromStream("filename", bytes.NewReader(p))
				if err != nil {
					t.Fatalf("Upload failed: %v", err)
				}

				var w *bytes.Buffer
				if test.bufSize == -1 {
					w = bytes.NewBuffer(make([]byte, 0))
				} else {
					w = bytes.NewBuffer(make([]byte, 0, test.bufSize))
				}

				_, err = bucket.DownloadToStreamByName("filename", w)
				if err != nil {
					t.Fatalf("Download failed: %v", err)
				}

				if !bytes.Equal(p, w.Bytes()) {
					t.Errorf("Downloaded file did not match p.")
				}

			})

		}

	})

	err = client.Disconnect(ctx)
	if err != nil {
		t.Fatalf("Problem disconnecting from client: %v", err)
	}
}
