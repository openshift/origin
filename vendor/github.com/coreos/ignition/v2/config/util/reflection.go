// Copyright 2019 Red Hat, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package util

import (
	"fmt"
	"reflect"
)

func IsPrimitive(k reflect.Kind) bool {
	switch k {
	case reflect.Bool,
		reflect.Int,
		reflect.Int8,
		reflect.Int16,
		reflect.Int32,
		reflect.Int64,
		reflect.Uint,
		reflect.Uint8,
		reflect.Uint16,
		reflect.Uint32,
		reflect.Uint64,
		reflect.Uintptr,
		reflect.Float32,
		reflect.Float64,
		reflect.Complex64,
		reflect.Complex128,
		reflect.String:
		return true
	default:
		return false
	}
}

func IsInvalidInConfig(k reflect.Kind) bool {
	switch {
	case IsPrimitive(k):
		return false
	case k == reflect.Ptr || k == reflect.Slice || k == reflect.Struct:
		return false
	default:
		return true
	}
}

// Return a fully non-zero value for the specified type, recursively
// setting all fields and slices.
func NonZeroValue(t reflect.Type) reflect.Value {
	v := reflect.New(t).Elem()
	setNonZero(v)
	return v
}

func setNonZero(v reflect.Value) {
	switch v.Kind() {
	case reflect.Bool:
		v.SetBool(true)
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		v.SetInt(1)
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		v.SetUint(1)
	case reflect.Float32, reflect.Float64:
		v.SetFloat(1)
	case reflect.String:
		v.SetString("aardvark")
	case reflect.Ptr:
		v.Set(reflect.New(v.Type().Elem()))
		setNonZero(v.Elem())
	case reflect.Slice:
		v.Set(reflect.MakeSlice(v.Type(), 1, 1))
		setNonZero(v.Index(0))
	case reflect.Struct:
		for i := 0; i < v.NumField(); i++ {
			setNonZero(v.Field(i))
		}
	default:
		panic(fmt.Sprintf("unexpected kind %s", v.Kind()))
	}
}
