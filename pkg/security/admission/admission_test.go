package admission

import (
	"reflect"
	"sort"
	"strings"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/diff"
	kadmission "k8s.io/apiserver/pkg/admission"
	"k8s.io/apiserver/pkg/authentication/user"
	"k8s.io/client-go/tools/cache"
	kapi "k8s.io/kubernetes/pkg/api"
	v1kapi "k8s.io/kubernetes/pkg/api/v1"
	clientset "k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset"
	clientsetfake "k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset/fake"

	allocator "github.com/openshift/origin/pkg/security"
	admissiontesting "github.com/openshift/origin/pkg/security/admission/testing"
	securityapi "github.com/openshift/origin/pkg/security/apis/security"
	securitylisters "github.com/openshift/origin/pkg/security/generated/listers/security/internalversion"
	oscc "github.com/openshift/origin/pkg/security/securitycontextconstraints"
)

func NewTestAdmission(lister securitylisters.SecurityContextConstraintsLister, kclient clientset.Interface) kadmission.Interface {
	return &constraint{
		Handler:   kadmission.NewHandler(kadmission.Create),
		client:    kclient,
		sccLister: lister,
	}
}

func TestFailClosedOnInvalidPod(t *testing.T) {
	plugin := NewTestAdmission(nil, nil)
	pod := &v1kapi.Pod{}
	attrs := kadmission.NewAttributesRecord(pod, nil, kapi.Kind("Pod").WithVersion("version"), pod.Namespace, pod.Name, kapi.Resource("pods").WithVersion("version"), "", kadmission.Create, &user.DefaultInfo{})
	err := plugin.Admit(attrs)

	if err == nil {
		t.Fatalf("expected versioned pod object to fail admission")
	}
	if !strings.Contains(err.Error(), "object was marked as kind pod but was unable to be converted") {
		t.Errorf("expected error to be conversion erorr but got: %v", err)
	}
}

func TestAdmitCaps(t *testing.T) {
	createPodWithCaps := func(caps *kapi.Capabilities) *kapi.Pod {
		pod := goodPod()
		pod.Spec.Containers[0].SecurityContext.Capabilities = caps
		return pod
	}

	restricted := restrictiveSCC()

	allowsFooInAllowed := restrictiveSCC()
	allowsFooInAllowed.Name = "allowCapInAllowed"
	allowsFooInAllowed.AllowedCapabilities = []kapi.Capability{"foo"}

	allowsFooInRequired := restrictiveSCC()
	allowsFooInRequired.Name = "allowCapInRequired"
	allowsFooInRequired.DefaultAddCapabilities = []kapi.Capability{"foo"}

	requiresFooToBeDropped := restrictiveSCC()
	requiresFooToBeDropped.Name = "requireDrop"
	requiresFooToBeDropped.RequiredDropCapabilities = []kapi.Capability{"foo"}

	allowAllInAllowed := restrictiveSCC()
	allowAllInAllowed.Name = "allowAllCapsInAllowed"
	allowAllInAllowed.AllowedCapabilities = []kapi.Capability{securityapi.AllowAllCapabilities}

	tc := map[string]struct {
		pod                  *kapi.Pod
		sccs                 []*securityapi.SecurityContextConstraints
		shouldPass           bool
		expectedCapabilities *kapi.Capabilities
	}{
		// UC 1: if an SCC does not define allowed or required caps then a pod requesting a cap
		// should be rejected.
		"should reject cap add when not allowed or required": {
			pod:        createPodWithCaps(&kapi.Capabilities{Add: []kapi.Capability{"foo"}}),
			sccs:       []*securityapi.SecurityContextConstraints{restricted},
			shouldPass: false,
		},
		// UC 2: if an SCC allows a cap in the allowed field it should accept the pod request
		// to add the cap.
		"should accept cap add when in allowed": {
			pod:        createPodWithCaps(&kapi.Capabilities{Add: []kapi.Capability{"foo"}}),
			sccs:       []*securityapi.SecurityContextConstraints{restricted, allowsFooInAllowed},
			shouldPass: true,
		},
		// UC 3: if an SCC requires a cap then it should accept the pod request
		// to add the cap.
		"should accept cap add when in required": {
			pod:        createPodWithCaps(&kapi.Capabilities{Add: []kapi.Capability{"foo"}}),
			sccs:       []*securityapi.SecurityContextConstraints{restricted, allowsFooInRequired},
			shouldPass: true,
		},
		// UC 4: if an SCC requires a cap to be dropped then it should fail both
		// in the verification of adds and verification of drops
		"should reject cap add when requested cap is required to be dropped": {
			pod:        createPodWithCaps(&kapi.Capabilities{Add: []kapi.Capability{"foo"}}),
			sccs:       []*securityapi.SecurityContextConstraints{restricted, requiresFooToBeDropped},
			shouldPass: false,
		},
		// UC 5: if an SCC requires a cap to be dropped it should accept
		// a manual request to drop the cap.
		"should accept cap drop when cap is required to be dropped": {
			pod:        createPodWithCaps(&kapi.Capabilities{Drop: []kapi.Capability{"foo"}}),
			sccs:       []*securityapi.SecurityContextConstraints{restricted, requiresFooToBeDropped},
			shouldPass: true,
		},
		// UC 6: required add is defaulted
		"required add is defaulted": {
			pod:        goodPod(),
			sccs:       []*securityapi.SecurityContextConstraints{allowsFooInRequired},
			shouldPass: true,
			expectedCapabilities: &kapi.Capabilities{
				Add: []kapi.Capability{"foo"},
			},
		},
		// UC 7: required drop is defaulted
		"required drop is defaulted": {
			pod:        goodPod(),
			sccs:       []*securityapi.SecurityContextConstraints{requiresFooToBeDropped},
			shouldPass: true,
			expectedCapabilities: &kapi.Capabilities{
				Drop: []kapi.Capability{"foo"},
			},
		},
		// UC 8: using '*' in allowed caps
		"should accept cap add when all caps are allowed": {
			pod:        createPodWithCaps(&kapi.Capabilities{Add: []kapi.Capability{"foo"}}),
			sccs:       []*securityapi.SecurityContextConstraints{restricted, allowAllInAllowed},
			shouldPass: true,
		},
	}

	for i := 0; i < 2; i++ {
		for k, v := range tc {
			v.pod.Spec.Containers, v.pod.Spec.InitContainers = v.pod.Spec.InitContainers, v.pod.Spec.Containers
			containers := v.pod.Spec.Containers
			if i == 0 {
				containers = v.pod.Spec.InitContainers
			}

			testSCCAdmit(k, v.sccs, v.pod, v.shouldPass, t)

			if v.expectedCapabilities != nil {
				if !reflect.DeepEqual(v.expectedCapabilities, containers[0].SecurityContext.Capabilities) {
					t.Errorf("%s resulted in caps that were not expected - expected: %v, received: %v", k, v.expectedCapabilities, containers[0].SecurityContext.Capabilities)
				}
			}
		}
	}
}

func testSCCAdmit(testCaseName string, sccs []*securityapi.SecurityContextConstraints, pod *kapi.Pod, shouldPass bool, t *testing.T) {
	namespace := admissiontesting.CreateNamespaceForTest()
	serviceAccount := admissiontesting.CreateSAForTest()
	tc := clientsetfake.NewSimpleClientset(namespace, serviceAccount)
	indexer := cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc})
	cache := securitylisters.NewSecurityContextConstraintsLister(indexer)
	for _, scc := range sccs {
		indexer.Add(scc)
	}

	plugin := NewTestAdmission(cache, tc)

	attrs := kadmission.NewAttributesRecord(pod, nil, kapi.Kind("Pod").WithVersion("version"), pod.Namespace, pod.Name, kapi.Resource("pods").WithVersion("version"), "", kadmission.Create, &user.DefaultInfo{})
	err := plugin.Admit(attrs)

	if shouldPass && err != nil {
		t.Errorf("%s expected no errors but received %v", testCaseName, err)
	}
	if !shouldPass && err == nil {
		t.Errorf("%s expected errors but received none", testCaseName)
	}
}

func TestAdmit(t *testing.T) {
	// create the annotated namespace and add it to the fake client
	namespace := admissiontesting.CreateNamespaceForTest()
	serviceAccount := admissiontesting.CreateSAForTest()

	// used for cases where things are preallocated
	defaultGroup := int64(2)

	tc := clientsetfake.NewSimpleClientset(namespace, serviceAccount)

	// create scc that requires allocation retrieval
	saSCC := &securityapi.SecurityContextConstraints{
		ObjectMeta: metav1.ObjectMeta{
			Name: "scc-sa",
		},
		RunAsUser: securityapi.RunAsUserStrategyOptions{
			Type: securityapi.RunAsUserStrategyMustRunAsRange,
		},
		SELinuxContext: securityapi.SELinuxContextStrategyOptions{
			Type: securityapi.SELinuxStrategyMustRunAs,
		},
		FSGroup: securityapi.FSGroupStrategyOptions{
			Type: securityapi.FSGroupStrategyMustRunAs,
		},
		SupplementalGroups: securityapi.SupplementalGroupsStrategyOptions{
			Type: securityapi.SupplementalGroupsStrategyMustRunAs,
		},
		Groups: []string{"system:serviceaccounts"},
	}
	// create scc that has specific requirements that shouldn't match but is permissioned to
	// service accounts to test that even though this has matching priorities (0) and a
	// lower point value score (which will cause it to be sorted in front of scc-sa) it should not
	// validate the requests so we should try scc-sa.
	var exactUID int64 = 999
	saExactSCC := &securityapi.SecurityContextConstraints{
		ObjectMeta: metav1.ObjectMeta{
			Name: "scc-sa-exact",
		},
		RunAsUser: securityapi.RunAsUserStrategyOptions{
			Type: securityapi.RunAsUserStrategyMustRunAs,
			UID:  &exactUID,
		},
		SELinuxContext: securityapi.SELinuxContextStrategyOptions{
			Type: securityapi.SELinuxStrategyMustRunAs,
			SELinuxOptions: &kapi.SELinuxOptions{
				Level: "s9:z0,z1",
			},
		},
		FSGroup: securityapi.FSGroupStrategyOptions{
			Type: securityapi.FSGroupStrategyMustRunAs,
			Ranges: []securityapi.IDRange{
				{Min: 999, Max: 999},
			},
		},
		SupplementalGroups: securityapi.SupplementalGroupsStrategyOptions{
			Type: securityapi.SupplementalGroupsStrategyMustRunAs,
			Ranges: []securityapi.IDRange{
				{Min: 999, Max: 999},
			},
		},
		Groups: []string{"system:serviceaccounts"},
	}

	indexer := cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc})
	cache := securitylisters.NewSecurityContextConstraintsLister(indexer)

	indexer.Add(saExactSCC)
	indexer.Add(saSCC)

	// create the admission plugin
	p := NewTestAdmission(cache, tc)

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

	for i := 0; i < 2; i++ {
		for k, v := range testCases {
			v.pod.Spec.Containers, v.pod.Spec.InitContainers = v.pod.Spec.InitContainers, v.pod.Spec.Containers
			containers := v.pod.Spec.Containers
			if i == 0 {
				containers = v.pod.Spec.InitContainers
			}
			attrs := kadmission.NewAttributesRecord(v.pod, nil, kapi.Kind("Pod").WithVersion("version"), v.pod.Namespace, v.pod.Name, kapi.Resource("pods").WithVersion("version"), "", kadmission.Create, &user.DefaultInfo{})
			err := p.Admit(attrs)

			if v.shouldAdmit && err != nil {
				t.Fatalf("%s expected no errors but received %v", k, err)
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
				if *containers[0].SecurityContext.RunAsUser != v.expectedUID {
					t.Errorf("%s expected UID %d but found %d", k, v.expectedUID, *containers[0].SecurityContext.RunAsUser)
				}
				if containers[0].SecurityContext.SELinuxOptions.Level != v.expectedLevel {
					t.Errorf("%s expected Level %s but found %s", k, v.expectedLevel, containers[0].SecurityContext.SELinuxOptions.Level)
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
	}

	// now add an escalated scc to the group and re-run the cases that expected failure, they should
	// now pass by validating against the escalated scc.
	adminSCC := &securityapi.SecurityContextConstraints{
		ObjectMeta: metav1.ObjectMeta{
			Name: "scc-admin",
		},
		AllowPrivilegedContainer: true,
		AllowHostNetwork:         true,
		AllowHostPorts:           true,
		AllowHostPID:             true,
		AllowHostIPC:             true,
		RunAsUser: securityapi.RunAsUserStrategyOptions{
			Type: securityapi.RunAsUserStrategyRunAsAny,
		},
		SELinuxContext: securityapi.SELinuxContextStrategyOptions{
			Type: securityapi.SELinuxStrategyRunAsAny,
		},
		FSGroup: securityapi.FSGroupStrategyOptions{
			Type: securityapi.FSGroupStrategyRunAsAny,
		},
		SupplementalGroups: securityapi.SupplementalGroupsStrategyOptions{
			Type: securityapi.SupplementalGroupsStrategyRunAsAny,
		},
		Groups: []string{"system:serviceaccounts"},
	}

	indexer.Add(adminSCC)

	for i := 0; i < 2; i++ {
		for k, v := range testCases {
			v.pod.Spec.Containers, v.pod.Spec.InitContainers = v.pod.Spec.InitContainers, v.pod.Spec.Containers

			if !v.shouldAdmit {
				attrs := kadmission.NewAttributesRecord(v.pod, nil, kapi.Kind("Pod").WithVersion("version"), v.pod.Namespace, v.pod.Name, kapi.Resource("pods").WithVersion("version"), "", kadmission.Create, &user.DefaultInfo{})
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
}

func hasSupGroup(group int64, groups []int64) bool {
	for _, g := range groups {
		if g == group {
			return true
		}
	}
	return false
}

func TestCreateProvidersFromConstraints(t *testing.T) {
	namespaceValid := &kapi.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: "default",
			Annotations: map[string]string{
				allocator.UIDRangeAnnotation:           "1/3",
				allocator.MCSAnnotation:                "s0:c1,c0",
				allocator.SupplementalGroupsAnnotation: "1/3",
			},
		},
	}
	namespaceNoUID := &kapi.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: "default",
			Annotations: map[string]string{
				allocator.MCSAnnotation:                "s0:c1,c0",
				allocator.SupplementalGroupsAnnotation: "1/3",
			},
		},
	}
	namespaceNoMCS := &kapi.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: "default",
			Annotations: map[string]string{
				allocator.UIDRangeAnnotation:           "1/3",
				allocator.SupplementalGroupsAnnotation: "1/3",
			},
		},
	}

	namespaceNoSupplementalGroupsFallbackToUID := &kapi.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: "default",
			Annotations: map[string]string{
				allocator.UIDRangeAnnotation: "1/3",
				allocator.MCSAnnotation:      "s0:c1,c0",
			},
		},
	}

	namespaceBadSupGroups := &kapi.Namespace{
		ObjectMeta: metav1.ObjectMeta{
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
		scc         func() *securityapi.SecurityContextConstraints
		namespace   *kapi.Namespace
		expectedErr string
	}{
		"valid non-preallocated scc": {
			scc: func() *securityapi.SecurityContextConstraints {
				return &securityapi.SecurityContextConstraints{
					ObjectMeta: metav1.ObjectMeta{
						Name: "valid non-preallocated scc",
					},
					SELinuxContext: securityapi.SELinuxContextStrategyOptions{
						Type: securityapi.SELinuxStrategyRunAsAny,
					},
					RunAsUser: securityapi.RunAsUserStrategyOptions{
						Type: securityapi.RunAsUserStrategyRunAsAny,
					},
					FSGroup: securityapi.FSGroupStrategyOptions{
						Type: securityapi.FSGroupStrategyRunAsAny,
					},
					SupplementalGroups: securityapi.SupplementalGroupsStrategyOptions{
						Type: securityapi.SupplementalGroupsStrategyRunAsAny,
					},
				}
			},
			namespace: namespaceValid,
		},
		"valid pre-allocated scc": {
			scc: func() *securityapi.SecurityContextConstraints {
				return &securityapi.SecurityContextConstraints{
					ObjectMeta: metav1.ObjectMeta{
						Name: "valid pre-allocated scc",
					},
					SELinuxContext: securityapi.SELinuxContextStrategyOptions{
						Type:           securityapi.SELinuxStrategyMustRunAs,
						SELinuxOptions: &kapi.SELinuxOptions{User: "myuser"},
					},
					RunAsUser: securityapi.RunAsUserStrategyOptions{
						Type: securityapi.RunAsUserStrategyMustRunAsRange,
					},
					FSGroup: securityapi.FSGroupStrategyOptions{
						Type: securityapi.FSGroupStrategyMustRunAs,
					},
					SupplementalGroups: securityapi.SupplementalGroupsStrategyOptions{
						Type: securityapi.SupplementalGroupsStrategyMustRunAs,
					},
				}
			},
			namespace: namespaceValid,
		},
		"pre-allocated no uid annotation": {
			scc: func() *securityapi.SecurityContextConstraints {
				return &securityapi.SecurityContextConstraints{
					ObjectMeta: metav1.ObjectMeta{
						Name: "pre-allocated no uid annotation",
					},
					SELinuxContext: securityapi.SELinuxContextStrategyOptions{
						Type: securityapi.SELinuxStrategyMustRunAs,
					},
					RunAsUser: securityapi.RunAsUserStrategyOptions{
						Type: securityapi.RunAsUserStrategyMustRunAsRange,
					},
					FSGroup: securityapi.FSGroupStrategyOptions{
						Type: securityapi.FSGroupStrategyRunAsAny,
					},
					SupplementalGroups: securityapi.SupplementalGroupsStrategyOptions{
						Type: securityapi.SupplementalGroupsStrategyRunAsAny,
					},
				}
			},
			namespace:   namespaceNoUID,
			expectedErr: "unable to find pre-allocated uid annotation",
		},
		"pre-allocated no mcs annotation": {
			scc: func() *securityapi.SecurityContextConstraints {
				return &securityapi.SecurityContextConstraints{
					ObjectMeta: metav1.ObjectMeta{
						Name: "pre-allocated no mcs annotation",
					},
					SELinuxContext: securityapi.SELinuxContextStrategyOptions{
						Type: securityapi.SELinuxStrategyMustRunAs,
					},
					RunAsUser: securityapi.RunAsUserStrategyOptions{
						Type: securityapi.RunAsUserStrategyMustRunAsRange,
					},
					FSGroup: securityapi.FSGroupStrategyOptions{
						Type: securityapi.FSGroupStrategyRunAsAny,
					},
					SupplementalGroups: securityapi.SupplementalGroupsStrategyOptions{
						Type: securityapi.SupplementalGroupsStrategyRunAsAny,
					},
				}
			},
			namespace:   namespaceNoMCS,
			expectedErr: "unable to find pre-allocated mcs annotation",
		},
		"pre-allocated group falls back to UID annotation": {
			scc: func() *securityapi.SecurityContextConstraints {
				return &securityapi.SecurityContextConstraints{
					ObjectMeta: metav1.ObjectMeta{
						Name: "pre-allocated no sup group annotation",
					},
					SELinuxContext: securityapi.SELinuxContextStrategyOptions{
						Type: securityapi.SELinuxStrategyRunAsAny,
					},
					RunAsUser: securityapi.RunAsUserStrategyOptions{
						Type: securityapi.RunAsUserStrategyRunAsAny,
					},
					FSGroup: securityapi.FSGroupStrategyOptions{
						Type: securityapi.FSGroupStrategyMustRunAs,
					},
					SupplementalGroups: securityapi.SupplementalGroupsStrategyOptions{
						Type: securityapi.SupplementalGroupsStrategyMustRunAs,
					},
				}
			},
			namespace: namespaceNoSupplementalGroupsFallbackToUID,
		},
		"pre-allocated group bad value fails": {
			scc: func() *securityapi.SecurityContextConstraints {
				return &securityapi.SecurityContextConstraints{
					ObjectMeta: metav1.ObjectMeta{
						Name: "pre-allocated no sup group annotation",
					},
					SELinuxContext: securityapi.SELinuxContextStrategyOptions{
						Type: securityapi.SELinuxStrategyRunAsAny,
					},
					RunAsUser: securityapi.RunAsUserStrategyOptions{
						Type: securityapi.RunAsUserStrategyRunAsAny,
					},
					FSGroup: securityapi.FSGroupStrategyOptions{
						Type: securityapi.FSGroupStrategyMustRunAs,
					},
					SupplementalGroups: securityapi.SupplementalGroupsStrategyOptions{
						Type: securityapi.SupplementalGroupsStrategyMustRunAs,
					},
				}
			},
			namespace:   namespaceBadSupGroups,
			expectedErr: "unable to find pre-allocated group annotation",
		},
		"bad scc strategy options": {
			scc: func() *securityapi.SecurityContextConstraints {
				return &securityapi.SecurityContextConstraints{
					ObjectMeta: metav1.ObjectMeta{
						Name: "bad scc user options",
					},
					SELinuxContext: securityapi.SELinuxContextStrategyOptions{
						Type: securityapi.SELinuxStrategyRunAsAny,
					},
					RunAsUser: securityapi.RunAsUserStrategyOptions{
						Type: securityapi.RunAsUserStrategyMustRunAs,
					},
					FSGroup: securityapi.FSGroupStrategyOptions{
						Type: securityapi.FSGroupStrategyRunAsAny,
					},
					SupplementalGroups: securityapi.SupplementalGroupsStrategyOptions{
						Type: securityapi.SupplementalGroupsStrategyRunAsAny,
					},
				}
			},
			namespace:   namespaceValid,
			expectedErr: "MustRunAs requires a UID",
		},
	}

	for k, v := range testCases {
		// create the admission handler
		tc := clientsetfake.NewSimpleClientset(v.namespace)
		scc := v.scc()

		// create the providers, this method only needs the namespace
		attributes := kadmission.NewAttributesRecord(nil, nil, kapi.Kind("Pod").WithVersion("version"), v.namespace.Name, "", kapi.Resource("pods").WithVersion("version"), "", kadmission.Create, nil)
		_, errs := oscc.CreateProvidersFromConstraints(attributes.GetNamespace(), []*securityapi.SecurityContextConstraints{scc}, tc)

		if !reflect.DeepEqual(scc, v.scc()) {
			diff := diff.ObjectDiff(scc, v.scc())
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
	sccs := []*securityapi.SecurityContextConstraints{
		{
			ObjectMeta: metav1.ObjectMeta{
				Name: "match group",
			},
			Groups: []string{"group"},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name: "match user",
			},
			Users: []string{"user"},
		},
	}

	indexer := cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc})
	cache := securitylisters.NewSecurityContextConstraintsLister(indexer)
	for _, scc := range sccs {
		indexer.Add(scc)
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
		sccMatcher := oscc.NewDefaultSCCMatcher(cache)
		sccs, err := sccMatcher.FindApplicableSCCs(v.userInfo)
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
	sccMatcher := oscc.NewDefaultSCCMatcher(cache)
	sccs, err := sccMatcher.FindApplicableSCCs(userInfo)
	if err != nil {
		t.Fatalf("matching many sccs returned error %v", err)
	}
	if len(sccs) != 2 {
		t.Errorf("matching many sccs expected to match 2 sccs but found %d: %#v", len(sccs), sccs)
	}
}

func TestAdmitWithPrioritizedSCC(t *testing.T) {
	// scc with high priority but very restrictive.
	restricted := restrictiveSCC()
	restrictedPriority := int32(100)
	restricted.Priority = &restrictedPriority

	// sccs with matching priorities but one will have a higher point score (by the run as user strategy)
	uidFive := int64(5)
	matchingPrioritySCCOne := laxSCC()
	matchingPrioritySCCOne.Name = "matchingPrioritySCCOne"
	matchingPrioritySCCOne.RunAsUser = securityapi.RunAsUserStrategyOptions{
		Type: securityapi.RunAsUserStrategyMustRunAs,
		UID:  &uidFive,
	}
	matchingPriority := int32(5)
	matchingPrioritySCCOne.Priority = &matchingPriority

	matchingPrioritySCCTwo := laxSCC()
	matchingPrioritySCCTwo.Name = "matchingPrioritySCCTwo"
	matchingPrioritySCCTwo.RunAsUser = securityapi.RunAsUserStrategyOptions{
		Type:        securityapi.RunAsUserStrategyMustRunAsRange,
		UIDRangeMin: &uidFive,
		UIDRangeMax: &uidFive,
	}
	matchingPrioritySCCTwo.Priority = &matchingPriority

	// sccs with matching priorities and scores so should be matched by sorted name
	uidSix := int64(6)
	matchingPriorityAndScoreSCCOne := laxSCC()
	matchingPriorityAndScoreSCCOne.Name = "matchingPriorityAndScoreSCCOne"
	matchingPriorityAndScoreSCCOne.RunAsUser = securityapi.RunAsUserStrategyOptions{
		Type: securityapi.RunAsUserStrategyMustRunAs,
		UID:  &uidSix,
	}
	matchingPriorityAndScorePriority := int32(1)
	matchingPriorityAndScoreSCCOne.Priority = &matchingPriorityAndScorePriority

	matchingPriorityAndScoreSCCTwo := laxSCC()
	matchingPriorityAndScoreSCCTwo.Name = "matchingPriorityAndScoreSCCTwo"
	matchingPriorityAndScoreSCCTwo.RunAsUser = securityapi.RunAsUserStrategyOptions{
		Type: securityapi.RunAsUserStrategyMustRunAs,
		UID:  &uidSix,
	}
	matchingPriorityAndScoreSCCTwo.Priority = &matchingPriorityAndScorePriority

	// we will expect these to sort as:
	expectedSort := []string{"restrictive", "matchingPrioritySCCOne", "matchingPrioritySCCTwo",
		"matchingPriorityAndScoreSCCOne", "matchingPriorityAndScoreSCCTwo"}
	sccsToSort := []*securityapi.SecurityContextConstraints{matchingPriorityAndScoreSCCTwo, matchingPriorityAndScoreSCCOne,
		matchingPrioritySCCTwo, matchingPrioritySCCOne, restricted}
	sort.Sort(oscc.ByPriority(sccsToSort))

	for i, scc := range sccsToSort {
		if scc.Name != expectedSort[i] {
			t.Fatalf("unexpected sort found %s at element %d but expected %s", scc.Name, i, expectedSort[i])
		}
	}

	// sorting works as we're expecting
	// now, to test we will craft some requests that are targeted to validate against specific
	// SCCs and ensure that they come out with the right annotation.  This means admission
	// is using the sort strategy we expect.

	namespace := admissiontesting.CreateNamespaceForTest()
	serviceAccount := admissiontesting.CreateSAForTest()
	serviceAccount.Namespace = namespace.Name
	tc := clientsetfake.NewSimpleClientset(namespace, serviceAccount)

	indexer := cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc})
	cache := securitylisters.NewSecurityContextConstraintsLister(indexer)

	for _, scc := range sccsToSort {
		err := indexer.Add(scc)
		if err != nil {
			t.Fatalf("error adding sccs to store: %v", err)
		}
	}

	// create the admission plugin
	plugin := NewTestAdmission(cache, tc)
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

func TestAdmitSeccomp(t *testing.T) {
	createPodWithSeccomp := func(podAnnotation, containerAnnotation string) *kapi.Pod {
		pod := goodPod()
		pod.Annotations = map[string]string{}
		if podAnnotation != "" {
			pod.Annotations[kapi.SeccompPodAnnotationKey] = podAnnotation
		}
		if containerAnnotation != "" {
			pod.Annotations[kapi.SeccompContainerAnnotationKeyPrefix+"container"] = containerAnnotation
		}
		pod.Spec.Containers[0].Name = "container"
		return pod
	}

	noSeccompSCC := restrictiveSCC()
	noSeccompSCC.Name = "noseccomp"

	seccompSCC := restrictiveSCC()
	seccompSCC.Name = "seccomp"
	seccompSCC.SeccompProfiles = []string{"foo"}

	wildcardSCC := restrictiveSCC()
	wildcardSCC.Name = "wildcard"
	wildcardSCC.SeccompProfiles = []string{"*"}

	tests := map[string]struct {
		pod                   *kapi.Pod
		sccs                  []*securityapi.SecurityContextConstraints
		shouldPass            bool
		expectedPodAnnotation string
		expectedSCC           string
	}{
		"no seccomp, no requests": {
			pod:         goodPod(),
			sccs:        []*securityapi.SecurityContextConstraints{noSeccompSCC},
			shouldPass:  true,
			expectedSCC: noSeccompSCC.Name,
		},
		"no seccomp, bad container requests": {
			pod:        createPodWithSeccomp("foo", "bar"),
			sccs:       []*securityapi.SecurityContextConstraints{noSeccompSCC},
			shouldPass: false,
		},
		"seccomp, no requests": {
			pod:                   goodPod(),
			sccs:                  []*securityapi.SecurityContextConstraints{seccompSCC},
			shouldPass:            true,
			expectedPodAnnotation: "foo",
			expectedSCC:           seccompSCC.Name,
		},
		"seccomp, valid pod annotation, no container annotation": {
			pod:                   createPodWithSeccomp("foo", ""),
			sccs:                  []*securityapi.SecurityContextConstraints{seccompSCC},
			shouldPass:            true,
			expectedPodAnnotation: "foo",
			expectedSCC:           seccompSCC.Name,
		},
		"seccomp, no pod annotation, valid container annotation": {
			pod:                   createPodWithSeccomp("", "foo"),
			sccs:                  []*securityapi.SecurityContextConstraints{seccompSCC},
			shouldPass:            true,
			expectedPodAnnotation: "foo",
			expectedSCC:           seccompSCC.Name,
		},
		"seccomp, valid pod annotation, invalid container annotation": {
			pod:        createPodWithSeccomp("foo", "bar"),
			sccs:       []*securityapi.SecurityContextConstraints{seccompSCC},
			shouldPass: false,
		},
		"wild card, no requests": {
			pod:         goodPod(),
			sccs:        []*securityapi.SecurityContextConstraints{wildcardSCC},
			shouldPass:  true,
			expectedSCC: wildcardSCC.Name,
		},
		"wild card, requests": {
			pod:                   createPodWithSeccomp("foo", "bar"),
			sccs:                  []*securityapi.SecurityContextConstraints{wildcardSCC},
			shouldPass:            true,
			expectedPodAnnotation: "foo",
			expectedSCC:           wildcardSCC.Name,
		},
	}

	for k, v := range tests {
		testSCCAdmit(k, v.sccs, v.pod, v.shouldPass, t)

		if v.shouldPass {
			validatedSCC, ok := v.pod.Annotations[allocator.ValidatedSCCAnnotation]
			if !ok {
				t.Errorf("expected to find the validated annotation on the pod for the scc but found none")
				return
			}
			if validatedSCC != v.expectedSCC {
				t.Errorf("should have validated against %s but found %s", v.expectedSCC, validatedSCC)
			}

			if len(v.expectedPodAnnotation) > 0 {
				annotation, found := v.pod.Annotations[kapi.SeccompPodAnnotationKey]
				if !found {
					t.Errorf("%s expected to have pod annotation for seccomp but found none", k)
				}
				if found && annotation != v.expectedPodAnnotation {
					t.Errorf("%s expected pod annotation to be %s but found %s", k, v.expectedPodAnnotation, annotation)
				}
			}
		}
	}

}

// testSCCAdmission is a helper to admit the pod and ensure it was validated against the expected
// SCC.
func testSCCAdmission(pod *kapi.Pod, plugin kadmission.Interface, expectedSCC string, t *testing.T) {
	attrs := kadmission.NewAttributesRecord(pod, nil, kapi.Kind("Pod").WithVersion("version"), pod.Namespace, pod.Name, kapi.Resource("pods").WithVersion("version"), "", kadmission.Create, &user.DefaultInfo{})
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

func laxSCC() *securityapi.SecurityContextConstraints {
	return &securityapi.SecurityContextConstraints{
		ObjectMeta: metav1.ObjectMeta{
			Name: "lax",
		},
		RunAsUser: securityapi.RunAsUserStrategyOptions{
			Type: securityapi.RunAsUserStrategyRunAsAny,
		},
		SELinuxContext: securityapi.SELinuxContextStrategyOptions{
			Type: securityapi.SELinuxStrategyRunAsAny,
		},
		FSGroup: securityapi.FSGroupStrategyOptions{
			Type: securityapi.FSGroupStrategyRunAsAny,
		},
		SupplementalGroups: securityapi.SupplementalGroupsStrategyOptions{
			Type: securityapi.SupplementalGroupsStrategyRunAsAny,
		},
		Groups: []string{"system:serviceaccounts"},
	}
}

func restrictiveSCC() *securityapi.SecurityContextConstraints {
	var exactUID int64 = 999
	return &securityapi.SecurityContextConstraints{
		ObjectMeta: metav1.ObjectMeta{
			Name: "restrictive",
		},
		RunAsUser: securityapi.RunAsUserStrategyOptions{
			Type: securityapi.RunAsUserStrategyMustRunAs,
			UID:  &exactUID,
		},
		SELinuxContext: securityapi.SELinuxContextStrategyOptions{
			Type: securityapi.SELinuxStrategyMustRunAs,
			SELinuxOptions: &kapi.SELinuxOptions{
				Level: "s9:z0,z1",
			},
		},
		FSGroup: securityapi.FSGroupStrategyOptions{
			Type: securityapi.FSGroupStrategyMustRunAs,
			Ranges: []securityapi.IDRange{
				{Min: 999, Max: 999},
			},
		},
		SupplementalGroups: securityapi.SupplementalGroupsStrategyOptions{
			Type: securityapi.SupplementalGroupsStrategyMustRunAs,
			Ranges: []securityapi.IDRange{
				{Min: 999, Max: 999},
			},
		},
		Groups: []string{"system:serviceaccounts"},
	}
}

// goodPod is empty and should not be used directly for testing since we're providing
// two different SCCs.  Since no values are specified it would be allowed to match any
// SCC when defaults are filled in.
func goodPod() *kapi.Pod {
	return &kapi.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "default",
		},
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
