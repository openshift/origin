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
	"fmt"
	"reflect"

	"github.com/coreos/vcontext/path"
)

// Translation represents how a path changes when translating. If something at $yaml.storage.filesystems.4
// generates content at $json.systemd.units.3 a translation can represent that. This allows validation errors
// in Ignition structs to be tracked back to their source in the yaml.
type Translation struct {
	From path.ContextPath
	To   path.ContextPath
}

// TranslationSet represents all of the translations that occurred. They're stored in a map from a string representation
// of the destination path to the translation struct. The map is purely an optimization to allow fast lookups. Ideally the
// map would just be from the destination path.ContextPath to the source path.ContextPath, but ContextPath contains a slice
// which are not comparable and thus cannot be used as keys in maps.
type TranslationSet struct {
	FromTag string
	ToTag   string
	Set     map[string]Translation
}

func NewTranslationSet(fromTag, toTag string) TranslationSet {
	return TranslationSet{
		FromTag: fromTag,
		ToTag:   toTag,
		Set:     map[string]Translation{},
	}
}

func (ts TranslationSet) String() string {
	str := fmt.Sprintf("from: %v\nto: %v\n", ts.FromTag, ts.ToTag)
	for k, v := range ts.Set {
		str += fmt.Sprintf("%s: %v -> %v\n", k, v.From.String(), v.To.String())
	}
	return str
}

// AddTranslation adds a translation to the set
func (ts TranslationSet) AddTranslation(from, to path.ContextPath) {
	// create copies of the paths so if someone else changes from.Path the added translation does not change.
	from = from.Copy()
	to = to.Copy()
	translation := Translation{
		From: from,
		To:   to,
	}
	toString := translation.To.String()
	ts.Set[toString] = translation
}

// Shortcut for AddTranslation for identity translations
func (ts TranslationSet) AddIdentity(paths ...string) {
	for _, p := range paths {
		from := path.New(ts.FromTag, p)
		to := path.New(ts.ToTag, p)
		ts.AddTranslation(from, to)
	}
}

// AddFromCommonSource adds translations for all of the paths in to from a single common path. This is useful
// if one part of a config generates a large struct and all of the large struct should map to one path in the
// config being translated.
func (ts TranslationSet) AddFromCommonSource(common path.ContextPath, toPrefix path.ContextPath, to interface{}) {
	v := reflect.ValueOf(to)
	vPaths := prefixPaths(getAllPaths(v, ts.ToTag), toPrefix.Path...)
	for _, path := range vPaths {
		ts.AddTranslation(common, path)
	}
}

// Merge adds all the entries to the set. It mutates the Set in place.
func (ts TranslationSet) Merge(from TranslationSet) {
	for _, t := range from.Set {
		ts.AddTranslation(t.From, t.To)
	}
}

// MergeP is like Merge, but first it calls Prefix on the set being merged in.
func (ts TranslationSet) MergeP(prefix interface{}, from TranslationSet) {
	from = from.Prefix(prefix)
	ts.Merge(from)
}

// Prefix returns a TranslationSet with all translation paths prefixed by prefix.
func (ts TranslationSet) Prefix(prefix interface{}) TranslationSet {
	ret := NewTranslationSet(ts.FromTag, ts.ToTag)
	from := path.New(ts.FromTag, prefix)
	to := path.New(ts.ToTag, prefix)
	for _, tr := range ts.Set {
		ret.AddTranslation(from.Append(tr.From.Path...), to.Append(tr.From.Path...))
	}
	return ret
}
