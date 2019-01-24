package status

import (
	"reflect"
	"testing"
	"time"
)

func TestVersionGetterBasic(t *testing.T) {
	versionGetter := NewVersionGetter()
	versions := versionGetter.GetVersions()
	if versions == nil {
		t.Fatal(versions)
	}

	ch := versionGetter.VersionChangedChannel()
	if ch == nil {
		t.Fatal(ch)
	}

	versionGetter.SetVersion("foo", "bar")

	select {
	case <-ch:
		actual := versionGetter.GetVersions()
		expected := map[string]string{"foo": "bar"}
		if !reflect.DeepEqual(expected, actual) {
			t.Fatal(actual)
		}

	case <-time.After(5 * time.Second):
		t.Fatal("missing")
	}

}
