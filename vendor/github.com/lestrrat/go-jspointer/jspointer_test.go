package jspointer_test

import (
	"encoding/json"
	"testing"

	"github.com/lestrrat/go-jspointer"
	"github.com/stretchr/testify/assert"
)

var src = `{
"foo": ["bar", "baz"],
"obj": { "a":1, "b":2, "c":[3,4], "d":[ {"e":9}, {"f":[50,51]} ] },
"": 0,
"a/b": 1,
"c%d": 2,
"e^f": 3,
"g|h": 4,
"i\\j": 5,
"k\"l": 6,
" ": 7,
"m~n": 8
}`
var target map[string]interface{}

func init() {
	if err := json.Unmarshal([]byte(src), &target); err != nil {
		panic(err)
	}
}

func TestEscaping(t *testing.T) {
	data := []string{
		`/a~1b`,
		`/m~0n`,
		`/a~1b/m~0n`,
	}
	for _, pat := range data {
		p, err := jspointer.New(pat)
		if !assert.NoError(t, err, "jspointer.New should succeed for '%s'", pat) {
			return
		}

		if !assert.Equal(t, pat, p.String(), "input pattern and generated expression should match") {
			return
		}
	}
}

func runmatch(t *testing.T, pat string, m interface{}) (interface{}, error) {
	p, err := jspointer.New(pat)
	if !assert.NoError(t, err, "jspointer.New should succeed for '%s'", pat) {
		return nil, err
	}

	return p.Get(m)
}

func TestFullDocument(t *testing.T) {
	res, err := runmatch(t, ``, target)
	if !assert.NoError(t, err, "jsonpointer.Get should succeed") {
		return
	}
	if !assert.Equal(t, res, target, "res should be equal to target") {
		return
	}
}

func TestGetObject(t *testing.T) {
	pats := map[string]interface{}{
		`/obj/a`:       float64(1),
		`/obj/b`:       float64(2),
		`/obj/c/0`:     float64(3),
		`/obj/c/1`:     float64(4),
		`/obj/d/1/f/0`: float64(50),
	}
	for pat, expected := range pats {
		res, err := runmatch(t, pat, target)
		if !assert.NoError(t, err, "jsonpointer.Get should succeed") {
			return
		}

		if !assert.Equal(t, res, expected, "res should be equal to expected") {
			return
		}
	}
}

func TestGetArray(t *testing.T) {
	foo := target["foo"].([]interface{})
	pats := map[string]interface{}{
		`/foo/0`: foo[0],
		`/foo/1`: foo[1],
	}
	for pat, expected := range pats {
		res, err := runmatch(t, pat, target)
		if !assert.NoError(t, err, "jsonpointer.Get should succeed") {
			return
		}

		if !assert.Equal(t, res, expected, "res should be equal to expected") {
			return
		}
	}
}

func TestSet(t *testing.T) {
	var m interface{}
	json.Unmarshal([]byte(`{
"a": [{"b": 1, "c": 2}], "d": 3
}`), &m)

	p, err := jspointer.New(`/a/0/c`)
	if !assert.NoError(t, err, "jspointer.New should succeed") {
		return
	}

	if !assert.NoError(t, p.Set(m, 999), "jspointer.Set should succeed") {
		return
	}

	res, err := runmatch(t, `/a/0/c`, m)
	if !assert.NoError(t, err, "jsonpointer.Get should succeed") {
		return
	}

	if !assert.Equal(t, res, 999, "res should be equal to expected") {
		return
	}
}

func TestStruct(t *testing.T) {
	var s struct {
		Foo string `json:"foo"`
		Bar map[string]interface{} `json:"bar"`
		Baz map[int]int `json:"baz"`
		quux int
	}

	s.Foo = "foooooo"
	s.Bar = map[string]interface{}{
		"a": 0,
		"b": 1,
	}
	s.Baz = map[int]int{
		2: 3,
	}

	res, err := runmatch(t, `/bar/b`, s)
	if !assert.NoError(t, err, "jsonpointer.Get should succeed") {
		return
	}

	if !assert.Equal(t, res, 1, "res should be equal to expected value") {
		return
	}

	res, err = runmatch(t, `/baz/2`, s)
	if !assert.NoError(t, err, "jsonpointer.Get should succeed") {
		return
	}

	if !assert.Equal(t, res, 3, "res should be equal to expected value") {
		return
	}
}


