package podsecuritypolicyreview

import (
	"reflect"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	apirequest "k8s.io/apiserver/pkg/endpoints/request"
	cache "k8s.io/client-go/tools/cache"
	kapi "k8s.io/kubernetes/pkg/api"
	clientsetfake "k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset/fake"
	"k8s.io/kubernetes/pkg/client/listers/core/internalversion"

	admissionttesting "github.com/openshift/origin/pkg/security/admission/testing"
	securityapi "github.com/openshift/origin/pkg/security/apis/security"
	securitylisters "github.com/openshift/origin/pkg/security/generated/listers/security/internalversion"
	oscc "github.com/openshift/origin/pkg/security/scc"

	_ "github.com/openshift/origin/pkg/api/install"
)

func TestNoErrors(t *testing.T) {
	var uid int64 = 999
	testcases := map[string]struct {
		request    *securityapi.PodSecurityPolicyReview
		sccs       []*securityapi.SecurityContextConstraints
		allowedSAs []string
	}{
		"default in pod": {
			request: &securityapi.PodSecurityPolicyReview{
				Spec: securityapi.PodSecurityPolicyReviewSpec{
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
			sccs: []*securityapi.SecurityContextConstraints{
				{
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
			sccs: []*securityapi.SecurityContextConstraints{
				{
					ObjectMeta: metav1.ObjectMeta{
						SelfLink: "/api/version/securitycontextconstraints/restrictive",
						Name:     "restrictive",
					},
					RunAsUser: securityapi.RunAsUserStrategyOptions{
						Type: securityapi.RunAsUserStrategyMustRunAs,
						UID:  &uid,
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
				},
			},
			allowedSAs: nil,
		},
	}

	for testName, testcase := range testcases {
		sccIndexer := cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc})
		sccCache := securitylisters.NewSecurityContextConstraintsLister(sccIndexer)
		for _, scc := range testcase.sccs {
			if err := sccIndexer.Add(scc); err != nil {
				t.Fatalf("error adding sccs to store: %v", err)
			}
		}
		saIndexer := cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc})
		saCache := internalversion.NewServiceAccountLister(saIndexer)
		namespace := admissionttesting.CreateNamespaceForTest()
		serviceAccount := admissionttesting.CreateSAForTest()
		serviceAccount.Namespace = namespace.Name
		saIndexer.Add(serviceAccount)
		csf := clientsetfake.NewSimpleClientset(namespace)
		storage := REST{oscc.NewDefaultSCCMatcher(sccCache), saCache, csf}
		ctx := apirequest.WithNamespace(apirequest.NewContext(), namespace.Name)
		obj, err := storage.Create(ctx, testcase.request, false)
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
		sccs           []*securityapi.SecurityContextConstraints
		serviceAccount *kapi.ServiceAccount
		errorMessage   string
	}{
		"invalid PSPR": {
			request: &securityapi.PodSecurityPolicyReview{
				Spec: securityapi.PodSecurityPolicyReviewSpec{
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
							ServiceAccountName: "A.B.C.D.E",
							SchedulerName:      kapi.DefaultSchedulerName,
						},
					},
				},
			},
			serviceAccount: admissionttesting.CreateSAForTest(),
			errorMessage:   `PodSecurityPolicyReview "" is invalid: spec.template.spec.serviceAccountName: Invalid value: "A.B.C.D.E": a DNS-1123 subdomain must consist of lower case alphanumeric characters, '-' or '.', and must start and end with an alphanumeric character (e.g. 'example.com', regex used for validation is '[a-z0-9]([-a-z0-9]*[a-z0-9])?(\.[a-z0-9]([-a-z0-9]*[a-z0-9])?)*')`,
		},
		"no SA": {
			request: &securityapi.PodSecurityPolicyReview{
				Spec: securityapi.PodSecurityPolicyReviewSpec{
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
			errorMessage: `unable to retrieve ServiceAccount default: serviceaccount "default" not found`,
		},
	}
	for testName, testcase := range testcases {
		sccIndexer := cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc})
		sccCache := securitylisters.NewSecurityContextConstraintsLister(sccIndexer)
		for _, scc := range testcase.sccs {
			if err := sccIndexer.Add(scc); err != nil {
				t.Fatalf("error adding sccs to store: %v", err)
			}
		}
		saIndexer := cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc})
		saCache := internalversion.NewServiceAccountLister(saIndexer)
		namespace := admissionttesting.CreateNamespaceForTest()
		serviceAccount := admissionttesting.CreateSAForTest()
		if testcase.serviceAccount != nil {
			serviceAccount.Namespace = namespace.Name
			saIndexer.Add(serviceAccount)
		}
		csf := clientsetfake.NewSimpleClientset(namespace)

		storage := REST{oscc.NewDefaultSCCMatcher(sccCache), saCache, csf}
		ctx := apirequest.WithNamespace(apirequest.NewContext(), namespace.Name)
		_, err := storage.Create(ctx, testcase.request, false)
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
		sccs            []*securityapi.SecurityContextConstraints
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
					ServiceAccountNames: []string{"my-sa", "yours-sa"},
				},
			},
			sccs: []*securityapi.SecurityContextConstraints{
				{
					ObjectMeta: metav1.ObjectMeta{
						SelfLink: "/api/version/securitycontextconstraints/myscc",
						Name:     "myscc",
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
				},
			},
			serviceAccounts: []*kapi.ServiceAccount{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "my-sa",
						Namespace: "default",
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "yours-sa",
						Namespace: "default",
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
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
					ServiceAccountNames: []string{"bad-sa"},
				},
			},
			sccs: []*securityapi.SecurityContextConstraints{
				{
					ObjectMeta: metav1.ObjectMeta{
						SelfLink: "/api/version/securitycontextconstraints/myscc",
						Name:     "myscc",
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
				},
			},
			serviceAccounts: []*kapi.ServiceAccount{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "my-sa",
						Namespace: "default",
					},
				},
			},
			errorMessage: `unable to retrieve ServiceAccount bad-sa: serviceaccount "bad-sa" not found`,
		},
	}

	for testName, testcase := range testcases {
		sccIndexer := cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc})
		sccCache := securitylisters.NewSecurityContextConstraintsLister(sccIndexer)
		for _, scc := range testcase.sccs {
			if err := sccIndexer.Add(scc); err != nil {
				t.Fatalf("error adding sccs to store: %v", err)
			}
		}
		namespace := admissionttesting.CreateNamespaceForTest()
		saIndexer := cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc})
		saCache := internalversion.NewServiceAccountLister(saIndexer)
		for i := range testcase.serviceAccounts {
			saIndexer.Add(testcase.serviceAccounts[i])
		}
		csf := clientsetfake.NewSimpleClientset(namespace)
		storage := REST{oscc.NewDefaultSCCMatcher(sccCache), saCache, csf}
		ctx := apirequest.WithNamespace(apirequest.NewContext(), namespace.Name)
		_, err := storage.Create(ctx, testcase.request, false)
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
