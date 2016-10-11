package podsecuritypolicyreview

import (
	"reflect"
	"testing"

	kapi "k8s.io/kubernetes/pkg/api"
	cache "k8s.io/kubernetes/pkg/client/cache"
	clientsetfake "k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset/fake"

	oscache "github.com/openshift/origin/pkg/client/cache"
	admissionttesting "github.com/openshift/origin/pkg/security/admission/testing"
	securityapi "github.com/openshift/origin/pkg/security/api"
	oscc "github.com/openshift/origin/pkg/security/scc"
)

func TestNoErrors(t *testing.T) {
	var uid int64 = 999
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
		"failure creating provider": {
			request: &securityapi.PodSecurityPolicyReview{
				Spec: securityapi.PodSecurityPolicyReviewSpec{
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
								},
							},
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
						SelfLink: "/api/version/securitycontextconstraints/restrictive",
						Name:     "restrictive",
					},
					RunAsUser: kapi.RunAsUserStrategyOptions{
						Type: kapi.RunAsUserStrategyMustRunAs,
						UID:  &uid,
					},
					SELinuxContext: kapi.SELinuxContextStrategyOptions{
						Type: kapi.SELinuxStrategyMustRunAs,
						SELinuxOptions: &kapi.SELinuxOptions{
							Level: "s9:z0,z1",
						},
					},
					FSGroup: kapi.FSGroupStrategyOptions{
						Type: kapi.FSGroupStrategyMustRunAs,
						Ranges: []kapi.IDRange{
							{Min: 999, Max: 999},
						},
					},
					SupplementalGroups: kapi.SupplementalGroupsStrategyOptions{
						Type: kapi.SupplementalGroupsStrategyMustRunAs,
						Ranges: []kapi.IDRange{
							{Min: 999, Max: 999},
						},
					},
					Groups: []string{"system:serviceaccounts"},
				},
			},
			allowedSAs: nil,
		},
	}

	for testName, testcase := range testcases {
		sccCache := &oscache.IndexerToSecurityContextConstraintsLister{
			Indexer: cache.NewIndexer(cache.MetaNamespaceKeyFunc,
				cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc}),
		}
		for _, scc := range testcase.sccs {
			if err := sccCache.Add(scc); err != nil {
				t.Fatalf("error adding sccs to store: %v", err)
			}
		}
		saCache := oscache.StoreToServiceAccountLister{
			Indexer: cache.NewIndexer(cache.MetaNamespaceKeyFunc,
				cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc}),
		}
		namespace := admissionttesting.CreateNamespaceForTest()
		serviceAccount := admissionttesting.CreateSAForTest()
		serviceAccount.Namespace = namespace.Name
		saCache.Add(serviceAccount)
		csf := clientsetfake.NewSimpleClientset(namespace)
		storage := REST{oscc.NewDefaultSCCMatcher(sccCache), saCache, csf}
		ctx := kapi.WithNamespace(kapi.NewContext(), namespace.Name)
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

func TestErrors(t *testing.T) {
	testcases := map[string]struct {
		request        *securityapi.PodSecurityPolicyReview
		sccs           []*kapi.SecurityContextConstraints
		serviceAccount *kapi.ServiceAccount
		errorMessage   string
	}{
		"invalid PSPR": {
			request: &securityapi.PodSecurityPolicyReview{
				Spec: securityapi.PodSecurityPolicyReviewSpec{
					Template: kapi.PodTemplateSpec{
						Spec: kapi.PodSpec{
							Containers:         []kapi.Container{{Name: "ctr", Image: "image", ImagePullPolicy: "IfNotPresent"}},
							RestartPolicy:      kapi.RestartPolicyAlways,
							SecurityContext:    &kapi.PodSecurityContext{},
							DNSPolicy:          kapi.DNSClusterFirst,
							ServiceAccountName: "A.B.C.D.E",
						},
					},
				},
			},
			serviceAccount: admissionttesting.CreateSAForTest(),
			errorMessage:   `PodSecurityPolicyReview "" is invalid: spec.template.spec.serviceAccountName: Invalid value: "A.B.C.D.E": must match the regex [a-z0-9]([-a-z0-9]*[a-z0-9])?(\.[a-z0-9]([-a-z0-9]*[a-z0-9])?)* (e.g. 'example.com')`,
		},
		"no SA": {
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
			errorMessage: `unable to retrieve ServiceAccount default: serviceaccount "default" not found`,
		},
	}
	for testName, testcase := range testcases {
		sccCache := &oscache.IndexerToSecurityContextConstraintsLister{
			Indexer: cache.NewIndexer(cache.MetaNamespaceKeyFunc,
				cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc}),
		}
		for _, scc := range testcase.sccs {
			if err := sccCache.Add(scc); err != nil {
				t.Fatalf("error adding sccs to store: %v", err)
			}
		}
		saCache := oscache.StoreToServiceAccountLister{
			Indexer: cache.NewIndexer(cache.MetaNamespaceKeyFunc,
				cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc}),
		}
		namespace := admissionttesting.CreateNamespaceForTest()
		serviceAccount := admissionttesting.CreateSAForTest()
		if testcase.serviceAccount != nil {
			serviceAccount.Namespace = namespace.Name
			saCache.Add(serviceAccount)
		}
		csf := clientsetfake.NewSimpleClientset(namespace)

		storage := REST{oscc.NewDefaultSCCMatcher(sccCache), saCache, csf}
		ctx := kapi.WithNamespace(kapi.NewContext(), namespace.Name)
		_, err := storage.Create(ctx, testcase.request)
		if err == nil {
			t.Errorf("%s - Expected error", testName)
			continue
		}
		if err.Error() != testcase.errorMessage {
			t.Errorf("%s - Bad error. Expected %q got %q", testName, testcase.errorMessage, err.Error())
		}
	}
}

func TestSpecificSAs(t *testing.T) {
	testcases := map[string]struct {
		request         *securityapi.PodSecurityPolicyReview
		sccs            []*kapi.SecurityContextConstraints
		errorMessage    string
		serviceAccounts []*kapi.ServiceAccount
	}{
		"SAs in PSPR": {
			request: &securityapi.PodSecurityPolicyReview{
				Spec: securityapi.PodSecurityPolicyReviewSpec{
					Template: kapi.PodTemplateSpec{
						Spec: kapi.PodSpec{
							Containers: []kapi.Container{
								{
									Name:            "ctr",
									Image:           "image",
									ImagePullPolicy: "IfNotPresent",
								},
							},
							RestartPolicy:      kapi.RestartPolicyAlways,
							SecurityContext:    &kapi.PodSecurityContext{},
							DNSPolicy:          kapi.DNSClusterFirst,
							ServiceAccountName: "default",
						},
					},
					ServiceAccountNames: []string{"my-sa", "yours-sa"},
				},
			},
			sccs: []*kapi.SecurityContextConstraints{
				{
					ObjectMeta: kapi.ObjectMeta{
						SelfLink: "/api/version/securitycontextconstraints/myscc",
						Name:     "myscc",
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
			serviceAccounts: []*kapi.ServiceAccount{
				{
					ObjectMeta: kapi.ObjectMeta{
						Name:      "my-sa",
						Namespace: "default",
					},
				},
				{
					ObjectMeta: kapi.ObjectMeta{
						Name:      "yours-sa",
						Namespace: "default",
					},
				},
				{
					ObjectMeta: kapi.ObjectMeta{
						Name:      "our-sa",
						Namespace: "default",
					},
				},
			},
			errorMessage: "",
		},
		"bad SAs in PSPR": {
			request: &securityapi.PodSecurityPolicyReview{
				Spec: securityapi.PodSecurityPolicyReviewSpec{
					Template: kapi.PodTemplateSpec{
						Spec: kapi.PodSpec{
							Containers: []kapi.Container{
								{
									Name:            "ctr",
									Image:           "image",
									ImagePullPolicy: "IfNotPresent",
								},
							},
							RestartPolicy:      kapi.RestartPolicyAlways,
							SecurityContext:    &kapi.PodSecurityContext{},
							DNSPolicy:          kapi.DNSClusterFirst,
							ServiceAccountName: "default",
						},
					},
					ServiceAccountNames: []string{"bad-sa"},
				},
			},
			sccs: []*kapi.SecurityContextConstraints{
				{
					ObjectMeta: kapi.ObjectMeta{
						SelfLink: "/api/version/securitycontextconstraints/myscc",
						Name:     "myscc",
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
			serviceAccounts: []*kapi.ServiceAccount{
				{
					ObjectMeta: kapi.ObjectMeta{
						Name:      "my-sa",
						Namespace: "default",
					},
				},
			},
			errorMessage: `unable to retrieve ServiceAccount bad-sa: serviceaccount "bad-sa" not found`,
		},
	}

	for testName, testcase := range testcases {
		sccCache := &oscache.IndexerToSecurityContextConstraintsLister{
			Indexer: cache.NewIndexer(cache.MetaNamespaceKeyFunc,
				cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc}),
		}
		for _, scc := range testcase.sccs {
			if err := sccCache.Add(scc); err != nil {
				t.Fatalf("error adding sccs to store: %v", err)
			}
		}
		namespace := admissionttesting.CreateNamespaceForTest()
		saCache := oscache.StoreToServiceAccountLister{
			Indexer: cache.NewIndexer(cache.MetaNamespaceKeyFunc,
				cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc}),
		}
		for i := range testcase.serviceAccounts {
			saCache.Add(testcase.serviceAccounts[i])
		}
		csf := clientsetfake.NewSimpleClientset(namespace)
		storage := REST{oscc.NewDefaultSCCMatcher(sccCache), saCache, csf}
		ctx := kapi.WithNamespace(kapi.NewContext(), namespace.Name)
		_, err := storage.Create(ctx, testcase.request)
		switch {
		case err == nil && len(testcase.errorMessage) == 0:
			continue
		case err == nil && len(testcase.errorMessage) > 0:
			t.Errorf("%s - Expected error %q. No error found", testName, testcase.errorMessage)
			continue
		case err.Error() != testcase.errorMessage:
			t.Errorf("%s - Expected error %q. But got %#v", testName, testcase.errorMessage, err)
		}
	}
}
