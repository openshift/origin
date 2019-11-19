// Copyright (C) MongoDB, Inc. 2017-present.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may
// not use this file except in compliance with the License. You may obtain
// a copy of the License at http://www.apache.org/licenses/LICENSE-2.0

package mongo

import (
	"context"
	"errors"
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/internal/testutil/assert"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.mongodb.org/mongo-driver/mongo/readconcern"
	"go.mongodb.org/mongo-driver/mongo/readpref"
	"go.mongodb.org/mongo-driver/mongo/writeconcern"
	"go.mongodb.org/mongo-driver/tag"
	"go.mongodb.org/mongo-driver/x/mongo/driver"
	"go.mongodb.org/mongo-driver/x/mongo/driver/description"
	"go.mongodb.org/mongo-driver/x/mongo/driver/session"
)

var bgCtx = context.Background()

func setupClient(opts ...*options.ClientOptions) *Client {
	if len(opts) == 0 {
		opts = append(opts, options.Client().ApplyURI("mongodb://localhost:27017"))
	}
	client, _ := NewClient(opts...)
	return client
}

type mockDeployment struct{}

func (md mockDeployment) SelectServer(context.Context, description.ServerSelector) (driver.Server, error) {
	return nil, nil
}

func (md mockDeployment) SupportsRetryWrites() bool {
	return false
}

func (md mockDeployment) Kind() description.TopologyKind {
	return description.Single
}

func TestClient(t *testing.T) {
	t.Run("new client", func(t *testing.T) {
		client := setupClient()
		assert.NotNil(t, client.deployment, "expected valid deployment, got nil")
	})
	t.Run("database", func(t *testing.T) {
		dbName := "foo"
		client := setupClient()
		db := client.Database(dbName)
		assert.Equal(t, dbName, db.Name(), "expected db name %v, got %v", dbName, db.Name())
		assert.Equal(t, client, db.Client(), "expected client %v, got %v", client, db.Client())
	})
	t.Run("replace topology error", func(t *testing.T) {
		client := setupClient()

		_, err := client.StartSession()
		assert.Equal(t, ErrClientDisconnected, err, "expected error %v, got %v", ErrClientDisconnected, err)

		_, err = client.ListDatabases(bgCtx, bson.D{})
		assert.Equal(t, ErrClientDisconnected, err, "expected error %v, got %v", ErrClientDisconnected, err)

		err = client.Ping(bgCtx, nil)
		assert.Equal(t, ErrClientDisconnected, err, "expected error %v, got %v", ErrClientDisconnected, err)

		err = client.Disconnect(bgCtx)
		assert.Equal(t, ErrClientDisconnected, err, "expected error %v, got %v", ErrClientDisconnected, err)

		_, err = client.Watch(bgCtx, []bson.D{})
		assert.Equal(t, ErrClientDisconnected, err, "expected error %v, got %v", ErrClientDisconnected, err)
	})
	t.Run("nil document error", func(t *testing.T) {
		// manually set session pool to non-nil because Watch will return ErrClientDisconnected
		client := setupClient()
		client.sessionPool = &session.Pool{}

		_, err := client.Watch(bgCtx, nil)
		watchErr := errors.New("can only transform slices and arrays into aggregation pipelines, but got invalid")
		assert.Equal(t, watchErr, err, "expected error %v, got %v", watchErr, err)

		_, err = client.ListDatabases(bgCtx, nil)
		assert.Equal(t, ErrNilDocument, err, "expected error %v, got %v", ErrNilDocument, err)

		_, err = client.ListDatabaseNames(bgCtx, nil)
		assert.Equal(t, ErrNilDocument, err, "expected error %v, got %v", ErrNilDocument, err)
	})
	t.Run("read preference", func(t *testing.T) {
		t.Run("absent", func(t *testing.T) {
			client := setupClient()
			gotMode := client.readPreference.Mode()
			wantMode := readpref.PrimaryMode
			assert.Equal(t, gotMode, wantMode, "expected mode %v, got %v", wantMode, gotMode)
			_, flag := client.readPreference.MaxStaleness()
			assert.False(t, flag, "expected max staleness to not be set but was")
		})
		t.Run("specified", func(t *testing.T) {
			tags := []tag.Set{
				{
					tag.Tag{
						Name:  "one",
						Value: "1",
					},
				},
				{
					tag.Tag{
						Name:  "two",
						Value: "2",
					},
				},
			}
			cs := "mongodb://localhost:27017/"
			cs += "?readpreference=secondary&readPreferenceTags=one:1&readPreferenceTags=two:2&maxStaleness=5"

			client := setupClient(options.Client().ApplyURI(cs))
			gotMode := client.readPreference.Mode()
			assert.Equal(t, gotMode, readpref.SecondaryMode, "expected mode %v, got %v", readpref.SecondaryMode, gotMode)
			gotTags := client.readPreference.TagSets()
			assert.Equal(t, gotTags, tags, "expected tags %v, got %v", tags, gotTags)
			gotStaleness, flag := client.readPreference.MaxStaleness()
			assert.True(t, flag, "expected max staleness to be set but was not")
			wantStaleness := time.Duration(5) * time.Second
			assert.Equal(t, gotStaleness, wantStaleness, "expected staleness %v, got %v", wantStaleness, gotStaleness)
		})
	})
	t.Run("custom deployment", func(t *testing.T) {
		t.Run("success", func(t *testing.T) {
			client := setupClient(&options.ClientOptions{Deployment: mockDeployment{}})
			_, ok := client.deployment.(mockDeployment)
			assert.True(t, ok, "expected deployment type %T, got %T", mockDeployment{}, client.deployment)
		})
		t.Run("error", func(t *testing.T) {
			errmsg := "cannot specify topology or server options with a deployment"

			t.Run("specify topology options", func(t *testing.T) {
				opts := &options.ClientOptions{Deployment: mockDeployment{}}
				opts.SetServerSelectionTimeout(1 * time.Second)
				_, err := NewClient(opts)
				assert.NotNil(t, err, "expected NewClient error, got nil")
				assert.Equal(t, errmsg, err.Error(), "expected error %v, got %v", errmsg, err.Error())
			})
			t.Run("specify server options", func(t *testing.T) {
				opts := &options.ClientOptions{Deployment: mockDeployment{}}
				opts.SetMinPoolSize(1)
				_, err := NewClient(opts)
				assert.NotNil(t, err, "expected NewClient error, got nil")
				assert.Equal(t, errmsg, err.Error(), "expected error %v, got %v", errmsg, err.Error())
			})
		})
	})
	t.Run("localThreshold", func(t *testing.T) {
		testCases := []struct {
			name              string
			opts              *options.ClientOptions
			expectedThreshold time.Duration
		}{
			{"default", options.Client(), defaultLocalThreshold},
			{"custom", options.Client().SetLocalThreshold(10 * time.Second), 10 * time.Second},
		}
		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				client := setupClient(tc.opts)
				assert.Equal(t, tc.expectedThreshold, client.localThreshold,
					"expected localThreshold %v, got %v", tc.expectedThreshold, client.localThreshold)
			})
		}
	})
	t.Run("read concern", func(t *testing.T) {
		rc := readconcern.Majority()
		client := setupClient(options.Client().SetReadConcern(rc))
		assert.Equal(t, rc, client.readConcern, "expected read concern %v, got %v", rc, client.readConcern)
	})
	t.Run("retry writes", func(t *testing.T) {
		retryWritesURI := "mongodb://localhost:27017/?retryWrites=false"
		retryWritesErrorURI := "mongodb://localhost:27017/?retryWrites=foobar"

		testCases := []struct {
			name          string
			opts          *options.ClientOptions
			expectErr     bool
			expectedRetry bool
		}{
			{"default", options.Client(), false, true},
			{"custom options", options.Client().SetRetryWrites(false), false, false},
			{"custom URI", options.Client().ApplyURI(retryWritesURI), false, false},
			{"custom URI error", options.Client().ApplyURI(retryWritesErrorURI), true, false},
		}
		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				client, err := NewClient(tc.opts)
				if tc.expectErr {
					assert.NotNil(t, err, "expected error, got nil")
					return
				}
				assert.Nil(t, err, "configuration error: %v", err)
				assert.Equal(t, tc.expectedRetry, client.retryWrites, "expected retryWrites %v, got %v",
					tc.expectedRetry, client.retryWrites)
			})
		}
	})
	t.Run("retry reads", func(t *testing.T) {
		retryReadsURI := "mongodb://localhost:27017/?retryReads=false"
		retryReadsErrorURI := "mongodb://localhost:27017/?retryReads=foobar"

		testCases := []struct {
			name          string
			opts          *options.ClientOptions
			expectErr     bool
			expectedRetry bool
		}{
			{"default", options.Client(), false, true},
			{"custom options", options.Client().SetRetryReads(false), false, false},
			{"custom URI", options.Client().ApplyURI(retryReadsURI), false, false},
			{"custom URI error", options.Client().ApplyURI(retryReadsErrorURI), true, false},
		}
		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				client, err := NewClient(tc.opts)
				if tc.expectErr {
					assert.NotNil(t, err, "expected error, got nil")
					return
				}
				assert.Nil(t, err, "configuration error: %v", err)
				assert.Equal(t, tc.expectedRetry, client.retryReads, "expected retryReads %v, got %v",
					tc.expectedRetry, client.retryReads)
			})
		}
	})
	t.Run("write concern", func(t *testing.T) {
		wc := writeconcern.New(writeconcern.WMajority())
		client := setupClient(options.Client().SetWriteConcern(wc))
		assert.Equal(t, wc, client.writeConcern, "mismatch; expected write concern %v, got %v", wc, client.writeConcern)
	})
}
