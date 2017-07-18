package util

import (
	"reflect"
	"testing"
)

func TestParseTags(t *testing.T) {
	testCases := map[string]struct {
		lines       []string
		expectTags  Tags
		expectError bool
	}{
		"genclient": {
			lines:      []string{`+genclient`},
			expectTags: Tags{GenerateClient: true},
		},
		"genclient:nonNamespaced": {
			lines:      []string{`+genclient`, `+genclient:nonNamespaced`},
			expectTags: Tags{GenerateClient: true, NonNamespaced: true},
		},
		"genclient:noVerbs": {
			lines:      []string{`+genclient`, `+genclient:noVerbs`},
			expectTags: Tags{GenerateClient: true, NoVerbs: true},
		},
		"genclient:noStatus": {
			lines:      []string{`+genclient`, `+genclient:noStatus`},
			expectTags: Tags{GenerateClient: true, NoStatus: true},
		},
		"genclient:onlyVerbs": {
			lines:      []string{`+genclient`, `+genclient:onlyVerbs=create,delete`},
			expectTags: Tags{GenerateClient: true, OnlyVerbs: []string{"create", "delete"}},
		},
		"genclient:invalid": {
			lines:       []string{`+genclient`, `+genclient:invalid`},
			expectError: true,
		},
	}
	for key, c := range testCases {
		result, err := ParseClientGenTags(c.lines)
		if err != nil && !c.expectError {
			t.Fatalf("unexpected error: %v", err)
		}
		if !c.expectError && !reflect.DeepEqual(result, c.expectTags) {
			t.Errorf("[%s] expected %#v to be %#v", key, result, c.expectTags)
		}
	}
}
