package admission

import (
	"reflect"
	"sort"
	"strings"
	"testing"

	v1kapi "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/diff"
	kadmission "k8s.io/apiserver/pkg/admission"
	"k8s.io/apiserver/pkg/authentication/user"
	"k8s.io/apiserver/pkg/authorization/authorizer"
	"k8s.io/client-go/tools/cache"
	kapi "k8s.io/kubernetes/pkg/apis/core"
	clientset "k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset"
	clientsetfake "k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset/fake"

	allocator "github.com/openshift/origin/pkg/security"
	admissiontesting "github.com/openshift/origin/pkg/security/admission/testing"
	securityapi "github.com/openshift/origin/pkg/security/apis/security"
	securitylisters "github.com/openshift/origin/pkg/security/generated/listers/security/internalversion"
	oscc "github.com/openshift/origin/pkg/security/securitycontextconstraints"
)

func newTestAdmission(lister securitylisters.SecurityContextConstraintsLister, kclient clientset.Interface, authorizer authorizer.Authorizer) kadmission.Interface {
	return &constraint{
		Handler:    kadmission.NewHandler(kadmission.Create),
		client:     kclient,
		sccLister:  lister,
		authorizer: authorizer,
	}
}

func TestFailClosedOnInvalidPod(t *testing.T) {
	plugin := newTestAdmission(nil, nil, nil)
	pod := &v1kapi.Pod{}
	attrs := kadmission.NewAttributesRecord(pod, nil, kapi.Kind("Pod").WithVersion("version"), pod.Namespace, pod.Name, kapi.Resource("pods").WithVersion("version"), "", kadmission.Create, &user.DefaultInfo{})
	err := plugin.(kadmission.MutationInterface).Admit(attrs)

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

			testSCCAdmit(k, v.sccs, v.pod, v.shouldPass, t)

			containers := v.pod.Spec.Containers
			if i == 0 {
				containers = v.pod.Spec.InitContainers
			}

			if v.expectedCapabilities != nil {
				if !reflect.DeepEqual(v.expectedCapabilities, containers[0].SecurityContext.Capabilities) {
					t.Errorf("%s resulted in caps that were not expected - expected: %#v, received: %#v", k, v.expectedCapabilities, containers[0].SecurityContext.Capabilities)
				}
			}
		}
	}
}

func testSCCAdmit(testCaseName string, sccs []*securityapi.SecurityContextConstraints, pod *kapi.Pod, shouldPass bool, t *testing.T) {
	t.Helper()
	tc := setupClientSet()
	lister := createSCCLister(t, sccs)
	testAuthorizer := &sccTestAuthorizer{t: t}
	plugin := newTestAdmission(lister, tc, testAuthorizer)

	attrs := kadmission.NewAttributesRecord(pod, nil, kapi.Kind("Pod").WithVersion("version"), pod.Namespace, pod.Name, kapi.Resource("pods").WithVersion("version"), "", kadmission.Create, &user.DefaultInfo{})
	err := plugin.(kadmission.MutationInterface).Admit(attrs)

	if shouldPass && err != nil {
		t.Errorf("%s expected no errors but received %v", testCaseName, err)
	}
	if !shouldPass && err == nil {
		t.Errorf("%s expected errors but received none", testCaseName)
	}
}

func TestAdmitSuccess(t *testing.T) {
	// create the annotated namespace and add it to the fake client
	namespace := admissiontesting.CreateNamespaceForTest()

	serviceAccount := admissiontesting.CreateSAForTest()
	serviceAccount.Namespace = namespace.Name

	tc := clientsetfake.NewSimpleClientset(namespace, serviceAccount)

	// used for cases where things are preallocated
	defaultGroup := int64(2)

	// create scc that requires allocation retrieval
	saSCC := saSCC()

	// create scc that has specific requirements that shouldn't match but is permissioned to
	// service accounts to test that even though this has matching priorities (0) and a
	// lower point value score (which will cause it to be sorted in front of scc-sa) it should not
	// validate the requests so we should try scc-sa.
	saExactSCC := saExactSCC()

	lister := createSCCLister(t, []*securityapi.SecurityContextConstraints{
		saExactSCC,
		saSCC,
	})

	testAuthorizer := &sccTestAuthorizer{t: t}

	// create the admission plugin
	p := newTestAdmission(lister, tc, testAuthorizer)

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

	seLinuxLevelFromNamespace := namespace.Annotations[allocator.MCSAnnotation]

	testCases := map[string]struct {
		pod                 *kapi.Pod
		expectedPodSC       *kapi.PodSecurityContext
		expectedContainerSC *kapi.SecurityContext
	}{
		"specifyUIDInRange": {
			pod:                 specifyUIDInRange,
			expectedPodSC:       podSC(seLinuxLevelFromNamespace, defaultGroup, defaultGroup),
			expectedContainerSC: containerSC(nil, goodUID),
		},
		"specifyLabels": {
			pod:                 specifyLabels,
			expectedPodSC:       podSC(seLinuxLevelFromNamespace, defaultGroup, defaultGroup),
			expectedContainerSC: containerSC(&seLinuxLevelFromNamespace, 1),
		},
		"specifyFSGroup": {
			pod:                 specifyFSGroupInRange,
			expectedPodSC:       podSC(seLinuxLevelFromNamespace, goodFSGroup, defaultGroup),
			expectedContainerSC: containerSC(nil, 1),
		},
		"specifySupGroup": {
			pod:                 specifySupGroup,
			expectedPodSC:       podSC(seLinuxLevelFromNamespace, defaultGroup, 3),
			expectedContainerSC: containerSC(nil, 1),
		},
		"specifyPodLevelSELinuxLevel": {
			pod:                 specifyPodLevelSELinux,
			expectedPodSC:       podSC(seLinuxLevelFromNamespace, defaultGroup, defaultGroup),
			expectedContainerSC: containerSC(nil, 1),
		},
	}

	for i := 0; i < 2; i++ {
		for k, v := range testCases {
			v.pod.Spec.Containers, v.pod.Spec.InitContainers = v.pod.Spec.InitContainers, v.pod.Spec.Containers

			hasErrors := testSCCAdmission(v.pod, p, saSCC.Name, k, t)
			if hasErrors {
				continue
			}

			containers := v.pod.Spec.Containers
			if i == 0 {
				containers = v.pod.Spec.InitContainers
			}

			if !reflect.DeepEqual(v.expectedPodSC, v.pod.Spec.SecurityContext) {
				t.Errorf("%s unexpected pod SecurityContext diff:\n%s", k, diff.ObjectGoPrintSideBySide(v.expectedPodSC, v.pod.Spec.SecurityContext))
			}

			if !reflect.DeepEqual(v.expectedContainerSC, containers[0].SecurityContext) {
				t.Errorf("%s unexpected container SecurityContext diff:\n%s", k, diff.ObjectGoPrintSideBySide(v.expectedContainerSC, containers[0].SecurityContext))
			}
		}
	}
}

func TestAdmitFailure(t *testing.T) {
	tc := setupClientSet()

	// create scc that requires allocation retrieval
	saSCC := saSCC()

	// create scc that has specific requirements that shouldn't match but is permissioned to
	// service accounts to test that even though this has matching priorities (0) and a
	// lower point value score (which will cause it to be sorted in front of scc-sa) it should not
	// validate the requests so we should try scc-sa.
	saExactSCC := saExactSCC()

	lister, indexer := createSCCListerAndIndexer(t, []*securityapi.SecurityContextConstraints{
		saExactSCC,
		saSCC,
	})

	testAuthorizer := &sccTestAuthorizer{t: t}

	// create the admission plugin
	p := newTestAdmission(lister, tc, testAuthorizer)

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

	requestsHostNetwork := goodPod()
	requestsHostNetwork.Spec.SecurityContext.HostNetwork = true

	requestsHostPorts := goodPod()
	requestsHostPorts.Spec.Containers[0].Ports = []kapi.ContainerPort{{HostPort: 1}}

	requestsHostPID := goodPod()
	requestsHostPID.Spec.SecurityContext.HostPID = true

	requestsHostIPC := goodPod()
	requestsHostIPC.Spec.SecurityContext.HostIPC = true

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
		pod *kapi.Pod
	}{
		"uidNotInRange": {
			pod: uidNotInRange,
		},
		"invalidMCSLabels": {
			pod: invalidMCSLabels,
		},
		"disallowedPriv": {
			pod: disallowedPriv,
		},
		"requestsHostNetwork": {
			pod: requestsHostNetwork,
		},
		"requestsHostPorts": {
			pod: requestsHostPorts,
		},
		"requestsHostPID": {
			pod: requestsHostPID,
		},
		"requestsHostIPC": {
			pod: requestsHostIPC,
		},
		"requestsSupplementalGroup": {
			pod: requestsSupplementalGroup,
		},
		"requestsFSGroup": {
			pod: requestsFSGroup,
		},
		"requestsPodLevelMCS": {
			pod: requestsPodLevelMCS,
		},
	}

	for i := 0; i < 2; i++ {
		for k, v := range testCases {
			v.pod.Spec.Containers, v.pod.Spec.InitContainers = v.pod.Spec.InitContainers, v.pod.Spec.Containers
			attrs := kadmission.NewAttributesRecord(v.pod, nil, kapi.Kind("Pod").WithVersion("version"), v.pod.Namespace, v.pod.Name, kapi.Resource("pods").WithVersion("version"), "", kadmission.Create, &user.DefaultInfo{})
			err := p.(kadmission.MutationInterface).Admit(attrs)

			if err == nil {
				t.Errorf("%s expected errors but received none", k)
			}
		}
	}

	// now add an escalated scc to the group and re-run the cases that expected failure, they should
	// now pass by validating against the escalated scc.
	adminSCC := laxSCC()
	adminSCC.Name = "scc-admin"
	indexer.Add(adminSCC)

	for i := 0; i < 2; i++ {
		for k, v := range testCases {
			v.pod.Spec.Containers, v.pod.Spec.InitContainers = v.pod.Spec.InitContainers, v.pod.Spec.Containers

			// pods that were rejected by strict SCC, should pass with relaxed SCC
			testSCCAdmission(v.pod, p, adminSCC.Name, k, t)
		}
	}
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

	lister := createSCCLister(t, sccs)

	// single match cases
	testCases := map[string]struct {
		userInfo    user.Info
		authorizer  *sccTestAuthorizer
		namespace   string
		expectedSCC string
	}{
		"find none": {
			userInfo: &user.DefaultInfo{
				Name:   "foo",
				Groups: []string{"bar"},
			},
			authorizer: &sccTestAuthorizer{t: t},
		},
		"find user": {
			userInfo: &user.DefaultInfo{
				Name:   "user",
				Groups: []string{"bar"},
			},
			authorizer:  &sccTestAuthorizer{t: t},
			expectedSCC: "match user",
		},
		"find group": {
			userInfo: &user.DefaultInfo{
				Name:   "foo",
				Groups: []string{"group"},
			},
			authorizer:  &sccTestAuthorizer{t: t},
			expectedSCC: "match group",
		},
		"not find user via authz": {
			userInfo: &user.DefaultInfo{
				Name:   "foo",
				Groups: []string{"bar"},
			},
			authorizer: &sccTestAuthorizer{t: t, user: "not-foo", scc: "match user"},
			namespace:  "fancy",
		},
		"find user via authz cluster wide": {
			userInfo: &user.DefaultInfo{
				Name:   "foo",
				Groups: []string{"bar"},
			},
			authorizer:  &sccTestAuthorizer{t: t, user: "foo", scc: "match user"},
			namespace:   "fancy",
			expectedSCC: "match user",
		},
		"find group via authz in namespace": {
			userInfo: &user.DefaultInfo{
				Name:   "foo",
				Groups: []string{"bar"},
			},
			authorizer:  &sccTestAuthorizer{t: t, user: "foo", namespace: "room", scc: "match group"},
			namespace:   "room",
			expectedSCC: "match group",
		},
	}

	for k, v := range testCases {
		sccMatcher := oscc.NewDefaultSCCMatcher(lister, v.authorizer)
		sccs, err := sccMatcher.FindApplicableSCCs(v.userInfo, v.namespace)
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
	testAuthorizer := &sccTestAuthorizer{t: t}
	namespace := "does-not-matter"
	sccMatcher := oscc.NewDefaultSCCMatcher(lister, testAuthorizer)
	sccs, err := sccMatcher.FindApplicableSCCs(userInfo, namespace)
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

	tc := setupClientSet()
	lister := createSCCLister(t, sccsToSort)
	testAuthorizer := &sccTestAuthorizer{t: t}

	// create the admission plugin
	plugin := newTestAdmission(lister, tc, testAuthorizer)

	testSCCAdmission(goodPod(), plugin, restricted.Name, "match the restricted SCC", t)

	matchingPrioritySCCOnePod := goodPod()
	matchingPrioritySCCOnePod.Spec.Containers[0].SecurityContext.RunAsUser = &uidFive
	testSCCAdmission(matchingPrioritySCCOnePod, plugin, matchingPrioritySCCOne.Name, "match matchingPrioritySCCOne by setting RunAsUser to 5", t)

	matchingPriorityAndScoreSCCOnePod := goodPod()
	matchingPriorityAndScoreSCCOnePod.Spec.Containers[0].SecurityContext.RunAsUser = &uidSix
	testSCCAdmission(matchingPriorityAndScoreSCCOnePod, plugin, matchingPriorityAndScoreSCCOne.Name, "match matchingPriorityAndScoreSCCOne by setting RunAsUser to 6", t)
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

func TestAdmitPreferNonmutatingWhenPossible(t *testing.T) {

	mutatingSCC := restrictiveSCC()
	mutatingSCC.Name = "mutating-scc"

	nonMutatingSCC := laxSCC()
	nonMutatingSCC.Name = "non-mutating-scc"

	simplePod := goodPod()
	simplePod.Spec.Containers[0].Name = "simple-pod"
	simplePod.Spec.Containers[0].Image = "test-image:0.1"

	modifiedPod := simplePod.DeepCopy()
	modifiedPod.Spec.Containers[0].Image = "test-image:0.2"

	tests := map[string]struct {
		oldPod      *kapi.Pod
		newPod      *kapi.Pod
		operation   kadmission.Operation
		sccs        []*securityapi.SecurityContextConstraints
		shouldPass  bool
		expectedSCC string
	}{
		"creation: the first SCC (even if it mutates) should be used": {
			newPod:      simplePod.DeepCopy(),
			operation:   kadmission.Create,
			sccs:        []*securityapi.SecurityContextConstraints{mutatingSCC, nonMutatingSCC},
			shouldPass:  true,
			expectedSCC: mutatingSCC.Name,
		},
		"updating: the first non-mutating SCC should be used": {
			oldPod:      simplePod.DeepCopy(),
			newPod:      modifiedPod.DeepCopy(),
			operation:   kadmission.Update,
			sccs:        []*securityapi.SecurityContextConstraints{mutatingSCC, nonMutatingSCC},
			shouldPass:  true,
			expectedSCC: nonMutatingSCC.Name,
		},
		"updating: a pod should be rejected when there are only mutating SCCs": {
			oldPod:     simplePod.DeepCopy(),
			newPod:     modifiedPod.DeepCopy(),
			operation:  kadmission.Update,
			sccs:       []*securityapi.SecurityContextConstraints{mutatingSCC},
			shouldPass: false,
		},
	}

	for testCaseName, testCase := range tests {
		// We can't use testSCCAdmission() here because it doesn't support Update operation.
		// We can't use testSCCAdmit() here because it doesn't support Update operation and doesn't check for the SCC annotation.

		tc := setupClientSet()
		lister := createSCCLister(t, testCase.sccs)
		testAuthorizer := &sccTestAuthorizer{t: t}
		plugin := newTestAdmission(lister, tc, testAuthorizer)

		attrs := kadmission.NewAttributesRecord(testCase.newPod, testCase.oldPod, kapi.Kind("Pod").WithVersion("version"), testCase.newPod.Namespace, testCase.newPod.Name, kapi.Resource("pods").WithVersion("version"), "", testCase.operation, &user.DefaultInfo{})
		err := plugin.(kadmission.MutationInterface).Admit(attrs)

		if testCase.shouldPass {
			if err != nil {
				t.Errorf("%s expected no errors but received %v", testCaseName, err)
			} else {
				validatedSCC, ok := testCase.newPod.Annotations[allocator.ValidatedSCCAnnotation]
				if !ok {
					t.Errorf("expected %q to find the validated annotation on the pod for the scc but found none", testCaseName)

				} else if validatedSCC != testCase.expectedSCC {
					t.Errorf("%q should have validated against %q but found %q", testCaseName, testCase.expectedSCC, validatedSCC)
				}
			}
		} else {
			if err == nil {
				t.Errorf("%s expected errors but received none", testCaseName)
			}
		}
	}

}

// testSCCAdmission is a helper to admit the pod and ensure it was validated against the expected
// SCC. Returns true when errors have been encountered.
func testSCCAdmission(pod *kapi.Pod, plugin kadmission.Interface, expectedSCC, testName string, t *testing.T) bool {
	t.Helper()
	attrs := kadmission.NewAttributesRecord(pod, nil, kapi.Kind("Pod").WithVersion("version"), pod.Namespace, pod.Name, kapi.Resource("pods").WithVersion("version"), "", kadmission.Create, &user.DefaultInfo{})
	err := plugin.(kadmission.MutationInterface).Admit(attrs)
	if err != nil {
		t.Errorf("%s error admitting pod: %v", testName, err)
		return true
	}

	validatedSCC, ok := pod.Annotations[allocator.ValidatedSCCAnnotation]
	if !ok {
		t.Errorf("expected %q to find the validated annotation on the pod for the scc but found none", testName)
		return true
	}
	if validatedSCC != expectedSCC {
		t.Errorf("%q should have validated against %s but found %s", testName, expectedSCC, validatedSCC)
		return true
	}
	return false
}

func laxSCC() *securityapi.SecurityContextConstraints {
	return &securityapi.SecurityContextConstraints{
		ObjectMeta: metav1.ObjectMeta{
			Name: "lax",
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

func saSCC() *securityapi.SecurityContextConstraints {
	return &securityapi.SecurityContextConstraints{
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
}

func saExactSCC() *securityapi.SecurityContextConstraints {
	var exactUID int64 = 999
	return &securityapi.SecurityContextConstraints{
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

func containerSC(seLinuxLevel *string, uid int64) *kapi.SecurityContext {
	sc := &kapi.SecurityContext{
		RunAsUser: &uid,
	}
	if seLinuxLevel != nil {
		sc.SELinuxOptions = &kapi.SELinuxOptions{
			Level: *seLinuxLevel,
		}
	}
	return sc
}

func podSC(seLinuxLevel string, fsGroup, supGroup int64) *kapi.PodSecurityContext {
	return &kapi.PodSecurityContext{
		SELinuxOptions: &kapi.SELinuxOptions{
			Level: seLinuxLevel,
		},
		SupplementalGroups: []int64{supGroup},
		FSGroup:            &fsGroup,
	}
}

func setupClientSet() *clientsetfake.Clientset {
	// create the annotated namespace and add it to the fake client
	namespace := admissiontesting.CreateNamespaceForTest()
	serviceAccount := admissiontesting.CreateSAForTest()
	serviceAccount.Namespace = namespace.Name

	return clientsetfake.NewSimpleClientset(namespace, serviceAccount)
}

func createSCCListerAndIndexer(t *testing.T, sccs []*securityapi.SecurityContextConstraints) (securitylisters.SecurityContextConstraintsLister, cache.Indexer) {
	t.Helper()
	indexer := cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc})
	lister := securitylisters.NewSecurityContextConstraintsLister(indexer)
	for _, scc := range sccs {
		if err := indexer.Add(scc); err != nil {
			t.Fatalf("error adding SCC to store: %v", err)
		}
	}
	return lister, indexer
}

func createSCCLister(t *testing.T, sccs []*securityapi.SecurityContextConstraints) securitylisters.SecurityContextConstraintsLister {
	t.Helper()
	lister, _ := createSCCListerAndIndexer(t, sccs)
	return lister
}

type sccTestAuthorizer struct {
	t *testing.T

	// this user, in this namespace, can use this SCC
	user      string
	namespace string
	scc       string
}

func (s *sccTestAuthorizer) Authorize(a authorizer.Attributes) (authorizer.Decision, string, error) {
	s.t.Helper()
	if !isValidSCCAttributes(a) {
		s.t.Errorf("invalid attributes seen: %#v", a)
		return authorizer.DecisionDeny, "", nil
	}

	allowedNamespace := len(s.namespace) == 0 || s.namespace == a.GetNamespace()
	if s.user == a.GetUser().GetName() && allowedNamespace && s.scc == a.GetName() {
		return authorizer.DecisionAllow, "", nil
	}

	return authorizer.DecisionNoOpinion, "", nil
}

func isValidSCCAttributes(a authorizer.Attributes) bool {
	return a.GetVerb() == "use" &&
		a.GetAPIGroup() == "security.openshift.io" &&
		a.GetResource() == "securitycontextconstraints" &&
		a.IsResourceRequest()
}
