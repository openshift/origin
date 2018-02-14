package osin

import (
	"errors"
	"fmt"
	"reflect"
	"testing"
)

type testLogger struct {
	Result string
}

func (l *testLogger) Printf(format string, v ...interface{}) {
	l.Result = fmt.Sprintf(format, v...)
}

func TestServerErrorLogger(t *testing.T) {
	sconfig := NewServerConfig()
	server := NewServer(sconfig, NewTestingStorage())

	tl := &testLogger{}
	server.Logger = tl

	r := server.NewResponse()
	r.ErrorStatusCode = 404

	server.setErrorAndLog(r, E_INVALID_GRANT, errors.New("foo"), "foo=%s, bar=%s", "bar", "baz")

	if r.ErrorId != E_INVALID_GRANT {
		t.Errorf("expected error to be set to %s", E_INVALID_GRANT)
	}
	if r.StatusText != deferror.Get(E_INVALID_GRANT) {
		t.Errorf("expected status text to be %s, got %s", deferror.Get(E_INVALID_GRANT), r.StatusText)
	}

	expectedResult := `error=invalid_grant, internal_error=&errors.errorString{s:"foo"} foo=bar, bar=baz`
	if !reflect.DeepEqual(tl.Result, expectedResult) {
		t.Errorf("expected %v, got %v", expectedResult, tl.Result)
	}
}