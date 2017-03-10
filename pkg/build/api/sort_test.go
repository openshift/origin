package api

import (
	"sort"
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kapi "k8s.io/kubernetes/pkg/api"
)

func TestSortBuildSliceByCreationTimestamp(t *testing.T) {
	present := metav1.Now()
	past := metav1.NewTime(present.Add(-time.Minute))
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
	sort.Sort(BuildSliceByCreationTimestamp(builds))
	if [2]string{builds[0].Name, builds[1].Name} != [2]string{"past", "present"} {
		t.Errorf("Unexpected sort order")
	}
}

func TestSortBuildPtrSliceByCreationTimestamp(t *testing.T) {
	present := metav1.Now()
	past := metav1.NewTime(present.Add(-time.Minute))
	builds := []*Build{
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
	sort.Sort(BuildPtrSliceByCreationTimestamp(builds))
	if [2]string{builds[0].Name, builds[1].Name} != [2]string{"past", "present"} {
		t.Errorf("Unexpected sort order")
	}
}
