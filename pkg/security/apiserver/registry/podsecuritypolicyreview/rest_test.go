package podsecuritypolicyreview

import (
	"reflect"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apiserver/pkg/authorization/authorizer"
	apirequest "k8s.io/apiserver/pkg/endpoints/request"
	"k8s.io/apiserver/pkg/registry/rest"
	"k8s.io/client-go/kubernetes/fake"
	corev1listers "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"
	coreapi "k8s.io/kubernetes/pkg/apis/core"

	securityv1 "github.com/openshift/api/security/v1"
	securityv1listers "github.com/openshift/client-go/security/listers/security/v1"
	"github.com/openshift/origin/pkg/cmd/openshift-kube-apiserver/admission/security/securitycontextconstraints/sccmatching"
	securityapi "github.com/openshift/origin/pkg/security/apis/security"
	admissionttesting "github.com/openshift/origin/pkg/security/apiserver/admission/testing"
)

func TestNoErrors(t *testing.T) {
	var uid int64 = 999
	testcases := map[string]struct {
		request    *securityapi.PodSecurityPolicyReview
		sccs       []*securityv1.SecurityContextConstraints
		allowedSAs []string
	}{
		"default in pod": {
			request: &securityapi.PodSecurityPolicyReview{
				Spec: securityapi.PodSecurityPolicyReviewSpec{
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
			sccs: []*securityv1.SecurityContextConstraints{
				{
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
				},
			},
			allowedSAs: []string{"default"},
		},
		"failure creating provider": {
			request: &securityapi.PodSecurityPolicyReview{
				Spec: securityapi.PodSecurityPolicyReviewSpec{
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
				},
			},
			sccs: []*securityv1.SecurityContextConstraints{
				{
					ObjectMeta: metav1.ObjectMeta{
						SelfLink: "/api/version/securitycontextconstraints/restrictive",
						Name:     "restrictive",
					},
					RunAsUser: securityv1.RunAsUserStrategyOptions{
						Type: securityv1.RunAsUserStrategyMustRunAs,
						UID:  &uid,
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
				},
			},
			allowedSAs: nil,
		},
	}

	for testName, testcase := range testcases {
		sccIndexer := cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc})
		sccCache := securityv1listers.NewSecurityContextConstraintsLister(sccIndexer)
		for _, scc := range testcase.sccs {
			if err := sccIndexer.Add(scc); err != nil {
				t.Fatalf("error adding sccs to store: %v", err)
			}
		}
		saIndexer := cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc})
		saCache := corev1listers.NewServiceAccountLister(saIndexer)
		namespace := admissionttesting.CreateNamespaceForTest()
		serviceAccount := admissionttesting.CreateSAForTest()
		serviceAccount.Namespace = namespace.Name
		saIndexer.Add(serviceAccount)
		csf := fake.NewSimpleClientset(namespace)
		storage := REST{sccmatching.NewDefaultSCCMatcher(sccCache, &noopTestAuthorizer{}), saCache, csf}
		ctx := apirequest.WithNamespace(apirequest.NewContext(), namespace.Name)
		obj, err := storage.Create(ctx, testcase.request, rest.ValidateAllObjectFunc, &metav1.CreateOptions{})
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
		sccs           []*securityv1.SecurityContextConstraints
		serviceAccount *corev1.ServiceAccount
		errorMessage   string
	}{
		"invalid PSPR": {
			request: &securityapi.PodSecurityPolicyReview{
				Spec: securityapi.PodSecurityPolicyReviewSpec{
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
							ServiceAccountName: "A.B.C.D.E",
							SchedulerName:      coreapi.DefaultSchedulerName,
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
			errorMessage: `unable to retrieve ServiceAccount default: serviceaccount "default" not found`,
		},
	}
	for testName, testcase := range testcases {
		sccIndexer := cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc})
		sccCache := securityv1listers.NewSecurityContextConstraintsLister(sccIndexer)
		for _, scc := range testcase.sccs {
			if err := sccIndexer.Add(scc); err != nil {
				t.Fatalf("error adding sccs to store: %v", err)
			}
		}
		saIndexer := cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc})
		saCache := corev1listers.NewServiceAccountLister(saIndexer)
		namespace := admissionttesting.CreateNamespaceForTest()
		serviceAccount := admissionttesting.CreateSAForTest()
		if testcase.serviceAccount != nil {
			serviceAccount.Namespace = namespace.Name
			saIndexer.Add(serviceAccount)
		}
		csf := fake.NewSimpleClientset(namespace)

		storage := REST{sccmatching.NewDefaultSCCMatcher(sccCache, &noopTestAuthorizer{}), saCache, csf}
		ctx := apirequest.WithNamespace(apirequest.NewContext(), namespace.Name)
		_, err := storage.Create(ctx, testcase.request, rest.ValidateAllObjectFunc, &metav1.CreateOptions{})
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
		sccs            []*securityv1.SecurityContextConstraints
		errorMessage    string
		serviceAccounts []*corev1.ServiceAccount
	}{
		"SAs in PSPR": {
			request: &securityapi.PodSecurityPolicyReview{
				Spec: securityapi.PodSecurityPolicyReviewSpec{
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
					ServiceAccountNames: []string{"my-sa", "yours-sa"},
				},
			},
			sccs: []*securityv1.SecurityContextConstraints{
				{
					ObjectMeta: metav1.ObjectMeta{
						SelfLink: "/api/version/securitycontextconstraints/myscc",
						Name:     "myscc",
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
				},
			},
			serviceAccounts: []*corev1.ServiceAccount{
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
					ServiceAccountNames: []string{"bad-sa"},
				},
			},
			sccs: []*securityv1.SecurityContextConstraints{
				{
					ObjectMeta: metav1.ObjectMeta{
						SelfLink: "/api/version/securitycontextconstraints/myscc",
						Name:     "myscc",
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
				},
			},
			serviceAccounts: []*corev1.ServiceAccount{
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
		sccCache := securityv1listers.NewSecurityContextConstraintsLister(sccIndexer)
		for _, scc := range testcase.sccs {
			if err := sccIndexer.Add(scc); err != nil {
				t.Fatalf("error adding sccs to store: %v", err)
			}
		}
		namespace := admissionttesting.CreateNamespaceForTest()
		saIndexer := cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc})
		saCache := corev1listers.NewServiceAccountLister(saIndexer)
		for i := range testcase.serviceAccounts {
			saIndexer.Add(testcase.serviceAccounts[i])
		}
		csf := fake.NewSimpleClientset(namespace)
		storage := REST{sccmatching.NewDefaultSCCMatcher(sccCache, &noopTestAuthorizer{}), saCache, csf}
		ctx := apirequest.WithNamespace(apirequest.NewContext(), namespace.Name)
		_, err := storage.Create(ctx, testcase.request, rest.ValidateAllObjectFunc, &metav1.CreateOptions{})
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

type noopTestAuthorizer struct{}

func (s *noopTestAuthorizer) Authorize(a authorizer.Attributes) (authorizer.Decision, string, error) {
	return authorizer.DecisionNoOpinion, "", nil
}
