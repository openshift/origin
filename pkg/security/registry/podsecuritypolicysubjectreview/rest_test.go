package podsecuritypolicysubjectreview

import (
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	apirequest "k8s.io/apiserver/pkg/endpoints/request"
	"k8s.io/apiserver/pkg/registry/rest"
	"k8s.io/client-go/tools/cache"
	kapi "k8s.io/kubernetes/pkg/apis/core"
	clientsetfake "k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset/fake"

	admissionttesting "github.com/openshift/origin/pkg/security/admission/testing"
	securityapi "github.com/openshift/origin/pkg/security/apis/security"
	securitylisters "github.com/openshift/origin/pkg/security/generated/listers/security/internalversion"
	oscc "github.com/openshift/origin/pkg/security/securitycontextconstraints"

	_ "github.com/openshift/origin/pkg/api/install"
)

func saSCC() *securityapi.SecurityContextConstraints {
	return &securityapi.SecurityContextConstraints{
		ObjectMeta: metav1.ObjectMeta{
			SelfLink: "/api/version/securitycontextconstraints/scc-sa",
			Name:     "scc-sa",
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

func TestAllowed(t *testing.T) {
	testcases := map[string]struct {
		sccs []*securityapi.SecurityContextConstraints
		// patch function modify nominal PodSecurityPolicySubjectReview request
		patch func(p *securityapi.PodSecurityPolicySubjectReview)
		check func(p *securityapi.PodSecurityPolicySubjectReview) (bool, string)
	}{
		"nominal case": {
			sccs: []*securityapi.SecurityContextConstraints{
				admissionttesting.UserScc("bar"),
				admissionttesting.UserScc("foo"),
			},
			check: func(p *securityapi.PodSecurityPolicySubjectReview) (bool, string) {
				// must be different due defaulting
				return p.Status.Template.Spec.SecurityContext != nil, "Status.Template should be defaulted"
			},
		},
		// if PodTemplateSpec.Spec.ServiceAccountName is empty it will not be defaulted
		"empty service account name": {
			sccs: []*securityapi.SecurityContextConstraints{
				admissionttesting.UserScc("bar"),
				admissionttesting.UserScc("foo"),
			},
			patch: func(p *securityapi.PodSecurityPolicySubjectReview) {
				p.Spec.Template.Spec.ServiceAccountName = "" // empty SA in podSpec
			},

			check: func(p *securityapi.PodSecurityPolicySubjectReview) (bool, string) {
				return p.Status.Template.Spec.SecurityContext == nil, "Status.PodTemplateSpec should not be defaulted"
			},
		},
		// If you specify "User" but not "Group", then is it interpreted as "What if User were not a member of any groups.
		"user - no group": {
			sccs: []*securityapi.SecurityContextConstraints{
				admissionttesting.UserScc("bar"),
				admissionttesting.UserScc("foo"),
			},
			patch: func(p *securityapi.PodSecurityPolicySubjectReview) {
				p.Spec.Groups = nil
			},
		},
		// If User and Groups are empty, then the check is performed using *only* the ServiceAccountName in the PodTemplateSpec.
		"no user - no group": {
			sccs: []*securityapi.SecurityContextConstraints{
				admissionttesting.UserScc("bar"),
				admissionttesting.UserScc("foo"),
				saSCC(),
			},
			patch: func(p *securityapi.PodSecurityPolicySubjectReview) {
				p.Spec.Groups = nil
				p.Spec.User = ""
			},
		},
	}

	namespace := admissionttesting.CreateNamespaceForTest()
	for testName, testcase := range testcases {
		serviceAccount := admissionttesting.CreateSAForTest()
		reviewRequest := &securityapi.PodSecurityPolicySubjectReview{
			Spec: securityapi.PodSecurityPolicySubjectReviewSpec{
				Template: kapi.PodTemplateSpec{
					Spec: kapi.PodSpec{
						Containers: []kapi.Container{
							{
								Name:                     "ctr",
								Image:                    "image",
								ImagePullPolicy:          "IfNotPresent",
								TerminationMessagePolicy: kapi.TerminationMessageReadFile,
							},
						},
						RestartPolicy:      kapi.RestartPolicyAlways,
						SecurityContext:    &kapi.PodSecurityContext{},
						DNSPolicy:          kapi.DNSClusterFirst,
						ServiceAccountName: "default",
						SchedulerName:      kapi.DefaultSchedulerName,
					},
				},
				User:   "foo",
				Groups: []string{"bar", "baz"},
			},
		}
		if testcase.patch != nil {
			testcase.patch(reviewRequest) // local modification of the nominal case
		}

		sccIndexer := cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc})
		sccCache := securitylisters.NewSecurityContextConstraintsLister(sccIndexer)

		for _, scc := range testcase.sccs {
			if err := sccIndexer.Add(scc); err != nil {
				t.Fatalf("error adding sccs to store: %v", err)
			}
		}

		csf := clientsetfake.NewSimpleClientset(namespace, serviceAccount)
		storage := REST{oscc.NewDefaultSCCMatcher(sccCache), csf}
		ctx := apirequest.WithNamespace(apirequest.NewContext(), metav1.NamespaceAll)
		obj, err := storage.Create(ctx, reviewRequest, rest.ValidateAllObjectFunc, false)
		if err != nil {
			t.Errorf("%s - Unexpected error: %v", testName, err)
			continue
		}
		pspsr, ok := obj.(*securityapi.PodSecurityPolicySubjectReview)
		if !ok {
			t.Errorf("%s - Unable to convert created runtime.Object to PodSecurityPolicySubjectReview", testName)
			continue
		}

		if testcase.check != nil {
			if ok, message := testcase.check(pspsr); !ok {
				t.Errorf("testcase '%s' is failing: %s", testName, message)
			}
		}
		if pspsr.Status.AllowedBy == nil {
			t.Errorf("testcase '%s' is failing AllowedBy shoult be not nil\n", testName)
		}
	}
}

func TestRequests(t *testing.T) {
	testcases := map[string]struct {
		request        *securityapi.PodSecurityPolicySubjectReview
		sccs           []*securityapi.SecurityContextConstraints
		serviceAccount *kapi.ServiceAccount
		errorMessage   string
	}{
		"invalid request": {
			request: &securityapi.PodSecurityPolicySubjectReview{
				Spec: securityapi.PodSecurityPolicySubjectReviewSpec{
					Template: kapi.PodTemplateSpec{
						Spec: kapi.PodSpec{
							Containers: []kapi.Container{
								{
									Name:                     "ctr",
									Image:                    "image",
									ImagePullPolicy:          "IfNotPresent",
									TerminationMessagePolicy: kapi.TerminationMessageReadFile,
								},
							},
							RestartPolicy:      kapi.RestartPolicyAlways,
							SecurityContext:    &kapi.PodSecurityContext{},
							DNSPolicy:          kapi.DNSClusterFirst,
							ServiceAccountName: "A.B.C.D",
							SchedulerName:      kapi.DefaultSchedulerName,
						},
					},
					User:   "foo",
					Groups: []string{"bar", "baz"},
				},
			},
			errorMessage: `PodSecurityPolicySubjectReview "" is invalid: spec.template.spec.serviceAccountName: Invalid value: "A.B.C.D": a DNS-1123 subdomain must consist of lower case alphanumeric characters, '-' or '.', and must start and end with an alphanumeric character (e.g. 'example.com', regex used for validation is '[a-z0-9]([-a-z0-9]*[a-z0-9])?(\.[a-z0-9]([-a-z0-9]*[a-z0-9])?)*')`,
		},
		"no provider": {
			request: &securityapi.PodSecurityPolicySubjectReview{
				Spec: securityapi.PodSecurityPolicySubjectReviewSpec{
					Template: kapi.PodTemplateSpec{
						Spec: kapi.PodSpec{
							Containers: []kapi.Container{
								{
									Name:                     "ctr",
									Image:                    "image",
									ImagePullPolicy:          "IfNotPresent",
									TerminationMessagePolicy: kapi.TerminationMessageReadFile,
								},
							},
							RestartPolicy:      kapi.RestartPolicyAlways,
							SecurityContext:    &kapi.PodSecurityContext{},
							DNSPolicy:          kapi.DNSClusterFirst,
							ServiceAccountName: "default",
							SchedulerName:      kapi.DefaultSchedulerName,
						},
					},
				},
			},
			// no errorMessage only pspr empty
		},
		"container capability": {
			request: &securityapi.PodSecurityPolicySubjectReview{
				Spec: securityapi.PodSecurityPolicySubjectReviewSpec{
					Template: kapi.PodTemplateSpec{
						Spec: kapi.PodSpec{
							Containers: []kapi.Container{
								{
									Name:            "ctr",
									Image:           "image",
									ImagePullPolicy: "IfNotPresent",
									SecurityContext: &kapi.SecurityContext{
										Capabilities: &kapi.Capabilities{
											Add: []kapi.Capability{"foo"},
										},
									},
									TerminationMessagePolicy: kapi.TerminationMessageReadFile,
								},
							},
							RestartPolicy:      kapi.RestartPolicyAlways,
							SecurityContext:    &kapi.PodSecurityContext{},
							DNSPolicy:          kapi.DNSClusterFirst,
							ServiceAccountName: "default",
							SchedulerName:      kapi.DefaultSchedulerName,
						},
					},
					User: "foo",
				},
			},
			sccs: []*securityapi.SecurityContextConstraints{
				admissionttesting.UserScc("bar"),
				admissionttesting.UserScc("foo"),
			},
			// no errorMessage
		},
	}
	namespace := admissionttesting.CreateNamespaceForTest()
	serviceAccount := admissionttesting.CreateSAForTest()
	for testName, testcase := range testcases {
		sccIndexer := cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc})
		sccCache := securitylisters.NewSecurityContextConstraintsLister(sccIndexer)
		for _, scc := range testcase.sccs {
			if err := sccIndexer.Add(scc); err != nil {
				t.Fatalf("error adding sccs to store: %v", err)
			}
		}
		csf := clientsetfake.NewSimpleClientset(namespace, serviceAccount)
		storage := REST{oscc.NewDefaultSCCMatcher(sccCache), csf}
		ctx := apirequest.WithNamespace(apirequest.NewContext(), metav1.NamespaceAll)
		_, err := storage.Create(ctx, testcase.request, rest.ValidateAllObjectFunc, false)
		switch {
		case err == nil && len(testcase.errorMessage) == 0:
			continue
		case err == nil && len(testcase.errorMessage) > 0:
			t.Errorf("%s - Expected error %q. No error found", testName, testcase.errorMessage)
			continue
		case err.Error() != testcase.errorMessage:
			t.Errorf("%s - Expected error %q. But got %q", testName, testcase.errorMessage, err.Error())
		}
	}

}
