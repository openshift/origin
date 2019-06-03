package imageutil

import (
	"testing"
)

func TestJoinImageStreamTag(t *testing.T) {
	if e, a := "foo:bar", JoinImageStreamTag("foo", "bar"); e != a {
		t.Errorf("Unexpected value: %s", a)
	}
	if e, a := "foo:"+DefaultImageTag, JoinImageStreamTag("foo", ""); e != a {
		t.Errorf("Unexpected value: %s", a)
	}
}
