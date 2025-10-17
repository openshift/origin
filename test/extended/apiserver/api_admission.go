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
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"
	"k8s.io/utils/ptr"

	oauthv1 "github.com/openshift/api/oauth/v1"
	exutil "github.com/openshift/origin/test/extended/util"
)

// TODO: remove this in favor of a better registration approach
func init() {
	MustRegisterGVRStub(oauthv1.SchemeGroupVersion.WithResource("useroauthaccesstokens"), &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": oauthv1.SchemeGroupVersion.String(),
			"kind":       "UserOAuthAccessToken",
			"metadata": map[string]interface{}{
				"name": "sha256~tokenneedstobelongenoughelseitwontworkg",
			},
			"clientName": "testclient",
			"userName":   "testuser",
			"userUID":    "notempty",
			"scopes": []string{
				"user:info",
			},
			"redirectURI": "https://something.com/",
		},
	})
}

// TODO: create a generic structure for stubbing out GVRs.
// TODO: Use a validating admission policy with a response warning for simulating the webhook behavior

type GVRStubRegistry map[schema.GroupVersionResource]*unstructured.Unstructured

var stubs = make(GVRStubRegistry)

func MustRegisterGVRStub(gvr schema.GroupVersionResource, stub *unstructured.Unstructured) {
	if _, ok := stubs[gvr]; ok {
		panic(fmt.Sprintf("gvr %v is already registered", gvr))
	}

	if stub == nil {
		panic("stub should not be nil")
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

var _ = g.Describe("[Jira:\"kube-apiserver\"] Admission behaves correctly for OpenShift APIs", func() {
	defer g.GinkgoRecover()
	oc := exutil.NewCLI("apiserver")

	// TODO: should this be created per GVR with a label selection matcher that only matches resources created by this test?
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

	warnHandler := &warningHandler{}
	dynamicClient := oc.DynamicClient(func(c *rest.Config) {
		c.WarningHandler = warnHandler
	})

	failures := []error{}
	for verb, action := range defaultResourceFuncs {
		for gvr, stub := range stubs {
			g.By(fmt.Sprintf("testing admission for verb %s with resource %s", verb, gvr.String()))

			err := action(context.TODO(), gvr, dynamicClient, stub)
			if err != nil {
				failures = append(failures, fmt.Errorf("%s-ing resource %s : %w", verb, gvr.String(), err))
			}

			if warnHandler.warning != "todo" {
				failures = append(failures, fmt.Errorf("admission warning did not match the expected warning when %s-ing resource %s", verb, gvr.String()))
			}
		}
	}

	o.Expect(failures).To(o.BeEmpty(), "should not have admission failures")
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
