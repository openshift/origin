// Copyright 2019 Red Hat, Inc
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
// limitations under the License.)

package validate

import (
	"reflect"
	"strings"

	"github.com/coreos/vcontext/path"
	"github.com/coreos/vcontext/report"
)

type CustomValidator func(v reflect.Value, c path.ContextPath) report.Report

// validator is the interface the DefaultValidator function uses when validating types.
// Most users should implement this interface on types they want to validate.
type validator interface {
	Validate(path.ContextPath) report.Report
}

// DefaultValidator checks if the type implements the validator interface and calls the
// validate function if it does, returning the report.
func DefaultValidator(v reflect.Value, c path.ContextPath) report.Report {
	// first check if this object has Validate(context) defined, but only on value
	// recievers. Both pointer and value receivers satisfy a value receiver interface
	// so ensure we're not a pointer too.
	if obj, ok := v.Interface().(validator); ok && v.Kind() != reflect.Ptr {
		return obj.Validate(c)
	}
	return report.Report{}
}

// ValidateCustom validates thing using the custom validation function supplied. Most users will not need this
// and should use Validate() instead.
func ValidateCustom(thing interface{}, tag string, customValidator CustomValidator) report.Report {
	if thing == nil {
		return report.Report{}
	}
	v := reflect.ValueOf(thing)
	ctx := path.ContextPath{Tag: tag}
	return validate(ctx, v, customValidator)
}

// Validate walks the structs, slices, and pointers in thing and calls any Validate(path.ContextPath) report.Report
// functions defined on the types, aggregating the results.
func Validate(thing interface{}, tag string) report.Report {
	return ValidateCustom(thing, tag, DefaultValidator)
}

func validate(context path.ContextPath, v reflect.Value, validateFunc CustomValidator) (r report.Report) {
	if !v.IsValid() {
		return
	}
	if v.Kind() == reflect.Interface {
		if v.IsNil() {
			return
		} else {
			v = makeConcrete(v)
		}
	}

	r.Merge(validateFunc(v, context))

	switch v.Kind() {
	case reflect.Struct:
		r.Merge(validateStruct(context, v, validateFunc))
	case reflect.Slice:
		r.Merge(validateSlice(context, v, validateFunc))
	case reflect.Ptr:
		if !v.IsNil() {
			r.Merge(validate(context, v.Elem(), validateFunc))
		}
	}

	return
}

// StructField is an extension of go's reflect.StructField that also includes the value.
type StructField struct {
	reflect.StructField
	Value reflect.Value
}

// makeConcrete takes a value and if it is a value of an interface returns the
// value of the actual underlying type implementing that interface. If the value
// is already concrete, it returns the same value.
func makeConcrete(v reflect.Value) reflect.Value {
	return reflect.ValueOf(v.Interface())
}

// GetFields takes a value of a struct and flattens all embedded structs in it.
// If any fields are interfaces, it "dereferences" the interface to its underlying type.
func GetFields(v reflect.Value) []StructField {
	ret := []StructField{}
	if v.Kind() != reflect.Struct {
		return ret
	}

	for i := 0; i < v.NumField(); i++ {
		field := v.Type().Field(i)
		if !field.Anonymous {
			ret = append(ret, StructField{
				StructField: field,
				Value:       v.Field(i),
			})
		} else {
			concrete := makeConcrete(v.Field(i))
			ret = append(ret, GetFields(concrete)...)
		}
	}
	return ret
}

func FieldName(s StructField, tag string) string {
	if tag == "" {
		return s.Name
	}
	tag = s.Tag.Get(tag)
	return strings.Split(tag, ",")[0]
}

func validateStruct(context path.ContextPath, v reflect.Value, f CustomValidator) (r report.Report) {
	fields := GetFields(v)
	for _, field := range fields {
		fieldContext := context.Append(FieldName(field, context.Tag))
		r.Merge(validate(fieldContext, field.Value, f))
	}
	return
}

func validateSlice(context path.ContextPath, v reflect.Value, f CustomValidator) (r report.Report) {
	for i := 0; i < v.Len(); i++ {
		childContext := context.Append(i)
		r.Merge(validate(childContext, v.Index(i), f))
	}
	return
}
