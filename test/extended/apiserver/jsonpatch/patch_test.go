package jsonpatch

import (
	"testing"
)

func TestJSONPatch(t *testing.T) {
	scenarios := []struct {
		name           string
		target         *PatchSet
		expectedOutput string
	}{
		{
			name:           "empty patch works and encodes as the null JSON value",
			target:         New(),
			expectedOutput: "null",
		},
		{
			name:           "patch WithRemove",
			target:         New().WithRemove("/status/condition/1"),
			expectedOutput: `[{"op":"remove","path":"/status/condition/1"}]`,
		},
		{
			name:           "patch WithTest",
			target:         New().WithTest("/metadata/resourceVersion", "1234"),
			expectedOutput: `[{"op":"test","path":"/metadata/resourceVersion","value":"1234"}]`,
		},
		{
			name:           "patch WithTest and WithRemove",
			target:         New().WithTest("/status/condition", "bar").WithRemove("/status/foo"),
			expectedOutput: `[{"op":"test","path":"/status/condition","value":"bar"},{"op":"remove","path":"/status/foo"}]`,
		},
		{
			name:           "patch WithTest and WithRemove multiple times",
			target:         New().WithTest("/status/condition", "bar").WithRemove("/status/foo").WithRemove("/status/bar"),
			expectedOutput: `[{"op":"test","path":"/status/condition","value":"bar"},{"op":"remove","path":"/status/foo"},{"op":"remove","path":"/status/bar"}]`,
		},
		{
			name:           "patch WithTest and WithRemove multiple times different order",
			target:         New().WithRemove("/status/bar").WithTest("/status/condition", "bar").WithRemove("/status/foo"),
			expectedOutput: `[{"op":"test","path":"/status/condition","value":"bar"},{"op":"remove","path":"/status/bar"},{"op":"remove","path":"/status/foo"}]`,
		},
		{
			name:           "patch WithTest and WithRemove different order",
			target:         New().WithRemove("/status/foo").WithTest("/status/condition", "bar"),
			expectedOutput: `[{"op":"test","path":"/status/condition","value":"bar"},{"op":"remove","path":"/status/foo"}]`,
		},
		{
			name:           "patch WithTest multiple times",
			target:         New().WithTest("/status/secondCondition", "foo").WithRemove("/status/foo").WithTest("/status/condition", "bar"),
			expectedOutput: `[{"op":"test","path":"/status/condition","value":"bar"},{"op":"test","path":"/status/secondCondition","value":"foo"},{"op":"remove","path":"/status/foo"}]`,
		},
	}
	for _, scenario := range scenarios {
		t.Run(scenario.name, func(t *testing.T) {
			patchBytes, err := scenario.target.Marshal()
			if err != nil {
				t.Fatal(err)
			}
			if string(patchBytes) != scenario.expectedOutput {
				t.Fatalf("expected = %s, got = %s", scenario.expectedOutput, patchBytes)
			}
		})
	}
}
