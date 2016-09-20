package podsecuritypolicyselfsubjectreview

import (
	"fmt"
	"testing"

	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/auth/user"
	"k8s.io/kubernetes/pkg/client/cache"
	clientsetfake "k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset/fake"

	oscache "github.com/openshift/origin/pkg/client/cache"
	admissionttesting "github.com/openshift/origin/pkg/security/admission/testing"
	securityapi "github.com/openshift/origin/pkg/security/api"
	oscc "github.com/openshift/origin/pkg/security/scc"
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
				admissionttesting.UserScc("bar"),
				admissionttesting.UserScc("foo"),
			},
			check: func(p *securityapi.PodSecurityPolicySelfSubjectReview) (bool, string) {
				fmt.Printf("-> Is %q", p.Status.AllowedBy.Name)
				return p.Status.AllowedBy.Name == "foo", "SCC should be foo"
			},
		},
		"user bar ": {
			sccs: []*kapi.SecurityContextConstraints{
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
			if err := cache.Add(scc); err != nil {
				t.Fatalf("error adding sccs to store: %v", err)
			}
		}

		csf := clientsetfake.NewSimpleClientset(namespace, serviceAccount)
		storage := REST{oscc.NewDefaultSCCMatcher(cache), csf}
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
