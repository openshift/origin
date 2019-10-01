// +build bench

package bench_test

import (
	"encoding/json"
	"testing"

	"github.com/lestrrat-go/jspointer"
	"github.com/xeipuuv/gojsonpointer"
)

const jsontxt = `{"a":[{"b": 1, "c": 2}], "d": 3}`

var m map[string]interface{}

func init() {
	if err := json.Unmarshal([]byte(jsontxt), &m); err != nil {
		panic(err)
	}
}

func BenchmarkGojsonpointer(b *testing.B) {
	p, _ := gojsonpointer.NewJsonPointer(`/a/0/c`)
	for i := 0; i < b.N; i++ {
		res, kind, err := p.Get(m)
		_ = res
		_ = kind
		_ = err
	}
}

func BenchmarkJspointer(b *testing.B) {
	p, _ := jspointer.New(`/a/0/c`)
	for i := 0; i < b.N; i++ {
		res, err := p.Get(m)
		_ = res
		_ = err
	}
}