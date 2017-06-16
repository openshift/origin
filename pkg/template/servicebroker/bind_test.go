package servicebroker

import (
	"testing"
)

func TestEvaluateJSONPathExpression(t *testing.T) {
	i := 1
	obj := struct {
		Int            int
		PointerToInt   *int
		InterfaceToInt interface{}
		NullPointer    *int
		NullInterface  interface{}
		IntSlice       []int
		ByteSlice      []byte
		String         string
		Nested         struct {
			NestedInt int
		}
	}{
		Int:            i,
		PointerToInt:   &i,
		InterfaceToInt: i,
		IntSlice:       []int{1, 2, 3},
		ByteSlice:      []byte("testbytes"),
		String:         "teststring",
		Nested: struct {
			NestedInt int
		}{
			NestedInt: 2,
		},
	}

	tests := []struct {
		expression     string
		base64encode   bool
		expectedError  string
		expectedResult string
	}{
		{
			expression:    "{",
			expectedError: "failed to parse annotation a: unclosed action",
		},
		{
			expression:    "{.DoesntExist}",
			expectedError: "FindResults failed on annotation a: DoesntExist is not found",
		},
		{
			expression:    "{.IntSlice[*]}",
			expectedError: "3 JSONPath results found on annotation a",
		},
		{
			expression:    "{.NullPointer}",
			expectedError: "nil kind ptr found in JSONPath result on annotation a",
		},
		{
			expression:    "{.NullInterface}",
			expectedError: "nil kind interface found in JSONPath result on annotation a",
		},
		{
			expression:     "{.Int}",
			expectedResult: "1",
		},
		{
			expression:     "{.PointerToInt}",
			expectedResult: "1",
		},
		{
			expression:     "{.InterfaceToInt}",
			expectedResult: "1",
		},
		{
			expression:     "{.String}",
			expectedResult: "teststring",
		},
		{
			expression:     "{.String}",
			expectedResult: "teststring",
			base64encode:   true,
		},
		{
			expression:     "{.ByteSlice}",
			expectedResult: "testbytes",
		},
		{
			expression:     "{.ByteSlice}",
			expectedResult: "dGVzdGJ5dGVz",
			base64encode:   true,
		},
		{
			expression:    "{.IntSlice}",
			expectedError: "unrepresentable kind slice found in JSONPath result on annotation a",
		},
		{
			expression:    "{.Nested}",
			expectedError: "unrepresentable kind struct found in JSONPath result on annotation a",
		},
		{
			expression:     "{.Nested.NestedInt}",
			expectedResult: "2",
		},
		{
			expression:     "foo/{.ByteSlice}",
			expectedResult: "foo/testbytes",
		},
		{
			expression:     "foo/{.ByteSlice}",
			expectedResult: "foo/dGVzdGJ5dGVz",
			base64encode:   true,
		},
	}

	for i, test := range tests {
		result, err := evaluateJSONPathExpression(obj, "a", test.expression, test.base64encode)
		if err != nil {
			if err.Error() != test.expectedError {
				t.Errorf("%d: error %q", i, err)
			}
			continue
		}
		if result != test.expectedResult {
			t.Errorf("%d: result %q", i, result)
		}
	}
}
