package docker

import (
	"encoding/json"
	"net/http"
	"reflect"
	"testing"

	"github.com/docker/docker/api/types/registry"
)

func TestInspectDistribution(t *testing.T) {
	t.Parallel()
	jsonDistribution := `{
   "Descriptor": {
    "MediaType": "application/vnd.docker.distribution.manifest.v2+json",
    "Digest": "sha256:c0537ff6a5218ef531ece93d4984efc99bbf3f7497c0a7726c88e2bb7584dc96",
    "Size": 3987495,
    "URLs": [
      ""
    ]
  },
  "Platforms": [
    {
      "Architecture": "amd64",
      "OS": "linux",
      "OSVersion": "",
      "OSFeatures": [
        ""
      ],
      "Variant": "",
      "Features": [
        ""
      ]
    }
  ]
}`

	var expected registry.DistributionInspect
	err := json.Unmarshal([]byte(jsonDistribution), &expected)
	if err != nil {
		t.Fatal(err)
	}
	fakeRT := &FakeRoundTripper{message: jsonDistribution, status: http.StatusOK}
	client := newTestClient(fakeRT)
	// image name/tag is not present in the reply, so it can be omitted for testing purposes
	distributionInspect, err := client.InspectDistribution("")
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(*distributionInspect, expected) {
		t.Errorf("InspectDistribution(%q): Expected %#v. Got %#v.", "", expected, distributionInspect)
	}
}
