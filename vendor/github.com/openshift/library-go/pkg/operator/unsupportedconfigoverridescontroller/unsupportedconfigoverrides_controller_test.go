package unsupportedconfigoverridescontroller

import (
	"testing"

	"k8s.io/apimachinery/pkg/util/sets"
)

func TestKeysSetInUnsupportedConfig(t *testing.T) {
	tests := []struct {
		name string

		yaml     string
		expected sets.String
	}{
		{
			name:     "empty",
			yaml:     "",
			expected: sets.NewString(),
		},
		{
			name: "nested maps",
			yaml: `
apple:
  banana:
    carrot: hammer
`,
			expected: sets.NewString(
				"apple.banana.carrot",
			),
		},
		{
			name: "multiple nested maps",
			yaml: `
apple:
  banana:
    carrot: hammer
  blueberry:
    cabbage: saw
artichoke: plane
`,
			expected: sets.NewString(
				"apple.banana.carrot",
				"apple.blueberry.cabbage",
				"artichoke",
			),
		},
		{
			name: "multiple nested slices with nested maps",
			yaml: `
apple:
  banana:
    carrot:
    - hammer
    - chisel
    - drawknife
  blueberry:
    - saw:
      chives:
        dill: square
artichoke: plane
`,
			expected: sets.NewString(
				"artichoke",
				"apple.banana.carrot.0",
				"apple.banana.carrot.1",
				"apple.banana.carrot.2",
				"apple.blueberry.0.chives.dill",
				"apple.blueberry.0.saw",
			),
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			actual, err := keysSetInUnsupportedConfig([]byte(test.yaml))
			if err != nil {
				t.Fatal(err)
			}

			if !actual.Equal(test.expected) {
				t.Fatalf("missing expected %v, extra actual %v", test.expected.Difference(actual).List(), actual.Difference(test.expected).List())
			}
		})
	}
}
