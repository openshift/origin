package docker

import (
	"encoding/json"
	"net/http"
	"reflect"
	"testing"
)

func TestPruneContainers(t *testing.T) {
	t.Parallel()
	results := `{
		"ContainersDeleted": [
			"a", "b", "c"
		],
		"SpaceReclaimed": 123
	}`

	expected := &PruneContainersResults{}
	err := json.Unmarshal([]byte(results), expected)
	if err != nil {
		t.Fatal(err)
	}
	client := newTestClient(&FakeRoundTripper{message: results, status: http.StatusOK})
	got, err := client.PruneContainers(PruneContainersOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, expected) {
		t.Errorf("PruneContainers: Expected %#v. Got %#v.", expected, got)
	}
}
