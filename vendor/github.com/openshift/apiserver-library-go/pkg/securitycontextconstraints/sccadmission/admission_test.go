package sccadmission

import (
	"context"
	"reflect"
	"sort"
	"strings"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/diff"
	"k8s.io/apiserver/pkg/admission"
	"k8s.io/apiserver/pkg/authentication/user"
	"k8s.io/apiserver/pkg/authorization/authorizer"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/tools/cache"
	coreapi "k8s.io/kubernetes/pkg/apis/core"

	securityv1 "github.com/openshift/api/security/v1"
	"github.com/openshift/apiserver-library-go/pkg/securitycontextconstraints/sccmatching"
	sccsort "github.com/openshift/apiserver-library-go/pkg/securitycontextconstraints/util/sort"
	securityv1listers "github.com/openshift/client-go/security/listers/security/v1"
)

// createSAForTest Build and Initializes a ServiceAccount for tests
func createSAForTest() *corev1.ServiceAccount {
	return &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "default",
			Namespace: "default",
		},
	}
}

// createNamespaceForTest builds and initializes a Namespaces for tests
func createNamespaceForTest() *corev1.Namespace {
	return &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: "default",
			Annotations: map[string]string{
				securityv1.UIDRangeAnnotation:           "1/3",
				securityv1.MCSAnnotation:                "s0:c1,c0",
				securityv1.SupplementalGroupsAnnotation: "2/3",
			},
		},
	}
}

func newTestAdmission(lister securityv1listers.SecurityContextConstraintsLister, kclient kubernetes.Interface, authorizer authorizer.Authorizer) admission.Interface {
	return &constraint{
		Handler:    admission.NewHandler(admission.Create),
		client:     kclient,
		sccLister:  lister,
		sccSynced:  func() bool { return true },
		authorizer: authorizer,
	}
}

func TestFailClosedOnInvalidPod(t *testing.T) {
	plugin := newTestAdmission(nil, nil, nil)
	pod := &corev1.Pod{}
	attrs := admission.NewAttributesRecord(pod, nil, coreapi.Kind("Pod").WithVersion("version"), pod.Namespace, pod.Name, coreapi.Resource("pods").WithVersion("version"), "", admission.Create, nil, false, &user.DefaultInfo{})
	err := plugin.(admission.MutationInterface).Admit(context.TODO(), attrs, nil)

	if err == nil {
		t.Fatalf("expected versioned pod object to fail admission")
	}
	if !strings.Contains(err.Error(), "object was marked as kind pod but was unable to be converted") {
		t.Errorf("expected error to be conversion erorr but got: %v", err)
	}
}

func TestAdmitCaps(t *testing.T) {
	createPodWithCaps := func(caps *coreapi.Capabilities) *coreapi.Pod {
		pod := goodPod()
		pod.Spec.Containers[0].SecurityContext.Capabilities = caps
		return pod
	}

	restricted := restrictiveSCC()

	allowsFooInAllowed := restrictiveSCC()
	allowsFooInAllowed.Name = "allowCapInAllowed"
	allowsFooInAllowed.AllowedCapabilities = []corev1.Capability{"foo"}

	allowsFooInRequired := restrictiveSCC()
	allowsFooInRequired.Name = "allowCapInRequired"
	allowsFooInRequired.DefaultAddCapabilities = []corev1.Capability{"foo"}

	requiresFooToBeDropped := restrictiveSCC()
	requiresFooToBeDropped.Name = "requireDrop"
	requiresFooToBeDropped.RequiredDropCapabilities = []corev1.Capability{"foo"}

	allowAllInAllowed := restrictiveSCC()
	allowAllInAllowed.Name = "allowAllCapsInAllowed"
	allowAllInAllowed.AllowedCapabilities = []corev1.Capability{securityv1.AllowAllCapabilities}

	tc := map[string]struct {
		pod                  *coreapi.Pod
		sccs                 []*securityv1.SecurityContextConstraints
		shouldPass           bool
		expectedCapabilities *coreapi.Capabilities
	}{
		// UC 1: if an SCC does not define allowed or required caps then a pod requesting a cap
		// should be rejected.
		"should reject cap add when not allowed or required": {
			pod:        createPodWithCaps(&coreapi.Capabilities{Add: []coreapi.Capability{"foo"}}),
			sccs:       []*securityv1.SecurityContextConstraints{restricted},
			shouldPass: false,
		},
		// UC 2: if an SCC allows a cap in the allowed field it should accept the pod request
		// to add the cap.
		"should accept cap add when in allowed": {
			pod:        createPodWithCaps(&coreapi.Capabilities{Add: []coreapi.Capability{"foo"}}),
			sccs:       []*securityv1.SecurityContextConstraints{restricted, allowsFooInAllowed},
			shouldPass: true,
		},
		// UC 3: if an SCC requires a cap then it should accept the pod request
		// to add the cap.
		"should accept cap add when in required": {
			pod:        createPodWithCaps(&coreapi.Capabilities{Add: []coreapi.Capability{"foo"}}),
			sccs:       []*securityv1.SecurityContextConstraints{restricted, allowsFooInRequired},
			shouldPass: true,
		},
		// UC 4: if an SCC requires a cap to be dropped then it should fail both
		// in the verification of adds and verification of drops
		"should reject cap add when requested cap is required to be dropped": {
			pod:        createPodWithCaps(&coreapi.Capabilities{Add: []coreapi.Capability{"foo"}}),
			sccs:       []*securityv1.SecurityContextConstraints{restricted, requiresFooToBeDropped},
			shouldPass: false,
		},
		// UC 5: if an SCC requires a cap to be dropped it should accept
		// a manual request to drop the cap.
		"should accept cap drop when cap is required to be dropped": {
			pod:        createPodWithCaps(&coreapi.Capabilities{Drop: []coreapi.Capability{"foo"}}),
			sccs:       []*securityv1.SecurityContextConstraints{restricted, requiresFooToBeDropped},
			shouldPass: true,
		},
		// UC 6: required add is defaulted
		"required add is defaulted": {
			pod:        goodPod(),
			sccs:       []*securityv1.SecurityContextConstraints{allowsFooInRequired},
			shouldPass: true,
			expectedCapabilities: &coreapi.Capabilities{
				Add: []coreapi.Capability{"foo"},
			},
		},
		// UC 7: required drop is defaulted
		"required drop is defaulted": {
			pod:        goodPod(),
			sccs:       []*securityv1.SecurityContextConstraints{requiresFooToBeDropped},
			shouldPass: true,
			expectedCapabilities: &coreapi.Capabilities{
				Drop: []coreapi.Capability{"foo"},
			},
		},
		// UC 8: using '*' in allowed caps
		"should accept cap add when all caps are allowed": {
			pod:        createPodWithCaps(&coreapi.Capabilities{Add: []coreapi.Capability{"foo"}}),
			sccs:       []*securityv1.SecurityContextConstraints{restricted, allowAllInAllowed},
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

func testSCCAdmit(testCaseName string, sccs []*securityv1.SecurityContextConstraints, pod *coreapi.Pod, shouldPass bool, t *testing.T) {
	t.Helper()
	tc := setupClientSet()
	lister := createSCCLister(t, sccs)
	testAuthorizer := &sccTestAuthorizer{t: t}
	plugin := newTestAdmission(lister, tc, testAuthorizer)

	attrs := admission.NewAttributesRecord(pod, nil, coreapi.Kind("Pod").WithVersion("version"), pod.Namespace, pod.Name, coreapi.Resource("pods").WithVersion("version"), "", admission.Create, nil, false, &user.DefaultInfo{})
	err := plugin.(admission.MutationInterface).Admit(context.TODO(), attrs, nil)
	if shouldPass && err != nil {
		t.Errorf("%s expected no mutating admission errors but received %v", testCaseName, err)
	}
	if !shouldPass && err == nil {
		t.Errorf("%s expected mutating admission errors but received none", testCaseName)
	}

	err = plugin.(admission.ValidationInterface).Validate(context.TODO(), attrs, nil)
	if shouldPass && err != nil {
		t.Errorf("%s expected no validating admission errors but received %v", testCaseName, err)
	}
	if !shouldPass && err == nil {
		t.Errorf("%s expected validating admission errors but received none", testCaseName)
	}
}

func TestAdmitSuccess(t *testing.T) {
	// create the annotated namespace and add it to the fake client
	namespace := createNamespaceForTest()

	serviceAccount := createSAForTest()
	serviceAccount.Namespace = namespace.Name

	tc := fake.NewSimpleClientset(namespace, serviceAccount)

	// used for cases where things are preallocated
	defaultGroup := int64(2)

	// create scc that requires allocation retrieval
	saSCC := saSCC()

	// create scc that has specific requirements that shouldn't match but is permissioned to
	// service accounts to test that even though this has matching priorities (0) and a
	// lower point value score (which will cause it to be sorted in front of scc-sa) it should not
	// validate the requests so we should try scc-sa.
	saExactSCC := saExactSCC()

	lister := createSCCLister(t, []*securityv1.SecurityContextConstraints{
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
	specifyLabels.Spec.Containers[0].SecurityContext.SELinuxOptions = &coreapi.SELinuxOptions{
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
	specifyPodLevelSELinux.Spec.SecurityContext.SELinuxOptions = &coreapi.SELinuxOptions{
		Level: "s0:c1,c0",
	}

	seLinuxLevelFromNamespace := namespace.Annotations[securityv1.MCSAnnotation]

	testCases := map[string]struct {
		pod                 *coreapi.Pod
		expectedPodSC       *coreapi.PodSecurityContext
		expectedContainerSC *coreapi.SecurityContext
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

	lister, indexer := createSCCListerAndIndexer(t, []*securityv1.SecurityContextConstraints{
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
	invalidMCSLabels.Spec.Containers[0].SecurityContext.SELinuxOptions = &coreapi.SELinuxOptions{
		Level: "s1:q0,q1",
	}

	disallowedPriv := goodPod()
	var priv bool = true
	disallowedPriv.Spec.Containers[0].SecurityContext.Privileged = &priv

	requestsHostNetwork := goodPod()
	requestsHostNetwork.Spec.SecurityContext.HostNetwork = true

	requestsHostPorts := goodPod()
	requestsHostPorts.Spec.Containers[0].Ports = []coreapi.ContainerPort{{HostPort: 1}}

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
	requestsPodLevelMCS.Spec.SecurityContext.SELinuxOptions = &coreapi.SELinuxOptions{
		User:  "user",
		Type:  "type",
		Role:  "role",
		Level: "level",
	}

	testCases := map[string]struct {
		pod *coreapi.Pod
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
			attrs := admission.NewAttributesRecord(v.pod, nil, coreapi.Kind("Pod").WithVersion("version"), v.pod.Namespace, v.pod.Name, coreapi.Resource("pods").WithVersion("version"), "", admission.Create, nil, false, &user.DefaultInfo{})
			err := p.(admission.MutationInterface).Admit(context.TODO(), attrs, nil)

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
	namespaceValid := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: "default",
			Annotations: map[string]string{
				securityv1.UIDRangeAnnotation:           "1/3",
				securityv1.MCSAnnotation:                "s0:c1,c0",
				securityv1.SupplementalGroupsAnnotation: "1/3",
			},
		},
	}
	namespaceNoUID := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: "default",
			Annotations: map[string]string{
				securityv1.MCSAnnotation:                "s0:c1,c0",
				securityv1.SupplementalGroupsAnnotation: "1/3",
			},
		},
	}
	namespaceNoMCS := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: "default",
			Annotations: map[string]string{
				securityv1.UIDRangeAnnotation:           "1/3",
				securityv1.SupplementalGroupsAnnotation: "1/3",
			},
		},
	}

	namespaceNoSupplementalGroupsFallbackToUID := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: "default",
			Annotations: map[string]string{
				securityv1.UIDRangeAnnotation: "1/3",
				securityv1.MCSAnnotation:      "s0:c1,c0",
			},
		},
	}

	namespaceBadSupGroups := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: "default",
			Annotations: map[string]string{
				securityv1.UIDRangeAnnotation:           "1/3",
				securityv1.MCSAnnotation:                "s0:c1,c0",
				securityv1.SupplementalGroupsAnnotation: "",
			},
		},
	}

	testCases := map[string]struct {
		// use a generating function so we can test for non-mutation
		scc         func() *securityv1.SecurityContextConstraints
		namespace   *corev1.Namespace
		expectedErr string
	}{
		"valid non-preallocated scc": {
			scc: func() *securityv1.SecurityContextConstraints {
				return &securityv1.SecurityContextConstraints{
					ObjectMeta: metav1.ObjectMeta{
						Name: "valid non-preallocated scc",
					},
					SELinuxContext: securityv1.SELinuxContextStrategyOptions{
						Type: securityv1.SELinuxStrategyRunAsAny,
					},
					RunAsUser: securityv1.RunAsUserStrategyOptions{
						Type: securityv1.RunAsUserStrategyRunAsAny,
					},
					FSGroup: securityv1.FSGroupStrategyOptions{
						Type: securityv1.FSGroupStrategyRunAsAny,
					},
					SupplementalGroups: securityv1.SupplementalGroupsStrategyOptions{
						Type: securityv1.SupplementalGroupsStrategyRunAsAny,
					},
				}
			},
			namespace: namespaceValid,
		},
		"valid pre-allocated scc": {
			scc: func() *securityv1.SecurityContextConstraints {
				return &securityv1.SecurityContextConstraints{
					ObjectMeta: metav1.ObjectMeta{
						Name: "valid pre-allocated scc",
					},
					SELinuxContext: securityv1.SELinuxContextStrategyOptions{
						Type:           securityv1.SELinuxStrategyMustRunAs,
						SELinuxOptions: &corev1.SELinuxOptions{User: "myuser"},
					},
					RunAsUser: securityv1.RunAsUserStrategyOptions{
						Type: securityv1.RunAsUserStrategyMustRunAsRange,
					},
					FSGroup: securityv1.FSGroupStrategyOptions{
						Type: securityv1.FSGroupStrategyMustRunAs,
					},
					SupplementalGroups: securityv1.SupplementalGroupsStrategyOptions{
						Type: securityv1.SupplementalGroupsStrategyMustRunAs,
					},
				}
			},
			namespace: namespaceValid,
		},
		"pre-allocated no uid annotation": {
			scc: func() *securityv1.SecurityContextConstraints {
				return &securityv1.SecurityContextConstraints{
					ObjectMeta: metav1.ObjectMeta{
						Name: "pre-allocated no uid annotation",
					},
					SELinuxContext: securityv1.SELinuxContextStrategyOptions{
						Type: securityv1.SELinuxStrategyMustRunAs,
					},
					RunAsUser: securityv1.RunAsUserStrategyOptions{
						Type: securityv1.RunAsUserStrategyMustRunAsRange,
					},
					FSGroup: securityv1.FSGroupStrategyOptions{
						Type: securityv1.FSGroupStrategyRunAsAny,
					},
					SupplementalGroups: securityv1.SupplementalGroupsStrategyOptions{
						Type: securityv1.SupplementalGroupsStrategyRunAsAny,
					},
				}
			},
			namespace:   namespaceNoUID,
			expectedErr: "unable to find pre-allocated uid annotation",
		},
		"pre-allocated no mcs annotation": {
			scc: func() *securityv1.SecurityContextConstraints {
				return &securityv1.SecurityContextConstraints{
					ObjectMeta: metav1.ObjectMeta{
						Name: "pre-allocated no mcs annotation",
					},
					SELinuxContext: securityv1.SELinuxContextStrategyOptions{
						Type: securityv1.SELinuxStrategyMustRunAs,
					},
					RunAsUser: securityv1.RunAsUserStrategyOptions{
						Type: securityv1.RunAsUserStrategyMustRunAsRange,
					},
					FSGroup: securityv1.FSGroupStrategyOptions{
						Type: securityv1.FSGroupStrategyRunAsAny,
					},
					SupplementalGroups: securityv1.SupplementalGroupsStrategyOptions{
						Type: securityv1.SupplementalGroupsStrategyRunAsAny,
					},
				}
			},
			namespace:   namespaceNoMCS,
			expectedErr: "unable to find pre-allocated mcs annotation",
		},
		"pre-allocated group falls back to UID annotation": {
			scc: func() *securityv1.SecurityContextConstraints {
				return &securityv1.SecurityContextConstraints{
					ObjectMeta: metav1.ObjectMeta{
						Name: "pre-allocated no sup group annotation",
					},
					SELinuxContext: securityv1.SELinuxContextStrategyOptions{
						Type: securityv1.SELinuxStrategyRunAsAny,
					},
					RunAsUser: securityv1.RunAsUserStrategyOptions{
						Type: securityv1.RunAsUserStrategyRunAsAny,
					},
					FSGroup: securityv1.FSGroupStrategyOptions{
						Type: securityv1.FSGroupStrategyMustRunAs,
					},
					SupplementalGroups: securityv1.SupplementalGroupsStrategyOptions{
						Type: securityv1.SupplementalGroupsStrategyMustRunAs,
					},
				}
			},
			namespace: namespaceNoSupplementalGroupsFallbackToUID,
		},
		"pre-allocated group bad value fails": {
			scc: func() *securityv1.SecurityContextConstraints {
				return &securityv1.SecurityContextConstraints{
					ObjectMeta: metav1.ObjectMeta{
						Name: "pre-allocated no sup group annotation",
					},
					SELinuxContext: securityv1.SELinuxContextStrategyOptions{
						Type: securityv1.SELinuxStrategyRunAsAny,
					},
					RunAsUser: securityv1.RunAsUserStrategyOptions{
						Type: securityv1.RunAsUserStrategyRunAsAny,
					},
					FSGroup: securityv1.FSGroupStrategyOptions{
						Type: securityv1.FSGroupStrategyMustRunAs,
					},
					SupplementalGroups: securityv1.SupplementalGroupsStrategyOptions{
						Type: securityv1.SupplementalGroupsStrategyMustRunAs,
					},
				}
			},
			namespace:   namespaceBadSupGroups,
			expectedErr: "unable to find pre-allocated group annotation",
		},
		"bad scc strategy options": {
			scc: func() *securityv1.SecurityContextConstraints {
				return &securityv1.SecurityContextConstraints{
					ObjectMeta: metav1.ObjectMeta{
						Name: "bad scc user options",
					},
					SELinuxContext: securityv1.SELinuxContextStrategyOptions{
						Type: securityv1.SELinuxStrategyRunAsAny,
					},
					RunAsUser: securityv1.RunAsUserStrategyOptions{
						Type: securityv1.RunAsUserStrategyMustRunAs,
					},
					FSGroup: securityv1.FSGroupStrategyOptions{
						Type: securityv1.FSGroupStrategyRunAsAny,
					},
					SupplementalGroups: securityv1.SupplementalGroupsStrategyOptions{
						Type: securityv1.SupplementalGroupsStrategyRunAsAny,
					},
				}
			},
			namespace:   namespaceValid,
			expectedErr: "MustRunAs requires a UID",
		},
	}

	for k, v := range testCases {
		// create the admission handler
		tc := fake.NewSimpleClientset(v.namespace)
		scc := v.scc()

		// create the providers, this method only needs the namespace
		attributes := admission.NewAttributesRecord(nil, nil, coreapi.Kind("Pod").WithVersion("version"), v.namespace.Name, "", coreapi.Resource("pods").WithVersion("version"), "", admission.Create, nil, false, nil)
		_, errs := sccmatching.CreateProvidersFromConstraints(attributes.GetNamespace(), []*securityv1.SecurityContextConstraints{scc}, tc)

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
	sccs := []*securityv1.SecurityContextConstraints{
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
		sccMatcher := sccmatching.NewDefaultSCCMatcher(lister, v.authorizer)
		sccs, err := sccMatcher.FindApplicableSCCs(context.TODO(), v.namespace, v.userInfo)
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
	sccMatcher := sccmatching.NewDefaultSCCMatcher(lister, testAuthorizer)
	sccs2, err := sccMatcher.FindApplicableSCCs(context.TODO(), namespace, userInfo)
	if err != nil {
		t.Fatalf("matching many sccs returned error %v", err)
	}
	if len(sccs2) != 2 {
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
	matchingPrioritySCCOne.RunAsUser = securityv1.RunAsUserStrategyOptions{
		Type: securityv1.RunAsUserStrategyMustRunAs,
		UID:  &uidFive,
	}
	matchingPriority := int32(5)
	matchingPrioritySCCOne.Priority = &matchingPriority

	matchingPrioritySCCTwo := laxSCC()
	matchingPrioritySCCTwo.Name = "matchingPrioritySCCTwo"
	matchingPrioritySCCTwo.RunAsUser = securityv1.RunAsUserStrategyOptions{
		Type:        securityv1.RunAsUserStrategyMustRunAsRange,
		UIDRangeMin: &uidFive,
		UIDRangeMax: &uidFive,
	}
	matchingPrioritySCCTwo.Priority = &matchingPriority

	// sccs with matching priorities and scores so should be matched by sorted name
	uidSix := int64(6)
	matchingPriorityAndScoreSCCOne := laxSCC()
	matchingPriorityAndScoreSCCOne.Name = "matchingPriorityAndScoreSCCOne"
	matchingPriorityAndScoreSCCOne.RunAsUser = securityv1.RunAsUserStrategyOptions{
		Type: securityv1.RunAsUserStrategyMustRunAs,
		UID:  &uidSix,
	}
	matchingPriorityAndScorePriority := int32(1)
	matchingPriorityAndScoreSCCOne.Priority = &matchingPriorityAndScorePriority

	matchingPriorityAndScoreSCCTwo := laxSCC()
	matchingPriorityAndScoreSCCTwo.Name = "matchingPriorityAndScoreSCCTwo"
	matchingPriorityAndScoreSCCTwo.RunAsUser = securityv1.RunAsUserStrategyOptions{
		Type: securityv1.RunAsUserStrategyMustRunAs,
		UID:  &uidSix,
	}
	matchingPriorityAndScoreSCCTwo.Priority = &matchingPriorityAndScorePriority

	// we will expect these to sort as:
	expectedSort := []string{"restrictive", "matchingPrioritySCCOne", "matchingPrioritySCCTwo",
		"matchingPriorityAndScoreSCCOne", "matchingPriorityAndScoreSCCTwo"}
	sccsToSort := []*securityv1.SecurityContextConstraints{matchingPriorityAndScoreSCCTwo, matchingPriorityAndScoreSCCOne,
		matchingPrioritySCCTwo, matchingPrioritySCCOne, restricted}

	sort.Sort(sccsort.ByPriority(sccsToSort))

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
	createPodWithSeccomp := func(podAnnotation, containerAnnotation string) *coreapi.Pod {
		pod := goodPod()
		pod.Annotations = map[string]string{}
		if podAnnotation != "" {
			pod.Annotations[coreapi.SeccompPodAnnotationKey] = podAnnotation
		}
		if containerAnnotation != "" {
			pod.Annotations[coreapi.SeccompContainerAnnotationKeyPrefix+"container"] = containerAnnotation
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
		pod                   *coreapi.Pod
		sccs                  []*securityv1.SecurityContextConstraints
		shouldPass            bool
		expectedPodAnnotation string
		expectedSCC           string
	}{
		"no seccomp, no requests": {
			pod:         goodPod(),
			sccs:        []*securityv1.SecurityContextConstraints{noSeccompSCC},
			shouldPass:  true,
			expectedSCC: noSeccompSCC.Name,
		},
		"no seccomp, bad container requests": {
			pod:        createPodWithSeccomp("foo", "bar"),
			sccs:       []*securityv1.SecurityContextConstraints{noSeccompSCC},
			shouldPass: false,
		},
		"seccomp, no requests": {
			pod:                   goodPod(),
			sccs:                  []*securityv1.SecurityContextConstraints{seccompSCC},
			shouldPass:            true,
			expectedPodAnnotation: "foo",
			expectedSCC:           seccompSCC.Name,
		},
		"seccomp, valid pod annotation, no container annotation": {
			pod:                   createPodWithSeccomp("foo", ""),
			sccs:                  []*securityv1.SecurityContextConstraints{seccompSCC},
			shouldPass:            true,
			expectedPodAnnotation: "foo",
			expectedSCC:           seccompSCC.Name,
		},
		"seccomp, no pod annotation, valid container annotation": {
			pod:                   createPodWithSeccomp("", "foo"),
			sccs:                  []*securityv1.SecurityContextConstraints{seccompSCC},
			shouldPass:            true,
			expectedPodAnnotation: "foo",
			expectedSCC:           seccompSCC.Name,
		},
		"seccomp, valid pod annotation, invalid container annotation": {
			pod:        createPodWithSeccomp("foo", "bar"),
			sccs:       []*securityv1.SecurityContextConstraints{seccompSCC},
			shouldPass: false,
		},
		"wild card, no requests": {
			pod:         goodPod(),
			sccs:        []*securityv1.SecurityContextConstraints{wildcardSCC},
			shouldPass:  true,
			expectedSCC: wildcardSCC.Name,
		},
		"wild card, requests": {
			pod:                   createPodWithSeccomp("foo", "bar"),
			sccs:                  []*securityv1.SecurityContextConstraints{wildcardSCC},
			shouldPass:            true,
			expectedPodAnnotation: "foo",
			expectedSCC:           wildcardSCC.Name,
		},
	}

	for k, v := range tests {
		testSCCAdmit(k, v.sccs, v.pod, v.shouldPass, t)

		if v.shouldPass {
			validatedSCC, ok := v.pod.Annotations[securityv1.ValidatedSCCAnnotation]
			if !ok {
				t.Errorf("expected to find the validated annotation on the pod for the scc but found none")
				return
			}
			if validatedSCC != v.expectedSCC {
				t.Errorf("should have validated against %s but found %s", v.expectedSCC, validatedSCC)
			}

			if len(v.expectedPodAnnotation) > 0 {
				annotation, found := v.pod.Annotations[coreapi.SeccompPodAnnotationKey]
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
		oldPod      *coreapi.Pod
		newPod      *coreapi.Pod
		operation   admission.Operation
		sccs        []*securityv1.SecurityContextConstraints
		shouldPass  bool
		expectedSCC string
	}{
		"creation: the first SCC (even if it mutates) should be used": {
			newPod:      simplePod.DeepCopy(),
			operation:   admission.Create,
			sccs:        []*securityv1.SecurityContextConstraints{mutatingSCC, nonMutatingSCC},
			shouldPass:  true,
			expectedSCC: mutatingSCC.Name,
		},
		"updating: the first non-mutating SCC should be used": {
			oldPod:      simplePod.DeepCopy(),
			newPod:      modifiedPod.DeepCopy(),
			operation:   admission.Update,
			sccs:        []*securityv1.SecurityContextConstraints{mutatingSCC, nonMutatingSCC},
			shouldPass:  true,
			expectedSCC: nonMutatingSCC.Name,
		},
		"updating: a pod should be rejected when there are only mutating SCCs": {
			oldPod:     simplePod.DeepCopy(),
			newPod:     modifiedPod.DeepCopy(),
			operation:  admission.Update,
			sccs:       []*securityv1.SecurityContextConstraints{mutatingSCC},
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

		attrs := admission.NewAttributesRecord(testCase.newPod, testCase.oldPod, coreapi.Kind("Pod").WithVersion("version"), testCase.newPod.Namespace, testCase.newPod.Name, coreapi.Resource("pods").WithVersion("version"), "", testCase.operation, nil, false, &user.DefaultInfo{})
		err := plugin.(admission.MutationInterface).Admit(context.TODO(), attrs, nil)

		if testCase.shouldPass {
			if err != nil {
				t.Errorf("%s expected no errors but received %v", testCaseName, err)
			} else {
				validatedSCC, ok := testCase.newPod.Annotations[securityv1.ValidatedSCCAnnotation]
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
func testSCCAdmission(pod *coreapi.Pod, plugin admission.Interface, expectedSCC, testName string, t *testing.T) bool {
	t.Helper()
	attrs := admission.NewAttributesRecord(pod, nil, coreapi.Kind("Pod").WithVersion("version"), pod.Namespace, pod.Name, coreapi.Resource("pods").WithVersion("version"), "", admission.Create, nil, false, &user.DefaultInfo{})
	err := plugin.(admission.MutationInterface).Admit(context.TODO(), attrs, nil)
	if err != nil {
		t.Errorf("%s error admitting pod: %v", testName, err)
		return true
	}

	validatedSCC, ok := pod.Annotations[securityv1.ValidatedSCCAnnotation]
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

func laxSCC() *securityv1.SecurityContextConstraints {
	return &securityv1.SecurityContextConstraints{
		ObjectMeta: metav1.ObjectMeta{
			Name: "lax",
		},
		AllowPrivilegedContainer: true,
		AllowHostNetwork:         true,
		AllowHostPorts:           true,
		AllowHostPID:             true,
		AllowHostIPC:             true,
		RunAsUser: securityv1.RunAsUserStrategyOptions{
			Type: securityv1.RunAsUserStrategyRunAsAny,
		},
		SELinuxContext: securityv1.SELinuxContextStrategyOptions{
			Type: securityv1.SELinuxStrategyRunAsAny,
		},
		FSGroup: securityv1.FSGroupStrategyOptions{
			Type: securityv1.FSGroupStrategyRunAsAny,
		},
		SupplementalGroups: securityv1.SupplementalGroupsStrategyOptions{
			Type: securityv1.SupplementalGroupsStrategyRunAsAny,
		},
		Groups: []string{"system:serviceaccounts"},
	}
}

func restrictiveSCC() *securityv1.SecurityContextConstraints {
	var exactUID int64 = 999
	return &securityv1.SecurityContextConstraints{
		ObjectMeta: metav1.ObjectMeta{
			Name: "restrictive",
		},
		RunAsUser: securityv1.RunAsUserStrategyOptions{
			Type: securityv1.RunAsUserStrategyMustRunAs,
			UID:  &exactUID,
		},
		SELinuxContext: securityv1.SELinuxContextStrategyOptions{
			Type: securityv1.SELinuxStrategyMustRunAs,
			SELinuxOptions: &corev1.SELinuxOptions{
				Level: "s9:z0,z1",
			},
		},
		FSGroup: securityv1.FSGroupStrategyOptions{
			Type: securityv1.FSGroupStrategyMustRunAs,
			Ranges: []securityv1.IDRange{
				{Min: 999, Max: 999},
			},
		},
		SupplementalGroups: securityv1.SupplementalGroupsStrategyOptions{
			Type: securityv1.SupplementalGroupsStrategyMustRunAs,
			Ranges: []securityv1.IDRange{
				{Min: 999, Max: 999},
			},
		},
		Groups: []string{"system:serviceaccounts"},
	}
}

func saSCC() *securityv1.SecurityContextConstraints {
	return &securityv1.SecurityContextConstraints{
		ObjectMeta: metav1.ObjectMeta{
			Name: "scc-sa",
		},
		RunAsUser: securityv1.RunAsUserStrategyOptions{
			Type: securityv1.RunAsUserStrategyMustRunAsRange,
		},
		SELinuxContext: securityv1.SELinuxContextStrategyOptions{
			Type: securityv1.SELinuxStrategyMustRunAs,
		},
		FSGroup: securityv1.FSGroupStrategyOptions{
			Type: securityv1.FSGroupStrategyMustRunAs,
		},
		SupplementalGroups: securityv1.SupplementalGroupsStrategyOptions{
			Type: securityv1.SupplementalGroupsStrategyMustRunAs,
		},
		Groups: []string{"system:serviceaccounts"},
	}
}

func saExactSCC() *securityv1.SecurityContextConstraints {
	var exactUID int64 = 999
	return &securityv1.SecurityContextConstraints{
		ObjectMeta: metav1.ObjectMeta{
			Name: "scc-sa-exact",
		},
		RunAsUser: securityv1.RunAsUserStrategyOptions{
			Type: securityv1.RunAsUserStrategyMustRunAs,
			UID:  &exactUID,
		},
		SELinuxContext: securityv1.SELinuxContextStrategyOptions{
			Type: securityv1.SELinuxStrategyMustRunAs,
			SELinuxOptions: &corev1.SELinuxOptions{
				Level: "s9:z0,z1",
			},
		},
		FSGroup: securityv1.FSGroupStrategyOptions{
			Type: securityv1.FSGroupStrategyMustRunAs,
			Ranges: []securityv1.IDRange{
				{Min: 999, Max: 999},
			},
		},
		SupplementalGroups: securityv1.SupplementalGroupsStrategyOptions{
			Type: securityv1.SupplementalGroupsStrategyMustRunAs,
			Ranges: []securityv1.IDRange{
				{Min: 999, Max: 999},
			},
		},
		Groups: []string{"system:serviceaccounts"},
	}
}

// goodPod is empty and should not be used directly for testing since we're providing
// two different SCCs.  Since no values are specified it would be allowed to match any
// SCC when defaults are filled in.
func goodPod() *coreapi.Pod {
	return &coreapi.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "default",
		},
		Spec: coreapi.PodSpec{
			ServiceAccountName: "default",
			SecurityContext:    &coreapi.PodSecurityContext{},
			Containers: []coreapi.Container{
				{
					SecurityContext: &coreapi.SecurityContext{},
				},
			},
		},
	}
}

func containerSC(seLinuxLevel *string, uid int64) *coreapi.SecurityContext {
	sc := &coreapi.SecurityContext{
		RunAsUser: &uid,
	}
	if seLinuxLevel != nil {
		sc.SELinuxOptions = &coreapi.SELinuxOptions{
			Level: *seLinuxLevel,
		}
	}
	return sc
}

func podSC(seLinuxLevel string, fsGroup, supGroup int64) *coreapi.PodSecurityContext {
	return &coreapi.PodSecurityContext{
		SELinuxOptions: &coreapi.SELinuxOptions{
			Level: seLinuxLevel,
		},
		SupplementalGroups: []int64{supGroup},
		FSGroup:            &fsGroup,
	}
}

func setupClientSet() *fake.Clientset {
	// create the annotated namespace and add it to the fake client
	namespace := createNamespaceForTest()
	serviceAccount := createSAForTest()
	serviceAccount.Namespace = namespace.Name

	return fake.NewSimpleClientset(namespace, serviceAccount)
}

func createSCCListerAndIndexer(t *testing.T, sccs []*securityv1.SecurityContextConstraints) (securityv1listers.SecurityContextConstraintsLister, cache.Indexer) {
	t.Helper()
	indexer := cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc})
	lister := securityv1listers.NewSecurityContextConstraintsLister(indexer)
	for _, scc := range sccs {
		if err := indexer.Add(scc); err != nil {
			t.Fatalf("error adding SCC to store: %v", err)
		}
	}
	return lister, indexer
}

func createSCCLister(t *testing.T, sccs []*securityv1.SecurityContextConstraints) securityv1listers.SecurityContextConstraintsLister {
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

func (s *sccTestAuthorizer) Authorize(ctx context.Context, a authorizer.Attributes) (authorizer.Decision, string, error) {
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
