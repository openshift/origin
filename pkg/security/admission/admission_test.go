package admission

import (
	"reflect"
	"strings"
	"testing"

	kadmission "k8s.io/kubernetes/pkg/admission"
	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/auth/user"
	"k8s.io/kubernetes/pkg/client/cache"
	client "k8s.io/kubernetes/pkg/client/unversioned"
	"k8s.io/kubernetes/pkg/client/unversioned/testclient"
	kscc "k8s.io/kubernetes/pkg/securitycontextconstraints"
	"k8s.io/kubernetes/pkg/util"

	allocator "github.com/openshift/origin/pkg/security"
	"github.com/openshift/origin/pkg/security/uid"
	"sort"
)

func NewTestAdmission(store cache.Store, kclient client.Interface) kadmission.Interface {
	return &constraint{
		Handler: kadmission.NewHandler(kadmission.Create),
		client:  kclient,
		store:   store,
	}
}

func TestAdmit(t *testing.T) {
	// create the annotated namespace and add it to the fake client
	namespace := createNamespaceForTest()
	serviceAccount := createSAForTest()

	// used for cases where things are preallocated
	defaultGroup := int64(2)

	tc := testclient.NewSimpleFake(namespace, serviceAccount)

	// create scc that requires allocation retrieval
	saSCC := &kapi.SecurityContextConstraints{
		ObjectMeta: kapi.ObjectMeta{
			Name: "scc-sa",
		},
		RunAsUser: kapi.RunAsUserStrategyOptions{
			Type: kapi.RunAsUserStrategyMustRunAsRange,
		},
		SELinuxContext: kapi.SELinuxContextStrategyOptions{
			Type: kapi.SELinuxStrategyMustRunAs,
		},
		FSGroup: kapi.FSGroupStrategyOptions{
			Type: kapi.FSGroupStrategyMustRunAs,
		},
		SupplementalGroups: kapi.SupplementalGroupsStrategyOptions{
			Type: kapi.SupplementalGroupsStrategyMustRunAs,
		},
		Groups: []string{"system:serviceaccounts"},
	}
	// create scc that has specific requirements that shouldn't match but is permissioned to
	// service accounts to test that even though this has matching priorities (0) and a
	// lower point value score (which will cause it to be sorted in front of scc-sa) it should not
	// validate the requests so we should try scc-sa.
	var exactUID int64 = 999
	saExactSCC := &kapi.SecurityContextConstraints{
		ObjectMeta: kapi.ObjectMeta{
			Name: "scc-sa-exact",
		},
		RunAsUser: kapi.RunAsUserStrategyOptions{
			Type: kapi.RunAsUserStrategyMustRunAs,
			UID:  &exactUID,
		},
		SELinuxContext: kapi.SELinuxContextStrategyOptions{
			Type: kapi.SELinuxStrategyMustRunAs,
			SELinuxOptions: &kapi.SELinuxOptions{
				Level: "s9:z0,z1",
			},
		},
		FSGroup: kapi.FSGroupStrategyOptions{
			Type: kapi.FSGroupStrategyMustRunAs,
			Ranges: []kapi.IDRange{
				{Min: 999, Max: 999},
			},
		},
		SupplementalGroups: kapi.SupplementalGroupsStrategyOptions{
			Type: kapi.SupplementalGroupsStrategyMustRunAs,
			Ranges: []kapi.IDRange{
				{Min: 999, Max: 999},
			},
		},
		Groups: []string{"system:serviceaccounts"},
	}
	store := cache.NewStore(cache.MetaNamespaceKeyFunc)
	store.Add(saExactSCC)
	store.Add(saSCC)

	// create the admission plugin
	p := NewTestAdmission(store, tc)

	// setup test data
	uidNotInRange := goodPod()
	var uid int64 = 1001
	uidNotInRange.Spec.Containers[0].SecurityContext.RunAsUser = &uid

	invalidMCSLabels := goodPod()
	invalidMCSLabels.Spec.Containers[0].SecurityContext.SELinuxOptions = &kapi.SELinuxOptions{
		Level: "s1:q0,q1",
	}

	disallowedPriv := goodPod()
	var priv bool = true
	disallowedPriv.Spec.Containers[0].SecurityContext.Privileged = &priv

	// specifies a UID in the range of the preallocated UID annotation
	specifyUIDInRange := goodPod()
	var goodUID int64 = 3
	specifyUIDInRange.Spec.Containers[0].SecurityContext.RunAsUser = &goodUID

	// specifies an mcs label that matches the preallocated mcs annotation
	specifyLabels := goodPod()
	specifyLabels.Spec.Containers[0].SecurityContext.SELinuxOptions = &kapi.SELinuxOptions{
		Level: "s0:c1,c0",
	}

	// specifies an FSGroup in the range of preallocated sup group annotation
	specifyFSGroupInRange := goodPod()
	// group in the range of a preallocated fs group which, by default is a single digit range
	// based on the first value of the ns annotation.
	goodFSGroup := int64(2)
	specifyFSGroupInRange.Spec.SecurityContext.FSGroup = &goodFSGroup

	// specifies a sup group in the range of preallocated sup group annotation
	specifySupGroup := goodPod()
	// group is not the default but still in the range
	specifySupGroup.Spec.SecurityContext.SupplementalGroups = []int64{3}

	specifyPodLevelSELinux := goodPod()
	specifyPodLevelSELinux.Spec.SecurityContext.SELinuxOptions = &kapi.SELinuxOptions{
		Level: "s0:c1,c0",
	}

	requestsHostNetwork := goodPod()
	requestsHostNetwork.Spec.SecurityContext.HostNetwork = true

	requestsHostPID := goodPod()
	requestsHostPID.Spec.SecurityContext.HostPID = true

	requestsHostIPC := goodPod()
	requestsHostIPC.Spec.SecurityContext.HostIPC = true

	requestsHostPorts := goodPod()
	requestsHostPorts.Spec.Containers[0].Ports = []kapi.ContainerPort{{HostPort: 1}}

	requestsSupplementalGroup := goodPod()
	requestsSupplementalGroup.Spec.SecurityContext.SupplementalGroups = []int64{1}

	requestsFSGroup := goodPod()
	fsGroup := int64(1)
	requestsFSGroup.Spec.SecurityContext.FSGroup = &fsGroup

	requestsPodLevelMCS := goodPod()
	requestsPodLevelMCS.Spec.SecurityContext.SELinuxOptions = &kapi.SELinuxOptions{
		User:  "user",
		Type:  "type",
		Role:  "role",
		Level: "level",
	}

	testCases := map[string]struct {
		pod               *kapi.Pod
		shouldAdmit       bool
		expectedUID       int64
		expectedLevel     string
		expectedFSGroup   int64
		expectedSupGroups []int64
		expectedPriv      bool
	}{
		"uidNotInRange": {
			pod:         uidNotInRange,
			shouldAdmit: false,
		},
		"invalidMCSLabels": {
			pod:         invalidMCSLabels,
			shouldAdmit: false,
		},
		"disallowedPriv": {
			pod:         disallowedPriv,
			shouldAdmit: false,
		},
		"specifyUIDInRange": {
			pod:               specifyUIDInRange,
			shouldAdmit:       true,
			expectedUID:       *specifyUIDInRange.Spec.Containers[0].SecurityContext.RunAsUser,
			expectedLevel:     "s0:c1,c0",
			expectedFSGroup:   defaultGroup,
			expectedSupGroups: []int64{defaultGroup},
		},
		"specifyLabels": {
			pod:               specifyLabels,
			shouldAdmit:       true,
			expectedUID:       1,
			expectedLevel:     specifyLabels.Spec.Containers[0].SecurityContext.SELinuxOptions.Level,
			expectedFSGroup:   defaultGroup,
			expectedSupGroups: []int64{defaultGroup},
		},
		"specifyFSGroup": {
			pod:               specifyFSGroupInRange,
			shouldAdmit:       true,
			expectedUID:       1,
			expectedLevel:     "s0:c1,c0",
			expectedFSGroup:   *specifyFSGroupInRange.Spec.SecurityContext.FSGroup,
			expectedSupGroups: []int64{defaultGroup},
		},
		"specifySupGroup": {
			pod:               specifySupGroup,
			shouldAdmit:       true,
			expectedUID:       1,
			expectedLevel:     "s0:c1,c0",
			expectedFSGroup:   defaultGroup,
			expectedSupGroups: []int64{specifySupGroup.Spec.SecurityContext.SupplementalGroups[0]},
		},
		"specifyPodLevelSELinuxLevel": {
			pod:               specifyPodLevelSELinux,
			shouldAdmit:       true,
			expectedUID:       1,
			expectedLevel:     "s0:c1,c0",
			expectedFSGroup:   defaultGroup,
			expectedSupGroups: []int64{defaultGroup},
		},
		"requestsHostNetwork": {
			pod:         requestsHostNetwork,
			shouldAdmit: false,
		},
		"requestsHostPorts": {
			pod:         requestsHostPorts,
			shouldAdmit: false,
		},
		"requestsHostPID": {
			pod:         requestsHostPID,
			shouldAdmit: false,
		},
		"requestsHostIPC": {
			pod:         requestsHostIPC,
			shouldAdmit: false,
		},
		"requestsSupplementalGroup": {
			pod:         requestsSupplementalGroup,
			shouldAdmit: false,
		},
		"requestsFSGroup": {
			pod:         requestsFSGroup,
			shouldAdmit: false,
		},
		"requestsPodLevelMCS": {
			pod:         requestsPodLevelMCS,
			shouldAdmit: false,
		},
	}

	for k, v := range testCases {
		attrs := kadmission.NewAttributesRecord(v.pod, "Pod", "namespace", "", string(kapi.ResourcePods), "", kadmission.Create, &user.DefaultInfo{})
		err := p.Admit(attrs)

		if v.shouldAdmit && err != nil {
			t.Errorf("%s expected no errors but received %v", k, err)
		}
		if !v.shouldAdmit && err == nil {
			t.Errorf("%s expected errors but received none", k)
		}

		if v.shouldAdmit {
			validatedSCC, ok := v.pod.Annotations[allocator.ValidatedSCCAnnotation]
			if !ok {
				t.Errorf("%s expected to find the validated annotation on the pod for the scc but found none", k)
			}
			if validatedSCC != saSCC.Name {
				t.Errorf("%s should have validated against %s but found %s", k, saSCC.Name, validatedSCC)
			}

			// ensure anything we expected to be defaulted on the container level is set
			if *v.pod.Spec.Containers[0].SecurityContext.RunAsUser != v.expectedUID {
				t.Errorf("%s expected UID %d but found %d", k, v.expectedUID, *v.pod.Spec.Containers[0].SecurityContext.RunAsUser)
			}
			if v.pod.Spec.Containers[0].SecurityContext.SELinuxOptions.Level != v.expectedLevel {
				t.Errorf("%s expected Level %s but found %s", k, v.expectedLevel, v.pod.Spec.Containers[0].SecurityContext.SELinuxOptions.Level)
			}

			// ensure anything we expected to be defaulted on the pod level is set
			if v.pod.Spec.SecurityContext.SELinuxOptions.Level != v.expectedLevel {
				t.Errorf("%s expected pod level SELinux Level %s but found %s", k, v.expectedLevel, v.pod.Spec.SecurityContext.SELinuxOptions.Level)
			}
			if *v.pod.Spec.SecurityContext.FSGroup != v.expectedFSGroup {
				t.Errorf("%s expected fsgroup %d but found %d", k, v.expectedFSGroup, *v.pod.Spec.SecurityContext.FSGroup)
			}
			if len(v.pod.Spec.SecurityContext.SupplementalGroups) != len(v.expectedSupGroups) {
				t.Errorf("%s found unexpected supplemental groups.  Expected: %v, actual %v", k, v.expectedSupGroups, v.pod.Spec.SecurityContext.SupplementalGroups)
			}
			for _, g := range v.expectedSupGroups {
				if !hasSupGroup(g, v.pod.Spec.SecurityContext.SupplementalGroups) {
					t.Errorf("%s expected sup group %d", k, g)
				}
			}
		}
	}

	// now add an escalated scc to the group and re-run the cases that expected failure, they should
	// now pass by validating against the escalated scc.
	adminSCC := &kapi.SecurityContextConstraints{
		ObjectMeta: kapi.ObjectMeta{
			Name: "scc-admin",
		},
		AllowPrivilegedContainer: true,
		AllowHostNetwork:         true,
		AllowHostPorts:           true,
		AllowHostPID:             true,
		AllowHostIPC:             true,
		RunAsUser: kapi.RunAsUserStrategyOptions{
			Type: kapi.RunAsUserStrategyRunAsAny,
		},
		SELinuxContext: kapi.SELinuxContextStrategyOptions{
			Type: kapi.SELinuxStrategyRunAsAny,
		},
		FSGroup: kapi.FSGroupStrategyOptions{
			Type: kapi.FSGroupStrategyRunAsAny,
		},
		SupplementalGroups: kapi.SupplementalGroupsStrategyOptions{
			Type: kapi.SupplementalGroupsStrategyRunAsAny,
		},
		Groups: []string{"system:serviceaccounts"},
	}
	store.Add(adminSCC)

	for k, v := range testCases {
		if !v.shouldAdmit {
			attrs := kadmission.NewAttributesRecord(v.pod, "Pod", "namespace", "", string(kapi.ResourcePods), "", kadmission.Create, &user.DefaultInfo{})
			err := p.Admit(attrs)
			if err != nil {
				t.Errorf("Expected %s to pass with escalated scc but got error %v", k, err)
			}
			validatedSCC, ok := v.pod.Annotations[allocator.ValidatedSCCAnnotation]
			if !ok {
				t.Errorf("%s expected to find the validated annotation on the pod for the scc but found none", k)
			}
			if validatedSCC != adminSCC.Name {
				t.Errorf("%s should have validated against %s but found %s", k, adminSCC.Name, validatedSCC)
			}
		}
	}
}

func hasSupGroup(group int64, groups []int64) bool {
	for _, g := range groups {
		if g == group {
			return true
		}
	}
	return false
}

func TestAssignSecurityContext(t *testing.T) {
	// set up test data
	// scc that will deny privileged container requests and has a default value for a field (uid)
	var uid int64 = 9999
	fsGroup := int64(1)
	scc := &kapi.SecurityContextConstraints{
		ObjectMeta: kapi.ObjectMeta{
			Name: "test scc",
		},
		SELinuxContext: kapi.SELinuxContextStrategyOptions{
			Type: kapi.SELinuxStrategyRunAsAny,
		},
		RunAsUser: kapi.RunAsUserStrategyOptions{
			Type: kapi.RunAsUserStrategyMustRunAs,
			UID:  &uid,
		},

		// require allocation for a field in the psc as well to test changes/no changes
		FSGroup: kapi.FSGroupStrategyOptions{
			Type: kapi.FSGroupStrategyMustRunAs,
			Ranges: []kapi.IDRange{
				{Min: fsGroup, Max: fsGroup},
			},
		},
		SupplementalGroups: kapi.SupplementalGroupsStrategyOptions{
			Type: kapi.SupplementalGroupsStrategyRunAsAny,
		},
	}
	provider, err := kscc.NewSimpleProvider(scc)
	if err != nil {
		t.Fatalf("failed to create provider: %v", err)
	}

	createContainer := func(priv bool) kapi.Container {
		return kapi.Container{
			SecurityContext: &kapi.SecurityContext{
				Privileged: &priv,
			},
		}
	}

	// these are set up such that the containers always have a nil uid.  If the case should not
	// validate then the uids should not have been updated by the strategy.  If the case should
	// validate then uids should be set.  This is ensuring that we're hanging on to the old SC
	// as we generate/validate and only updating the original container if the entire pod validates
	testCases := map[string]struct {
		pod            *kapi.Pod
		shouldValidate bool
		expectedUID    *int64
	}{
		"pod and container SC is not changed when invalid": {
			pod: &kapi.Pod{
				Spec: kapi.PodSpec{
					SecurityContext: &kapi.PodSecurityContext{},
					Containers:      []kapi.Container{createContainer(true)},
				},
			},
			shouldValidate: false,
		},
		"must validate all containers": {
			pod: &kapi.Pod{
				Spec: kapi.PodSpec{
					// good pod and bad pod
					SecurityContext: &kapi.PodSecurityContext{},
					Containers:      []kapi.Container{createContainer(false), createContainer(true)},
				},
			},
			shouldValidate: false,
		},
		"pod validates": {
			pod: &kapi.Pod{
				Spec: kapi.PodSpec{
					SecurityContext: &kapi.PodSecurityContext{},
					Containers:      []kapi.Container{createContainer(false)},
				},
			},
			shouldValidate: true,
		},
	}

	for k, v := range testCases {
		errs := assignSecurityContext(provider, v.pod)
		if v.shouldValidate && len(errs) > 0 {
			t.Errorf("%s expected to validate but received errors %v", k, errs)
			continue
		}
		if !v.shouldValidate && len(errs) == 0 {
			t.Errorf("%s expected validation errors but received none", k)
			continue
		}

		// if we shouldn't have validated ensure that uid is not set on the containers
		// and ensure the psc does not have fsgroup set
		if !v.shouldValidate {
			if v.pod.Spec.SecurityContext.FSGroup != nil {
				t.Errorf("%s had a non-nil FSGroup %d.  FSGroup should not be set on test cases that don't validate", k, *v.pod.Spec.SecurityContext.FSGroup)
			}
			for _, c := range v.pod.Spec.Containers {
				if c.SecurityContext.RunAsUser != nil {
					t.Errorf("%s had non-nil UID %d.  UID should not be set on test cases that don't validate", k, *c.SecurityContext.RunAsUser)
				}
			}
		}

		// if we validated then the pod sc should be updated now with the defaults from the SCC
		if v.shouldValidate {
			if *v.pod.Spec.SecurityContext.FSGroup != fsGroup {
				t.Errorf("%s expected fsgroup to be defaulted but found %v", k, v.pod.Spec.SecurityContext.FSGroup)
			}
			for _, c := range v.pod.Spec.Containers {
				if *c.SecurityContext.RunAsUser != uid {
					t.Errorf("%s expected uid to be defaulted to %d but found %v", k, uid, c.SecurityContext.RunAsUser)
				}
			}
		}
	}
}

func TestCreateProvidersFromConstraints(t *testing.T) {
	namespaceValid := &kapi.Namespace{
		ObjectMeta: kapi.ObjectMeta{
			Name: "default",
			Annotations: map[string]string{
				allocator.UIDRangeAnnotation:           "1/3",
				allocator.MCSAnnotation:                "s0:c1,c0",
				allocator.SupplementalGroupsAnnotation: "1/3",
			},
		},
	}
	namespaceNoUID := &kapi.Namespace{
		ObjectMeta: kapi.ObjectMeta{
			Name: "default",
			Annotations: map[string]string{
				allocator.MCSAnnotation:                "s0:c1,c0",
				allocator.SupplementalGroupsAnnotation: "1/3",
			},
		},
	}
	namespaceNoMCS := &kapi.Namespace{
		ObjectMeta: kapi.ObjectMeta{
			Name: "default",
			Annotations: map[string]string{
				allocator.UIDRangeAnnotation:           "1/3",
				allocator.SupplementalGroupsAnnotation: "1/3",
			},
		},
	}

	namespaceNoSupplementalGroupsFallbackToUID := &kapi.Namespace{
		ObjectMeta: kapi.ObjectMeta{
			Name: "default",
			Annotations: map[string]string{
				allocator.UIDRangeAnnotation: "1/3",
				allocator.MCSAnnotation:      "s0:c1,c0",
			},
		},
	}

	namespaceBadSupGroups := &kapi.Namespace{
		ObjectMeta: kapi.ObjectMeta{
			Name: "default",
			Annotations: map[string]string{
				allocator.UIDRangeAnnotation:           "1/3",
				allocator.MCSAnnotation:                "s0:c1,c0",
				allocator.SupplementalGroupsAnnotation: "",
			},
		},
	}

	testCases := map[string]struct {
		// use a generating function so we can test for non-mutation
		scc         func() *kapi.SecurityContextConstraints
		namespace   *kapi.Namespace
		expectedErr string
	}{
		"valid non-preallocated scc": {
			scc: func() *kapi.SecurityContextConstraints {
				return &kapi.SecurityContextConstraints{
					ObjectMeta: kapi.ObjectMeta{
						Name: "valid non-preallocated scc",
					},
					SELinuxContext: kapi.SELinuxContextStrategyOptions{
						Type: kapi.SELinuxStrategyRunAsAny,
					},
					RunAsUser: kapi.RunAsUserStrategyOptions{
						Type: kapi.RunAsUserStrategyRunAsAny,
					},
					FSGroup: kapi.FSGroupStrategyOptions{
						Type: kapi.FSGroupStrategyRunAsAny,
					},
					SupplementalGroups: kapi.SupplementalGroupsStrategyOptions{
						Type: kapi.SupplementalGroupsStrategyRunAsAny,
					},
				}
			},
			namespace: namespaceValid,
		},
		"valid pre-allocated scc": {
			scc: func() *kapi.SecurityContextConstraints {
				return &kapi.SecurityContextConstraints{
					ObjectMeta: kapi.ObjectMeta{
						Name: "valid pre-allocated scc",
					},
					SELinuxContext: kapi.SELinuxContextStrategyOptions{
						Type:           kapi.SELinuxStrategyMustRunAs,
						SELinuxOptions: &kapi.SELinuxOptions{User: "myuser"},
					},
					RunAsUser: kapi.RunAsUserStrategyOptions{
						Type: kapi.RunAsUserStrategyMustRunAsRange,
					},
					FSGroup: kapi.FSGroupStrategyOptions{
						Type: kapi.FSGroupStrategyMustRunAs,
					},
					SupplementalGroups: kapi.SupplementalGroupsStrategyOptions{
						Type: kapi.SupplementalGroupsStrategyMustRunAs,
					},
				}
			},
			namespace: namespaceValid,
		},
		"pre-allocated no uid annotation": {
			scc: func() *kapi.SecurityContextConstraints {
				return &kapi.SecurityContextConstraints{
					ObjectMeta: kapi.ObjectMeta{
						Name: "pre-allocated no uid annotation",
					},
					SELinuxContext: kapi.SELinuxContextStrategyOptions{
						Type: kapi.SELinuxStrategyMustRunAs,
					},
					RunAsUser: kapi.RunAsUserStrategyOptions{
						Type: kapi.RunAsUserStrategyMustRunAsRange,
					},
					FSGroup: kapi.FSGroupStrategyOptions{
						Type: kapi.FSGroupStrategyRunAsAny,
					},
					SupplementalGroups: kapi.SupplementalGroupsStrategyOptions{
						Type: kapi.SupplementalGroupsStrategyRunAsAny,
					},
				}
			},
			namespace:   namespaceNoUID,
			expectedErr: "unable to find pre-allocated uid annotation",
		},
		"pre-allocated no mcs annotation": {
			scc: func() *kapi.SecurityContextConstraints {
				return &kapi.SecurityContextConstraints{
					ObjectMeta: kapi.ObjectMeta{
						Name: "pre-allocated no mcs annotation",
					},
					SELinuxContext: kapi.SELinuxContextStrategyOptions{
						Type: kapi.SELinuxStrategyMustRunAs,
					},
					RunAsUser: kapi.RunAsUserStrategyOptions{
						Type: kapi.RunAsUserStrategyMustRunAsRange,
					},
					FSGroup: kapi.FSGroupStrategyOptions{
						Type: kapi.FSGroupStrategyRunAsAny,
					},
					SupplementalGroups: kapi.SupplementalGroupsStrategyOptions{
						Type: kapi.SupplementalGroupsStrategyRunAsAny,
					},
				}
			},
			namespace:   namespaceNoMCS,
			expectedErr: "unable to find pre-allocated mcs annotation",
		},
		"pre-allocated group falls back to UID annotation": {
			scc: func() *kapi.SecurityContextConstraints {
				return &kapi.SecurityContextConstraints{
					ObjectMeta: kapi.ObjectMeta{
						Name: "pre-allocated no sup group annotation",
					},
					SELinuxContext: kapi.SELinuxContextStrategyOptions{
						Type: kapi.SELinuxStrategyRunAsAny,
					},
					RunAsUser: kapi.RunAsUserStrategyOptions{
						Type: kapi.RunAsUserStrategyRunAsAny,
					},
					FSGroup: kapi.FSGroupStrategyOptions{
						Type: kapi.FSGroupStrategyMustRunAs,
					},
					SupplementalGroups: kapi.SupplementalGroupsStrategyOptions{
						Type: kapi.SupplementalGroupsStrategyMustRunAs,
					},
				}
			},
			namespace: namespaceNoSupplementalGroupsFallbackToUID,
		},
		"pre-allocated group bad value fails": {
			scc: func() *kapi.SecurityContextConstraints {
				return &kapi.SecurityContextConstraints{
					ObjectMeta: kapi.ObjectMeta{
						Name: "pre-allocated no sup group annotation",
					},
					SELinuxContext: kapi.SELinuxContextStrategyOptions{
						Type: kapi.SELinuxStrategyRunAsAny,
					},
					RunAsUser: kapi.RunAsUserStrategyOptions{
						Type: kapi.RunAsUserStrategyRunAsAny,
					},
					FSGroup: kapi.FSGroupStrategyOptions{
						Type: kapi.FSGroupStrategyMustRunAs,
					},
					SupplementalGroups: kapi.SupplementalGroupsStrategyOptions{
						Type: kapi.SupplementalGroupsStrategyMustRunAs,
					},
				}
			},
			namespace:   namespaceBadSupGroups,
			expectedErr: "unable to find pre-allocated group annotation",
		},
		"bad scc strategy options": {
			scc: func() *kapi.SecurityContextConstraints {
				return &kapi.SecurityContextConstraints{
					ObjectMeta: kapi.ObjectMeta{
						Name: "bad scc user options",
					},
					SELinuxContext: kapi.SELinuxContextStrategyOptions{
						Type: kapi.SELinuxStrategyRunAsAny,
					},
					RunAsUser: kapi.RunAsUserStrategyOptions{
						Type: kapi.RunAsUserStrategyMustRunAs,
					},
					FSGroup: kapi.FSGroupStrategyOptions{
						Type: kapi.FSGroupStrategyRunAsAny,
					},
					SupplementalGroups: kapi.SupplementalGroupsStrategyOptions{
						Type: kapi.SupplementalGroupsStrategyRunAsAny,
					},
				}
			},
			namespace:   namespaceValid,
			expectedErr: "MustRunAs requires a UID",
		},
	}

	for k, v := range testCases {
		store := cache.NewStore(cache.MetaNamespaceKeyFunc)

		// create the admission handler
		tc := testclient.NewSimpleFake(v.namespace)
		admit := &constraint{
			Handler: kadmission.NewHandler(kadmission.Create),
			client:  tc,
			store:   store,
		}

		scc := v.scc()

		// create the providers, this method only needs the namespace
		attributes := kadmission.NewAttributesRecord(nil, "", v.namespace.Name, "", "", "", kadmission.Create, nil)
		_, errs := admit.createProvidersFromConstraints(attributes.GetNamespace(), []*kapi.SecurityContextConstraints{scc})

		if !reflect.DeepEqual(scc, v.scc()) {
			diff := util.ObjectDiff(scc, v.scc())
			t.Errorf("%s createProvidersFromConstraints mutated constraints. diff:\n%s", k, diff)
		}
		if len(v.expectedErr) > 0 && len(errs) != 1 {
			t.Errorf("%s expected a single error '%s' but received %v", k, v.expectedErr, errs)
			continue
		}
		if len(v.expectedErr) == 0 && len(errs) != 0 {
			t.Errorf("%s did not expect an error but received %v", k, errs)
			continue
		}

		// check that we got the error we expected
		if len(v.expectedErr) > 0 {
			if !strings.Contains(errs[0].Error(), v.expectedErr) {
				t.Errorf("%s expected error '%s' but received %v", k, v.expectedErr, errs[0])
			}
		}
	}
}

func TestMatchingSecurityContextConstraints(t *testing.T) {
	sccs := []*kapi.SecurityContextConstraints{
		{
			ObjectMeta: kapi.ObjectMeta{
				Name: "match group",
			},
			Groups: []string{"group"},
		},
		{
			ObjectMeta: kapi.ObjectMeta{
				Name: "match user",
			},
			Users: []string{"user"},
		},
	}
	store := cache.NewStore(cache.MetaNamespaceKeyFunc)
	for _, v := range sccs {
		store.Add(v)
	}

	// single match cases
	testCases := map[string]struct {
		userInfo    user.Info
		expectedSCC string
	}{
		"find none": {
			userInfo: &user.DefaultInfo{
				Name:   "foo",
				Groups: []string{"bar"},
			},
		},
		"find user": {
			userInfo: &user.DefaultInfo{
				Name:   "user",
				Groups: []string{"bar"},
			},
			expectedSCC: "match user",
		},
		"find group": {
			userInfo: &user.DefaultInfo{
				Name:   "foo",
				Groups: []string{"group"},
			},
			expectedSCC: "match group",
		},
	}

	for k, v := range testCases {
		sccs, err := getMatchingSecurityContextConstraints(store, v.userInfo)
		if err != nil {
			t.Errorf("%s received error %v", k, err)
			continue
		}
		if v.expectedSCC == "" {
			if len(sccs) > 0 {
				t.Errorf("%s expected to match 0 sccs but found %d: %#v", k, len(sccs), sccs)
			}
		}
		if v.expectedSCC != "" {
			if len(sccs) != 1 {
				t.Errorf("%s returned more than one scc, use case can not validate: %#v", k, sccs)
				continue
			}
			if v.expectedSCC != sccs[0].Name {
				t.Errorf("%s expected to match %s but found %s", k, v.expectedSCC, sccs[0].Name)
			}
		}
	}

	// check that we can match many at once
	userInfo := &user.DefaultInfo{
		Name:   "user",
		Groups: []string{"group"},
	}
	sccs, err := getMatchingSecurityContextConstraints(store, userInfo)
	if err != nil {
		t.Fatalf("matching many sccs returned error %v", err)
	}
	if len(sccs) != 2 {
		t.Errorf("matching many sccs expected to match 2 sccs but found %d: %#v", len(sccs), sccs)
	}
}

func TestRequiresPreAllocatedUIDRange(t *testing.T) {
	var uid int64 = 1

	testCases := map[string]struct {
		scc      *kapi.SecurityContextConstraints
		requires bool
	}{
		"must run as": {
			scc: &kapi.SecurityContextConstraints{
				RunAsUser: kapi.RunAsUserStrategyOptions{
					Type: kapi.RunAsUserStrategyMustRunAs,
				},
			},
		},
		"run as any": {
			scc: &kapi.SecurityContextConstraints{
				RunAsUser: kapi.RunAsUserStrategyOptions{
					Type: kapi.RunAsUserStrategyRunAsAny,
				},
			},
		},
		"run as non-root": {
			scc: &kapi.SecurityContextConstraints{
				RunAsUser: kapi.RunAsUserStrategyOptions{
					Type: kapi.RunAsUserStrategyMustRunAsNonRoot,
				},
			},
		},
		"run as range": {
			scc: &kapi.SecurityContextConstraints{
				RunAsUser: kapi.RunAsUserStrategyOptions{
					Type: kapi.RunAsUserStrategyMustRunAsRange,
				},
			},
			requires: true,
		},
		"run as range with specified params": {
			scc: &kapi.SecurityContextConstraints{
				RunAsUser: kapi.RunAsUserStrategyOptions{
					Type:        kapi.RunAsUserStrategyMustRunAsRange,
					UIDRangeMin: &uid,
					UIDRangeMax: &uid,
				},
			},
		},
	}

	for k, v := range testCases {
		result := requiresPreAllocatedUIDRange(v.scc)
		if result != v.requires {
			t.Errorf("%s expected result %t but got %t", k, v.requires, result)
		}
	}
}

func TestRequiresPreAllocatedSELinuxLevel(t *testing.T) {
	testCases := map[string]struct {
		scc      *kapi.SecurityContextConstraints
		requires bool
	}{
		"must run as": {
			scc: &kapi.SecurityContextConstraints{
				SELinuxContext: kapi.SELinuxContextStrategyOptions{
					Type: kapi.SELinuxStrategyMustRunAs,
				},
			},
			requires: true,
		},
		"must with level specified": {
			scc: &kapi.SecurityContextConstraints{
				SELinuxContext: kapi.SELinuxContextStrategyOptions{
					Type: kapi.SELinuxStrategyMustRunAs,
					SELinuxOptions: &kapi.SELinuxOptions{
						Level: "foo",
					},
				},
			},
		},
		"run as any": {
			scc: &kapi.SecurityContextConstraints{
				SELinuxContext: kapi.SELinuxContextStrategyOptions{
					Type: kapi.SELinuxStrategyRunAsAny,
				},
			},
		},
	}

	for k, v := range testCases {
		result := requiresPreAllocatedSELinuxLevel(v.scc)
		if result != v.requires {
			t.Errorf("%s expected result %t but got %t", k, v.requires, result)
		}
	}
}

func TestDeduplicateSecurityContextConstraints(t *testing.T) {
	duped := []*kapi.SecurityContextConstraints{
		{ObjectMeta: kapi.ObjectMeta{Name: "a"}},
		{ObjectMeta: kapi.ObjectMeta{Name: "a"}},
		{ObjectMeta: kapi.ObjectMeta{Name: "b"}},
		{ObjectMeta: kapi.ObjectMeta{Name: "b"}},
		{ObjectMeta: kapi.ObjectMeta{Name: "c"}},
		{ObjectMeta: kapi.ObjectMeta{Name: "d"}},
		{ObjectMeta: kapi.ObjectMeta{Name: "e"}},
		{ObjectMeta: kapi.ObjectMeta{Name: "e"}},
	}

	deduped := deduplicateSecurityContextConstraints(duped)

	if len(deduped) != 5 {
		t.Fatalf("expected to have 5 remaining sccs but found %d: %v", len(deduped), deduped)
	}

	constraintCounts := map[string]int{}

	for _, scc := range deduped {
		if _, ok := constraintCounts[scc.Name]; !ok {
			constraintCounts[scc.Name] = 0
		}
		constraintCounts[scc.Name] = constraintCounts[scc.Name] + 1
	}

	for k, v := range constraintCounts {
		if v > 1 {
			t.Errorf("%s was found %d times after de-duping", k, v)
		}
	}

}

func TestRequiresPreallocatedSupplementalGroups(t *testing.T) {
	testCases := map[string]struct {
		scc      *kapi.SecurityContextConstraints
		requires bool
	}{
		"must run as": {
			scc: &kapi.SecurityContextConstraints{
				SupplementalGroups: kapi.SupplementalGroupsStrategyOptions{
					Type: kapi.SupplementalGroupsStrategyMustRunAs,
				},
			},
			requires: true,
		},
		"must with range specified": {
			scc: &kapi.SecurityContextConstraints{
				SupplementalGroups: kapi.SupplementalGroupsStrategyOptions{
					Type: kapi.SupplementalGroupsStrategyMustRunAs,
					Ranges: []kapi.IDRange{
						{Min: 1, Max: 1},
					},
				},
			},
		},
		"run as any": {
			scc: &kapi.SecurityContextConstraints{
				SupplementalGroups: kapi.SupplementalGroupsStrategyOptions{
					Type: kapi.SupplementalGroupsStrategyRunAsAny,
				},
			},
		},
	}
	for k, v := range testCases {
		result := requiresPreallocatedSupplementalGroups(v.scc)
		if result != v.requires {
			t.Errorf("%s expected result %t but got %t", k, v.requires, result)
		}
	}
}

func TestRequiresPreallocatedFSGroup(t *testing.T) {
	testCases := map[string]struct {
		scc      *kapi.SecurityContextConstraints
		requires bool
	}{
		"must run as": {
			scc: &kapi.SecurityContextConstraints{
				FSGroup: kapi.FSGroupStrategyOptions{
					Type: kapi.FSGroupStrategyMustRunAs,
				},
			},
			requires: true,
		},
		"must with range specified": {
			scc: &kapi.SecurityContextConstraints{
				FSGroup: kapi.FSGroupStrategyOptions{
					Type: kapi.FSGroupStrategyMustRunAs,
					Ranges: []kapi.IDRange{
						{Min: 1, Max: 1},
					},
				},
			},
		},
		"run as any": {
			scc: &kapi.SecurityContextConstraints{
				FSGroup: kapi.FSGroupStrategyOptions{
					Type: kapi.FSGroupStrategyRunAsAny,
				},
			},
		},
	}
	for k, v := range testCases {
		result := requiresPreallocatedFSGroup(v.scc)
		if result != v.requires {
			t.Errorf("%s expected result %t but got %t", k, v.requires, result)
		}
	}
}

func TestParseSupplementalGroupAnnotation(t *testing.T) {
	tests := map[string]struct {
		groups     string
		expected   []uid.Block
		shouldFail bool
	}{
		"single block slash": {
			groups: "1/5",
			expected: []uid.Block{
				{Start: 1, End: 5},
			},
		},
		"single block dash": {
			groups: "1-5",
			expected: []uid.Block{
				{Start: 1, End: 5},
			},
		},
		"multiple blocks": {
			groups: "1/5,6/5,11/5",
			expected: []uid.Block{
				{Start: 1, End: 5},
				{Start: 6, End: 10},
				{Start: 11, End: 15},
			},
		},
		"dash format": {
			groups: "1-5,6-10,11-15",
			expected: []uid.Block{
				{Start: 1, End: 5},
				{Start: 6, End: 10},
				{Start: 11, End: 15},
			},
		},
		"no blocks": {
			groups:     "",
			shouldFail: true,
		},
	}
	for k, v := range tests {
		blocks, err := parseSupplementalGroupAnnotation(v.groups)

		if v.shouldFail && err == nil {
			t.Errorf("%s was expected to fail but received no error and blocks %v", k, blocks)
			continue
		}

		if !v.shouldFail && err != nil {
			t.Errorf("%s had an unexpected error %v", k, err)
			continue
		}

		if len(blocks) != len(v.expected) {
			t.Errorf("%s received unexpected number of blocks expected: %v, actual %v", k, v.expected, blocks)
		}

		for _, b := range v.expected {
			if !hasBlock(b, blocks) {
				t.Errorf("%s was missing block %v", k, b)
			}
		}
	}
}

func hasBlock(block uid.Block, blocks []uid.Block) bool {
	for _, b := range blocks {
		if b.Start == block.Start && b.End == block.End {
			return true
		}
	}
	return false
}

func TestGetPreallocatedFSGroup(t *testing.T) {
	ns := func() *kapi.Namespace {
		return &kapi.Namespace{
			ObjectMeta: kapi.ObjectMeta{
				Annotations: map[string]string{},
			},
		}
	}

	fallbackNS := ns()
	fallbackNS.Annotations[allocator.UIDRangeAnnotation] = "1/5"

	emptyAnnotationNS := ns()
	emptyAnnotationNS.Annotations[allocator.SupplementalGroupsAnnotation] = ""

	badBlockNS := ns()
	badBlockNS.Annotations[allocator.SupplementalGroupsAnnotation] = "foo"

	goodNS := ns()
	goodNS.Annotations[allocator.SupplementalGroupsAnnotation] = "1/5"

	tests := map[string]struct {
		ns         *kapi.Namespace
		expected   []kapi.IDRange
		shouldFail bool
	}{
		"fall back to uid if sup group doesn't exist": {
			ns: fallbackNS,
			expected: []kapi.IDRange{
				{Min: 1, Max: 1},
			},
		},
		"no annotation": {
			ns:         ns(),
			shouldFail: true,
		},
		"empty annotation": {
			ns:         emptyAnnotationNS,
			shouldFail: true,
		},
		"bad block": {
			ns:         badBlockNS,
			shouldFail: true,
		},
		"good sup group annotation": {
			ns: goodNS,
			expected: []kapi.IDRange{
				{Min: 1, Max: 1},
			},
		},
	}

	for k, v := range tests {
		ranges, err := getPreallocatedFSGroup(v.ns)
		if v.shouldFail && err == nil {
			t.Errorf("%s was expected to fail but received no error and ranges %v", k, ranges)
			continue
		}

		if !v.shouldFail && err != nil {
			t.Errorf("%s had an unexpected error %v", k, err)
			continue
		}

		if len(ranges) != len(v.expected) {
			t.Errorf("%s received unexpected number of ranges expected: %v, actual %v", k, v.expected, ranges)
		}

		for _, r := range v.expected {
			if !hasRange(r, ranges) {
				t.Errorf("%s was missing range %v", k, r)
			}
		}
	}
}

func TestGetPreallocatedSupplementalGroups(t *testing.T) {
	ns := func() *kapi.Namespace {
		return &kapi.Namespace{
			ObjectMeta: kapi.ObjectMeta{
				Annotations: map[string]string{},
			},
		}
	}

	fallbackNS := ns()
	fallbackNS.Annotations[allocator.UIDRangeAnnotation] = "1/5"

	emptyAnnotationNS := ns()
	emptyAnnotationNS.Annotations[allocator.SupplementalGroupsAnnotation] = ""

	badBlockNS := ns()
	badBlockNS.Annotations[allocator.SupplementalGroupsAnnotation] = "foo"

	goodNS := ns()
	goodNS.Annotations[allocator.SupplementalGroupsAnnotation] = "1/5"

	tests := map[string]struct {
		ns         *kapi.Namespace
		expected   []kapi.IDRange
		shouldFail bool
	}{
		"fall back to uid if sup group doesn't exist": {
			ns: fallbackNS,
			expected: []kapi.IDRange{
				{Min: 1, Max: 5},
			},
		},
		"no annotation": {
			ns:         ns(),
			shouldFail: true,
		},
		"empty annotation": {
			ns:         emptyAnnotationNS,
			shouldFail: true,
		},
		"bad block": {
			ns:         badBlockNS,
			shouldFail: true,
		},
		"good sup group annotation": {
			ns: goodNS,
			expected: []kapi.IDRange{
				{Min: 1, Max: 5},
			},
		},
	}

	for k, v := range tests {
		ranges, err := getPreallocatedSupplementalGroups(v.ns)
		if v.shouldFail && err == nil {
			t.Errorf("%s was expected to fail but received no error and ranges %v", k, ranges)
			continue
		}

		if !v.shouldFail && err != nil {
			t.Errorf("%s had an unexpected error %v", k, err)
			continue
		}

		if len(ranges) != len(v.expected) {
			t.Errorf("%s received unexpected number of ranges expected: %v, actual %v", k, v.expected, ranges)
		}

		for _, r := range v.expected {
			if !hasRange(r, ranges) {
				t.Errorf("%s was missing range %v", k, r)
			}
		}
	}
}

func hasRange(rng kapi.IDRange, ranges []kapi.IDRange) bool {
	for _, r := range ranges {
		if r.Min == rng.Min && r.Max == rng.Max {
			return true
		}
	}
	return false
}

func TestAdmitWithPrioritizedSCC(t *testing.T) {
	// scc with high priority but very restrictive.
	restricted := restrictiveSCC()
	restrictedPriority := 100
	restricted.Priority = &restrictedPriority

	// sccs with matching priorities but one will have a higher point score (by the run as user strategy)
	uidFive := int64(5)
	matchingPrioritySCCOne := laxSCC()
	matchingPrioritySCCOne.Name = "matchingPrioritySCCOne"
	matchingPrioritySCCOne.RunAsUser = kapi.RunAsUserStrategyOptions{
		Type: kapi.RunAsUserStrategyMustRunAs,
		UID:  &uidFive,
	}
	matchingPriority := 5
	matchingPrioritySCCOne.Priority = &matchingPriority

	matchingPrioritySCCTwo := laxSCC()
	matchingPrioritySCCTwo.Name = "matchingPrioritySCCTwo"
	matchingPrioritySCCTwo.RunAsUser = kapi.RunAsUserStrategyOptions{
		Type:        kapi.RunAsUserStrategyMustRunAsRange,
		UIDRangeMin: &uidFive,
		UIDRangeMax: &uidFive,
	}
	matchingPrioritySCCTwo.Priority = &matchingPriority

	// sccs with matching priorities and scores so should be matched by sorted name
	uidSix := int64(6)
	matchingPriorityAndScoreSCCOne := laxSCC()
	matchingPriorityAndScoreSCCOne.Name = "matchingPriorityAndScoreSCCOne"
	matchingPriorityAndScoreSCCOne.RunAsUser = kapi.RunAsUserStrategyOptions{
		Type: kapi.RunAsUserStrategyMustRunAs,
		UID:  &uidSix,
	}
	matchingPriorityAndScorePriority := 1
	matchingPriorityAndScoreSCCOne.Priority = &matchingPriorityAndScorePriority

	matchingPriorityAndScoreSCCTwo := laxSCC()
	matchingPriorityAndScoreSCCTwo.Name = "matchingPriorityAndScoreSCCTwo"
	matchingPriorityAndScoreSCCTwo.RunAsUser = kapi.RunAsUserStrategyOptions{
		Type: kapi.RunAsUserStrategyMustRunAs,
		UID:  &uidSix,
	}
	matchingPriorityAndScoreSCCTwo.Priority = &matchingPriorityAndScorePriority

	// we will expect these to sort as:
	expectedSort := []string{"restrictive", "matchingPrioritySCCOne", "matchingPrioritySCCTwo",
		"matchingPriorityAndScoreSCCOne", "matchingPriorityAndScoreSCCTwo"}
	sccsToSort := []*kapi.SecurityContextConstraints{matchingPriorityAndScoreSCCTwo, matchingPriorityAndScoreSCCOne,
		matchingPrioritySCCTwo, matchingPrioritySCCOne, restricted}
	sort.Sort(ByPriority(sccsToSort))

	for i, scc := range sccsToSort {
		if scc.Name != expectedSort[i] {
			t.Fatalf("unexpected sort found %s at element %d but expected %s", scc.Name, i, expectedSort[i])
		}
	}

	// sorting works as we're expecting
	// now, to test we will craft some requests that are targeted to validate against specific
	// SCCs and ensure that they come out with the right annotation.  This means admission
	// is using the sort strategy we expect.

	namespace := createNamespaceForTest()
	serviceAccount := createSAForTest()
	tc := testclient.NewSimpleFake(namespace, serviceAccount)

	store := cache.NewStore(cache.MetaNamespaceKeyFunc)
	for _, scc := range sccsToSort {
		err := store.Add(scc)
		if err != nil {
			t.Fatalf("error adding sccs to store: %v", err)
		}
	}

	// create the admission plugin
	plugin := NewTestAdmission(store, tc)
	// match the restricted SCC
	testSCCAdmission(goodPod(), plugin, restricted.Name, t)
	// match matchingPrioritySCCOne by setting RunAsUser to 5
	matchingPrioritySCCOnePod := goodPod()
	matchingPrioritySCCOnePod.Spec.Containers[0].SecurityContext.RunAsUser = &uidFive
	testSCCAdmission(matchingPrioritySCCOnePod, plugin, matchingPrioritySCCOne.Name, t)
	// match matchingPriorityAndScoreSCCOne by setting RunAsUser to 6
	matchingPriorityAndScoreSCCOnePod := goodPod()
	matchingPriorityAndScoreSCCOnePod.Spec.Containers[0].SecurityContext.RunAsUser = &uidSix
	testSCCAdmission(matchingPriorityAndScoreSCCOnePod, plugin, matchingPriorityAndScoreSCCOne.Name, t)
}

// testSCCAdmission is a helper to admit the pod and ensure it was validated against the expected
// SCC.
func testSCCAdmission(pod *kapi.Pod, plugin kadmission.Interface, expectedSCC string, t *testing.T) {
	attrs := kadmission.NewAttributesRecord(pod, "Pod", "namespace", "", string(kapi.ResourcePods), "", kadmission.Create, &user.DefaultInfo{})
	err := plugin.Admit(attrs)
	if err != nil {
		t.Errorf("error admitting pod: %v", err)
		return
	}

	validatedSCC, ok := pod.Annotations[allocator.ValidatedSCCAnnotation]
	if !ok {
		t.Errorf("expected to find the validated annotation on the pod for the scc but found none")
		return
	}
	if validatedSCC != expectedSCC {
		t.Errorf("should have validated against %s but found %s", expectedSCC, validatedSCC)
	}
}

func laxSCC() *kapi.SecurityContextConstraints {
	return &kapi.SecurityContextConstraints{
		ObjectMeta: kapi.ObjectMeta{
			Name: "lax",
		},
		RunAsUser: kapi.RunAsUserStrategyOptions{
			Type: kapi.RunAsUserStrategyRunAsAny,
		},
		SELinuxContext: kapi.SELinuxContextStrategyOptions{
			Type: kapi.SELinuxStrategyRunAsAny,
		},
		FSGroup: kapi.FSGroupStrategyOptions{
			Type: kapi.FSGroupStrategyRunAsAny,
		},
		SupplementalGroups: kapi.SupplementalGroupsStrategyOptions{
			Type: kapi.SupplementalGroupsStrategyRunAsAny,
		},
		Groups: []string{"system:serviceaccounts"},
	}
}

func restrictiveSCC() *kapi.SecurityContextConstraints {
	var exactUID int64 = 999
	return &kapi.SecurityContextConstraints{
		ObjectMeta: kapi.ObjectMeta{
			Name: "restrictive",
		},
		RunAsUser: kapi.RunAsUserStrategyOptions{
			Type: kapi.RunAsUserStrategyMustRunAs,
			UID:  &exactUID,
		},
		SELinuxContext: kapi.SELinuxContextStrategyOptions{
			Type: kapi.SELinuxStrategyMustRunAs,
			SELinuxOptions: &kapi.SELinuxOptions{
				Level: "s9:z0,z1",
			},
		},
		FSGroup: kapi.FSGroupStrategyOptions{
			Type: kapi.FSGroupStrategyMustRunAs,
			Ranges: []kapi.IDRange{
				{Min: 999, Max: 999},
			},
		},
		SupplementalGroups: kapi.SupplementalGroupsStrategyOptions{
			Type: kapi.SupplementalGroupsStrategyMustRunAs,
			Ranges: []kapi.IDRange{
				{Min: 999, Max: 999},
			},
		},
		Groups: []string{"system:serviceaccounts"},
	}
}

func createNamespaceForTest() *kapi.Namespace {
	return &kapi.Namespace{
		ObjectMeta: kapi.ObjectMeta{
			Name: "default",
			Annotations: map[string]string{
				allocator.UIDRangeAnnotation:           "1/3",
				allocator.MCSAnnotation:                "s0:c1,c0",
				allocator.SupplementalGroupsAnnotation: "2/3",
			},
		},
	}
}

func createSAForTest() *kapi.ServiceAccount {
	return &kapi.ServiceAccount{
		ObjectMeta: kapi.ObjectMeta{
			Name: "default",
		},
	}
}

// goodPod is empty and should not be used directly for testing since we're providing
// two different SCCs.  Since no values are specified it would be allowed to match any
// SCC when defaults are filled in.
func goodPod() *kapi.Pod {
	return &kapi.Pod{
		Spec: kapi.PodSpec{
			ServiceAccountName: "default",
			SecurityContext:    &kapi.PodSecurityContext{},
			Containers: []kapi.Container{
				{
					SecurityContext: &kapi.SecurityContext{},
				},
			},
		},
	}
}
