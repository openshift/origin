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

	"github.com/coreos/ignition/v2/config/util"
)

/*
 * This is an automatic translator that replace boilerplate code to copy one
 * struct into a nearly identical struct in another package. To use it first
 * call NewTranslator() to get a translator instance. This can then have
 * additional translation rules (in the form of functions) to translate from
 * types in one struct to the other. Those functions are in the form:
 *     func(typeFromInputStruct) -> typeFromOutputStruct
 * These can be closures that reference the translator as well. This allows for
 * manually translating some fields but resuming automatic translation on the
 * other fields through the Translator.Translate() function.
 */

// Returns if this type can be translated without a custom translator. Children or other
// ancestors might require custom translators however
func (t translator) translatable(t1, t2 reflect.Type) bool {
	k1 := t1.Kind()
	k2 := t2.Kind()
	if k1 != k2 {
		return false
	}
	switch {
	case util.IsPrimitive(k1):
		return true
	case util.IsInvalidInConfig(k1):
		panic(fmt.Sprintf("Encountered invalid kind %s in config. This is a bug, please file a report", k1))
	case k1 == reflect.Ptr || k1 == reflect.Slice:
		return t.translatable(t1.Elem(), t2.Elem()) || t.hasTranslator(t1.Elem(), t2.Elem())
	case k1 == reflect.Struct:
		return t.translatableStruct(t1, t2)
	default:
		panic(fmt.Sprintf("Encountered unknown kind %s in config. This is a bug, please file a report", k1))
	}
}

// precondition: t1, t2 are both of Kind 'struct'
func (t translator) translatableStruct(t1, t2 reflect.Type) bool {
	if t1.NumField() != t2.NumField() || t1.Name() != t2.Name() {
		return false
	}
	for i := 0; i < t1.NumField(); i++ {
		t1f := t1.Field(i)
		t2f, ok := t2.FieldByName(t1f.Name)

		if !ok {
			return false
		}
		if !t.translatable(t1f.Type, t2f.Type) && !t.hasTranslator(t1f.Type, t2f.Type) {
			return false
		}
	}
	return true
}

// checks that t could reasonably be the type of a translator function
func couldBeValidTranslator(t reflect.Type) bool {
	if t.Kind() != reflect.Func {
		return false
	}
	if t.NumIn() != 1 || t.NumOut() != 1 {
		return false
	}
	if util.IsInvalidInConfig(t.In(0).Kind()) || util.IsInvalidInConfig(t.Out(0).Kind()) {
		return false
	}
	return true
}

// translate from one type to another, but deep copy all data
// precondition: vFrom and vTo are the same type as defined by translatable()
// precondition: vTo is addressable and settable
func (t translator) translateSameType(vFrom, vTo reflect.Value) {
	k := vFrom.Kind()
	switch {
	case util.IsPrimitive(k):
		// Use convert, even if not needed; type alias to primitives are not
		// directly assignable and calling Convert on primitives does no harm
		vTo.Set(vFrom.Convert(vTo.Type()))
	case k == reflect.Ptr:
		if vFrom.IsNil() {
			return
		}
		vTo.Set(reflect.New(vTo.Type().Elem()))
		t.translate(vFrom.Elem(), vTo.Elem())
	case k == reflect.Slice:
		if vFrom.IsNil() {
			return
		}
		vTo.Set(reflect.MakeSlice(vTo.Type(), vFrom.Len(), vFrom.Len()))
		for i := 0; i < vFrom.Len(); i++ {
			t.translate(vFrom.Index(i), vTo.Index(i))
		}
	case k == reflect.Struct:
		for i := 0; i < vFrom.NumField(); i++ {
			t.translate(vFrom.Field(i), vTo.FieldByName(vFrom.Type().Field(i).Name))
		}
	default:
		panic("Encountered types that are not the same when they should be. This is a bug, please file a report")
	}
}

// helper to return if a custom translator was defined
func (t translator) hasTranslator(tFrom, tTo reflect.Type) bool {
	return t.getTranslator(tFrom, tTo).IsValid()
}

// vTo must be addressable, should be acquired by calling reflect.ValueOf() on a variable of the correct type
func (t translator) translate(vFrom, vTo reflect.Value) {
	tFrom := vFrom.Type()
	tTo := vTo.Type()
	if fnv := t.getTranslator(tFrom, tTo); fnv.IsValid() {
		vTo.Set(fnv.Call([]reflect.Value{vFrom})[0])
		return
	}
	if t.translatable(tFrom, tTo) {
		t.translateSameType(vFrom, vTo)
		return
	}

	panic(fmt.Sprintf("Translator not defined for %v to %v", tFrom, tTo))
}

type Translator interface {
	AddCustomTranslator(t interface{})
	Translate(from, to interface{})
}

func NewTranslator() Translator {
	return &translator{}
}

type translator struct {
	// List of custom translation funcs, must pass couldBeValidTranslator
	// This is only for fields that cannot or should not be trivially translated,
	// All trivially translated fields use the default behavior.
	translators []reflect.Value
}

func (t *translator) AddCustomTranslator(fn interface{}) {
	fnv := reflect.ValueOf(fn)
	if !couldBeValidTranslator(fnv.Type()) {
		panic("Tried to register invalid translator function")
	}
	t.translators = append(t.translators, fnv)
}

func (t translator) getTranslator(from, to reflect.Type) reflect.Value {
	for _, fn := range t.translators {
		if fn.Type().In(0) == from && fn.Type().Out(0) == to {
			return fn
		}
	}
	return reflect.Value{}
}

func (t translator) Translate(from, to interface{}) {
	fv := reflect.ValueOf(from)
	tv := reflect.ValueOf(to)
	if fv.Kind() != reflect.Ptr || tv.Kind() != reflect.Ptr {
		panic("Translate needs to be called on pointers")
	}
	fv = fv.Elem()
	tv = tv.Elem()
	t.translate(fv, tv)
}
