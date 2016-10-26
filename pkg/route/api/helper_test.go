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

func TestGetDomainForHost(t *testing.T) {
	tests := []struct {
		name        string
		host        string
		expectation string
	}{
		{
			name:        "plain",
			host:        "www.host.test",
			expectation: "host.test",
		},
		{
			name:        "aceswild",
			host:        "www777.aceswild.test",
			expectation: "aceswild.test",
		},
		{
			name:        "subdomain1",
			host:        "one.test",
			expectation: "test",
		},
		{
			name:        "subdomain2",
			host:        "two.test",
			expectation: "test",
		},
		{
			name:        "subdomain3",
			host:        "three.org",
			expectation: "org",
		},
		{
			name:        "nested subdomain",
			host:        "www.acme.test",
			expectation: "acme.test",
		},
		{
			name:        "nested subdomain2",
			host:        "www.edge.acme.test",
			expectation: "edge.acme.test",
		},
		{
			name:        "nested subdomain3",
			host:        "www.mail.edge.acme.test",
			expectation: "mail.edge.acme.test",
		},
		{
			name:        "No host",
			host:        "",
			expectation: "",
		},
		{
			name:        "tld1",
			host:        "test",
			expectation: "",
		},
		{
			name:        "tld2",
			host:        "org",
			expectation: "",
		},
		{
			name:        "semi-longish host",
			host:        "www1.dept2.group3.div4.co5.akamai.test",
			expectation: "dept2.group3.div4.co5.akamai.test",
		},
	}

	for _, tc := range tests {
		subdomain := GetDomainForHost(tc.host)

		if subdomain != tc.expectation {
			t.Errorf("Test case %s expected %v got %v", tc.name, tc.expectation, subdomain)
		}
	}
}
