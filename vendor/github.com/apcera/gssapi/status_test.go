// Copyright 2013-2015 Apcera Inc. All rights reserved.

package gssapi

import (
	"testing"
)

func TestStatus(t *testing.T) {
	l, err := testLoad()
	if err != nil {
		t.Error(err)
		return
	}
	defer l.Unload()

}
