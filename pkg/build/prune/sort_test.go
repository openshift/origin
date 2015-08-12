package prune

import (
	"sort"
	"testing"
	"time"

	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/util"

	buildapi "github.com/openshift/origin/pkg/build/api"
)

// TestSort verifies that builds are sorted by most recently created
func TestSort(t *testing.T) {
	present := util.Now()
	past := util.NewTime(present.Time.Add(-1 * time.Minute))
	builds := []*buildapi.Build{
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
	sort.Sort(sortableBuilds(builds))
	if builds[0].Name != "present" {
		t.Errorf("Unexpected sort order")
	}
	if builds[1].Name != "past" {
		t.Errorf("Unexpected sort order")
	}
}
