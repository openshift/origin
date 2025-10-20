package apiserver

import (
	"context"
	"errors"
	"fmt"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"

	v1 "k8s.io/api/admissionregistration/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"
	"k8s.io/utils/ptr"

	oauthv1 "github.com/openshift/api/oauth/v1"
	exutil "github.com/openshift/origin/test/extended/util"
)

// TODO: remove this in favor of a better registration approach
func init() {
	MustRegisterGVRStub(oauthv1.SchemeGroupVersion.WithResource("useroauthaccesstokens"), Stub{
		Object: &unstructured.Unstructured{
			Object: map[string]interface{}{
				"apiVersion": oauthv1.SchemeGroupVersion.String(),
				"kind":       "UserOAuthAccessToken",
				"metadata": map[string]interface{}{
					"name": "sha256~tokenneedstobelongenoughelseitwontworkg",
				},
				"clientName": "testclient",
				"userName":   "admin",
				"userUID":    "notempty",
				"scopes": []string{
					"user:info",
				},
				"redirectURI": "https://something.com/",
			},
		},
		UnsupportedVerbs: sets.New("create", "update", "patch", "deletecollection"),
		DependentResources: map[schema.GroupVersionResource][]unstructured.Unstructured{
			oauthv1.SchemeGroupVersion.WithResource("oauthaccesstokens"): {
				{
					Object: map[string]interface{}{
						"apiVersion": oauthv1.SchemeGroupVersion.String(),
						"kind":       "OAuthAccessToken",
						"metadata": map[string]interface{}{
							"name": "sha256~tokenneedstobelongenoughelseitwontworkg",
						},
						"clientName": "openshift-challenging-client",
						"userName":   "testuser",
						"userUID":    "notempty",
						"scopes": []string{
							"user:info",
						},
						"redirectURI": "https://something.com/",
					},
				},
			},
		},
	})
}

// TODO: create a generic structure for stubbing out GVRs.
// TODO: Use a validating admission policy with a response warning for simulating the webhook behavior

type Stub struct {
	Object             *unstructured.Unstructured
	UnsupportedVerbs   sets.Set[string]
	DependentResources map[schema.GroupVersionResource][]unstructured.Unstructured
}

type GVRStubRegistry map[schema.GroupVersionResource]Stub

var stubs = make(GVRStubRegistry)

func MustRegisterGVRStub(gvr schema.GroupVersionResource, stub Stub) {
	if _, ok := stubs[gvr]; ok {
		panic(fmt.Sprintf("gvr %v is already registered", gvr))
	}

	stubs[gvr] = stub
}

type resourceFunc func(context.Context, schema.GroupVersionResource, dynamic.Interface, *unstructured.Unstructured) error

// TODO: opt-out of these operations for apis that don't support it?
var defaultResourceFuncs = map[string]resourceFunc{
	"create":           testCreateFunc,
	"update":           testUpdateFunc,
	"patch":            testPatchFunc,
	"delete":           testDeleteFunc,
	"deletecollection": testDeleteCollectionFunc,
}

var _ = g.Describe("[sig-api-machinery][kube-apiserver] Admission behaves correctly for OpenShift APIs", func() {
	defer g.GinkgoRecover()
	oc := exutil.NewCLI("apiserver")

	g.BeforeEach(func() {
		vap := &v1.ValidatingAdmissionPolicy{
			ObjectMeta: metav1.ObjectMeta{
				Name: "openshift-admission-testing.openshift.io",
			},
			Spec: v1.ValidatingAdmissionPolicySpec{
				FailurePolicy: ptr.To(v1.Fail),
				MatchConstraints: &v1.MatchResources{
					ResourceRules: []v1.NamedRuleWithOperations{
						{
							RuleWithOperations: v1.RuleWithOperations{
								Operations: []v1.OperationType{
									v1.OperationAll,
								},
								Rule: v1.Rule{
									APIGroups:   []string{"*"},
									APIVersions: []string{"*"},
									Resources:   []string{"*"},
								},
							},
						},
					},
				},
				Validations: []v1.Validation{
					{
						Expression: "1 != 1",
						Message:    "oat-boom",
					},
				},
			},
		}

		_, err := oc.AdminKubeClient().AdmissionregistrationV1().ValidatingAdmissionPolicies().Create(context.TODO(), vap, metav1.CreateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred(), "should not encounter an error creating VAP")

		vapBinding := &v1.ValidatingAdmissionPolicyBinding{
			ObjectMeta: metav1.ObjectMeta{
				Name: "openshift-admission-testing-binding.openshift.io",
			},
			Spec: v1.ValidatingAdmissionPolicyBindingSpec{
				PolicyName: "openshift-admission-testing.openshift.io",
				ValidationActions: []v1.ValidationAction{
					v1.Warn,
				},
			},
		}

		_, err = oc.AdminKubeClient().AdmissionregistrationV1().ValidatingAdmissionPolicyBindings().Create(context.TODO(), vapBinding, metav1.CreateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred(), "should not encounter an error creating VAP Binding")
	})

	g.AfterEach(func() {
		err := oc.AdminKubeClient().AdmissionregistrationV1().ValidatingAdmissionPolicies().Delete(context.TODO(), "openshift-admission-testing.openshift.io", metav1.DeleteOptions{})
		o.Expect(err).NotTo(o.HaveOccurred(), "should not encounter an error cleaning up VAP")

		err = oc.AdminKubeClient().AdmissionregistrationV1().ValidatingAdmissionPolicyBindings().Delete(context.TODO(), "openshift-admission-testing-binding.openshift.io", metav1.DeleteOptions{})
		o.Expect(err).NotTo(o.HaveOccurred(), "should not encounter an error cleaning up VAP binding")
	})

	g.It("should work", func() {
		// TODO: should this be created per GVR with a label selection matcher that only matches resources created by this test?
		warnHandler := &warningHandler{}
		dynamicClient := oc.AdminDynamicClient(func(c *rest.Config) {
			c.WarningHandler = warnHandler
		})

		for verb, action := range defaultResourceFuncs {
			for gvr, stub := range stubs {
				if stub.UnsupportedVerbs.Has(verb) {
					continue
				}

				for depGVR, dependentResource := range stub.DependentResources {
					for _, depRes := range dependentResource {
						_, err := dynamicClient.Resource(depGVR).Create(context.TODO(), &depRes, metav1.CreateOptions{})
						o.Expect(err).NotTo(o.HaveOccurred())
					}
				}

				err := action(context.TODO(), gvr, dynamicClient, stub.Object)
				o.Expect(err).NotTo(o.HaveOccurred())
				o.Expect(warnHandler.warning).To(o.Equal("oat-boom"))

				for depGVR, dependentResource := range stub.DependentResources {
					for _, depRes := range dependentResource {
						err := dynamicClient.Resource(depGVR).Delete(context.TODO(), depRes.GetName(), metav1.DeleteOptions{})
						o.Expect(err).NotTo(o.HaveOccurred())
					}
				}
			}
		}
	})
})

type warningHandler struct {
	warning string
}

func (w *warningHandler) HandleWarningHeader(code int, agent string, text string) {
	w.warning = text
}

func testCreateFunc(ctx context.Context, gvr schema.GroupVersionResource, client dynamic.Interface, obj *unstructured.Unstructured) error {
	_, err := client.Resource(gvr).Create(ctx, obj, metav1.CreateOptions{})
	return err
}

func testUpdateFunc(ctx context.Context, gvr schema.GroupVersionResource, client dynamic.Interface, obj *unstructured.Unstructured) error {
	_, err := client.Resource(gvr).Update(ctx, obj, metav1.UpdateOptions{})
	return err
}

func testPatchFunc(ctx context.Context, gvr schema.GroupVersionResource, client dynamic.Interface, obj *unstructured.Unstructured) error {
	return errors.New("patch testing not yet implemented")
}

func testDeleteFunc(ctx context.Context, gvr schema.GroupVersionResource, client dynamic.Interface, obj *unstructured.Unstructured) error {
	return client.Resource(gvr).Delete(ctx, obj.GetName(), metav1.DeleteOptions{})
}

func testDeleteCollectionFunc(ctx context.Context, gvr schema.GroupVersionResource, client dynamic.Interface, obj *unstructured.Unstructured) error {
	// TODO: first do an update to set labels for the resource to delete
	return client.Resource(gvr).DeleteCollection(ctx, metav1.DeleteOptions{}, metav1.ListOptions{})
}
