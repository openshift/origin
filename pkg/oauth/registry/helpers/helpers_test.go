package helpers

import "testing"

// TestMakeClientAuthorizationName makes sure that the raw names do not drift over time
// Do not change any test data
func TestMakeClientAuthorizationName(t *testing.T) {
	for _, test := range []struct {
		userName   string
		clientName string
		expected   string
	}{
		{
			userName:   "",
			clientName: "",
			expected:   "::",
		},
		{
			userName:   "1",
			clientName: "",
			expected:   "1::",
		},
		{
			userName:   "",
			clientName: "2",
			expected:   "::2",
		},
		{
			userName:   "a",
			clientName: "b",
			expected:   "a::b",
		},
	} {
		actual := MakeClientAuthorizationName(test.userName, test.clientName)
		if test.expected != actual {
			t.Errorf("Expected MakeClientAuthorizationName(%s, %s) == %s, got: %s", test.userName, test.clientName, test.expected, actual)
		}
	}
}

// TestGetKeyWithUsername makes sure that the raw paths do not drift over time
// Do not change any test data
func TestGetKeyWithUsername(t *testing.T) {
	for _, test := range []struct {
		prefix   string
		userName string
		expected string
	}{
		{
			prefix:   "",
			userName: "",
			expected: "/",
		},
		{
			prefix:   "1",
			userName: "",
			expected: "1/",
		},
		{
			prefix:   "",
			userName: "2",
			expected: "/2",
		},
		{
			prefix:   "a",
			userName: "b",
			expected: "a/b",
		},
	} {
		actual := GetKeyWithUsername(test.prefix, test.userName)
		if test.expected != actual {
			t.Errorf("Expected GetKeyWithUsername(%s, %s) == %s, got: %s", test.prefix, test.userName, test.expected, actual)
		}
	}
}

func TestSplitClientAuthorizationName(t *testing.T) {
	for _, test := range []struct {
		clientAuthorizationName string
		userName                string
		clientName              string
		expectedErr             error
	}{
		{
			clientAuthorizationName: "",
			expectedErr:             InvalidClientAuthorizationNameErr,
		},
		{
			clientAuthorizationName: ":",
			expectedErr:             InvalidClientAuthorizationNameErr,
		},
		{
			clientAuthorizationName: "1:",
			expectedErr:             InvalidClientAuthorizationNameErr,
		},
		{
			clientAuthorizationName: ":2",
			expectedErr:             InvalidClientAuthorizationNameErr,
		},
		{
			clientAuthorizationName: "a:b",
			expectedErr:             InvalidClientAuthorizationNameErr,
		},
		{
			clientAuthorizationName: "::",
			expectedErr:             InvalidClientAuthorizationNameErr,
		},
		{
			clientAuthorizationName: "a::",
			expectedErr:             InvalidClientAuthorizationNameErr,
		},
		{
			clientAuthorizationName: "::b",
			expectedErr:             InvalidClientAuthorizationNameErr,
		},
		{
			clientAuthorizationName: ":::",
			expectedErr:             InvalidClientAuthorizationNameErr,
		},
		{
			clientAuthorizationName: "::::",
			expectedErr:             InvalidClientAuthorizationNameErr,
		},
		{
			clientAuthorizationName: ":::::",
			expectedErr:             InvalidClientAuthorizationNameErr,
		},
		{
			clientAuthorizationName: "::::::",
			expectedErr:             InvalidClientAuthorizationNameErr,
		},
		{
			clientAuthorizationName: "a::b::",
			expectedErr:             InvalidClientAuthorizationNameErr,
		},
		{
			clientAuthorizationName: "::a::b",
			expectedErr:             InvalidClientAuthorizationNameErr,
		},
		{
			clientAuthorizationName: "a::b::c",
			expectedErr:             InvalidClientAuthorizationNameErr,
		},
		{
			clientAuthorizationName: "a::b",
			userName:                "a",
			clientName:              "b",
		},
	} {
		username, clientname, err := SplitClientAuthorizationName(test.clientAuthorizationName)
		if test.expectedErr != nil {
			if test.expectedErr != err {
				t.Errorf("Expected error %#v, got SplitClientAuthorizationName(%s) == (%s, %s, %#v)", test.expectedErr, test.clientAuthorizationName, username, clientname, err)
			}
			continue
		}

		if test.userName != username {
			t.Errorf("Expected username %s, got: %s", test.userName, username)
		}

		if test.clientName != clientname {
			t.Errorf("Expected username %s, got: %s", test.clientName, clientname)
		}
	}
}
