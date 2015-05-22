package reaper

import (
	"testing"
	"time"

	ktestclient "github.com/GoogleCloudPlatform/kubernetes/pkg/client/testclient"

	"github.com/openshift/origin/pkg/client/testclient"
)

func TestStop(t *testing.T) {
	fakeOsc := &testclient.Fake{}
	fakeKc := &ktestclient.Fake{}
	reaper := &DeploymentConfigReaper{osc: fakeOsc, kc: fakeKc, pollInterval: time.Millisecond, timeout: time.Millisecond}

	expectedOsc := []string{
		"get-deploymentconfig",
		"update-deploymentconfig",
		"get-deploymentconfig",
		"delete-deploymentconfig",
	}
	expectedKc := []string{
		"get-replicationController",
		"update-replicationController",
		"get-replicationController",
		"delete-replicationController",
	}

	str, err := reaper.Stop("default", "foo", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(fakeOsc.Actions) != len(expectedOsc) {
		t.Fatalf("unexpected actions: %v, expected %v", fakeOsc.Actions, expectedOsc)
	}
	for i, fake := range fakeOsc.Actions {
		if fake.Action != expectedOsc[i] {
			t.Fatalf("unexpected action: %s, expected %s", fake.Action, expectedOsc[i])
		}
	}
	if len(fakeKc.Actions) != len(expectedKc) {
		t.Fatalf("unexpected actions: %v, expected %v", fakeKc.Actions, expectedKc)
	}
	for i, fake := range fakeKc.Actions {
		if fake.Action != expectedKc[i] {
			t.Fatalf("unexpected action: %s, expected %s", fake.Action, expectedKc[i])
		}
	}
	if str != "foo stopped" {
		t.Fatalf("unexpected output %q, expected 'foo stopped'", str)
	}

}
