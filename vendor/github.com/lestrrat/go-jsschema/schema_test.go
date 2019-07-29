package schema_test

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lestrrat-go/jsschema"
	"github.com/lestrrat-go/jsschema/validator"
	"github.com/stretchr/testify/assert"
)

func TestReadSchema(t *testing.T) {
	files := []string{"schema.json", "qiita.json"}
	for _, f := range files {
		file := filepath.Join("test", f)
		_, err := readSchema(file)
		if !assert.NoError(t, err, "readSchema(%s) should succeed", file) {
			return
		}
	}
}

func readSchema(f string) (*schema.Schema, error) {
	in, err := os.Open(f)
	if err != nil {
		return nil, err
	}
	return schema.Read(in)
}

func TestValidate(t *testing.T) {
	tests := []string{
		"allof",
		"anyof",
		"array",
		"arraylength",
		"arraytuple",
		"arraytuple_disallow_additional",
		"arrayunique",
		"boolean",
		"business",
		"integer",
		"not",
		"null",
		"numrange",
		"numrange_exclmax",
		"objectpatterns",
		"objectpropdepend",
		"objectpropsize",
		"objectproprequired",
		"oneof",
		"strlen",
		"strpattern",
	}
	for _, name := range tests {
		schemaf := filepath.Join("test", name+".json")
		t.Logf("Reading schema file %s", schemaf)
		schema, err := readSchema(schemaf)
		if !assert.NoError(t, err, "reading schema file %s should succeed", schemaf) {
			return
		}

		valid := validator.New(schema)

		pat := filepath.Join("test", fmt.Sprintf("%s_pass*.json", name))
		files, _ := filepath.Glob(pat)
		for _, passf := range files {
			t.Logf("Testing schema against %s (expect to PASS)", passf)
			passin, err := os.Open(passf)
			if !assert.NoError(t, err, "os.Open(%s) should succeed", passf) {
				return
			}
			var m map[string]interface{} // XXX should test against structs
			if !assert.NoError(t, json.NewDecoder(passin).Decode(&m), "json.Decode should succeed") {
				return
			}

			if !assert.NoError(t, valid.Validate(m), "schema.Validate should succeed") {
				return
			}
		}

		pat = filepath.Join("test", fmt.Sprintf("%s_fail*.json", name))
		files, _ = filepath.Glob(pat)
		for _, failf := range files {
			t.Logf("Testing schema against %s (expect to FAIL)", failf)
			failin, err := os.Open(failf)
			if !assert.NoError(t, err, "os.Open(%s) should succeed", failf) {
				return
			}
			var m map[string]interface{} // XXX should test against structs
			if !assert.NoError(t, json.NewDecoder(failin).Decode(&m), "json.Decode should succeed") {
				return
			}

			if !assert.Error(t, valid.Validate(m), "schema.Validate should fail") {
				return
			}
		}
	}
}

func TestExtras(t *testing.T) {
	const src = `{
  "extra1": "foo",
  "extra2": ["bar", "baz"]
}`
	s, err := schema.Read(strings.NewReader(src))
	if !assert.NoError(t, err, "schema.Read should succeed") {
		return
	}

	for _, ek := range []string{"extra1", "extra2"} {
		_, ok := s.Extras[ek]
		if !assert.True(t, ok, "Extra item '%s' should exist", ek) {
			return
		}
	}
}
