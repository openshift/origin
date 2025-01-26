// Copyright 2016 CoreOS, Inc.
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

package v2_0

import (
	"reflect"

	"github.com/coreos/ignition/config/v2_0/types"
)

// Append appends newConfig to oldConfig and returns the result. Appending one
// config to another is accomplished by iterating over every field in the
// config structure, appending slices, recursively appending structs, and
// overwriting old values with new values for all other types.
func Append(oldConfig, newConfig types.Config) types.Config {
	vOld := reflect.ValueOf(oldConfig)
	vNew := reflect.ValueOf(newConfig)

	vResult := appendStruct(vOld, vNew)

	return vResult.Interface().(types.Config)
}

// appendStruct is an internal helper function to AppendConfig. Given two values
// of structures (assumed to be the same type), recursively iterate over every
// field in the struct, appending slices, recursively appending structs, and
// overwriting old values with the new for all other types. Some individual
// struct fields have alternate merge strategies, determined by the field name.
// Currently these fields are "ignition.version", which uses the old value, and
// "ignition.config" which uses the new value.
func appendStruct(vOld, vNew reflect.Value) reflect.Value {
	tOld := vOld.Type()
	vRes := reflect.New(tOld)

	for i := 0; i < tOld.NumField(); i++ {
		vfOld := vOld.Field(i)
		vfNew := vNew.Field(i)
		vfRes := vRes.Elem().Field(i)

		switch tOld.Field(i).Name {
		case "Version":
			vfRes.Set(vfOld)
			continue
		case "Config":
			vfRes.Set(vfNew)
			continue
		}

		switch vfOld.Type().Kind() {
		case reflect.Struct:
			vfRes.Set(appendStruct(vfOld, vfNew))
		case reflect.Slice:
			vfRes.Set(reflect.AppendSlice(vfOld, vfNew))
		default:
			if vfNew.Kind() == reflect.Ptr && vfNew.IsNil() {
				vfRes.Set(vfOld)
			} else {
				vfRes.Set(vfNew)
			}
		}
	}

	return vRes.Elem()
}
