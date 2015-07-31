package api

import (
	"sort"
	"testing"
	"time"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/util"
)

func TestSortByCreationTimestamp(t *testing.T) {
	present := util.Now()
	past := util.NewTime(present.Add(-time.Minute))
	builds := []Build{
		{
			ObjectMeta: kapi.ObjectMeta{
				Name:              "present",
				CreationTimestamp: present,
			},
		},
		{
			ObjectMeta: kapi.ObjectMeta{
				Name:              "past",
				CreationTimestamp: past,
			},
		},
	}
	sort.Sort(ByCreationTimestamp(builds))
	if [2]string{builds[0].Name, builds[1].Name} != [2]string{"past", "present"} {
		t.Errorf("Unexpected sort order")
	}
}
