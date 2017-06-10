/*
Copyright 2016 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controller

import (
	"encoding/json"
	"reflect"
	"testing"

	"github.com/google/gofuzz"
)

// Tests in this file test this package's serialize() function by "round
// tripping." Values are serialized using the serialize() function, then are
// deserialized using the inverse of the process used by serialize(). Comparison
// of the original value to the output of the round trip is used to assert the
// correcness of the serialize() function.

const fuzzIters = 20

var fuzzer = fuzz.New()

func TestSerializeInt(t *testing.T) {
	for i := 0; i < fuzzIters; i++ {
		var intVal int
		fuzzer.Fuzz(&intVal)
		bytes, err := serialize(intVal)
		if err != nil {
			t.Fatalf("Unexpected error: %s", err)
		}
		var intValPrime int
		err = json.Unmarshal(bytes, &intValPrime)
		if err != nil {
			t.Fatalf("Unexpected error: %s", err)
		}
		if intVal != intValPrime {
			t.Fatalf("Round trip failed; expected %v; got %v", intVal, intValPrime)
		}
	}
}

func TestSerializeFloat(t *testing.T) {
	for i := 0; i < fuzzIters; i++ {
		var floatVal float64
		fuzzer.Fuzz(&floatVal)
		bytes, err := serialize(floatVal)
		if err != nil {
			t.Fatalf("Unexpected error: %s", err)
		}
		var floatValPrime float64
		err = json.Unmarshal(bytes, &floatValPrime)
		if err != nil {
			t.Fatalf("Unexpected error: %s", err)
		}
		if floatVal != floatValPrime {
			t.Fatalf("Round trip failed; expected %v; got %v", floatVal, floatValPrime)
		}
	}
}

func TestSerializeString(t *testing.T) {
	for i := 0; i < fuzzIters; i++ {
		var strVal string
		fuzzer.Fuzz(&strVal)
		bytes, err := serialize(strVal)
		if err != nil {
			t.Fatalf("Unexpected error: %s", err)
		}
		strValPrime := string(bytes)
		if strVal != strValPrime {
			t.Fatalf("Round trip failed; expected %v; got %v", strVal, strValPrime)
		}
	}
}

func TestSerializeMap(t *testing.T) {
	var mapVal map[string]string
	for i := 0; i < fuzzIters; i++ {
		fuzzer.Fuzz(&mapVal)
		bytes, err := serialize(mapVal)
		if err != nil {
			t.Fatalf("Unexpected error: %s", err)
		}
		mapValPrime := make(map[string]string)
		err = json.Unmarshal(bytes, &mapValPrime)
		if err != nil {
			t.Fatalf("Unexpected error: %s", err)
		}
		if !reflect.DeepEqual(mapVal, mapValPrime) {
			t.Fatalf("Round trip failed; expected %v; got %v", mapVal, mapValPrime)
		}
	}
}

func TestSerializeSlice(t *testing.T) {
	var sliceVal []string
	for i := 0; i < fuzzIters; i++ {
		fuzzer.Fuzz(&sliceVal)
		bytes, err := serialize(sliceVal)
		if err != nil {
			t.Fatalf("Unexpected error: %s", err)
		}
		sliceValPrime := make([]string, 4)
		err = json.Unmarshal(bytes, &sliceValPrime)
		if err != nil {
			t.Fatalf("Unexpected error: %s", err)
		}
		if !reflect.DeepEqual(sliceVal, sliceValPrime) {
			t.Fatalf("Round trip failed; expected %v; got %v", sliceVal, sliceValPrime)
		}
	}
}
