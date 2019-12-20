package main

import (
	"reflect"
	"testing"

	"github.com/pkg/errors"
)

func verifyExpectedError(t *testing.T, resp interface{}, actual, expected error) {
	if actual == nil || errors.Cause(actual) != expected {
		t.Fatalf("expected error: %v, got: %v", expected, actual)
	}

	isnil := false
	ty := reflect.TypeOf(resp)
	if ty == nil {
		isnil = true
	} else {
		isnil = reflect.ValueOf(resp).IsNil()
	}
	if !isnil {
		t.Fatalf("expect nil response for error return, got: %v", resp)
	}
}
