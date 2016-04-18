package api

import (
	"testing"
	"time"

	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/unversioned"
)

func TestRouteLessThan(t *testing.T) {
	r := Route{
		ObjectMeta: kapi.ObjectMeta{
			CreationTimestamp: unversioned.Now().Rfc3339Copy(),
			UID:               "alpha",
			Namespace:         "alpha",
			Name:              "alpha",
		},
	}
	tcs := []struct {
		r        Route
		expected bool
	}{
		{Route{
			ObjectMeta: kapi.ObjectMeta{
				CreationTimestamp: unversioned.Time{
					Time: r.CreationTimestamp.Add(time.Minute),
				},
			},
		}, true},
		{Route{
			ObjectMeta: kapi.ObjectMeta{
				CreationTimestamp: r.CreationTimestamp,
				UID:               "beta",
			},
		}, true},
		{Route{
			ObjectMeta: kapi.ObjectMeta{
				CreationTimestamp: r.CreationTimestamp,
				UID:               r.UID,
				Namespace:         "beta",
			},
		}, true},
		{Route{
			ObjectMeta: kapi.ObjectMeta{
				CreationTimestamp: r.CreationTimestamp,
				UID:               r.UID,
				Namespace:         r.Namespace,
				Name:              "beta",
			},
		}, true},
		{r, false},
	}

	for _, tc := range tcs {
		if RouteLessThan(&r, &tc.r) != tc.expected {
			var msg string
			if tc.expected {
				msg = "Expected %v to be less than %v"
			} else {
				msg = "Expected %v to not be less than %v"
			}
			t.Errorf(msg, r, tc.r)
		}
	}
}
