// Copyright (C) MongoDB, Inc. 2017-present.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may
// not use this file except in compliance with the License. You may obtain
// a copy of the License at http://www.apache.org/licenses/LICENSE-2.0

package topology

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.mongodb.org/mongo-driver/x/mongo/driver/connstring"
)

func TestOptionsSetting(t *testing.T) {
	conf := &config{}
	ssts := time.Minute
	assert.Zero(t, conf.cs)

	opt := WithConnString(func(connstring.ConnString) connstring.ConnString {
		return connstring.ConnString{
			ServerSelectionTimeout:    ssts,
			ServerSelectionTimeoutSet: true,
		}

	})

	assert.NoError(t, opt(conf))

	assert.Equal(t, ssts, conf.serverSelectionTimeout)
}
