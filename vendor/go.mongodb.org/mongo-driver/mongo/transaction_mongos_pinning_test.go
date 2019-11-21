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
	"time"

	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/internal/testutil"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func shouldSkipMongosPinningTests(t *testing.T, serverVersion string) bool {
	return os.Getenv("TOPOLOGY") != "sharded_cluster" || compareVersions(t, serverVersion, "4.1") < 0
}

func TestMongosPinning(t *testing.T) {
	dbName := "admin"
	dbAdmin := createTestDatabase(t, &dbName)
	version, err := getServerVersion(dbAdmin)
	require.NoError(t, err)

	mongodbURI := testutil.ConnString(t)
	opts := options.Client().ApplyURI(mongodbURI.String()).SetLocalThreshold(time.Second)
	hosts := opts.Hosts

	if shouldSkipMongosPinningTests(t, version) || len(hosts) < 2 {
		t.Skip("Not enough mongoses")
	}

	client, err := Connect(ctx, opts)
	require.NoError(t, err)
	defer func() { _ = client.Disconnect(ctx) }()
	db := client.Database("TestMongosPinning")

	t.Run("unpinForNextTransaction", func(t *testing.T) {
		collName := "unpinForNextTransaction"
		db.RunCommand(
			context.Background(),
			bson.D{{"drop", collName}},
		)

		coll := db.Collection(collName)
		_, err = coll.InsertOne(ctx, bson.D{{"x", 1}})
		require.NoError(t, err)

		addresses := map[string]struct{}{}
		err = client.UseSession(ctx, func(sctx SessionContext) error {
			require.NoError(t, sctx.StartTransaction(options.Transaction()))

			_, err = coll.InsertOne(sctx, bson.D{{"x", 1}})
			require.NoError(t, err)

			require.NoError(t, sctx.CommitTransaction(sctx))

			for i := 0; i < 50; i++ {
				require.NoError(t, sctx.StartTransaction(options.Transaction()))

				cursor, err := coll.Find(context.Background(), bson.D{})
				require.NoError(t, err)
				require.True(t, cursor.Next(context.Background()))

				descConn, err := cursor.bc.Server().Connection(ctx)
				require.NoError(t, err)
				addresses[descConn.Description().Addr.String()] = struct{}{}
				require.NoError(t, descConn.Close())

				require.NoError(t, sctx.CommitTransaction(sctx))
			}
			return nil
		})
		require.NoError(t, err)
		require.True(t, len(addresses) > 1)
	})
	t.Run("unpinForNonTransactionOperation", func(t *testing.T) {
		collName := "unpinForNonTransaction"
		db.RunCommand(
			context.Background(),
			bson.D{{"drop", collName}},
		)

		coll := db.Collection(collName)
		_, err = coll.InsertOne(ctx, bson.D{{"x", 1}})
		require.NoError(t, err)

		addresses := map[string]bool{}
		err = client.UseSession(ctx, func(sctx SessionContext) error {
			require.NoError(t, sctx.StartTransaction(options.Transaction()))

			_, err = coll.InsertOne(sctx, bson.D{{"x", 1}})
			require.NoError(t, err)

			require.NoError(t, sctx.CommitTransaction(sctx))

			for i := 0; i < 50; i++ {
				cursor, err := coll.Find(context.Background(), bson.D{})
				require.NoError(t, err)
				require.True(t, cursor.Next(context.Background()))

				descConn, err := cursor.bc.Server().Connection(ctx)
				require.NoError(t, err)
				addresses[descConn.Description().Addr.String()] = true
				require.NoError(t, descConn.Close())
			}
			return nil
		})
		require.NoError(t, err)
		require.True(t, len(addresses) > 1)
	})
}
