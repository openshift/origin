// Copyright (C) MongoDB, Inc. 2017-present.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may
// not use this file except in compliance with the License. You may obtain
// a copy of the License at http://www.apache.org/licenses/LICENSE-2.0

package mongo

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/bson"
)

// Individual commands can be sent to the server and response retrieved via run command.
func ExampleDatabase_RunCommand() {
	var db *Database
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	err := db.RunCommand(ctx, bson.D{{"ping", 1}}).Err()
	if err != nil {
		return
	}
	return
}
