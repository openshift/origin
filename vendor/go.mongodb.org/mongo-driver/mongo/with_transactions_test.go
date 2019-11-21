// Copyright (C) MongoDB, Inc. 2017-present.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may
// not use this file except in compliance with the License. You may obtain
// a copy of the License at http://www.apache.org/licenses/LICENSE-2.0

package mongo

import (
	"context"
	"errors"
	"os"
	"path"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/internal/testutil"
	testhelpers "go.mongodb.org/mongo-driver/internal/testutil/helpers"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.mongodb.org/mongo-driver/mongo/readconcern"
	"go.mongodb.org/mongo-driver/mongo/readpref"
	"go.mongodb.org/mongo-driver/x/mongo/driver"
)

const convenientTransactionTestsDir = "../data/convenient-transactions"

type withTransactionArgs struct {
	Callback *struct {
		Operations []*transOperation `json:"operations"`
	} `json:"callback"`
	Options map[string]interface{} `json:"options"`
}

// test case for all TransactionSpec tests
func TestConvTransactionSpec(t *testing.T) {
	for _, file := range testhelpers.FindJSONFilesInDir(t, convenientTransactionTestsDir) {
		runTransactionTestFile(t, path.Join(convenientTransactionTestsDir, file))
	}
}

func runWithTransactionOperations(t *testing.T, operations []*transOperation, sess *sessionImpl, collName string, db *Database) error {
	for _, op := range operations {
		if op.Name == "count" {
			t.Skip("count has been deprecated")
		}

		// Arguments aren't marshaled directly into a map because runcommand
		// needs to convert them into BSON docs.  We convert them to a map here
		// for getting the session and for all other collection operations
		op.ArgMap = getArgMap(t, op.Arguments)

		// create collection with default read preference Primary (needed to prevent server selection fail)
		coll := db.Collection(collName, options.Collection().SetReadPreference(readpref.Primary()).SetReadConcern(readconcern.Local()))
		addCollectionOptions(coll, op.CollectionOptions)

		// execute the command on given object
		var err error
		switch op.Object {
		case "session0":
			err = executeSessionOperation(t, op, sess, collName, db)
		case "collection":
			err = executeCollectionOperation(t, op, sess, coll)
		}

		if err != nil {
			return err
		}
	}
	return nil
}

func TestConvenientTransactions(t *testing.T) {
	cs := testutil.ConnString(t)
	opts := options.Client().ApplyURI(cs.String())
	if os.Getenv("TOPOLOGY") == "sharded_cluster" {
		opts.SetHosts([]string{opts.Hosts[0]})
	}
	client, err := Connect(context.Background(), opts)
	require.NoError(t, err)
	defer func() { _ = client.Disconnect(context.Background()) }()
	dbName := "TestConvenientTransactions"
	db := client.Database(dbName)

	dbAdmin := client.Database("admin")
	version, err := getServerVersion(dbAdmin)
	require.NoError(t, err)

	if compareVersions(t, version, "4.1") < 0 || os.Getenv("TOPOLOGY") == "server" {
		t.Skip()
	}

	t.Run("CallbackRaisesCustomError", func(t *testing.T) {
		collName := "unpinForNextTransaction"
		db.RunCommand(
			context.Background(),
			bson.D{{"drop", collName}},
		)

		coll := db.Collection(collName)
		_, err = coll.InsertOne(ctx, bson.D{{"x", 1}})
		testErr := errors.New("Test Error")

		sess, err := client.StartSession()
		require.NoError(t, err)
		defer sess.EndSession(context.Background())
		_, err = sess.WithTransaction(context.Background(), func(sessCtx SessionContext) (interface{}, error) {
			return nil, testErr
		})
		require.Error(t, err)
		require.Equal(t, err, testErr)
	})
	t.Run("CallbackReturnsAValue", func(t *testing.T) {
		collName := "CallbackReturnsAValue"
		db.RunCommand(
			context.Background(),
			bson.D{{"drop", collName}},
		)

		coll := db.Collection(collName)
		_, err = coll.InsertOne(ctx, bson.D{{"x", 1}})

		sess, err := client.StartSession()
		require.NoError(t, err)
		defer sess.EndSession(context.Background())
		res, err := sess.WithTransaction(context.Background(), func(sessCtx SessionContext) (interface{}, error) {
			return false, nil
		})
		require.NoError(t, err)
		resBool, ok := res.(bool)
		require.True(t, ok)
		require.False(t, resBool)
	})
	t.Run("RetryTimeoutEnforced", func(t *testing.T) {
		withTransactionTimeout = time.Second

		collName := "RetryTimeoutEnforced"
		db.RunCommand(
			context.Background(),
			bson.D{{"drop", collName}},
		)

		coll := db.Collection(collName)
		_, err = coll.InsertOne(ctx, bson.D{{"x", 1}})

		t.Run("CallbackWithTransientTransactionError", func(t *testing.T) {
			sess, err := client.StartSession()
			require.NoError(t, err)
			defer sess.EndSession(context.Background())
			_, err = sess.WithTransaction(context.Background(), func(sessCtx SessionContext) (interface{}, error) {
				return nil, CommandError{Name: "test Error", Labels: []string{driver.TransientTransactionError}}
			})
			require.Error(t, err)
			cmdErr, ok := err.(CommandError)
			require.True(t, ok)
			require.True(t, cmdErr.HasErrorLabel(driver.TransientTransactionError))
		})
		t.Run("UnknownTransactionCommitResult", func(t *testing.T) {
			//set failpoint
			failpoint := bson.D{{"configureFailPoint", "failCommand"},
				{"mode", "alwaysOn"},
				{"data", bson.D{{"failCommands", bson.A{"commitTransaction"}}, {"closeConnection", true}}}}
			require.NoError(t, dbAdmin.RunCommand(ctx, failpoint).Err())
			defer func() {
				require.NoError(t, dbAdmin.RunCommand(ctx, bson.D{
					{"configureFailPoint", "failCommand"},
					{"mode", "off"},
				}).Err())
			}()

			sess, err := client.StartSession()
			require.NoError(t, err)
			defer sess.EndSession(context.Background())
			_, err = sess.WithTransaction(context.Background(), func(sessCtx SessionContext) (interface{}, error) {
				_, err := sessCtx.Client().Database(dbName).Collection(collName).InsertOne(sessCtx, bson.D{{"x", 1}})
				return nil, err
			})
			require.Error(t, err)
			cmdErr, ok := err.(CommandError)
			require.True(t, ok)
			require.True(t, cmdErr.HasErrorLabel(driver.UnknownTransactionCommitResult))
		})
		t.Run("CommitWithTransientTransactionError", func(t *testing.T) {
			//set failpoint
			failpoint := bson.D{{"configureFailPoint", "failCommand"},
				{"mode", "alwaysOn"},
				{"data", bson.D{{"failCommands", bson.A{"commitTransaction"}}, {"errorCode", 251}}}}
			err = dbAdmin.RunCommand(ctx, failpoint).Err()
			require.NoError(t, err)
			defer func() {
				require.NoError(t, dbAdmin.RunCommand(ctx, bson.D{
					{"configureFailPoint", "failCommand"},
					{"mode", "off"},
				}).Err())
			}()

			sess, err := client.StartSession()
			require.NoError(t, err)
			defer sess.EndSession(context.Background())
			_, err = sess.WithTransaction(context.Background(), func(sessCtx SessionContext) (interface{}, error) {
				_, err := sessCtx.Client().Database(dbName).Collection(collName).InsertOne(sessCtx, bson.D{{"x", 1}})
				return nil, err
			})
			require.Error(t, err)
			cmdErr, ok := err.(CommandError)
			require.True(t, ok)
			require.True(t, cmdErr.HasErrorLabel(driver.TransientTransactionError))
		})
	})
}
