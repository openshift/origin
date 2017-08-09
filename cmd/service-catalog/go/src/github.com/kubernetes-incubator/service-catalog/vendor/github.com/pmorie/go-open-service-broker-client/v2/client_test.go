package v2

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"reflect"
	"testing"
)

// func init() {
// 	flag.Set("alsologtostderr", "true")
// 	flag.Set("v", "5")
// }

const malformedResponse = `{`

const conventionalFailureResponseBody = `{
	"error": "TestError",
	"description": "test error description"
}`

func testHttpStatusCodeError() error {
	errorMessage := "TestError"
	description := "test error description"
	return HTTPStatusCodeError{
		StatusCode:   http.StatusInternalServerError,
		ErrorMessage: &errorMessage,
		Description:  &description,
	}
}

func truePtr() *bool {
	b := true
	return &b
}

func falsePtr() *bool {
	b := false
	return &b
}

func closer(s string) io.ReadCloser {
	return nopCloser{bytes.NewBufferString(s)}
}

type nopCloser struct {
	io.Reader
}

func (nopCloser) Close() error { return nil }

type httpChecks struct {
	URL    string
	body   string
	params map[string]string
}

type httpReaction struct {
	status int
	body   string
	err    error
}

func newTestClient(t *testing.T, name string, version APIVersion, enableAlpha bool, httpChecks httpChecks, httpReaction httpReaction) *client {
	return &client{
		Name:                "test client",
		APIVersion:          version,
		Verbose:             true,
		URL:                 "https://example.com",
		EnableAlphaFeatures: enableAlpha,
		doRequestFunc:       doHTTP(t, name, httpChecks, httpReaction),
	}
}

var walkingGhostErr = fmt.Errorf("test has already failed")

func doHTTP(t *testing.T, name string, checks httpChecks, reaction httpReaction) func(*http.Request) (*http.Response, error) {
	return func(request *http.Request) (*http.Response, error) {
		if len(checks.URL) > 0 && checks.URL != request.URL.Path {
			t.Errorf("%v: unexpected URL; expected %v, got %v", name, checks.URL, request.URL.Path)
			return nil, walkingGhostErr
		}

		if len(checks.params) > 0 {
			for k, v := range checks.params {
				actualValue := request.URL.Query().Get(k)
				if e, a := v, actualValue; e != a {
					t.Errorf("%v: unexpected parameter value for key %v; expected %v, got %v", name, k, e, a)
					return nil, walkingGhostErr
				}
			}
		}

		var bodyBytes []byte
		if request.Body != nil {
			var err error
			bodyBytes, err = ioutil.ReadAll(request.Body)
			if err != nil {
				t.Errorf("%v: error reading request body bytes: %v", name, err)
				return nil, walkingGhostErr
			}
		}

		if e, a := checks.body, string(bodyBytes); e != a {
			t.Errorf("%v: unexpected request body: expected %v, got %v", name, e, a)
			return nil, walkingGhostErr
		}

		return &http.Response{
			StatusCode: reaction.status,
			Body:       closer(reaction.body),
		}, reaction.err
	}
}

func doResponseChecks(t *testing.T, name string, response interface{}, err error, expectedResponse interface{}, expectedErrMessage string, expectedErr error) {
	if err != nil && expectedErrMessage == "" && expectedErr == nil {
		t.Errorf("%v: error performing request: %v", name, err)
		return
	} else if err != nil && expectedErrMessage != "" && expectedErrMessage != err.Error() {
		t.Errorf("%v: unexpected error message: expected %v, got %v", name, expectedErrMessage, err)
		return
	} else if err != nil && expectedErr != nil && !reflect.DeepEqual(expectedErr, err) {
		t.Errorf("%v: unexpected error: expected %+v, got %v", name, expectedErr, err)
		return
	}

	if e, a := expectedResponse, response; !reflect.DeepEqual(e, a) {
		t.Errorf("%v: unexpected diff in response; expected %+v, got %+v", name, e, a)
		return
	}
}
