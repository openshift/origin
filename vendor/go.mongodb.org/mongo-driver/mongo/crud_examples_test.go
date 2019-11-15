// Copyright (C) MongoDB, Inc. 2017-present.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may
// not use this file except in compliance with the License. You may obtain
// a copy of the License at http://www.apache.org/licenses/LICENSE-2.0

package mongo_test

import (
	"context"
	"fmt"
	"log"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.mongodb.org/mongo-driver/mongo/readpref"
)

// Client examples

func ExampleClient_ListDatabaseNames() {
	var client *mongo.Client

	// use a filter to only select non-empty databases
	result, err := client.ListDatabaseNames(context.TODO(), bson.D{{"empty", false}})
	if err != nil {
		log.Fatal(err)
	}

	for _, db := range result {
		fmt.Println(db)
	}
}

func ExampleClient_Watch() {
	var client *mongo.Client

	// specify a pipeline that will only match "insert" events
	// specify the MaxAwaitTimeOption to have each attempt wait two seconds for new documents
	matchStage := bson.D{{"$match", bson.D{{"operationType", "insert"}}}}
	opts := options.ChangeStream().SetMaxAwaitTime(2 * time.Second)
	changeStream, err := client.Watch(context.TODO(), mongo.Pipeline{matchStage}, opts)
	if err != nil {
		log.Fatal(err)
	}

	// print out all change stream events in the order they're received
	// see the mongo.ChangeStream documentation for more examples of using change streams
	for changeStream.Next(context.TODO()) {
		fmt.Println(changeStream.Current)
	}
}

// Database examples

func ExampleDatabase_ListCollectionNames() {
	var db *mongo.Database

	// use a filter to only select capped collections
	result, err := db.ListCollectionNames(context.TODO(), bson.D{{"options.capped", true}})
	if err != nil {
		log.Fatal(err)
	}

	for _, coll := range result {
		fmt.Println(coll)
	}
}

func ExampleDatabase_RunCommand() {
	var db *mongo.Database

	// run an explain command to see the query plan for when a "find" is executed on collection "bar"
	// specify the ReadPreference option to explicitly set the read preference to primary
	findCmd := bson.D{{"find", "bar"}}
	command := bson.D{{"explain", findCmd}}
	opts := options.RunCmd().SetReadPreference(readpref.Primary())
	var result bson.M
	if err := db.RunCommand(context.TODO(), command, opts).Decode(&result); err != nil {
		log.Fatal(err)
	}
	fmt.Println(result)
}

func ExampleDatabase_Watch() {
	var db *mongo.Database

	// specify a pipeline that will only match "insert" events
	// specify the MaxAwaitTimeOption to have each attempt wait two seconds for new documents
	matchStage := bson.D{{"$match", bson.D{{"operationType", "insert"}}}}
	opts := options.ChangeStream().SetMaxAwaitTime(2 * time.Second)
	changeStream, err := db.Watch(context.TODO(), mongo.Pipeline{matchStage}, opts)
	if err != nil {
		log.Fatal(err)
	}

	// print out all change stream events in the order they're received
	// see the mongo.ChangeStream documentation for more examples of using change streams
	for changeStream.Next(context.TODO()) {
		fmt.Println(changeStream.Current)
	}
}

// Collection examples

func ExampleCollection_Aggregate() {
	var coll *mongo.Collection

	// specify a pipeline that will return the number of times each name appears in the collection
	// specify the MaxTime option to limit the amount of time the operation can run on the server
	groupStage := bson.D{
		{"$group", bson.D{
			{"_id", "$name"},
			{"numTimes", bson.D{
				{"$sum", 1},
			}},
		}},
	}
	opts := options.Aggregate().SetMaxTime(2 * time.Second)
	cursor, err := coll.Aggregate(context.TODO(), mongo.Pipeline{groupStage}, opts)
	if err != nil {
		log.Fatal(err)
	}

	// get a list of all returned documents and print them out
	// see the mongo.Cursor documentation for more examples of using cursors
	var results []bson.M
	if err = cursor.All(context.TODO(), &results); err != nil {
		log.Fatal(err)
	}
	for _, result := range results {
		fmt.Printf("name %v appears %v times\n", result["_id"], result["numTimes"])
	}
}

func ExampleCollection_BulkWrite() {
	var coll *mongo.Collection
	var firstID, secondID primitive.ObjectID

	// update the "email" field for two users
	// for each update, specify the Upsert option to insert a new document if a document matching the filter isn't
	// found
	// set the Ordered option to false to allow both operations to happen even if one of them errors
	firstUpdate := bson.D{{"$set", bson.D{{"email", "firstEmail@example.com"}}}}
	secondUpdate := bson.D{{"$set", bson.D{{"email", "secondEmail@example.com"}}}}
	models := []mongo.WriteModel{
		mongo.NewUpdateOneModel().SetFilter(bson.D{{"_id", firstID}}).SetUpdate(firstUpdate).SetUpsert(true),
		mongo.NewUpdateOneModel().SetFilter(bson.D{{"_id", secondID}}).SetUpdate(secondUpdate).SetUpsert(true),
	}
	opts := options.BulkWrite().SetOrdered(false)
	res, err := coll.BulkWrite(context.TODO(), models, opts)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("inserted %v and deleted %v documents\n", res.InsertedCount, res.DeletedCount)
}

func ExampleCollection_CountDocuments() {
	var coll *mongo.Collection

	// count the number of times the name "Bob" appears in the collection
	// specify the MaxTime option to limit the amount of time the operation can run on the server
	opts := options.Count().SetMaxTime(2 * time.Second)
	count, err := coll.CountDocuments(context.TODO(), bson.D{{"name", "Bob"}}, opts)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("name Bob appears in %v documents", count)
}

func ExampleCollection_DeleteMany() {
	var coll *mongo.Collection

	// delete all documents in which the "name" field is "Bob" or "bob"
	// specify the Collation option to provide a collation that will ignore case for string comparisons
	opts := options.Delete().SetCollation(&options.Collation{
		Locale:    "en_US",
		Strength:  1,
		CaseLevel: false,
	})
	res, err := coll.DeleteMany(context.TODO(), bson.D{{"name", "bob"}}, opts)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("deleted %v documents\n", res.DeletedCount)
}

func ExampleCollection_DeleteOne() {
	var coll *mongo.Collection

	// delete at most one document in which the "name" field is "Bob" or "bob"
	// specify the SetCollation option to provide a collation that will ignore case for string comparisons
	opts := options.Delete().SetCollation(&options.Collation{
		Locale:    "en_US",
		Strength:  1,
		CaseLevel: false,
	})
	res, err := coll.DeleteOne(context.TODO(), bson.D{{"name", "bob"}}, opts)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("deleted %v documents\n", res.DeletedCount)
}

func ExampleCollection_Distinct() {
	var coll *mongo.Collection

	// find all unique values for the "name" field for documents in which the "age" field is greater than 25
	// specify the MaxTime option to limit the amount of time the operation can run on the server
	filter := bson.D{{"age", bson.D{{"$gt", 25}}}}
	opts := options.Distinct().SetMaxTime(2 * time.Second)
	values, err := coll.Distinct(context.TODO(), "name", filter, opts)
	if err != nil {
		log.Fatal(err)
	}

	for _, value := range values {
		fmt.Println(value)
	}
}

func ExampleCollection_EstimatedDocumentCount() {
	var coll *mongo.Collection

	// get and print an estimated of the number of documents in the collection
	// specify the MaxTime option to limit the amount of time the operation can run on the server
	opts := options.EstimatedDocumentCount().SetMaxTime(2 * time.Second)
	count, err := coll.EstimatedDocumentCount(context.TODO(), opts)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("estimated document count: %v", count)
}

func ExampleCollection_Find() {
	var coll *mongo.Collection

	// find all documents in which the "name" field is "Bob"
	// specify the Sort option to sort the returned documents by age in ascending order
	opts := options.Find().SetSort(bson.D{{"age", 1}})
	cursor, err := coll.Find(context.TODO(), bson.D{{"name", "Bob"}}, opts)
	if err != nil {
		log.Fatal(err)
	}

	// get a list of all returned documents and print them out
	// see the mongo.Cursor documentation for more examples of using cursors
	var results []bson.M
	if err = cursor.All(context.TODO(), &results); err != nil {
		log.Fatal(err)
	}
	for _, result := range results {
		fmt.Println(result)
	}
}

func ExampleCollection_FindOne() {
	var coll *mongo.Collection
	var id primitive.ObjectID

	// find the document for which the _id field matches id
	// specify the Sort option to sort the documents by age
	// the first document in the sorted order will be returned
	opts := options.FindOne().SetSort(bson.D{{"age", 1}})
	var result bson.M
	err := coll.FindOne(context.TODO(), bson.D{{"_id", id}}, opts).Decode(&result)
	if err != nil {
		// ErrNoDocuments means that the filter did not match any documents in the collection
		if err == mongo.ErrNoDocuments {
			return
		}
		log.Fatal(err)
	}
	fmt.Printf("found document %v", result)
}

func ExampleCollection_FindOneAndDelete() {
	var coll *mongo.Collection
	var id primitive.ObjectID

	// find and delete the document for which the _id field matches id
	// specify the Projection option to only include the name and age fields in the returned document
	opts := options.FindOneAndDelete().SetProjection(bson.D{{"name", 1}, {"age", 1}})
	var deletedDocument bson.M
	err := coll.FindOneAndDelete(context.TODO(), bson.D{{"_id", id}}, opts).Decode(&deletedDocument)
	if err != nil {
		// ErrNoDocuments means that the filter did not match any documents in the collection
		if err == mongo.ErrNoDocuments {
			return
		}
		log.Fatal(err)
	}
	fmt.Printf("deleted document %v", deletedDocument)
}

func ExampleCollection_FindOneAndReplace() {
	var coll *mongo.Collection
	var id primitive.ObjectID

	// find the document for which the _id field matches id and add a field called "location"
	// specify the Upsert option to insert a new document if a document matching the filter isn't found
	opts := options.FindOneAndReplace().SetUpsert(true)
	filter := bson.D{{"_id", id}}
	replacement := bson.D{{"location", "NYC"}}
	var replacedDocument bson.M
	err := coll.FindOneAndReplace(context.TODO(), filter, replacement, opts).Decode(&replacedDocument)
	if err != nil {
		// ErrNoDocuments means that the filter did not match any documents in the collection
		if err == mongo.ErrNoDocuments {
			return
		}
		log.Fatal(err)
	}
	fmt.Printf("replaced document %v", replacedDocument)
}

func ExampleCollection_FindOneAndUpdate() {
	var coll *mongo.Collection
	var id primitive.ObjectID

	// find the document for which the _id field matches id and set the email to "newemail@example.com"
	// specify the Upsert option to insert a new document if a document matching the filter isn't found
	opts := options.FindOneAndUpdate().SetUpsert(true)
	filter := bson.D{{"_id", id}}
	update := bson.D{{"$set", bson.D{{"email", "newemail@example.com"}}}}
	var updatedDocument bson.M
	err := coll.FindOneAndUpdate(context.TODO(), filter, update, opts).Decode(&updatedDocument)
	if err != nil {
		// ErrNoDocuments means that the filter did not match any documents in the collection
		if err == mongo.ErrNoDocuments {
			return
		}
		log.Fatal(err)
	}
	fmt.Printf("updated document %v", updatedDocument)
}

func ExampleCollection_InsertMany() {
	var coll *mongo.Collection

	// insert documents {name: "Alice"} and {name: "Bob"}
	// set the Ordered option to false to allow both operations to happen even if one of them errors
	docs := []interface{}{
		bson.D{{"name", "Alice"}},
		bson.D{{"name", "Bob"}},
	}
	opts := options.InsertMany().SetOrdered(false)
	res, err := coll.InsertMany(context.TODO(), docs, opts)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("inserted documents with IDs %v\n", res.InsertedIDs)
}

func ExampleCollection_InsertOne() {
	var coll *mongo.Collection

	// insert the document {name: "Alice"}
	res, err := coll.InsertOne(context.TODO(), bson.D{{"name", "Alice"}})
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("inserted document with ID %v\n", res.InsertedID)
}

func ExampleCollection_ReplaceOne() {
	var coll *mongo.Collection
	var id primitive.ObjectID

	// find the document for which the _id field matches id and add a field called "location"
	// specify the Upsert option to insert a new document if a document matching the filter isn't found
	opts := options.Replace().SetUpsert(true)
	filter := bson.D{{"_id", id}}
	replacement := bson.D{{"location", "NYC"}}
	result, err := coll.ReplaceOne(context.TODO(), filter, replacement, opts)
	if err != nil {
		log.Fatal(err)
	}

	if result.MatchedCount != 0 {
		fmt.Println("matched and replaced an existing document")
		return
	}
	if result.UpsertedCount != 0 {
		fmt.Printf("inserted a new document with ID %v\n", result.UpsertedID)
	}
}

func ExampleCollection_UpdateMany() {
	var coll *mongo.Collection

	// increment the age for all users whose birthday is today
	today := time.Now().Format("01-01-1970")
	filter := bson.D{{"birthday", today}}
	update := bson.D{{"$inc", bson.D{{"age", 1}}}}

	result, err := coll.UpdateMany(context.TODO(), filter, update)
	if err != nil {
		log.Fatal(err)
	}

	if result.MatchedCount != 0 {
		fmt.Println("matched and replaced an existing document")
		return
	}
}

func ExampleCollection_UpdateOne() {
	var coll *mongo.Collection
	var id primitive.ObjectID

	// find the document for which the _id field matches id and set the email to "newemail@example.com"
	// specify the Upsert option to insert a new document if a document matching the filter isn't found
	opts := options.Update().SetUpsert(true)
	filter := bson.D{{"_id", id}}
	update := bson.D{{"$set", bson.D{{"email", "newemail@example.com"}}}}

	result, err := coll.UpdateOne(context.TODO(), filter, update, opts)
	if err != nil {
		log.Fatal(err)
	}

	if result.MatchedCount != 0 {
		fmt.Println("matched and replaced an existing document")
		return
	}
	if result.UpsertedCount != 0 {
		fmt.Printf("inserted a new document with ID %v\n", result.UpsertedID)
	}
}

func ExampleCollection_Watch() {
	var collection *mongo.Collection

	// specify a pipeline that will only match "insert" events
	// specify the MaxAwaitTimeOption to have each attempt wait two seconds for new documents
	matchStage := bson.D{{"$match", bson.D{{"operationType", "insert"}}}}
	opts := options.ChangeStream().SetMaxAwaitTime(2 * time.Second)
	changeStream, err := collection.Watch(context.TODO(), mongo.Pipeline{matchStage}, opts)
	if err != nil {
		log.Fatal(err)
	}

	// print out all change stream events in the order they're received
	// see the mongo.ChangeStream documentation for more examples of using change streams
	for changeStream.Next(context.TODO()) {
		fmt.Println(changeStream.Current)
	}
}
