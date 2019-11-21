// Copyright (C) MongoDB, Inc. 2017-present.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may
// not use this file except in compliance with the License. You may obtain
// a copy of the License at http://www.apache.org/licenses/LICENSE-2.0

package mongo

import (
	"context"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.mongodb.org/mongo-driver/mongo/readconcern"
	"go.mongodb.org/mongo-driver/mongo/readpref"
	"go.mongodb.org/mongo-driver/mongo/writeconcern"
)

func ExampleClient_Connect() {
	client, err := NewClient(options.Client().ApplyURI("mongodb://foo:bar@localhost:27017"))
	if err != nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	err = client.Connect(ctx)
	if err != nil {
		return
	}

	return
}

func TestClient(t *testing.T) {
	t.Run("Can configure LocalThreshold", func(t *testing.T) {
		t.Run("Should have default LocalThreshold set", func(t *testing.T) {
			client := new(Client)
			err := client.configure(options.Client())
			noerr(t, err)
			want, got := defaultLocalThreshold, client.localThreshold
			if got != want {
				t.Errorf("Default LocalThreshold not set on client. got %v; want %v", got, want)
			}
		})
		opts := options.Client().SetLocalThreshold(10 * time.Second)
		client := new(Client)
		err := client.configure(opts)
		noerr(t, err)
		want, got := 10*time.Second, client.localThreshold
		if got != want {
			t.Errorf("Couldn't configure LocalThreshold. got %v; want %v", got, want)
		}
	})
	t.Run("Can configure ReadConcern", func(t *testing.T) {
		opts := options.Client().SetReadConcern(readconcern.Majority())
		client := new(Client)
		err := client.configure(opts)
		noerr(t, err)
		wantT, wantData, err := readconcern.Majority().MarshalBSONValue()
		noerr(t, err)
		gotT, gotData, err := client.readConcern.MarshalBSONValue()
		noerr(t, err)
		want, got := bson.RawValue{Type: wantT, Value: wantData}, bson.RawValue{Type: gotT, Value: gotData}
		if !cmp.Equal(got, want) {
			t.Errorf("ReadConcern mode not set correctly. got %v; want %v", got, want)
		}
	})
	t.Run("Can configure ReadPreference", func(t *testing.T) {
		want, err := readpref.New(readpref.SecondaryMode)
		noerr(t, err)
		opts := options.Client().SetReadPreference(want)
		client := new(Client)
		err = client.configure(opts)
		noerr(t, err)
		got := client.readPreference
		if got == nil || got.Mode() != want.Mode() {
			t.Errorf("Couldn't configure ReadPreference. got %v; want %v", got, want)
		}
	})
	t.Run("Can configure RetryWrites", func(t *testing.T) {
		opts := options.Client().SetRetryWrites(true)
		client := new(Client)
		err := client.configure(opts)
		noerr(t, err)
		want, got := true, client.retryWrites
		if got != want {
			t.Errorf("Couldn't configure RetryWrites. got %v; want %v", got, want)
		}
	})
	t.Run("Can configure WriteConcern", func(t *testing.T) {
		wc := writeconcern.New(writeconcern.WMajority())
		opts := options.Client().SetWriteConcern(wc)
		client := new(Client)
		err := client.configure(opts)
		noerr(t, err)
		wantT, wantData, err := wc.MarshalBSONValue()
		noerr(t, err)
		gotT, gotData, err := client.writeConcern.MarshalBSONValue()
		noerr(t, err)
		want, got := bson.RawValue{Type: wantT, Value: wantData}, bson.RawValue{Type: gotT, Value: gotData}
		if !got.Equal(want) {
			t.Errorf("Couldn't configure WriteConcern. got %v; want %v", got, want)
		}
	})
}
