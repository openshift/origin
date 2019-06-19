package podsecuritypolicyselfsubjectreview

import (
	"fmt"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apiserver/pkg/authentication/user"
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

func TestPodSecurityPolicySelfSubjectReview(t *testing.T) {
	testcases := map[string]struct {
		sccs  []*securityv1.SecurityContextConstraints
		check func(p *securityapi.PodSecurityPolicySelfSubjectReview) (bool, string)
	}{
		"user foo": {
			sccs: []*securityv1.SecurityContextConstraints{
				admissionttesting.UserScc("bar"),
				admissionttesting.UserScc("foo"),
			},
			check: func(p *securityapi.PodSecurityPolicySelfSubjectReview) (bool, string) {
				fmt.Printf("-> Is %q", p.Status.AllowedBy.Name)
				return p.Status.AllowedBy.Name == "foo", "SCC should be foo"
			},
		},
		"user bar ": {
			sccs: []*securityv1.SecurityContextConstraints{
				admissionttesting.UserScc("bar"),
			},
			check: func(p *securityapi.PodSecurityPolicySelfSubjectReview) (bool, string) {
				return p.Status.AllowedBy == nil, "Allowed by should be nil"
			},
		},
	}
	for testName, testcase := range testcases {
		namespace := admissionttesting.CreateNamespaceForTest()
		serviceAccount := admissionttesting.CreateSAForTest()
		reviewRequest := &securityapi.PodSecurityPolicySelfSubjectReview{
			Spec: securityapi.PodSecurityPolicySelfSubjectReviewSpec{
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
		ctx := apirequest.WithUser(apirequest.WithNamespace(apirequest.NewContext(), metav1.NamespaceAll), &user.DefaultInfo{Name: "foo", Groups: []string{"bar", "baz"}})
		obj, err := storage.Create(ctx, reviewRequest, rest.ValidateAllObjectFunc, &metav1.CreateOptions{})
		if err != nil {
			t.Errorf("%s - Unexpected error", testName)
		}
		pspssr, ok := obj.(*securityapi.PodSecurityPolicySelfSubjectReview)
		if !ok {
			t.Errorf("%s - Unable to convert created runtime.Object to PodSecurityPolicySelfSubjectReview", testName)
			continue
		}
		if ok, message := testcase.check(pspssr); !ok {
			t.Errorf("%s - %s", testName, message)
		}
	}
}

type noopTestAuthorizer struct{}

func (s *noopTestAuthorizer) Authorize(a authorizer.Attributes) (authorizer.Decision, string, error) {
	return authorizer.DecisionNoOpinion, "", nil
}
