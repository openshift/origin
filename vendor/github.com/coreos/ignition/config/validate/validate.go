// Copyright 2015 CoreOS, Inc.
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
	"bytes"
	"fmt"
	"io"
	"reflect"
	"strings"

	json "github.com/ajeddeloh/go-json"
	"github.com/coreos/ignition/config/validate/astjson"
	"github.com/coreos/ignition/config/validate/astnode"
	"github.com/coreos/ignition/config/validate/report"
)

type validator interface {
	Validate() report.Report
}

// ValidateConfig validates a raw config object into a given config version
func ValidateConfig(rawConfig []byte, config interface{}) report.Report {
	// Unmarshal again to a json.Node to get offset information for building a report
	var ast json.Node
	var r report.Report
	configValue := reflect.ValueOf(config)
	if err := json.Unmarshal(rawConfig, &ast); err != nil {
		r.Add(report.Entry{
			Kind:    report.EntryWarning,
			Message: "Ignition could not unmarshal your config for reporting line numbers. This should never happen. Please file a bug.",
		})
		r.Merge(ValidateWithoutSource(configValue))
	} else {
		r.Merge(Validate(configValue, astjson.FromJsonRoot(ast), bytes.NewReader(rawConfig), true))
	}
	return r
}

// Validate walks down a struct tree calling Validate on every node that implements it, building
// A report of all the errors, warnings, info, and deprecations it encounters. If checkUnusedKeys
// is true, Validate will generate warnings for unused keys in the ast, otherwise it will not.
func Validate(vObj reflect.Value, ast astnode.AstNode, source io.ReadSeeker, checkUnusedKeys bool) (r report.Report) {
	if !vObj.IsValid() {
		return
	}

	line, col, highlight := 0, 0, ""
	if ast != nil {
		line, col, highlight = ast.ValueLineCol(source)
	}

	// See if we A) can call Validate on vObj, and B) should call Validate. Validate should NOT be called
	// when vObj is nil, as it will panic or when vObj is a pointer to a value with Validate implemented with a
	// value receiver. This is to prevent Validate being called twice, as otherwise it would be called on the
	// pointer version (due to go's automatic deferencing) and once when the pointer is deferenced below. The only
	// time Validate should be called on a pointer is when the function is implemented with a pointer reciever.
	if obj, ok := vObj.Interface().(validator); ok &&
		((vObj.Kind() != reflect.Ptr) ||
			(!vObj.IsNil() && !vObj.Elem().Type().Implements(reflect.TypeOf((*validator)(nil)).Elem()))) {
		sub_r := obj.Validate()
		sub_r.AddPosition(line, col, highlight)
		r.Merge(sub_r)

		// Dont recurse on invalid inner nodes, it mostly leads to bogus messages
		if sub_r.IsFatal() {
			return
		}
	}

	switch vObj.Kind() {
	case reflect.Ptr:
		sub_report := Validate(vObj.Elem(), ast, source, checkUnusedKeys)
		sub_report.AddPosition(line, col, "")
		r.Merge(sub_report)
	case reflect.Struct:
		sub_report := validateStruct(vObj, ast, source, checkUnusedKeys)
		sub_report.AddPosition(line, col, "")
		r.Merge(sub_report)
	case reflect.Slice:
		for i := 0; i < vObj.Len(); i++ {
			sub_node := ast
			if ast != nil {
				if n, ok := ast.SliceChild(i); ok {
					sub_node = n
				}
			}
			sub_report := Validate(vObj.Index(i), sub_node, source, checkUnusedKeys)
			sub_report.AddPosition(line, col, "")
			r.Merge(sub_report)
		}
	}
	return
}

func ValidateWithoutSource(cfg reflect.Value) (report report.Report) {
	return Validate(cfg, nil, nil, false)
}

type field struct {
	Type  reflect.StructField
	Value reflect.Value
}

// getFields returns a field of all the fields in the struct, including the fields of
// embedded structs and structs inside interface{}'s
func getFields(vObj reflect.Value) []field {
	if vObj.Kind() != reflect.Struct {
		return nil
	}
	ret := []field{}
	for i := 0; i < vObj.Type().NumField(); i++ {
		if vObj.Type().Field(i).Anonymous {
			// in the case of an embedded type that is an alias to interface, extract the
			// real type contained by the interface
			realObj := reflect.ValueOf(vObj.Field(i).Interface())
			ret = append(ret, getFields(realObj)...)
		} else {
			ret = append(ret, field{Type: vObj.Type().Field(i), Value: vObj.Field(i)})
		}
	}
	return ret
}

func validateStruct(vObj reflect.Value, ast astnode.AstNode, source io.ReadSeeker, checkUnusedKeys bool) report.Report {
	r := report.Report{}

	// isFromObject will be true if this struct was unmarshalled from a JSON object.
	keys, isFromObject := map[string]astnode.AstNode{}, false
	if ast != nil {
		keys, isFromObject = ast.KeyValueMap()
	}

	// Maintain a set of key's that have been used.
	usedKeys := map[string]struct{}{}

	// Maintain a list of all the tags in the struct for fuzzy matching later.
	tags := []string{}

	for _, f := range getFields(vObj) {
		// Default to nil astnode.AstNode if the field's corrosponding node cannot be found.
		var sub_node astnode.AstNode
		// Default to passing a nil source if the field's corrosponding node cannot be found.
		// This ensures the line numbers reported from all sub-structs are 0 and will be changed by AddPosition
		var src io.ReadSeeker

		// Try to determine the json.Node that corrosponds with the struct field
		if isFromObject {
			tag := strings.SplitN(f.Type.Tag.Get(ast.Tag()), ",", 2)[0]
			// Save the tag so we have a list of all the tags in the struct
			tags = append(tags, tag)
			// mark that this key was used
			usedKeys[tag] = struct{}{}

			if sub, ok := keys[tag]; ok {
				// Found it
				sub_node = sub
				src = source
			}
		}

		// Default to deepest node if the node's type isn't an object,
		// such as when a json string actually unmarshal to structs (like with version)
		line, col := 0, 0
		highlight := ""
		if ast != nil {
			line, col, highlight = ast.ValueLineCol(src)
		}

		// If there's a Validate<Name> func for the given field, call it
		funct := vObj.MethodByName("Validate" + f.Type.Name)
		if funct.IsValid() {
			if sub_node != nil {
				// if sub_node is non-nil, we can get better line/col info
				line, col, highlight = sub_node.ValueLineCol(src)
			}
			res := funct.Call(nil)
			sub_report := res[0].Interface().(report.Report)
			sub_report.AddPosition(line, col, highlight)
			r.Merge(sub_report)
		}

		sub_report := Validate(f.Value, sub_node, src, checkUnusedKeys)
		sub_report.AddPosition(line, col, highlight)
		r.Merge(sub_report)
	}
	if !isFromObject || !checkUnusedKeys {
		// If this struct was not unmarshalled from a JSON object, there cannot be unused keys.
		return r
	}

	for k, v := range keys {
		if _, hasKey := usedKeys[k]; hasKey {
			continue
		}
		line, col, highlight := v.KeyLineCol(source)
		typo := similar(k, tags)

		r.Add(report.Entry{
			Kind:      report.EntryWarning,
			Message:   fmt.Sprintf("Config has unrecognized key: %s", k),
			Line:      line,
			Column:    col,
			Highlight: highlight,
		})

		if typo != "" {
			r.Add(report.Entry{
				Kind:      report.EntryInfo,
				Message:   fmt.Sprintf("Did you mean %s instead of %s", typo, k),
				Line:      line,
				Column:    col,
				Highlight: highlight,
			})
		}
	}

	return r
}

// similar returns a string in candidates that is similar to str. Currently it just does case
// insensitive comparison, but it should be updated to use levenstein distances to catch typos
func similar(str string, candidates []string) string {
	for _, candidate := range candidates {
		if strings.EqualFold(str, candidate) {
			return candidate
		}
	}
	return ""
}
