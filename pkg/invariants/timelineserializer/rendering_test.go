package timelineserializer

import (
	_ "embed"
	"testing"

	monitorserialization "github.com/openshift/origin/pkg/monitor/serialization"
)

//go:embed render_test_01_skip_e2e.json
var skipE2e []byte

func TestBelongsInKubeAPIServer(t *testing.T) {
	inputIntervals, err := monitorserialization.EventsFromJSON(skipE2e)
	if err != nil {
		t.Fatal(err)
	}

	for _, event := range inputIntervals {
		t.Logf("%#v", event)

		if actual := isPlatformPodEvent(event); actual {
			t.Error("expected false")
		}
		if actual := BelongsInKubeAPIServer(event); actual {
			t.Error("expected false")
		}
	}
}
