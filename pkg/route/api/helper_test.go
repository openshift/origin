package api

import (
	"testing"
	"time"

	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/unversioned"
)

func TestRouteLessThan(t *testing.T) {
	current := unversioned.Now()
	older := unversioned.Time{Time: current.Add(-1 * time.Minute)}

	r := Route{
		ObjectMeta: kapi.ObjectMeta{
			CreationTimestamp: current.Rfc3339Copy(),
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
		{Route{
			ObjectMeta: kapi.ObjectMeta{
				CreationTimestamp: older,
				UID:               r.UID,
				Namespace:         r.Namespace,
				Name:              "beta",
			},
		}, false},
		{Route{
			ObjectMeta: kapi.ObjectMeta{
				CreationTimestamp: older,
				UID:               r.UID,
				Name:              "gamma",
			},
		}, false},
		{Route{
			ObjectMeta: kapi.ObjectMeta{
				CreationTimestamp: older,
				Name:              "delta",
			},
		}, false},
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

func TestNormalizeWildcardHost(t *testing.T) {
	tests := []struct {
		name        string
		host        string
		expectation string
		wildcard    bool
	}{
		{
			name:        "plain",
			host:        "www.host.test",
			expectation: "www.host.test",
			wildcard:    false,
		},
		{
			name:        "aceswild",
			host:        "*.aceswild.test",
			expectation: "aceswild.test",
			wildcard:    true,
		},
		{
			name:        "otherwild",
			host:        "aces.*.test",
			expectation: "aces.*.test",
			wildcard:    false,
		},
		{
			name:        "Invalid host",
			host:        "*.aces.*.test",
			expectation: "aces.*.test",
			wildcard:    true,
		},
		{
			name:        "No host",
			host:        "",
			expectation: "",
			wildcard:    false,
		},
	}

	for _, tc := range tests {
		host, flag := NormalizeWildcardHost(tc.host)

		if flag != tc.wildcard {
			t.Errorf("Test case %s expected %t got %t", tc.name, tc.wildcard, flag)
		} else if host != tc.expectation {
			t.Errorf("Test case %s expected %v got %v", tc.name, tc.expectation, host)
		}
	}
}
