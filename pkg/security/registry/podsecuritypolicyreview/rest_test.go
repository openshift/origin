package podsecuritypolicyreview

import (
	"reflect"
	"testing"

	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/client/cache"
	clientsetfake "k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset/fake"

	oscache "github.com/openshift/origin/pkg/client/cache"
	securityapi "github.com/openshift/origin/pkg/security/api"
	pspsubjectreviewtesting "github.com/openshift/origin/pkg/security/registry/podsecuritypolicysubjectreview/testing"
)

func TestNoErrors(t *testing.T) {
	testcases := map[string]struct {
		request    *securityapi.PodSecurityPolicyReview
		sccs       []*kapi.SecurityContextConstraints
		allowedSAs []string
	}{
		"default in pod": {
			request: &securityapi.PodSecurityPolicyReview{
				Spec: securityapi.PodSecurityPolicyReviewSpec{
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
			},
			sccs: []*kapi.SecurityContextConstraints{
				{
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
				},
			},
			allowedSAs: []string{"default"},
		},
	}
	for testName, testcase := range testcases {
		namespace := pspsubjectreviewtesting.NewNamespace()
		serviceAccount := pspsubjectreviewtesting.NewSA()
		csf := clientsetfake.NewSimpleClientset(namespace, serviceAccount)
		cache := &oscache.IndexerToSecurityContextConstraintsLister{
			Indexer: cache.NewIndexer(cache.MetaNamespaceKeyFunc,
				cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc}),
		}
		for _, scc := range testcase.sccs {
			cache.Add(scc)
		}
		storage := REST{sccLister: cache, client: csf}
		ctx := kapi.WithNamespace(kapi.NewContext(), kapi.NamespaceAll)
		obj, err := storage.Create(ctx, testcase.request)
		if err != nil {
			t.Errorf("%s - Unexpected error: %v", testName, err)
			continue
		}
		pspsr, ok := obj.(*securityapi.PodSecurityPolicyReview)
		if !ok {
			t.Errorf("%s - unable to convert cretated runtime.Object to PodSecurityPolicyReview", testName)
			continue
		}
		var allowedSas []string
		for _, sa := range pspsr.Status.AllowedServiceAccounts {
			allowedSas = append(allowedSas, sa.Name)
		}
		if !reflect.DeepEqual(allowedSas, testcase.allowedSAs) {
			t.Errorf("%s - expected allowed ServiceAccout names %v got %v", testName, testcase.allowedSAs, allowedSas)
		}
	}
}
