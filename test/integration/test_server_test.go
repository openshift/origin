// +build integration,!no-etcd

package integration

import (
	"testing"

	"github.com/GoogleCloudPlatform/kubernetes/pkg/labels"
)

func TestStartTestAllInOne(t *testing.T) {
	client, err := StartTestAllInOne()
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	policies, err := client.Policies("master").List(labels.Everything(), labels.Everything())
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if len(policies.Items) == 0 {
		t.Errorf("expected policies, but didn't get any")
	}
}
func TestStartTestMaster(t *testing.T) {
	client, err := StartTestMaster()
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	policies, err := client.Policies("master").List(labels.Everything(), labels.Everything())
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if len(policies.Items) == 0 {
		t.Errorf("expected policies, but didn't get any")
	}
}
