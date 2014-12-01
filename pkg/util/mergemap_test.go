package util

import (
	"reflect"
	"testing"
)

func TestMergeMaps(t *testing.T) {
	testCases := []struct {
		dst        interface{}
		src        interface{}
		flags      int
		shouldPass bool
		expected   interface{}
	}{
		{ // Test empty maps
			map[int]int{},
			map[int]int{},
			0,
			true,
			map[int]int{},
		},
		{ // Test dst + src => expected
			map[int]string{1: "foo"},
			map[int]string{2: "bar"},
			0,
			true,
			map[int]string{1: "foo", 2: "bar"},
		},
		{ // Test dst + src => expected, do not overwrite dst
			map[string]string{"foo": "bar"},
			map[string]string{"foo": ""},
			0,
			true,
			map[string]string{"foo": "bar"},
		},
		{ // Test dst + src => expected, overwrite dst
			map[string]string{"foo": "bar"},
			map[string]string{"foo": ""},
			OverwriteExistingDstKey,
			true,
			map[string]string{"foo": ""},
		},
		{ // Test dst + src => expected, error on existing key value
			map[string]string{"foo": "bar"},
			map[string]string{"foo": "bar"},
			ErrorOnExistingDstKey | OverwriteExistingDstKey,
			false,
			map[string]string{"foo": "bar"},
		},
		{ // Test dst + src => expected, do not error on same key value
			map[string]string{"foo": "bar"},
			map[string]string{"foo": "bar"},
			ErrorOnDifferentDstKeyValue | OverwriteExistingDstKey,
			true,
			map[string]string{"foo": "bar"},
		},
		{ // Test dst + src => expected, error on different key value
			map[string]string{"foo": "bar"},
			map[string]string{"foo": ""},
			ErrorOnDifferentDstKeyValue | OverwriteExistingDstKey,
			false,
			map[string]string{"foo": "bar"},
		},
	}

	for i, test := range testCases {
		err := MergeInto(test.dst, test.src, test.flags)
		if err != nil && test.shouldPass {
			t.Errorf("Unexpected error while merging maps on testCase[%v].", i)
		}
		if err == nil && !test.shouldPass {
			t.Errorf("Unexpected non-error while merging maps on testCase[%v].", i)
		}
		if !reflect.DeepEqual(test.dst, test.expected) {
			t.Errorf("Unexpected map on testCase[%v]. Expected: %v, got: %v.", i, test.expected, test.dst)
		}
	}
}
