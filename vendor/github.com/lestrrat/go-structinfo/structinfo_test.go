package structinfo_test

import (
	"reflect"
	"testing"

	"github.com/lestrrat/go-structinfo"
	"github.com/stretchr/testify/assert"
)

type Quux struct {
	Baz string `json:"baz"`
}

type X struct {
	private int
	Quux
	Foo string `json:"foo"`
	Bar string `json:"bar,omitempty"`
}

func TestStructFields(t *testing.T) {
	fields := make(map[string]struct{})
	for _, name := range structinfo.JSONFieldsFromStruct(reflect.ValueOf(X{})) {
		fields[name] = struct{}{}
	}

	expected := map[string]struct{}{
		"foo": {},
		"bar": {},
		"baz": {},
	}

	if !assert.Equal(t, expected, fields, "expected fields match") {
		return
	}
}

func TestLookupSructFieldFromJSONName(t *testing.T) {
	rv := reflect.ValueOf(X{})

	data := map[string]string{
		"foo": "Foo",
		"bar": "Bar",
		"baz": "Baz",
	}

	for jsname, fname := range data {
		fn := structinfo.StructFieldFromJSONName(rv, jsname)
		if !assert.NotEqual(t, fn, "", "should find '%s'", jsname) {
			return
		}

		sf, ok := rv.Type().FieldByName(fn)
		if !assert.True(t, ok, "should be able resolve '%s' (%s)", jsname, fn) {
			return
		}

		if !assert.Equal(t, sf.Name, fname, "'%s' should map to '%s'", jsname, fname) {
			return
		}
	}
}
