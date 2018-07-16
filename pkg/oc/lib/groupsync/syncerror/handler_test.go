package syncerror

import (
	"errors"
	"io/ioutil"
	"reflect"
	"testing"

	"github.com/openshift/origin/pkg/oauthserver/ldaputil"
)

func TestSuppressMemberLookupErrorOutOfBounds(t *testing.T) {
	var testCases = []struct {
		name               string
		err                error
		expectedHandled    bool
		expectedFatalError error
	}{
		{
			name:               "nil error",
			err:                nil,
			expectedHandled:    false,
			expectedFatalError: nil,
		},
		{
			name:               "other error",
			err:                errors.New("generic error"),
			expectedHandled:    false,
			expectedFatalError: nil,
		},
		{
			name:               "non-out-of-bounds member lookup error",
			err:                NewMemberLookupError("", "", nil),
			expectedHandled:    false,
			expectedFatalError: nil,
		},
		{
			name:               "out-of-bounds member lookup error",
			err:                NewMemberLookupError("", "", ldaputil.NewQueryOutOfBoundsError("", "")),
			expectedHandled:    true,
			expectedFatalError: nil,
		},
	}

	for _, testCase := range testCases {
		handler := NewMemberLookupOutOfBoundsSuppressor(ioutil.Discard)

		actualHandled, actualFatalErr := handler.HandleError(testCase.err)
		if actualHandled != testCase.expectedHandled {
			t.Errorf("%s: handler did not handle as expected: wanted handled=%t, got handled=%t", testCase.name, testCase.expectedHandled, actualHandled)
		}

		if !reflect.DeepEqual(actualFatalErr, testCase.expectedFatalError) {
			t.Errorf("%s: handler did not return correct error:\n\twanted\n\t%v,\n\tgot\n\t%v", testCase.name, testCase.expectedFatalError, actualFatalErr)
		}
	}
}

func TestSuppressMemberLookupErrorMemberNotFound(t *testing.T) {
	var testCases = []struct {
		name               string
		err                error
		expectedHandled    bool
		expectedFatalError error
	}{
		{
			name:               "nil error",
			err:                nil,
			expectedHandled:    false,
			expectedFatalError: nil,
		},
		{
			name:               "other error",
			err:                errors.New("generic error"),
			expectedHandled:    false,
			expectedFatalError: nil,
		},
		{
			name:               "non-member-not-found member lookup error",
			err:                NewMemberLookupError("", "", nil),
			expectedHandled:    false,
			expectedFatalError: nil,
		},
		{
			name:               "no such object member lookup error",
			err:                NewMemberLookupError("", "", ldaputil.NewNoSuchObjectError("")),
			expectedHandled:    true,
			expectedFatalError: nil,
		},
		{
			name:               "member not found member lookup error",
			err:                NewMemberLookupError("", "", ldaputil.NewEntryNotFoundError("", "")),
			expectedHandled:    true,
			expectedFatalError: nil,
		},
	}

	for _, testCase := range testCases {
		handler := NewMemberLookupMemberNotFoundSuppressor(ioutil.Discard)

		actualHandled, actualFatalErr := handler.HandleError(testCase.err)
		if actualHandled != testCase.expectedHandled {
			t.Errorf("%s: handler did not handle as expected: wanted handled=%t, got handled=%t", testCase.name, testCase.expectedHandled, actualHandled)
		}

		if !reflect.DeepEqual(actualFatalErr, testCase.expectedFatalError) {
			t.Errorf("%s: handler did not return correct error:\n\twanted\n\t%v,\n\tgot\n\t%v", testCase.name, testCase.expectedFatalError, actualFatalErr)
		}
	}
}
