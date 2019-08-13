package upgrade

import (
	"math/rand"
	"reflect"
	"testing"

	configv1 "github.com/openshift/api/config/v1"
)

func TestSortSemanticVersions(t *testing.T) {
	expected := []configv1.Update{
		{Version: "not-sem-ver-1"},
		{Version: "not-sem-ver-2"},
		{Version: "2.0.0"},
		{Version: "2.0.1"},
		{Version: "10.0.0"},
	}

	actual := make([]configv1.Update, len(expected))
	for i, j := range rand.Perm(len(expected)) {
		actual[i] = expected[j]
	}

	sortSemanticVersions(actual)
	if !reflect.DeepEqual(actual, expected) {
		t.Errorf("%v != %v", actual, expected)
	}
}
