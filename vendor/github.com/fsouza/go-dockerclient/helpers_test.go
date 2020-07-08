// Copyright 2020 go-dockerclient authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package docker

import (
	"errors"
	"testing"
)

func expectNoSuchContainer(t *testing.T, id string, err error) {
	t.Helper()
	var containerErr *NoSuchContainer
	if !errors.As(err, &containerErr) {
		t.Fatalf("Container: Wrong error information. Want %#v. Got %#v.", containerErr, err)
	}
	if containerErr.ID != id {
		t.Errorf("Container: wrong container in error\nWant %q\ngot  %q", id, containerErr.ID)
	}
}

func expectNoSuchNode(t *testing.T, nodeID string, err error) {
	t.Helper()
	var nodeErr *NoSuchNode
	if !errors.As(err, &nodeErr) {
		t.Fatalf("Node: Wrong error information. Want %#v. Got %#v.", nodeErr, err)
	}
	if nodeErr.ID != nodeID {
		t.Errorf("Node: wrong node in error\nWant %q\ngot  %q", nodeID, nodeErr.ID)
	}
}

func expectNoSuchSecret(t *testing.T, secretID string, err error) {
	t.Helper()
	var nodeErr *NoSuchSecret
	if !errors.As(err, &nodeErr) {
		t.Fatalf("Secret: Wrong error information. Want %#v. Got %#v.", nodeErr, err)
	}
	if nodeErr.ID != secretID {
		t.Errorf("Secret: wrong secret in error\nWant %q\ngot  %q", secretID, nodeErr.ID)
	}
}

func expectNoSuchConfig(t *testing.T, secretID string, err error) {
	t.Helper()
	var nodeErr *NoSuchConfig
	if !errors.As(err, &nodeErr) {
		t.Fatalf("Config: Wrong error information. Want %#v. Got %#v.", nodeErr, err)
	}
	if nodeErr.ID != secretID {
		t.Errorf("Config: wrong secret in error\nWant %q\ngot  %q", secretID, nodeErr.ID)
	}
}
