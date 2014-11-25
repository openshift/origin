package util

import (
	"testing"
	"fmt"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/labels"
)


type StringObj struct{
	StringField string
}

type IntObj struct{
	IntField int

	Int8Field int8
	Int16Field int16
	Int32Field int32
	Int64Field int64

	UIntField uint

	UInt8Field uint
	UInt16Field uint
	UInt32Field uint
	UInt64Field uint
}

type FloatObj struct{
	Float32Field float32
	Float64Field float64
}

type BoolObj struct{
	BoolField bool
}

type EmbeddedObj struct{
	StringObj
	BoolObj
}

type TestCase struct{
	Name string
	TestObject interface{}
	ExpectedFieldMap map[string]string
}

var TestCases = []TestCase{
	{
		"string",
		StringObj{"test"},
		map[string]string{"StringField": "test"},
	},
	{
		"ints",
		IntObj{0, 1, 2, 3, 4, 5, 6, 7, 8, 9,},
		map[string]string{"IntField":"0",
							"Int8Field":"1",
							"Int16Field":"2",
							"Int32Field":"3",
							"Int64Field":"4",
							"UIntField":"5",
							"UInt8Field":"6",
							"UInt16Field":"7",
							"UInt32Field":"8",
							"UInt64Field":"9", },
	},
	{
		"floats",
		FloatObj{0.0, 1.1},
		map[string]string{"Float32Field": "0", "Float64Field": "1.1"},
	},
	{
		"booleans",
		BoolObj{true},
		map[string]string{"BoolField": "true"},
	},
	{
		"embedded objects",
		EmbeddedObj{
			StringObj{"embedded"},
			BoolObj{false},
		},
		map[string]string{"StringField": "embedded", "BoolField": "false"},
	},
}

func TestReflect(t *testing.T){
	for _, tc := range TestCases {
		results := FieldSet(tc.TestObject)

		for k, v := range tc.ExpectedFieldMap {
			if !results.Has(k) {
				dumpResultMap(results)
				t.Fatalf("Test case %s failed: result map did not contain key %s", tc.Name, k)
			}

			resultVal := results.Get(k)

			if resultVal != v {
				dumpResultMap(results)
				t.Fatalf("Test case %s failed: result map did not contain key %s with value %s", tc.Name, k, v)
			}
		}
	}
}

func dumpResultMap(set labels.Set){
	for k, v := range set {
		fmt.Printf("key: %v, val: %v\n", k, v)
	}
}
