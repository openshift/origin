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

package validate

import (
	"fmt"
	"reflect"

	"github.com/coreos/ignition/v2/config/shared/errors"
	"github.com/coreos/ignition/v2/config/util"

	"github.com/coreos/vcontext/json"
	"github.com/coreos/vcontext/path"
	"github.com/coreos/vcontext/report"
	"github.com/coreos/vcontext/tree"
	"github.com/coreos/vcontext/validate"
)

func ValidateDups(v reflect.Value, c path.ContextPath) (r report.Report) {
	if v.Kind() != reflect.Struct {
		return
	}
	dupsLists := map[string]map[string]struct{}{}
	ignoreDups := map[string]struct{}{}
	if i, ok := v.Interface().(util.IgnoresDups); ok {
		ignoreDups = i.IgnoreDuplicates()
	}
	mergedKeys := map[string]string{}
	if m, ok := v.Interface().(util.MergesKeys); ok {
		mergedKeys = m.MergedKeys()
	}

	fields := validate.GetFields(v)
	for _, field := range fields {
		if field.Type.Kind() != reflect.Slice {
			continue
		}
		if _, ignored := ignoreDups[field.Name]; ignored {
			continue
		}
		dupListName := field.Name
		if mergedName, ok := mergedKeys[field.Name]; ok {
			dupListName = mergedName
		}
		dupList := dupsLists[dupListName]
		if dupList == nil {
			dupsLists[dupListName] = make(map[string]struct{}, field.Value.Len())
			dupList = dupsLists[dupListName]
		}
		for i := 0; i < field.Value.Len(); i++ {
			key := util.CallKey(field.Value.Index(i))
			if _, isDup := dupList[key]; isDup {
				r.AddOnError(c.Append(validate.FieldName(field, c.Tag), i), errors.ErrDuplicate)
			}
			dupList[key] = struct{}{}
		}
	}
	return
}

func ValidateUnusedKeys(v reflect.Value, c path.ContextPath, root tree.Node) (r report.Report) {
	if v.Kind() != reflect.Struct {
		return
	}
	node, err := root.Get(c)
	if err != nil {
		// not every node will have corresponding json
		return
	}

	mapNode, ok := node.(tree.MapNode)
	if !ok {
		// Something is wrong, we won't be able to report unused keys here, so just warn about it and stop trying
		r.AddOnWarn(c, fmt.Errorf("context tree does not match content tree at %s. Line and column reporting may be inconsistent. Unused keys may not be reported.", c.String()))
		return
	}

	fields := validate.GetFields(v)
	fieldMap := make(map[string]struct{}, len(fields))
	for _, field := range fields {
		fieldMap[validate.FieldName(field, c.Tag)] = struct{}{}
	}
	for key := range mapNode.Keys {
		if _, ok := fieldMap[key]; !ok {
			r.AddOnWarn(c.Append(tree.Key(key)), fmt.Errorf("Unused key %s", key))
		}
	}
	return
}

func ValidateWithContext(cfg interface{}, raw []byte) report.Report {
	r := validate.Validate(cfg, "json")
	r.Merge(validate.ValidateCustom(cfg, "json", ValidateDups))
	if raw == nil {
		return r
	}

	if cxt, err := json.UnmarshalToContext(raw); err == nil {
		unusedKeyCheck := func(v reflect.Value, c path.ContextPath) report.Report {
			return ValidateUnusedKeys(v, c, cxt)
		}
		r.Merge(validate.ValidateCustom(cfg, "json", unusedKeyCheck))
		r.Correlate(cxt)
	}
	return r
}
