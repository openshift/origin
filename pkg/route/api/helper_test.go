package api

import (
	"testing"
	"time"

	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/unversioned"
	"k8s.io/kubernetes/pkg/types"
)

func TestRouteLessThan(t *testing.T) {
	olderTimestamp := unversioned.Now().Rfc3339Copy()
	newerTimestamp := unversioned.Time{
		Time: olderTimestamp.Add(1 * time.Minute),
	}

	tcs := []struct {
		testName   string
		timestamp1 unversioned.Time
		timestamp2 unversioned.Time
		uid1       types.UID
		uid2       types.UID
		expected   bool
	}{
		{
			testName:   "Older route less than newer route",
			timestamp1: olderTimestamp,
			timestamp2: newerTimestamp,
			expected:   true,
		},
		{
			testName:   "Newer route not less than older route",
			timestamp1: newerTimestamp,
			timestamp2: olderTimestamp,
			expected:   false,
		},
		{
			testName:   "Same age route less with smaller uid",
			timestamp1: newerTimestamp,
			timestamp2: newerTimestamp,
			uid1:       "alpha",
			uid2:       "beta",
			expected:   true,
		},
		{
			testName:   "Same age route not less with greater uid",
			timestamp1: newerTimestamp,
			timestamp2: newerTimestamp,
			uid1:       "beta",
			uid2:       "alpha",
			expected:   false,
		},
	}

	for _, tc := range tcs {
		r1 := &Route{
			ObjectMeta: kapi.ObjectMeta{
				CreationTimestamp: tc.timestamp1,
				UID:               tc.uid1,
			},
		}
		r2 := &Route{
			ObjectMeta: kapi.ObjectMeta{
				CreationTimestamp: tc.timestamp2,
				UID:               tc.uid2,
			},
		}

		if RouteLessThan(r1, r2) != tc.expected {
			var msg string
			if tc.expected {
				msg = "Expected %v to be less than %v"
			} else {
				msg = "Expected %v to not be less than %v"
			}
			t.Errorf(msg, r1, r2)
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
