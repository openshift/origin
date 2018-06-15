package app

import (
	"testing"
)

func TestIsComponentReference(t *testing.T) {
	tests := map[string]struct {
		ref         string
		expectedErr string
	}{
		"empty string": {
			ref:         "",
			expectedErr: "empty string provided to component reference check",
		},
		"string with +": {
			ref: "foo+bar",
		},
		"image~code good": {
			ref: "foo~bar",
		},
		"image~code empty image name": {
			ref:         "~",
			expectedErr: "when using '[image]~[code]' form for \"~\", you must specify a image name",
		},
		"image~code empty seg 1 empty": {
			ref: "foo~",
		},
		"non image~code format": {
			ref: "foo",
		},
	}
	for name, test := range tests {
		err := IsComponentReference(test.ref)
		checkError(err, test.expectedErr, name, t)
	}
}
