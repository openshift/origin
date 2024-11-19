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

package merge

import (
	"fmt"
	"reflect"
	"strings"

	"github.com/coreos/ignition/v2/config/util"

	"github.com/coreos/vcontext/path"
)

// Rules of Config Merging:
// 1) Parent and child configs must be the same version/type
// 2) Only valid configs can be merged
// 3) It is possible to merge two valid configs and get an invalid config
// 3) For structs:
//   a) Members that are structs get merged recursively (i.e. ignition.storage)
//   b) Members that are primitives get replaced by the child's member (e.g. ignition.storage.files[i].path)
//   c) Members that are pointers only get replaced by the child's value if the child's value is non-nil (e.g. ignition.config.replace.source)
//   d) List merging of a list with IgnoreDuplicates: append the lists (e.g. ignition.storage.files[i].append)
//   e) List merging of a list not merged with other lists: merge any entries with the same Key() and append the others (e.g. ignition.storage.filesystems by path)
//   f) List merging of a list merged with other lists: (e.g. ignition.storage.{files,links,directories} by path)
//      - merge entries with the same Key() that are in the same list
//      - remove entries from the parent with the same Key() that are not in the same list
//      - append entries that are unique to the child

const (
	TAG_PARENT = "parent"
	TAG_CHILD  = "child"
	TAG_RESULT = "result"
)

// The path to one output field, and its corresponding input.  From.Tag will
// be TAG_PARENT or TAG_CHILD depending on the origin of the field.
type Mapping struct {
	From path.ContextPath
	To   path.ContextPath
}

func (m Mapping) String() string {
	return fmt.Sprintf("%s:%s → %s", m.From.Tag, m.From, m.To)
}

type Transcript struct {
	Mappings []Mapping
}

func (t Transcript) String() string {
	var lines []string
	for _, m := range t.Mappings {
		lines = append(lines, m.String())
	}
	return strings.Join(lines, "\n")
}

// pathAppendField looks up the JSON field name for field and returns base
// with that field name appended.
func pathAppendField(base path.ContextPath, field reflect.StructField) path.ContextPath {
	tagName := strings.Split(field.Tag.Get("json"), ",")[0]
	if tagName != "" {
		return base.Append(tagName)
	}
	if field.Anonymous {
		// field is a struct embedded in another struct (e.g.
		// FileEmbedded1).  Pretend it doesn't exist.
		return base
	}
	panic("no JSON struct tag for " + field.Name)
}

// appendToSlice is a helper that appends to a slice without returning a new one.
// panics if len >= cap
func appendToSlice(s, v reflect.Value) {
	s.SetLen(s.Len() + 1)
	s.Index(s.Len() - 1).Set(v)
}

type handleKey struct {
	handle string
	key    string
}

// structInfo holds information about a struct being processed and has helper methods for querying that
// information in a way that is more clear what the intent is.
type structInfo struct {
	// set of field names to not do duplicate merging on
	ignoreDups map[string]struct{}

	// map from field names to a handle indicating all those with the same handle should have duplication
	// checking done across all fields that share that handle
	mergedKeys map[string]string

	// map from each handle + key() to the corresponding item
	keysToValues map[handleKey]reflect.Value

	// map from each handle + key() to the list it came from
	keysToLists map[handleKey]string

	// map from each handle + key() to the index within the list
	keysToListIndexes map[handleKey]int
}

// returns if this field should not do duplicate checking/merging
func (s structInfo) ignoreField(name string) bool {
	_, ignore := s.ignoreDups[name]
	return ignore
}

// getChildEntryByKey takes the name of a field (not handle) in the parent and a key and looks that entry
// up in the child. It will look up across all slices that share the same handle. It returns the value,
// name of the field in the child it was found in, and the list index within that field. The bool indicates
// whether it was found.
func (s structInfo) getChildEntryByKey(fieldName, key string) (reflect.Value, string, int, bool) {
	handle := fieldName
	if tmp, ok := s.mergedKeys[fieldName]; ok {
		handle = tmp
	}

	hkey := handleKey{
		handle: handle,
		key:    key,
	}
	if v, ok := s.keysToValues[hkey]; ok {
		return v, s.keysToLists[hkey], s.keysToListIndexes[hkey], true
	}
	return reflect.Value{}, "", 0, false
}

func newStructInfo(parent, child reflect.Value) structInfo {
	ignoreDups := map[string]struct{}{}
	if ignorer, ok := parent.Interface().(util.IgnoresDups); ok {
		ignoreDups = ignorer.IgnoreDuplicates()
	}

	mergedKeys := map[string]string{}
	if merger, ok := parent.Interface().(util.MergesKeys); ok {
		mergedKeys = merger.MergedKeys()
	}

	keysToValues := map[handleKey]reflect.Value{}
	keysToLists := map[handleKey]string{}
	keysToListIndexes := map[handleKey]int{}
	for i := 0; i < child.NumField(); i++ {
		field := child.Field(i)
		if field.Kind() != reflect.Slice {
			continue
		}

		fieldName := child.Type().Field(i).Name
		if _, ok := ignoreDups[fieldName]; ok {
			continue
		}

		handle := fieldName
		if tmp, ok := mergedKeys[handle]; ok {
			handle = tmp
		}

		for j := 0; j < field.Len(); j++ {
			v := field.Index(j)
			hkey := handleKey{
				handle: handle,
				key:    util.CallKey(v),
			}
			keysToValues[hkey] = v
			keysToLists[hkey] = fieldName
			keysToListIndexes[hkey] = j
		}
	}

	return structInfo{
		ignoreDups:        ignoreDups,
		mergedKeys:        mergedKeys,
		keysToValues:      keysToValues,
		keysToLists:       keysToLists,
		keysToListIndexes: keysToListIndexes,
	}
}

// Deprecated: Use MergeStructTranscribe() instead.
func MergeStruct(parent, child reflect.Value) reflect.Value {
	result, _ := MergeStructTranscribe(parent.Interface(), child.Interface())
	return reflect.ValueOf(result)
}

// MergeStructTranscribe is intended for use by config/vX_Y/ packages and
// by generic external translation code.  Most users should use the properly
// typed wrappers provided by the config/vX_Y/ packages.
//
// MergeStructTranscribe merges the specified configs and returns a
// transcript of the actions taken.  parent and child MUST be the same type.
func MergeStructTranscribe(parent, child interface{}) (interface{}, Transcript) {
	var transcript Transcript
	result := mergeStruct(reflect.ValueOf(parent), path.New(TAG_PARENT), reflect.ValueOf(child), path.New(TAG_CHILD), path.New(TAG_RESULT), &transcript)
	return result.Interface(), transcript
}

// parent and child MUST be the same type
// the transcript lists children before parents
// all interior nodes that have contributions from both parent and child
// receive separate transcript mappings for parent and child, in that order
func mergeStruct(parent reflect.Value, parentPath path.ContextPath, child reflect.Value, childPath path.ContextPath, resultPath path.ContextPath, transcript *Transcript) reflect.Value {
	// use New() so it's settable, addr-able, etc
	result := reflect.New(parent.Type()).Elem()
	info := newStructInfo(parent, child)

	for i := 0; i < parent.NumField(); i++ {
		fieldMeta := parent.Type().Field(i)
		parentField := parent.Field(i)
		childField := child.Field(i)
		resultField := result.Field(i)
		parentFieldPath := pathAppendField(parentPath, fieldMeta)
		childFieldPath := pathAppendField(childPath, fieldMeta)
		resultFieldPath := pathAppendField(resultPath, fieldMeta)

		kind := parentField.Kind()
		switch {
		case util.IsPrimitive(kind):
			resultField.Set(childField)
			transcribe(childFieldPath, resultFieldPath, resultField, fieldMeta, transcript)
		case kind == reflect.Ptr && !parentField.IsNil() && !childField.IsNil() && parentField.Elem().Kind() == reflect.Struct:
			// we're not supposed to have struct pointers, but some
			// ended up in the Clevis and Luks structs in spec 3.2.0
			// https://github.com/coreos/ignition/issues/1132
			resultField.Set(mergeStruct(parentField.Elem(), parentFieldPath, childField.Elem(), childFieldPath, resultFieldPath, transcript).Addr())
			transcribeOne(parentFieldPath, resultFieldPath, transcript)
			transcribeOne(childFieldPath, resultFieldPath, transcript)
		case kind == reflect.Ptr && childField.IsNil():
			resultField.Set(parentField)
			transcribe(parentFieldPath, resultFieldPath, resultField, fieldMeta, transcript)
		case kind == reflect.Ptr && !childField.IsNil():
			resultField.Set(childField)
			transcribe(childFieldPath, resultFieldPath, resultField, fieldMeta, transcript)
		case kind == reflect.Struct && childField.IsZero():
			resultField.Set(parentField)
			transcribe(parentFieldPath, resultFieldPath, resultField, fieldMeta, transcript)
		case kind == reflect.Struct && parentField.IsZero():
			resultField.Set(childField)
			transcribe(childFieldPath, resultFieldPath, resultField, fieldMeta, transcript)
		case kind == reflect.Struct:
			resultField.Set(mergeStruct(parentField, parentFieldPath, childField, childFieldPath, resultFieldPath, transcript))
			if !fieldMeta.Anonymous {
				transcribeOne(parentFieldPath, resultFieldPath, transcript)
				transcribeOne(childFieldPath, resultFieldPath, transcript)
			}
		case kind == reflect.Slice && info.ignoreField(fieldMeta.Name):
			if parentField.Len()+childField.Len() == 0 {
				continue
			}
			resultField.Set(reflect.MakeSlice(parentField.Type(), 0, parentField.Len()+childField.Len()))
			for i := 0; i < parentField.Len(); i++ {
				item := parentField.Index(i)
				appendToSlice(resultField, item)
				transcribe(parentFieldPath.Append(i), resultFieldPath.Append(i), item, fieldMeta, transcript)
			}
			for i := 0; i < childField.Len(); i++ {
				item := childField.Index(i)
				appendToSlice(resultField, item)
				transcribe(childFieldPath.Append(i), resultFieldPath.Append(parentField.Len()+i), item, fieldMeta, transcript)
			}
			// transcribe the list itself
			if parentField.Len() > 0 {
				transcribeOne(parentFieldPath, resultFieldPath, transcript)
			}
			if childField.Len() > 0 {
				transcribeOne(childFieldPath, resultFieldPath, transcript)
			}
		case kind == reflect.Slice && !info.ignoreField(fieldMeta.Name):
			// ooph, this is a doosey
			maxlen := parentField.Len() + childField.Len()
			if maxlen == 0 {
				continue
			}
			resultField.Set(reflect.MakeSlice(parentField.Type(), 0, parentField.Len()+childField.Len()))
			parentKeys := getKeySet(parentField)
			var itemFromParent, itemFromChild bool

			// walk parent items
			for i := 0; i < parentField.Len(); i++ {
				parentItem := parentField.Index(i)
				parentItemPath := parentFieldPath.Append(i)
				resultItemPath := resultFieldPath.Append(resultField.Len())
				key := util.CallKey(parentItem)

				if childItem, childList, childListIndex, ok := info.getChildEntryByKey(fieldMeta.Name, key); ok {
					if childList == fieldMeta.Name {
						// case 1: in child config in same list
						childItemPath := childFieldPath.Append(childListIndex)
						// record the contribution of both parent and child, even if one wins
						// or cancels the other
						itemFromParent = true
						itemFromChild = true
						if childItem.Kind() == reflect.Struct {
							// If HTTP header Value is nil, it means that we should remove the
							// parent header from the result.
							if fieldMeta.Name == "HTTPHeaders" && childItem.FieldByName("Value").IsNil() {
								continue
							}
							appendToSlice(resultField, mergeStruct(parentItem, parentItemPath, childItem, childItemPath, resultItemPath, transcript))
							transcribeOne(parentItemPath, resultItemPath, transcript)
							transcribeOne(childItemPath, resultItemPath, transcript)
						} else if util.IsPrimitive(childItem.Kind()) {
							appendToSlice(resultField, childItem)
							transcribe(childItemPath, resultItemPath, childItem, fieldMeta, transcript)
						} else {
							panic("List of pointers or slices or something else weird")
						}
					} else { // nolint:staticcheck
						// case 2: in child config in different list. Do nothing since it'll be handled iterating over that list
					}
				} else {
					// case 3: not in child config, append it
					appendToSlice(resultField, parentItem)
					transcribe(parentItemPath, resultItemPath, parentItem, fieldMeta, transcript)
					itemFromParent = true
				}
			}
			// append child items not in parent
			for i := 0; i < childField.Len(); i++ {
				childItem := childField.Index(i)
				childItemPath := childFieldPath.Append(i)
				resultItemPath := resultFieldPath.Append(resultField.Len())
				key := util.CallKey(childItem)
				if _, alreadyMerged := parentKeys[key]; !alreadyMerged {
					// We only check the parentMap for this field. If the parent had a matching entry in a different field
					// then it would be skipped as case 2 above
					appendToSlice(resultField, childItem)
					transcribe(childItemPath, resultItemPath, childItem, fieldMeta, transcript)
					itemFromChild = true
				}
			}
			if itemFromParent {
				transcribeOne(parentFieldPath, resultFieldPath, transcript)
			}
			if itemFromChild {
				transcribeOne(childFieldPath, resultFieldPath, transcript)
			}
		default:
			panic("unreachable code reached")
		}
	}

	return result
}

// transcribe is called by mergeStruct when the latter decides to merge a
// subtree wholesale from either the parent or child, and thus loses
// interest in that subtree.  transcribe descends the rest of that subtree,
// transcribing all of its populated leaves.  It returns true if we
// transcribed anything.
func transcribe(fromPath path.ContextPath, toPath path.ContextPath, value reflect.Value, fieldMeta reflect.StructField, transcript *Transcript) bool {
	kind := value.Kind()
	switch {
	case util.IsPrimitive(kind):
		if value.IsZero() {
			return false
		}
		transcribeOne(fromPath, toPath, transcript)
	case kind == reflect.Ptr:
		if value.IsNil() {
			return false
		}
		if value.Elem().Kind() == reflect.Struct {
			// we're not supposed to have struct pointers, but some
			// ended up in the Clevis and Luks structs in spec 3.2.0
			// https://github.com/coreos/ignition/issues/1132
			return transcribe(fromPath, toPath, value.Elem(), fieldMeta, transcript)
		}
		transcribeOne(fromPath, toPath, transcript)
	case kind == reflect.Struct:
		var transcribed bool
		for i := 0; i < value.NumField(); i++ {
			valueFieldMeta := value.Type().Field(i)
			transcribed = transcribe(pathAppendField(fromPath, valueFieldMeta), pathAppendField(toPath, valueFieldMeta), value.Field(i), valueFieldMeta, transcript) || transcribed
		}
		// embedded structs and empty structs should be invisible
		if transcribed && !fieldMeta.Anonymous {
			transcribeOne(fromPath, toPath, transcript)
		}
		return transcribed
	case kind == reflect.Slice:
		var transcribed bool
		for i := 0; i < value.Len(); i++ {
			transcribed = transcribe(fromPath.Append(i), toPath.Append(i), value.Index(i), fieldMeta, transcript) || transcribed
		}
		if transcribed {
			transcribeOne(fromPath, toPath, transcript)
		}
		return transcribed
	default:
		panic("unreachable code reached")
	}
	return true
}

// transcribeOne records one Mapping into a Transcript.
func transcribeOne(from, to path.ContextPath, transcript *Transcript) {
	transcript.Mappings = append(transcript.Mappings, Mapping{
		From: from.Copy(),
		To:   to.Copy(),
	})
}

// getKeySet takes a value of a slice and returns the set of all the Key() values in that slice
func getKeySet(list reflect.Value) map[string]struct{} {
	m := map[string]struct{}{}
	for i := 0; i < list.Len(); i++ {
		m[util.CallKey(list.Index(i))] = struct{}{}
	}
	return m
}
