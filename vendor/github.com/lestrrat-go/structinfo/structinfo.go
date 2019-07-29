// Package structinfo contains tools to inspect structs.

package structinfo

import (
	"reflect"
	"sync"
)

type jsonFieldMap struct {
	lock   sync.Mutex
	fields map[string]string
}

var type2jfm = map[reflect.Type]jsonFieldMap{}
var type2jfmMutex = sync.Mutex{}

// JSONFieldsFromStruct returns the names of JSON fields associated
// with the given struct. Returns nil if v is not a struct
func JSONFieldsFromStruct(v reflect.Value) []string {
	if v.Kind() != reflect.Struct {
		return nil
	}

	m := getType2jfm(v.Type())
	m.lock.Lock()
	defer m.lock.Unlock()

	l := make([]string, 0, len(m.fields))
	for k := range m.fields {
		l = append(l, k)
	}
	return l
}

// StructFieldFromJSONName returns the struct field name on the
// given struct value. Empty value means the field is either not
// public, or does not exist.
//
// This can be used to map JSON field names to actual struct fields.
func StructFieldFromJSONName(v reflect.Value, name string) string {
	if v.Kind() != reflect.Struct {
		return ""
	}

	m := getType2jfm(v.Type())
	m.lock.Lock()
	defer m.lock.Unlock()

	s, ok := m.fields[name]
	if !ok {
		return ""
	}
	return s
}

func getType2jfm(t reflect.Type) jsonFieldMap {
	type2jfmMutex.Lock()
	defer type2jfmMutex.Unlock()

	return getType2jfm_nolock(t)
}

func getType2jfm_nolock(t reflect.Type) jsonFieldMap {
	fm, ok := type2jfm[t]
	if ok {
		return fm
	}

	fm = constructJfm(t)
	type2jfm[t] = fm
	return fm
}

func constructJfm(t reflect.Type) jsonFieldMap {
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}

	fm := jsonFieldMap{
		fields: make(map[string]string),
	}
	for i := 0; i < t.NumField(); i++ {
		sf := t.Field(i)
		if sf.Anonymous { // embedded! got to recurse
			fm2 := getType2jfm_nolock(sf.Type)
			for k, v := range fm2.fields {
				fm.fields[k] = v
			}
			continue
		}

		if sf.PkgPath != "" { // unexported
			continue
		}

		tag := sf.Tag.Get("json")
		if tag == "-" {
			continue
		}

		if tag == "" || tag[0] == ',' {
			fm.fields[sf.Name] = sf.Name
			continue
		}

		flen := 0
		for j := 0; j < len(tag); j++ {
			if tag[j] == ',' {
				break
			}
			flen = j
		}
		fm.fields[tag[:flen+1]] = sf.Name
	}

	return fm
}