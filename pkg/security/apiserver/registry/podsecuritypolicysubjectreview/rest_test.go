package podsecuritypolicysubjectreview

import (
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apiserver/pkg/authorization/authorizer"
	apirequest "k8s.io/apiserver/pkg/endpoints/request"
	"k8s.io/apiserver/pkg/registry/rest"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/tools/cache"
	coreapi "k8s.io/kubernetes/pkg/apis/core"

	securityv1 "github.com/openshift/api/security/v1"
	securityv1listers "github.com/openshift/client-go/security/listers/security/v1"
	securityapi "github.com/openshift/origin/pkg/security/apis/security"
	admissionttesting "github.com/openshift/origin/pkg/security/apiserver/admission/testing"
	"k8s.io/kubernetes/openshift-kube-apiserver/admission/security/securitycontextconstraints/sccmatching"
)

func saSCC() *securityv1.SecurityContextConstraints {
	return &securityv1.SecurityContextConstraints{
		ObjectMeta: metav1.ObjectMeta{
			SelfLink: "/api/version/securitycontextconstraints/scc-sa",
			Name:     "scc-sa",
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

func TestAllowed(t *testing.T) {
	testcases := map[string]struct {
		sccs []*securityv1.SecurityContextConstraints
		// patch function modify nominal PodSecurityPolicySubjectReview request
		patch func(p *securityapi.PodSecurityPolicySubjectReview)
		check func(p *securityapi.PodSecurityPolicySubjectReview) (bool, string)
	}{
		"nominal case": {
			sccs: []*securityv1.SecurityContextConstraints{
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
			sccs: []*securityv1.SecurityContextConstraints{
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
			sccs: []*securityv1.SecurityContextConstraints{
				admissionttesting.UserScc("bar"),
				admissionttesting.UserScc("foo"),
			},
			patch: func(p *securityapi.PodSecurityPolicySubjectReview) {
				p.Spec.Groups = nil
			},
		},
		// If User and Groups are empty, then the check is performed using *only* the ServiceAccountName in the PodTemplateSpec.
		"no user - no group": {
			sccs: []*securityv1.SecurityContextConstraints{
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
				Template: coreapi.PodTemplateSpec{
					Spec: coreapi.PodSpec{
						Containers: []coreapi.Container{
							{
								Name:                     "ctr",
								Image:                    "image",
								ImagePullPolicy:          "IfNotPresent",
								TerminationMessagePolicy: coreapi.TerminationMessageReadFile,
							},
						},
						RestartPolicy:      coreapi.RestartPolicyAlways,
						SecurityContext:    &coreapi.PodSecurityContext{},
						DNSPolicy:          coreapi.DNSClusterFirst,
						ServiceAccountName: "default",
						SchedulerName:      coreapi.DefaultSchedulerName,
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
		sccCache := securityv1listers.NewSecurityContextConstraintsLister(sccIndexer)

		for _, scc := range testcase.sccs {
			if err := sccIndexer.Add(scc); err != nil {
				t.Fatalf("error adding sccs to store: %v", err)
			}
		}

		csf := fake.NewSimpleClientset(namespace, serviceAccount)
		storage := REST{sccmatching.NewDefaultSCCMatcher(sccCache, &noopTestAuthorizer{}), csf}
		ctx := apirequest.WithNamespace(apirequest.NewContext(), metav1.NamespaceAll)
		obj, err := storage.Create(ctx, reviewRequest, rest.ValidateAllObjectFunc, &metav1.CreateOptions{})
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
		sccs           []*securityv1.SecurityContextConstraints
		serviceAccount *coreapi.ServiceAccount
		errorMessage   string
	}{
		"invalid request": {
			request: &securityapi.PodSecurityPolicySubjectReview{
				Spec: securityapi.PodSecurityPolicySubjectReviewSpec{
					Template: coreapi.PodTemplateSpec{
						Spec: coreapi.PodSpec{
							Containers: []coreapi.Container{
								{
									Name:                     "ctr",
									Image:                    "image",
									ImagePullPolicy:          "IfNotPresent",
									TerminationMessagePolicy: coreapi.TerminationMessageReadFile,
								},
							},
							RestartPolicy:      coreapi.RestartPolicyAlways,
							SecurityContext:    &coreapi.PodSecurityContext{},
							DNSPolicy:          coreapi.DNSClusterFirst,
							ServiceAccountName: "A.B.C.D",
							SchedulerName:      coreapi.DefaultSchedulerName,
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
					Template: coreapi.PodTemplateSpec{
						Spec: coreapi.PodSpec{
							Containers: []coreapi.Container{
								{
									Name:                     "ctr",
									Image:                    "image",
									ImagePullPolicy:          "IfNotPresent",
									TerminationMessagePolicy: coreapi.TerminationMessageReadFile,
								},
							},
							RestartPolicy:      coreapi.RestartPolicyAlways,
							SecurityContext:    &coreapi.PodSecurityContext{},
							DNSPolicy:          coreapi.DNSClusterFirst,
							ServiceAccountName: "default",
							SchedulerName:      coreapi.DefaultSchedulerName,
						},
					},
				},
			},
			// no errorMessage only pspr empty
		},
		"container capability": {
			request: &securityapi.PodSecurityPolicySubjectReview{
				Spec: securityapi.PodSecurityPolicySubjectReviewSpec{
					Template: coreapi.PodTemplateSpec{
						Spec: coreapi.PodSpec{
							Containers: []coreapi.Container{
								{
									Name:            "ctr",
									Image:           "image",
									ImagePullPolicy: "IfNotPresent",
									SecurityContext: &coreapi.SecurityContext{
										Capabilities: &coreapi.Capabilities{
											Add: []coreapi.Capability{"foo"},
										},
									},
									TerminationMessagePolicy: coreapi.TerminationMessageReadFile,
								},
							},
							RestartPolicy:      coreapi.RestartPolicyAlways,
							SecurityContext:    &coreapi.PodSecurityContext{},
							DNSPolicy:          coreapi.DNSClusterFirst,
							ServiceAccountName: "default",
							SchedulerName:      coreapi.DefaultSchedulerName,
						},
					},
					User: "foo",
				},
			},
			sccs: []*securityv1.SecurityContextConstraints{
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
		sccCache := securityv1listers.NewSecurityContextConstraintsLister(sccIndexer)
		for _, scc := range testcase.sccs {
			if err := sccIndexer.Add(scc); err != nil {
				t.Fatalf("error adding sccs to store: %v", err)
			}
		}
		csf := fake.NewSimpleClientset(namespace, serviceAccount)
		storage := REST{sccmatching.NewDefaultSCCMatcher(sccCache, &noopTestAuthorizer{}), csf}
		ctx := apirequest.WithNamespace(apirequest.NewContext(), metav1.NamespaceAll)
		_, err := storage.Create(ctx, testcase.request, rest.ValidateAllObjectFunc, &metav1.CreateOptions{})
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

type noopTestAuthorizer struct{}

func (s *noopTestAuthorizer) Authorize(a authorizer.Attributes) (authorizer.Decision, string, error) {
	return authorizer.DecisionNoOpinion, "", nil
}
