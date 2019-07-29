package jsref_test

import (
	"encoding/json"
	"fmt"
	"log"

	jsref "github.com/lestrrat-go/jsref"
	"github.com/lestrrat-go/jsref/provider"
)

func Example() {
	var v interface{}
	src := []byte(`
{
  "foo": ["bar", {"$ref": "#/sub"}, {"$ref": "obj2#/sub"}],
  "sub": "baz"
}`)
	if err := json.Unmarshal(src, &v); err != nil {
		log.Printf("%s", err)
		return
	}

	// External reference
	mp := provider.NewMap()
	mp.Set("obj2", map[string]string{"sub": "quux"})

	res := jsref.New()
	res.AddProvider(mp) // Register the provider

	data := []struct {
		Ptr string
		Options []jsref.Option
	}{
		{
			Ptr: "#/foo/0", // "bar"
		},
		{
			Ptr: "#/foo/1", // "baz"
		},
		{
			Ptr: "#/foo/2", // "quux" (resolves via `mp`)
		},
		{
			Ptr: "#/foo",   // ["bar",{"$ref":"#/sub"},{"$ref":"obj2#/sub"}]
		},
		{
			Ptr: "#/foo",   // ["bar","baz","quux"]
			// experimental option to resolve all resulting values
			Options: []jsref.Option{ jsref.WithRecursiveResolution(true) },
		},
	}
	for _, set := range data {
		result, err := res.Resolve(v, set.Ptr, set.Options...)
		if err != nil { // failed to resolve
			fmt.Printf("err: %s\n", err)
			continue
		}
		b, _ := json.Marshal(result)
		fmt.Printf("%s -> %s\n", set.Ptr, string(b))
	}

	// OUTPUT:
	// #/foo/0 -> "bar"
	// #/foo/1 -> "baz"
	// #/foo/2 -> "quux"
	// #/foo -> ["bar",{"$ref":"#/sub"},{"$ref":"obj2#/sub"}]
	// #/foo -> ["bar","baz","quux"]
}
