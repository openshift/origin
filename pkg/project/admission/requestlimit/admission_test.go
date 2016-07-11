package requestlimit

import (
	"bytes"
	"testing"

	"k8s.io/kubernetes/pkg/admission"
	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/auth/user"
	"k8s.io/kubernetes/pkg/client/cache"
	ktestclient "k8s.io/kubernetes/pkg/client/unversioned/testclient"
	"k8s.io/kubernetes/pkg/labels"
	"k8s.io/kubernetes/pkg/runtime"

	"github.com/openshift/origin/pkg/client/testclient"
	oadmission "github.com/openshift/origin/pkg/cmd/server/admission"
	requestlimitapi "github.com/openshift/origin/pkg/project/admission/requestlimit/api"
	projectapi "github.com/openshift/origin/pkg/project/api"
	projectcache "github.com/openshift/origin/pkg/project/cache"
	userapi "github.com/openshift/origin/pkg/user/api"
	apierrors "k8s.io/kubernetes/pkg/api/errors"

	// install all APIs
	_ "github.com/openshift/origin/pkg/api/install"
)

func TestReadConfig(t *testing.T) {

	tests := []struct {
		config      string
		expected    requestlimitapi.ProjectRequestLimitConfig
		errExpected bool
	}{
		{
			// multiple selectors
			config: `apiVersion: v1
kind: ProjectRequestLimitConfig
limits:
- selector:
    level:
      platinum
- selector:
    level:
      gold
  maxProjects: 500
- selector:
    level:
      silver
  maxProjects: 100
- selector:
    level:
      bronze
  maxProjects: 20
- selector: {}
  maxProjects: 1
`,
			expected: requestlimitapi.ProjectRequestLimitConfig{
				Limits: []requestlimitapi.ProjectLimitBySelector{
					{
						Selector:    map[string]string{"level": "platinum"},
						MaxProjects: nil,
					},
					{
						Selector:    map[string]string{"level": "gold"},
						MaxProjects: intp(500),
					},
					{
						Selector:    map[string]string{"level": "silver"},
						MaxProjects: intp(100),
					},
					{
						Selector:    map[string]string{"level": "bronze"},
						MaxProjects: intp(20),
					},
					{
						Selector:    map[string]string{},
						MaxProjects: intp(1),
					},
				},
			},
		},
		{
			// single selector
			config: `apiVersion: v1
kind: ProjectRequestLimitConfig
limits:
- maxProjects: 1
`,
			expected: requestlimitapi.ProjectRequestLimitConfig{
				Limits: []requestlimitapi.ProjectLimitBySelector{
					{
						Selector:    nil,
						MaxProjects: intp(1),
					},
				},
			},
		},
		{
			// no selectors
			config: `apiVersion: v1
kind: ProjectRequestLimitConfig
`,
			expected: requestlimitapi.ProjectRequestLimitConfig{},
		},
	}

	for n, tc := range tests {
		cfg, err := readConfig(bytes.NewBufferString(tc.config))
		if err != nil && !tc.errExpected {
			t.Errorf("%d: unexpected error: %v", n, err)
			continue
		}
		if err == nil && tc.errExpected {
			t.Errorf("%d: expected error, got none", n)
			continue
		}
		if !configEquals(cfg, &tc.expected) {
			t.Errorf("%d: unexpected result. Got %#v. Expected %#v", n, cfg, tc.expected)
		}
	}
}

func TestMaxProjectByRequester(t *testing.T) {
	tests := []struct {
		userLabels      map[string]string
		expectUnlimited bool
		expectedLimit   int
	}{
		{
			userLabels:      map[string]string{"platinum": "yes"},
			expectUnlimited: true,
		},
		{
			userLabels:    map[string]string{"gold": "yes"},
			expectedLimit: 10,
		},
		{
			userLabels:    map[string]string{"silver": "yes", "bronze": "yes"},
			expectedLimit: 3,
		},
		{
			userLabels:    map[string]string{"unknown": "yes"},
			expectedLimit: 1,
		},
	}

	for _, tc := range tests {
		reqLimit, err := NewProjectRequestLimit(multiLevelConfig())
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}
		user := fakeUser("testuser", tc.userLabels)
		client := testclient.NewSimpleFake(user)
		reqLimit.(oadmission.WantsOpenshiftClient).SetOpenshiftClient(client)

		maxProjects, hasLimit, err := reqLimit.(*projectRequestLimit).maxProjectsByRequester("testuser")
		if err != nil {
			t.Errorf("Unexpected error: %v", err)
		}

		if tc.expectUnlimited {

			if hasLimit {
				t.Errorf("Expected no limit, but got limit for labels %v", tc.userLabels)
			}
			continue
		}
		if !tc.expectUnlimited && !hasLimit {
			t.Errorf("Did not expect unlimited for labels %v", tc.userLabels)
			continue
		}
		if maxProjects != tc.expectedLimit {
			t.Errorf("Did not get expected limit for labels %v. Got: %d. Expected: %d", tc.userLabels, maxProjects, tc.expectedLimit)
		}
	}
}

func TestProjectCountByRequester(t *testing.T) {
	pCache := fakeProjectCache(map[string]projectCount{
		"user1": {1, 5}, // total 6, expect 4
		"user2": {5, 1}, // total 6, expect 5
		"user3": {1, 0}, // total 1, expect 1
	})
	reqLimit := &projectRequestLimit{
		cache: pCache,
	}
	tests := []struct {
		user   string
		expect int
	}{
		{
			user:   "user1",
			expect: 4,
		},
		{
			user:   "user2",
			expect: 5,
		},
		{
			user:   "user3",
			expect: 1,
		},
	}

	for _, test := range tests {
		actual, err := reqLimit.projectCountByRequester(test.user)
		if err != nil {
			t.Errorf("unexpected: %v", err)
		}
		if actual != test.expect {
			t.Errorf("user %s got %d, expected %d", test.user, actual, test.expect)
		}
	}

}

func TestAdmit(t *testing.T) {
	tests := []struct {
		config          *requestlimitapi.ProjectRequestLimitConfig
		user            string
		expectForbidden bool
	}{
		{
			config: multiLevelConfig(),
			user:   "user1",
		},
		{
			config:          multiLevelConfig(),
			user:            "user2",
			expectForbidden: true,
		},
		{
			config: multiLevelConfig(),
			user:   "user3",
		},
		{
			config:          multiLevelConfig(),
			user:            "user4",
			expectForbidden: true,
		},
		{
			config: emptyConfig(),
			user:   "user2",
		},
		{
			config:          singleDefaultConfig(),
			user:            "user3",
			expectForbidden: true,
		},
		{
			config: singleDefaultConfig(),
			user:   "user1",
		},
		{
			config: nil,
			user:   "user3",
		},
	}

	for _, tc := range tests {
		pCache := fakeProjectCache(map[string]projectCount{
			"user1": {0, 1},
			"user2": {2, 2},
			"user3": {5, 3},
			"user4": {1, 0},
		})
		client := &testclient.Fake{}
		client.AddReactor("get", "users", userFn(map[string]labels.Set{
			"user2": {"bronze": "yes"},
			"user3": {"platinum": "yes"},
			"user4": {"unknown": "yes"},
		}))
		reqLimit, err := NewProjectRequestLimit(tc.config)
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}
		reqLimit.(oadmission.WantsOpenshiftClient).SetOpenshiftClient(client)
		reqLimit.(oadmission.WantsProjectCache).SetProjectCache(pCache)
		if err = reqLimit.(oadmission.Validator).Validate(); err != nil {
			t.Fatalf("validation error: %v", err)
		}
		err = reqLimit.Admit(admission.NewAttributesRecord(
			&projectapi.ProjectRequest{},
			nil,
			projectapi.Kind("ProjectRequest").WithVersion("version"),
			"foo",
			"name",
			projectapi.Resource("projectrequests").WithVersion("version"),
			"",
			"CREATE",
			&user.DefaultInfo{Name: tc.user}))
		if err != nil && !tc.expectForbidden {
			t.Errorf("Got unexpected error for user %s: %v", tc.user, err)
			continue
		}
		if !apierrors.IsForbidden(err) && tc.expectForbidden {
			t.Errorf("Expecting forbidden error for user %s and config %#v. Got: %v", tc.user, tc.config, err)
		}
	}
}

func intp(n int) *int {
	return &n
}

func selectorEquals(a, b map[string]string) bool {
	if len(a) != len(b) {
		return false
	}
	for k, v := range a {
		if b[k] != v {
			return false
		}
	}
	return true
}

func configEquals(a, b *requestlimitapi.ProjectRequestLimitConfig) bool {
	if len(a.Limits) != len(b.Limits) {
		return false
	}
	for n, limit := range a.Limits {
		limit2 := b.Limits[n]
		if !selectorEquals(limit.Selector, limit2.Selector) {
			return false
		}
		if (limit.MaxProjects == nil || limit2.MaxProjects == nil) && limit.MaxProjects != limit2.MaxProjects {
			return false
		}
		if limit.MaxProjects == nil {
			continue
		}
		if *limit.MaxProjects != *limit2.MaxProjects {
			return false
		}
	}
	return true
}

func fakeNs(name string, terminating bool) *kapi.Namespace {
	ns := &kapi.Namespace{}
	ns.Name = kapi.SimpleNameGenerator.GenerateName("testns")
	ns.Annotations = map[string]string{
		"openshift.io/requester": name,
	}
	if terminating {
		ns.Status.Phase = kapi.NamespaceTerminating
	}
	return ns
}

func fakeUser(name string, labels map[string]string) *userapi.User {
	user := &userapi.User{}
	user.Name = name
	user.Labels = labels
	return user
}

type projectCount struct {
	active      int
	terminating int
}

func fakeProjectCache(requesters map[string]projectCount) *projectcache.ProjectCache {
	kclient := &ktestclient.Fake{}
	pCache := projectcache.NewFake(kclient.Namespaces(), projectcache.NewCacheStore(cache.MetaNamespaceKeyFunc), "")
	for requester, count := range requesters {
		for i := 0; i < count.active; i++ {
			pCache.Store.Add(fakeNs(requester, false))
		}
		for i := 0; i < count.terminating; i++ {
			pCache.Store.Add(fakeNs(requester, true))
		}
	}
	return pCache
}

func userFn(usersAndLabels map[string]labels.Set) ktestclient.ReactionFunc {
	return func(action ktestclient.Action) (handled bool, ret runtime.Object, err error) {
		name := action.(ktestclient.GetAction).GetName()
		return true, fakeUser(name, map[string]string(usersAndLabels[name])), nil
	}
}

func multiLevelConfig() *requestlimitapi.ProjectRequestLimitConfig {
	return &requestlimitapi.ProjectRequestLimitConfig{
		Limits: []requestlimitapi.ProjectLimitBySelector{
			{
				Selector:    map[string]string{"platinum": "yes"},
				MaxProjects: nil,
			},
			{
				Selector:    map[string]string{"gold": "yes"},
				MaxProjects: intp(10),
			},
			{
				Selector:    map[string]string{"silver": "yes"},
				MaxProjects: intp(3),
			},
			{
				Selector:    map[string]string{"bronze": "yes"},
				MaxProjects: intp(2),
			},
			{
				Selector:    map[string]string{},
				MaxProjects: intp(1),
			},
		},
	}
}

func emptyConfig() *requestlimitapi.ProjectRequestLimitConfig {
	return &requestlimitapi.ProjectRequestLimitConfig{}
}

func singleDefaultConfig() *requestlimitapi.ProjectRequestLimitConfig {
	return &requestlimitapi.ProjectRequestLimitConfig{
		Limits: []requestlimitapi.ProjectLimitBySelector{
			{
				Selector:    nil,
				MaxProjects: intp(1),
			},
		},
	}
}
