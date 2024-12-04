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

package translate

import (
	"reflect"
	"strings"

	"github.com/coreos/ignition/v2/config/util"
	"github.com/coreos/vcontext/path"
)

// fieldName returns the name uses when (un)marshalling a field. t should be a reflect.Value of a struct,
// index is the field index, and tag is the struct tag used when (un)marshalling (e.g. "json" or "yaml")
func fieldName(t reflect.Value, index int, tag string) string {
	f := t.Type().Field(index)
	if tag == "" {
		return f.Name
	}
	return strings.Split(f.Tag.Get(tag), ",")[0]
}

func prefixPath(p path.ContextPath, prefix ...interface{}) path.ContextPath {
	return path.New(p.Tag, prefix...).Append(p.Path...)
}

func prefixPaths(ps []path.ContextPath, prefix ...interface{}) []path.ContextPath {
	ret := []path.ContextPath{}
	for _, p := range ps {
		ret = append(ret, prefixPath(p, prefix...))
	}
	return ret
}

func getAllPaths(v reflect.Value, tag string) []path.ContextPath {
	k := v.Kind()
	t := v.Type()
	switch {
	case util.IsPrimitive(k):
		return nil
	case k == reflect.Ptr:
		if v.IsNil() {
			return nil
		}
		return getAllPaths(v.Elem(), tag)
	case k == reflect.Slice:
		ret := []path.ContextPath{}
		for i := 0; i < v.Len(); i++ {
			ret = append(ret, prefixPaths(getAllPaths(v.Index(i), tag), i)...)
		}
		return ret
	case k == reflect.Struct:
		ret := []path.ContextPath{}
		for i := 0; i < t.NumField(); i++ {
			name := fieldName(v, i, tag)
			field := v.Field(i)
			if t.Field(i).Anonymous {
				ret = append(ret, getAllPaths(field, tag)...)
			} else {
				ret = append(ret, prefixPaths(getAllPaths(field, tag), name)...)
				ret = append(ret, path.New(tag, name))
			}
		}
		return ret
	default:
		panic("Encountered types that are not the same when they should be. This is a bug, please file a report")
	}
}
