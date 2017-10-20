package v2

import (
	"testing"
)

func TestDefaultClientConfiguration(t *testing.T) {
	testConfiguration := DefaultClientConfiguration()

	if testConfiguration.APIVersion != LatestAPIVersion() {
		t.Error("unexpected API Version")
	}
	if testConfiguration.TimeoutSeconds != 60 {
		t.Error("unexpected TimeoutSeconds")
	}
	if testConfiguration.EnableAlphaFeatures != false {
		t.Error("expected Alpha Features to be disabled")
	}
}
