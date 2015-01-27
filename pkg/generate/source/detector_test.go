package source

import (
	"strings"
	"testing"
)

func TestDetectSource(t *testing.T) {
	d := Detectors{fake1, fake2}
	i, ok := d.DetectSource("test_fake1")
	if !ok {
		t.Errorf("Unable to detect source for test_fake1")
	}
	if i.Platform != "fake1" {
		t.Errorf("Invalid platform for test_fake1")
	}
	if i, ok = d.DetectSource("test_fake3"); ok {
		t.Errorf("Detected source for invalid dir: test_fake3")
	}
	i, ok = d.DetectSource("test_fake2")
	if !ok {
		t.Errorf("Unable to detect source for test_fake2")
	}
	if i.Platform != "fake2" {
		t.Errorf("Invalid platform for test_fake2")
	}
}

func fake1(dir string) (*Info, bool) {
	if strings.Contains(dir, "fake1") {
		return &Info{
			Platform: "fake1",
		}, true
	}
	return nil, false
}

func fake2(dir string) (*Info, bool) {
	if strings.Contains(dir, "fake2") {
		return &Info{
			Platform: "fake2",
		}, true
	}
	return nil, false

}
