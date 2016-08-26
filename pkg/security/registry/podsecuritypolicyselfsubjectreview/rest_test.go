package podsecuritypolicyselfsubjectreview

import (
	"testing"

	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/auth/user"
	"k8s.io/kubernetes/pkg/client/cache"
	clientsetfake "k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset/fake"

	oscache "github.com/openshift/origin/pkg/client/cache"
	securityapi "github.com/openshift/origin/pkg/security/api"
	pspsubjectreviewtesting "github.com/openshift/origin/pkg/security/registry/podsecuritypolicysubjectreview/testing"
)

func validPodTemplateSpec() kapi.PodTemplateSpec {
	activeDeadlineSeconds := int64(1)
	return kapi.PodTemplateSpec{
		Spec: kapi.PodSpec{
			Volumes: []kapi.Volume{
				{Name: "vol", VolumeSource: kapi.VolumeSource{EmptyDir: &kapi.EmptyDirVolumeSource{}}},
			},
			Containers:    []kapi.Container{{Name: "ctr", Image: "image", ImagePullPolicy: "IfNotPresent"}},
			RestartPolicy: kapi.RestartPolicyAlways,
			NodeSelector: map[string]string{
				"key": "value",
			},
			NodeName:              "foobar",
			DNSPolicy:             kapi.DNSClusterFirst,
			ActiveDeadlineSeconds: &activeDeadlineSeconds,
			ServiceAccountName:    "acct",
		},
	}
}

func TestPodSecurityPolicySelfSubjectReview(t *testing.T) {
	testcases := map[string]struct {
		sccs  []*kapi.SecurityContextConstraints
		check func(p *securityapi.PodSecurityPolicySelfSubjectReview) (bool, string)
	}{
		"user foo": {
			sccs: []*kapi.SecurityContextConstraints{
				pspsubjectreviewtesting.UserBarScc(),
				pspsubjectreviewtesting.UserFooScc(),
			},
			check: func(p *securityapi.PodSecurityPolicySelfSubjectReview) (bool, string) {
				return p.Status.AllowedBy.Name == "foo", "SCC should be foo"
			},
		},
		"user bar ": {
			sccs: []*kapi.SecurityContextConstraints{
				pspsubjectreviewtesting.UserBarScc(),
			},
			check: func(p *securityapi.PodSecurityPolicySelfSubjectReview) (bool, string) {
				return p.Status.AllowedBy == nil, "Allowed by should be nil"
			},
		},
	}
	for testName, testcase := range testcases {
		namespace := pspsubjectreviewtesting.NewNamespace()
		serviceAccount := pspsubjectreviewtesting.NewSA()
		reviewRequest := &securityapi.PodSecurityPolicySelfSubjectReview{
			Spec: securityapi.PodSecurityPolicySelfSubjectReviewSpec{
				Template: kapi.PodTemplateSpec{
					Spec: kapi.PodSpec{
						Containers:         []kapi.Container{{Name: "ctr", Image: "image", ImagePullPolicy: "IfNotPresent"}},
						RestartPolicy:      kapi.RestartPolicyAlways,
						SecurityContext:    &kapi.PodSecurityContext{},
						DNSPolicy:          kapi.DNSClusterFirst,
						ServiceAccountName: "default",
					},
				},
			},
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
		ctx := kapi.WithUser(kapi.WithNamespace(kapi.NewContext(), kapi.NamespaceAll), &user.DefaultInfo{Name: "foo", Groups: []string{"bar", "baz"}})
		obj, err := storage.Create(ctx, reviewRequest)
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
