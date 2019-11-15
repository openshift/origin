// Copyright (C) MongoDB, Inc. 2017-present.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may
// not use this file except in compliance with the License. You may obtain
// a copy of the License at http://www.apache.org/licenses/LICENSE-2.0

package mongo

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"log"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func Example_clientSideEncryption() {
	// This would have to be the same master key that was used to create the encryption key
	localKey := make([]byte, 96)
	if _, err := rand.Read(localKey); err != nil {
		log.Fatal(err)
	}
	kmsProviders := map[string]map[string]interface{}{
		"local": {
			"key": localKey,
		},
	}
	keyVaultNamespace := "admin.datakeys"

	uri := "mongodb://localhost:27017"
	autoEncryptionOpts := options.AutoEncryption().
		SetKeyVaultNamespace(keyVaultNamespace).
		SetKmsProviders(kmsProviders)
	clientOpts := options.Client().ApplyURI(uri).SetAutoEncryptionOptions(autoEncryptionOpts)
	client, err := Connect(context.TODO(), clientOpts)
	if err != nil {
		log.Fatalf("Connect error: %v", err)
	}
	defer func() {
		if err = client.Disconnect(context.TODO()); err != nil {
			log.Fatalf("Disconnect error: %v", err)
		}
	}()

	collection := client.Database("test").Collection("coll")
	if err := collection.Drop(context.TODO()); err != nil {
		log.Fatalf("Collection.Drop error: %v", err)
	}

	if _, err = collection.InsertOne(context.TODO(), bson.D{{"encryptedField", "123456789"}}); err != nil {
		log.Fatalf("InsertOne error: %v", err)
	}
	res, err := collection.FindOne(context.TODO(), bson.D{}).DecodeBytes()
	if err != nil {
		log.Fatalf("FindOne error: %v", err)
	}
	fmt.Println(res)
}

func Example_clientSideEncryptionCreateKey() {
	keyVaultNamespace := "admin.datakeys"
	uri := "mongodb://localhost:27017"
	// kmsProviders would have to be populated with the correct KMS provider information before it's used
	var kmsProviders map[string]map[string]interface{}

	// Create Client and ClientEncryption
	clientEncryptionOpts := options.ClientEncryption().
		SetKeyVaultNamespace(keyVaultNamespace).
		SetKmsProviders(kmsProviders)
	keyVaultClient, err := Connect(context.TODO(), options.Client().ApplyURI(uri))
	if err != nil {
		log.Fatalf("Connect error for keyVaultClient: %v", err)
	}
	clientEnc, err := NewClientEncryption(keyVaultClient, clientEncryptionOpts)
	if err != nil {
		log.Fatalf("NewClientEncryption error: %v", err)
	}
	defer func() {
		// this will disconnect the keyVaultClient as well
		if err = clientEnc.Close(context.TODO()); err != nil {
			log.Fatalf("Close error: %v", err)
		}
	}()

	// Create a new data key and encode it as base64
	dataKeyID, err := clientEnc.CreateDataKey(context.TODO(), "local")
	if err != nil {
		log.Fatalf("CreateDataKey error: %v", err)
	}
	dataKeyBase64 := base64.StdEncoding.EncodeToString(dataKeyID.Data)

	// Create a JSON schema using the new data key. This schema could also be written in a separate file and read in
	// using I/O functions.
	schema := `{
		"properties": {
			"encryptedField": {
				"encrypt": {
					"keyId": [{
						"$binary": {
							"base64": "%s",
							"subType": "04"
						}
					}],
					"bsonType": "string",
					"algorithm": "AEAD_AES_256_CBC_HMAC_SHA_512-Deterministic"
				}
			}
		},
		"bsonType": "object"
	}`
	schema = fmt.Sprintf(schema, dataKeyBase64)
	var schemaDoc bson.Raw
	if err = bson.UnmarshalExtJSON([]byte(schema), true, &schemaDoc); err != nil {
		log.Fatalf("UnmarshalExtJSON error: %v", err)
	}

	// Configure a Client with auto encryption using the new schema
	dbName := "test"
	collName := "coll"
	schemaMap := map[string]interface{}{
		dbName + "." + collName: schemaDoc,
	}
	autoEncryptionOpts := options.AutoEncryption().
		SetKmsProviders(kmsProviders).
		SetKeyVaultNamespace(keyVaultNamespace).
		SetSchemaMap(schemaMap)
	client, err := Connect(context.TODO(), options.Client().ApplyURI(uri).SetAutoEncryptionOptions(autoEncryptionOpts))
	if err != nil {
		log.Fatalf("Connect error for encrypted client: %v", err)
	}

	// Use client for operations.

	if err = client.Disconnect(context.TODO()); err != nil {
		log.Fatalf("Disconnect error: %v", err)
	}
}
