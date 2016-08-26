package podsecuritypolicysubjectreview

import (
	"testing"

	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/client/cache"
	clientsetfake "k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset/fake"

	oscache "github.com/openshift/origin/pkg/client/cache"
	securityapi "github.com/openshift/origin/pkg/security/api"
	pspsubjectreviewtesting "github.com/openshift/origin/pkg/security/registry/podsecuritypolicysubjectreview/testing"
)

func saSCC() *kapi.SecurityContextConstraints {
	return &kapi.SecurityContextConstraints{
		ObjectMeta: kapi.ObjectMeta{
			SelfLink: "/api/version/securitycontextconstraints/scc-sa",
			Name:     "scc-sa",
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
}

func TestAllowed(t *testing.T) {
	testcases := map[string]struct {
		sccs []*kapi.SecurityContextConstraints
		// patch function modify nominal PodSecurityPolicySubjectReview request
		patch func(p *securityapi.PodSecurityPolicySubjectReview)
		check func(p *securityapi.PodSecurityPolicySubjectReview) (bool, string)
	}{
		"nominal case": {
			sccs: []*kapi.SecurityContextConstraints{
				pspsubjectreviewtesting.UserBarScc(),
				pspsubjectreviewtesting.UserFooScc(),
			},
			check: func(p *securityapi.PodSecurityPolicySubjectReview) (bool, string) {
				// must be different due defaulting
				return p.Status.Template.Spec.SecurityContext != nil, "Status.Template should be defaulted"
			},
		},
		// if PodTemplateSpec.Spec.ServiceAccountName is empty it will not be defaulted
		"empty service account name": {
			sccs: []*kapi.SecurityContextConstraints{
				pspsubjectreviewtesting.UserBarScc(),
				pspsubjectreviewtesting.UserFooScc(),
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
			sccs: []*kapi.SecurityContextConstraints{
				pspsubjectreviewtesting.UserBarScc(),
				pspsubjectreviewtesting.UserFooScc(),
			},
			patch: func(p *securityapi.PodSecurityPolicySubjectReview) {
				p.Spec.Groups = nil
			},
		},
		// If User and Groups are empty, then the check is performed using *only* the ServiceAccountName in the PodTemplateSpec.
		"no user - no group": {
			sccs: []*kapi.SecurityContextConstraints{
				pspsubjectreviewtesting.UserBarScc(),
				pspsubjectreviewtesting.UserFooScc(),
				saSCC(),
			},
			patch: func(p *securityapi.PodSecurityPolicySubjectReview) {
				p.Spec.Groups = nil
				p.Spec.User = ""
			},
		},
	}

	for testName, testcase := range testcases {
		namespace := pspsubjectreviewtesting.NewNamespace()
		serviceAccount := pspsubjectreviewtesting.NewSA()
		reviewRequest := &securityapi.PodSecurityPolicySubjectReview{
			Spec: securityapi.PodSecurityPolicySubjectReviewSpec{
				Template: kapi.PodTemplateSpec{
					Spec: kapi.PodSpec{
						Containers:         []kapi.Container{{Name: "ctr", Image: "image", ImagePullPolicy: "IfNotPresent"}},
						RestartPolicy:      kapi.RestartPolicyAlways,
						SecurityContext:    &kapi.PodSecurityContext{},
						DNSPolicy:          kapi.DNSClusterFirst,
						ServiceAccountName: "default",
					},
				},
				User:   "foo",
				Groups: []string{"bar", "baz"},
			},
		}
		if testcase.patch != nil {
			testcase.patch(reviewRequest) // local modification of the nominal case
		}

		cache := &oscache.IndexerToSecurityContextConstraintsLister{
			Indexer: cache.NewIndexer(cache.MetaNamespaceKeyFunc,
				cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc}),
		}
		for _, scc := range testcase.sccs {
			cache.Add(scc)
		}

		csf := clientsetfake.NewSimpleClientset(namespace, serviceAccount)
		storage := REST{cache, csf}
		ctx := kapi.WithNamespace(kapi.NewContext(), kapi.NamespaceAll)
		obj, err := storage.Create(ctx, reviewRequest)
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
