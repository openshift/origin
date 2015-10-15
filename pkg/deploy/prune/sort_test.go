package prune

import (
	"sort"
	"testing"
	"time"

	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/util"
)

// TestSort verifies that builds are sorted by most recently created
func TestSort(t *testing.T) {
	present := unversioned.Now()
	past := unversioned.NewTime(present.Time.Add(-1 * time.Minute))
	controllers := []*kapi.ReplicationController{
		{
			ObjectMeta: kapi.ObjectMeta{
				Name:              "past",
				CreationTimestamp: past,
			},
		},
		{
			ObjectMeta: kapi.ObjectMeta{
				Name:              "present",
				CreationTimestamp: present,
			},
		},
	}
	sort.Sort(sortableReplicationControllers(controllers))
	if controllers[0].Name != "present" {
		t.Errorf("Unexpected sort order")
	}
	if controllers[1].Name != "past" {
		t.Errorf("Unexpected sort order")
	}
}
