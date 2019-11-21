// Copyright (C) MongoDB, Inc. 2017-present.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may
// not use this file except in compliance with the License. You may obtain
// a copy of the License at http://www.apache.org/licenses/LICENSE-2.0

package mongo

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path"
	"reflect"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/bsoncodec"
	"go.mongodb.org/mongo-driver/bson/bsonrw"
	"go.mongodb.org/mongo-driver/event"
	"go.mongodb.org/mongo-driver/internal/testutil"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.mongodb.org/mongo-driver/mongo/readpref"
	"go.mongodb.org/mongo-driver/mongo/writeconcern"
	"go.mongodb.org/mongo-driver/tag"
	"go.mongodb.org/mongo-driver/x/bsonx"
	"go.mongodb.org/mongo-driver/x/bsonx/bsoncore"
	"go.mongodb.org/mongo-driver/x/mongo/driver"
	"go.mongodb.org/mongo-driver/x/mongo/driver/address"
	"go.mongodb.org/mongo-driver/x/mongo/driver/connstring"
	"go.mongodb.org/mongo-driver/x/mongo/driver/description"
	"go.mongodb.org/mongo-driver/x/mongo/driver/drivertest"
	"go.mongodb.org/mongo-driver/x/mongo/driver/operation"
	"go.mongodb.org/mongo-driver/x/mongo/driver/session"
	"go.mongodb.org/mongo-driver/x/mongo/driver/uuid"
)

func createTestClient(t *testing.T) *Client {
	id, _ := uuid.New()
	return &Client{
		id:             id,
		topology:       testutil.Topology(t),
		connString:     testutil.ConnString(t),
		readPreference: readpref.Primary(),
		clock:          &session.ClusterClock{},
		registry:       bson.DefaultRegistry,
		retryWrites:    true,
	}
}

func createTestClientWithConnstring(t *testing.T, cs connstring.ConnString) *Client {
	id, _ := uuid.New()
	return &Client{
		id:             id,
		topology:       testutil.TopologyWithConnString(t, cs),
		connString:     cs,
		readPreference: readpref.Primary(),
		clock:          &session.ClusterClock{},
		registry:       bson.DefaultRegistry,
	}
}

func skipIfBelow30(t *testing.T) {
	serverVersion, err := getServerVersion(createTestDatabase(t, nil))
	require.NoError(t, err)

	if compareVersions(t, serverVersion, "3.0") < 0 {
		t.Skip()
	}
}

func TestNewClient(t *testing.T) {
	t.Parallel()

	c := createTestClient(t)
	require.NotNil(t, c.topology)
}

func TestClient_Database(t *testing.T) {
	t.Parallel()

	dbName := "foo"

	c := createTestClient(t)
	db := c.Database(dbName)
	require.Equal(t, db.Name(), dbName)
	require.Exactly(t, c, db.Client())
}

type NewCodec struct {
	ID int64 `bson:"_id"`
}

func (e *NewCodec) EncodeValue(ectx bsoncodec.EncodeContext, vw bsonrw.ValueWriter, val reflect.Value) error {
	return vw.WriteInt64(val.Int())
}

// DecodeValue negates the value of ID when reading
func (e *NewCodec) DecodeValue(ectx bsoncodec.DecodeContext, vr bsonrw.ValueReader, val reflect.Value) error {
	i, err := vr.ReadInt64()
	if err != nil {
		return err
	}

	val.SetInt(i * -1)
	return nil
}

func TestClientRegistryPassedToCursors(t *testing.T) {
	// register a new codec for the int64 type that does the default encoding for an int64 and negates the value when
	// decoding

	rb := bson.NewRegistryBuilder()
	cod := &NewCodec{}
	rb.RegisterCodec(reflect.TypeOf(int64(0)), cod)

	cs := testutil.ConnString(t)
	client, err := NewClient(options.Client().ApplyURI(cs.String()).SetRegistry(rb.Build()))
	require.NoError(t, err)
	err = client.Connect(ctx)
	require.NoError(t, err)

	db := client.Database("TestRegistryDB")
	defer func() {
		_ = db.Drop(ctx)
		_ = client.Disconnect(ctx)
	}()

	coll := db.Collection("TestRegistryColl")

	_, err = coll.InsertOne(ctx, NewCodec{ID: 10})
	require.NoError(t, err)

	c, err := coll.Find(ctx, bsonx.Doc{})
	require.NoError(t, err)

	require.True(t, c.Next(ctx))

	var foundDoc NewCodec
	err = c.Decode(&foundDoc)
	require.NoError(t, err)

	require.Equal(t, foundDoc.ID, int64(-10))
}

func TestClient_TLSConnection(t *testing.T) {
	skipIfBelow30(t) // 3.0 doesn't return a security field in the serverStatus response
	t.Parallel()

	if testing.Short() {
		t.Skip()
	}

	caFile := os.Getenv("MONGO_GO_DRIVER_CA_FILE")

	if len(caFile) == 0 {
		t.Skip()
	}

	c := createTestClient(t)
	db := c.Database("test")

	var result bsonx.Doc
	err := db.RunCommand(context.Background(), bsonx.Doc{{"serverStatus", bsonx.Int32(1)}}).Decode(&result)
	require.NoError(t, err)

	security, err := result.LookupErr("security")
	require.Nil(t, err)

	require.Equal(t, security.Type(), bson.TypeEmbeddedDocument)

	_, found := security.Document().LookupErr("SSLServerSubjectName")
	require.Nil(t, found)

	_, found = security.Document().LookupErr("SSLServerHasCertificateAuthority")
	require.Nil(t, found)

}

func TestClient_X509Auth(t *testing.T) {
	t.Parallel()

	if testing.Short() {
		t.Skip()
	}

	caFile := os.Getenv("MONGO_GO_DRIVER_CA_FILE")

	if len(caFile) == 0 || os.Getenv("AUTH") == "auth" {
		t.Skip()
	}

	const user = "C=US,ST=New York,L=New York City,O=MongoDB,OU=other,CN=external"

	c := createTestClient(t)
	db := c.Database("$external")

	// We don't care if the user doesn't already exist.
	_ = db.RunCommand(
		context.Background(),
		bsonx.Doc{{"dropUser", bsonx.String(user)}},
	)

	err := db.RunCommand(
		context.Background(),
		bsonx.Doc{
			{"createUser", bsonx.String(user)},
			{"roles", bsonx.Array(bsonx.Arr{bsonx.Document(
				bsonx.Doc{{"role", bsonx.String("readWrite")}, {"db", bsonx.String("test")}},
			)})},
		},
	).Err()
	require.NoError(t, err)

	basePath := path.Join("..", "data", "certificates")
	baseConnString := testutil.ConnString(t)
	cs := fmt.Sprintf(
		"%s&sslClientCertificateKeyFile=%s&authMechanism=MONGODB-X509",
		baseConnString.String(),
		path.Join(basePath, "client.pem"),
	)

	authClient, err := NewClient(options.Client().ApplyURI(cs))
	require.NoError(t, err)

	err = authClient.Connect(context.Background())
	require.NoError(t, err)

	db = authClient.Database("test")
	var rdr bson.Raw
	rdr, err = db.RunCommand(
		context.Background(),
		bsonx.Doc{{"connectionStatus", bsonx.Int32(1)}},
	).DecodeBytes()
	require.NoError(t, err)

	users, err := rdr.LookupErr("authInfo", "authenticatedUsers")
	require.NoError(t, err)

	array := users.Array()
	elems, err := array.Elements()
	require.NoError(t, err)

	for _, v := range elems {
		rdr := v.Value().Document()
		var u struct {
			User string
			DB   string
		}

		if err := bson.Unmarshal(rdr, &u); err != nil {
			continue
		}

		if u.User == user && u.DB == "$external" {
			return
		}
	}

	t.Error("unable to find authenticated user")
}

func TestClient_ReplaceTopologyError(t *testing.T) {
	t.Parallel()

	if testing.Short() {
		t.Skip()
	}

	cs := testutil.ConnString(t)
	c, err := NewClient(options.Client().ApplyURI(cs.String()))
	require.NoError(t, err)
	require.NotNil(t, c)

	_, err = c.StartSession()
	require.Equal(t, err, ErrClientDisconnected)

	_, err = c.ListDatabases(ctx, bsonx.Doc{})
	require.Equal(t, err, ErrClientDisconnected)

	err = c.Ping(ctx, nil)
	require.Equal(t, err, ErrClientDisconnected)

	err = c.Disconnect(ctx)
	require.Equal(t, err, ErrClientDisconnected)

}

type retryableSSD struct {
	C driver.Connection
}

var _ driver.Deployment = retryableSSD{}
var _ driver.Server = retryableSSD{}

func (rssd retryableSSD) SelectServer(context.Context, description.ServerSelector) (driver.Server, error) {
	return rssd, nil
}

func (rssd retryableSSD) Kind() description.TopologyKind {
	return description.Single
}

func (rssd retryableSSD) Connection(context.Context) (driver.Connection, error) {
	return rssd.C, nil
}

func (rssd retryableSSD) SupportsRetryWrites() bool {
	return true
}

func TestRetryWritesError20Wrapped(t *testing.T) {
	serverVersion, err := getServerVersion(createTestDatabase(t, nil))
	require.NoError(t, err)

	if compareVersions(t, serverVersion, "3.6") < 0 {
		t.Skip()
	}

	idx, writeError := bsoncore.AppendDocumentStart(nil)
	writeError = bsoncore.AppendInt32Element(writeError, "ok", 1)
	elemIdx, elem := bsoncore.AppendDocumentStart(nil)
	elem = bsoncore.AppendInt32Element(elem, "index", 0)
	elem = bsoncore.AppendStringElement(elem, "errmsg", "Transaction numbers")
	elem = bsoncore.AppendInt32Element(elem, "code", 20)
	elem, _ = bsoncore.AppendDocumentEnd(elem, elemIdx)
	writeErrorsIdx, writeErrors := bsoncore.AppendArrayStart(nil)
	writeErrors = bsoncore.AppendDocumentElement(writeErrors, strconv.Itoa(0), elem)
	writeErrors, _ = bsoncore.AppendArrayEnd(writeErrors, writeErrorsIdx)
	writeError = bsoncore.AppendArrayElement(writeError, "writeErrors", writeErrors)
	writeError, _ = bsoncore.AppendDocumentEnd(writeError, idx)

	idx, writeErrorNot20 := bsoncore.AppendDocumentStart(nil)
	writeErrorNot20 = bsoncore.AppendInt32Element(writeErrorNot20, "ok", 1)
	elemIdx, elem = bsoncore.AppendDocumentStart(nil)
	elem = bsoncore.AppendInt32Element(elem, "index", 0)
	elem = bsoncore.AppendStringElement(elem, "errmsg", "Transaction numbers")
	elem = bsoncore.AppendInt32Element(elem, "code", 19)
	elem, _ = bsoncore.AppendDocumentEnd(elem, elemIdx)
	writeErrorsIdx, writeErrors = bsoncore.AppendArrayStart(nil)
	writeErrors = bsoncore.AppendDocumentElement(writeErrors, strconv.Itoa(0), elem)
	writeErrors, _ = bsoncore.AppendArrayEnd(writeErrors, writeErrorsIdx)
	writeErrorNot20 = bsoncore.AppendArrayElement(writeErrorNot20, "writeErrors", writeErrors)
	writeErrorNot20, _ = bsoncore.AppendDocumentEnd(writeErrorNot20, idx)

	idx, writeErrorOnly20 := bsoncore.AppendDocumentStart(nil)
	writeErrorOnly20 = bsoncore.AppendInt32Element(writeErrorOnly20, "ok", 1)
	elemIdx, elem = bsoncore.AppendDocumentStart(nil)
	elem = bsoncore.AppendInt32Element(elem, "index", 0)
	elem = bsoncore.AppendStringElement(elem, "errmsg", "something other than transaction numbers")
	elem = bsoncore.AppendInt32Element(elem, "code", 20)
	elem, _ = bsoncore.AppendDocumentEnd(elem, elemIdx)
	writeErrorsIdx, writeErrors = bsoncore.AppendArrayStart(nil)
	writeErrors = bsoncore.AppendDocumentElement(writeErrors, strconv.Itoa(0), elem)
	writeErrors, _ = bsoncore.AppendArrayEnd(writeErrors, writeErrorsIdx)
	writeErrorOnly20 = bsoncore.AppendArrayElement(writeErrorOnly20, "writeErrors", writeErrors)
	writeErrorOnly20, _ = bsoncore.AppendDocumentEnd(writeErrorOnly20, idx)

	idx, notOk := bsoncore.AppendDocumentStart(nil)
	notOk = bsoncore.AppendInt64Element(notOk, "ok", 0)
	notOk = bsoncore.AppendStringElement(notOk, "errmsg", "Transaction numbers")
	notOk = bsoncore.AppendInt32Element(notOk, "code", 20)
	notOk, _ = bsoncore.AppendDocumentEnd(notOk, idx)

	idx, not20notOK := bsoncore.AppendDocumentStart(nil)
	not20notOK = bsoncore.AppendInt64Element(not20notOK, "ok", 0)
	not20notOK = bsoncore.AppendStringElement(not20notOK, "errmsg", "Transaction numbers")
	not20notOK = bsoncore.AppendInt32Element(not20notOK, "code", 19)
	not20notOK, _ = bsoncore.AppendDocumentEnd(not20notOK, idx)

	idx, only20NotOK := bsoncore.AppendDocumentStart(nil)
	only20NotOK = bsoncore.AppendInt64Element(only20NotOK, "ok", 0)
	only20NotOK = bsoncore.AppendStringElement(only20NotOK, "errmsg", "something other than transaction numbers")
	only20NotOK = bsoncore.AppendInt32Element(only20NotOK, "code", 20)
	only20NotOK, _ = bsoncore.AppendDocumentEnd(only20NotOK, idx)

	tests := []struct {
		name                 string
		wireMessage          []byte // bsoncore byte slice
		shouldError          bool
		expectedErrorMessage string
	}{
		{"writeError", writeError, true, driver.ErrUnsupportedStorageEngine.Error()},
		{"writeError with only err code 20 and wrong err message", writeErrorOnly20, true, "write command error: [{write errors: [{something other than transaction numbers}]}, {<nil>}]"},
		{"writeError with only err code 19 and right err message", writeErrorNot20, true, "write command error: [{write errors: [{Transaction numbers}]}, {<nil>}]"},
		{"NotOkError", notOk, true, driver.ErrUnsupportedStorageEngine.Error()},
		{"NotOkError with err code 20 and wrong err message", only20NotOK, true, "something other than transaction numbers"},
		{"NotOkError with err code 19 and right err message", not20notOK, true, "Transaction numbers"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			conn := &drivertest.ChannelConn{
				Written: make(chan []byte, 1),
				Desc: description.Server{
					CanonicalAddr:         address.Address("localhost:27017"),
					MaxDocumentSize:       16777216,
					MaxMessageSize:        48000000,
					MaxBatchCount:         100000,
					SessionTimeoutMinutes: 30,
					Kind:                  description.RSPrimary,
					WireVersion: &description.VersionRange{
						Max: 8,
					},
				},
				ReadResp: make(chan []byte, 1),
			}

			conn.ReadResp <- drivertest.MakeReply(test.wireMessage)

			deployment := retryableSSD{C: conn}

			client := createTestClient(t)
			coll := client.Database("test").Collection("test")

			sess, err := client.StartSession()
			defer sess.EndSession(context.Background())
			noerr(t, err)

			idx, writeError = bsoncore.AppendDocumentStart(nil)
			writeError = bsoncore.AppendStringElement(writeError, "_id", "1")
			writeError, _ = bsoncore.AppendDocumentEnd(writeError, idx)

			op := operation.NewInsert(writeError).CommandMonitor(coll.client.monitor).ClusterClock(coll.client.clock).
				Database(coll.db.name).Collection(coll.name).
				Deployment(coll.client.topology).Deployment(deployment).Retry(driver.RetryOnce).Session(sess.(*sessionImpl).clientSession)

			err = op.Execute(context.Background())
			if test.shouldError {
				if err == nil || err.Error() != test.expectedErrorMessage {
					t.Fatalf("unexpected error occured, wanted: %v got: %v", test.expectedErrorMessage, err)
				}
				return
			}

			if err != nil {
				t.Fatalf("did not expect an error, instead recieved: %v", err)
			}

		})
	}
}

func TestClient_ListDatabases_noFilter(t *testing.T) {
	t.Parallel()

	if testing.Short() {
		t.Skip()
	}

	dbName := "listDatabases_noFilter"
	c := createTestClient(t)
	db := c.Database(dbName)
	coll := db.Collection("test")
	coll.writeConcern = writeconcern.New(writeconcern.WMajority())
	_, err := coll.InsertOne(
		context.Background(),
		bsonx.Doc{{"x", bsonx.Int32(1)}},
	)
	require.NoError(t, err)

	dbs, err := c.ListDatabases(context.Background(), bsonx.Doc{})
	require.NoError(t, err)
	found := false

	for _, db := range dbs.Databases {

		if db.Name == dbName {
			found = true
			break
		}
	}
	require.True(t, found)
}

func TestClient_ListDatabases_filter(t *testing.T) {
	t.Parallel()

	if testing.Short() {
		t.Skip()
	}

	skipIfBelow36(t)

	dbName := "listDatabases_filter"

	c := createTestClient(t)
	db := c.Database(dbName)
	coll := db.Collection("test")
	coll.writeConcern = writeconcern.New(writeconcern.WMajority())
	_, err := coll.InsertOne(
		context.Background(),
		bsonx.Doc{{"x", bsonx.Int32(1)}},
	)
	require.NoError(t, err)

	dbs, err := c.ListDatabases(
		context.Background(),
		bsonx.Doc{{"name", bsonx.Regex(dbName, "")}},
	)

	require.Equal(t, len(dbs.Databases), 1)
	require.Equal(t, dbName, dbs.Databases[0].Name)
}

func TestClient_ListDatabaseNames_noFilter(t *testing.T) {
	t.Parallel()

	if testing.Short() {
		t.Skip()
	}

	dbName := "listDatabasesNames_noFilter"

	c := createTestClient(t)
	db := c.Database(dbName)
	coll := db.Collection("test")

	coll.writeConcern = writeconcern.New(writeconcern.WMajority())
	_, err := coll.InsertOne(
		context.Background(),
		bsonx.Doc{{"x", bsonx.Int32(1)}},
	)
	require.NoError(t, err)

	dbs, err := c.ListDatabaseNames(context.Background(), bsonx.Doc{})
	found := false

	for _, name := range dbs {
		if name == dbName {
			found = true
			break
		}
	}
	require.True(t, found)
}

func TestClient_ListDatabaseNames_filter(t *testing.T) {
	t.Parallel()

	if testing.Short() {
		t.Skip()
	}

	skipIfBelow36(t)

	dbName := "listDatabasesNames_filter"

	c := createTestClient(t)
	db := c.Database(dbName)
	coll := db.Collection("test")
	coll.writeConcern = writeconcern.New(writeconcern.WMajority())
	_, err := coll.InsertOne(
		context.Background(),
		bsonx.Doc{{"x", bsonx.Int32(1)}},
	)
	require.NoError(t, err)

	dbs, err := c.ListDatabaseNames(
		context.Background(),
		bsonx.Doc{{"name", bsonx.Regex(dbName, "")}},
	)

	require.NoError(t, err)
	require.Len(t, dbs, 1)
	require.Equal(t, dbName, dbs[0])
}

func TestClient_NilDocumentError(t *testing.T) {
	t.Parallel()

	c := createTestClient(t)

	_, err := c.Watch(context.Background(), nil)
	require.Equal(t, err, errors.New("can only transform slices and arrays into aggregation pipelines, but got invalid"))

	_, err = c.ListDatabases(context.Background(), nil)
	require.Equal(t, err, ErrNilDocument)

	_, err = c.ListDatabaseNames(context.Background(), nil)
	require.Equal(t, err, ErrNilDocument)
}

func TestClient_ReadPreference(t *testing.T) {
	t.Parallel()

	if testing.Short() {
		t.Skip()
	}
	var tags = []tag.Set{
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
	baseConnString := testutil.ConnString(t)
	cs := testutil.AddOptionsToURI(baseConnString.String(), "readpreference=secondary&readPreferenceTags=one:1&readPreferenceTags=two:2&maxStaleness=5")

	c, err := NewClient(options.Client().ApplyURI(cs))
	require.NoError(t, err)
	require.NotNil(t, c)
	require.Equal(t, readpref.SecondaryMode, c.readPreference.Mode())
	require.Equal(t, tags, c.readPreference.TagSets())
	d, flag := c.readPreference.MaxStaleness()
	require.True(t, flag)
	require.Equal(t, time.Duration(5)*time.Second, d)
}

func TestClient_ReadPreferenceAbsent(t *testing.T) {
	t.Parallel()

	cs := testutil.ConnString(t)
	c, err := NewClient(options.Client().ApplyURI(cs.String()))
	require.NoError(t, err)
	require.NotNil(t, c)
	require.Equal(t, readpref.PrimaryMode, c.readPreference.Mode())
	require.Empty(t, c.readPreference.TagSets())
	_, flag := c.readPreference.MaxStaleness()
	require.False(t, flag)
}

func TestClient_CausalConsistency(t *testing.T) {
	cs := testutil.ConnString(t)
	c, err := NewClient(options.Client().ApplyURI(cs.String()))
	require.NoError(t, err)
	require.NotNil(t, c)

	err = c.Connect(ctx)
	require.NoError(t, err)

	s, err := c.StartSession(options.Session().SetCausalConsistency(true))
	sess := s.(*sessionImpl)
	require.NoError(t, err)
	require.NotNil(t, sess)
	require.True(t, sess.clientSession.Consistent)
	sess.EndSession(ctx)

	s, err = c.StartSession(options.Session().SetCausalConsistency(false))
	sess = s.(*sessionImpl)
	require.NoError(t, err)
	require.NotNil(t, sess)
	require.False(t, sess.clientSession.Consistent)
	sess.EndSession(ctx)

	s, err = c.StartSession()
	sess = s.(*sessionImpl)
	require.NoError(t, err)
	require.NotNil(t, sess)
	require.True(t, sess.clientSession.Consistent)
	sess.EndSession(ctx)
}

func TestClient_Ping_DefaultReadPreference(t *testing.T) {
	cs := testutil.ConnString(t)
	c, err := NewClient(options.Client().ApplyURI(cs.String()))
	require.NoError(t, err)
	require.NotNil(t, c)

	err = c.Connect(ctx)
	require.NoError(t, err)

	err = c.Ping(ctx, nil)
	require.NoError(t, err)
}

func TestClient_Ping_InvalidHost(t *testing.T) {
	c, err := NewClient(
		options.Client().
			SetServerSelectionTimeout(1 * time.Millisecond).
			SetHosts([]string{"not-a-valid-hostanme.wrong:12345"}),
	)
	require.NoError(t, err)
	require.NotNil(t, c)

	err = c.Connect(ctx)
	require.NoError(t, err)

	err = c.Ping(ctx, nil)
	require.NotNil(t, err)
}

func TestClient_Disconnect_NilContext(t *testing.T) {
	cs := testutil.ConnString(t)
	c, err := NewClient(options.Client().ApplyURI(cs.String()))
	require.NoError(t, err)
	err = c.Connect(nil)
	require.NoError(t, err)
	err = c.Disconnect(nil)
	require.NoError(t, err)
}

func TestClient_Watch_Disconnected(t *testing.T) {
	cs := testutil.ConnString(t)
	c, err := NewClient(options.Client().ApplyURI(cs.String()))
	require.NoError(t, err)
	change, err := c.Watch(context.Background(), []bson.D{})
	require.Nil(t, change)
	require.Equal(t, err, ErrClientDisconnected)
}

func TestEndSessions(t *testing.T) {
	skipIfBelow36(t)
	cs := testutil.ConnString(t)
	client, err := NewClient(options.Client().ApplyURI(cs.String()).SetMonitor(monitor))
	require.NoError(t, err)
	err = client.Connect(nil)
	require.NoError(t, err)

	_, err = client.ListDatabases(ctx, bsonx.Doc{})
	require.NoError(t, err)

	drainChannels()

	err = client.Disconnect(ctx)
	require.NoError(t, err)

	var started *event.CommandStartedEvent
	select {
	case started = <-startedChan:
	default:
		t.Fatalf("expected a CommandStartedEvent but none found")
	}

	require.Equal(t, "endSessions", started.CommandName)
}

func TestIsMaster(t *testing.T) {
	if os.Getenv("TOPOLOGY") != "replica_set" {
		t.Skip("Needs to run on a replica set")
	}
	cs := testutil.ConnString(t)
	client, err := NewClient(options.Client().ApplyURI(cs.String()))
	require.NoError(t, err)
	err = client.Connect(nil)
	require.NoError(t, err)

	coll := createTestCollection(t, nil, nil)
	skipIfBelow34(t, coll.db)
	_, err = coll.InsertOne(
		context.Background(),
		bsonx.Doc{{"x", bsonx.Int32(1)}},
	)
	require.NoError(t, err)

	isMaster := operation.NewIsMaster().ClusterClock(client.clock).Deployment(client.topology).
		AppName(cs.AppName).Compressors(cs.Compressors)

	err = isMaster.Execute(ctx)
	require.NoError(t, err)

	res := isMaster.Result("")
	require.False(t, res.LastWriteTime.IsZero())
}
