package buildscheme

import (
	"strings"
	"testing"

	"github.com/davecgh/go-spew/spew"

	"k8s.io/apimachinery/pkg/runtime"

	buildv1 "github.com/openshift/api/build/v1"
)

const legacyBC = `{
  "apiVersion": "v1",
  "kind": "BuildConfig",
  "metadata": {
    "name": "sinatra-app-example-a"
  }
}
`

func TestLegacyDecoding(t *testing.T) {
	result, err := runtime.Decode(Decoder, []byte(legacyBC))
	if err != nil {
		t.Fatal(err)
	}
	if result.(*buildv1.BuildConfig).Name != "sinatra-app-example-a" {
		t.Fatal(spew.Sdump(result))
	}

	groupfiedBytes, err := runtime.Encode(Encoder, result)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(groupfiedBytes), "build.openshift.io/v1") {
		t.Fatal(string(groupfiedBytes))
	}

	result2, err := runtime.Decode(Decoder, groupfiedBytes)
	if err != nil {
		t.Fatal(err)
	}
	if result2.(*buildv1.BuildConfig).Name != "sinatra-app-example-a" {
		t.Fatal(spew.Sdump(result2))
	}
}
