package appsserialization

import (
	"strings"
	"testing"

	"k8s.io/apimachinery/pkg/runtime"

	"github.com/davecgh/go-spew/spew"

	appsv1 "github.com/openshift/api/apps/v1"
)

const legacyDC = `{
  "apiVersion": "v1",
  "kind": "DeploymentConfig",
  "metadata": {
    "name": "sinatra-app-example-a"
  }
}
`

func TestLegacyDecoding(t *testing.T) {
	result, err := runtime.Decode(annotationDecoder, []byte(legacyDC))
	if err != nil {
		t.Fatal(err)
	}
	if result.(*appsv1.DeploymentConfig).Name != "sinatra-app-example-a" {
		t.Fatal(spew.Sdump(result))
	}

	groupfiedBytes, err := runtime.Encode(annotationEncoder, result)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(groupfiedBytes), "apps.openshift.io/v1") {
		t.Fatal(string(groupfiedBytes))
	}

	result2, err := runtime.Decode(annotationDecoder, groupfiedBytes)
	if err != nil {
		t.Fatal(err)
	}
	if result2.(*appsv1.DeploymentConfig).Name != "sinatra-app-example-a" {
		t.Fatal(spew.Sdump(result2))
	}
}
