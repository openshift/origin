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
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
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
			},
		},
		ClientOptions: func(cfg *rest.Config) {
			cfg.Impersonate = rest.ImpersonationConfig{
				UserName: "testuser",
				// system:masters is given cluster-admin by default
				Groups: []string{
					"system:masters",
				},
			}
		},
		Operations: map[Operation]OperationAction{
			OperationDelete: {
				Do: testDeleteFunc,
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
			},
		},
	})
}

type Stub struct {
	Object        *unstructured.Unstructured
	ClientOptions func(*rest.Config)
	Operations    map[Operation]OperationAction
}

type GVRStubRegistry map[schema.GroupVersionResource]Stub

var stubs = make(GVRStubRegistry)

func MustRegisterGVRStub(gvr schema.GroupVersionResource, stub Stub) {
	if _, ok := stubs[gvr]; ok {
		panic(fmt.Sprintf("gvr %v is already registered", gvr))
	}

	stubs[gvr] = stub
}

type OperationFunc func(context.Context, schema.GroupVersionResource, dynamic.Interface, *unstructured.Unstructured) error

type Operation string

const (
	OperationCreate           Operation = "create"
	OperationUpdate           Operation = "update"
	OperationPatch            Operation = "patch"
	OperationDelete           Operation = "delete"
	OperationDeleteCollection Operation = "deletecollection"
)

type OperationAction struct {
	Do                 OperationFunc
	DependentResources map[schema.GroupVersionResource][]unstructured.Unstructured
}

var DefaultOperationFuncs = map[Operation]OperationAction{
	OperationCreate:           {Do: testCreateFunc},
	OperationUpdate:           {Do: testUpdateFunc},
	OperationPatch:            {Do: testPatchFunc},
	OperationDelete:           {Do: testDeleteFunc},
	OperationDeleteCollection: {Do: testDeleteCollectionFunc},
}

var _ = g.Describe("[sig-api-machinery][kube-apiserver] Admission behaves correctly for OpenShift APIs", func() {
	defer g.GinkgoRecover()
	oc := exutil.NewCLI("apiserver")

	// TODO: better error handling
	g.It("should work", func() {
		for gvr, stub := range stubs {
			warnHandler := &warningHandler{}
			dynamicClient := oc.AdminDynamicClient(
				stub.ClientOptions,
				// ensure we always set the warning handler to ours for tracking
				/*
					func(c *rest.Config) {
						c.WarningHandler = warnHandler
						c.WarningHandlerWithContext = warnHandler
					},
				*/
			)

			for operation, action := range stub.Operations {
				g.By(fmt.Sprintf("checking admission is successful for GVR %q for %s operations", gvr.String(), operation))

				errs := []error{}

				cleanup := func() {
					err := cleanupAction(dynamicClient, action)
					if err != nil {
						errs = append(errs, fmt.Errorf("cleaning up after running the action: %w", err))
					}

					err = cleanupVap(oc)
					if err != nil {
						errs = append(errs, fmt.Errorf("cleaning up vap: %w", err))
					}
				}

				// Replicate a BeforeEach for creating resources that performing an action depends on.
				err := prepareForAction(dynamicClient, action)
				if err != nil {
					errs = append(errs, fmt.Errorf("preparing for running the action: %w", err))
				}

				// fail out early
				if len(errs) > 0 {
					cleanup()
					o.Expect(errors.Join(errs...)).NotTo(o.HaveOccurred())
				}

				err = createVap(oc, gvr)
				if err != nil {
					errs = append(errs, fmt.Errorf("creating the vap: %w", err))
				}

				// fail out early
				if len(errs) > 0 {
					cleanup()
					o.Expect(errors.Join(errs...)).NotTo(o.HaveOccurred())
				}

				// Do the action.
				// Expect no error and that the most recent warning from the client is associated with this action.
				err = action.Do(context.TODO(), gvr, dynamicClient, stub.Object)
				if err != nil {
					errs = append(errs, fmt.Errorf("running the action: %w", err))
				}

				if warnHandler.warning != "oat-boom" {
					errs = append(errs, fmt.Errorf("received warning %q but expected warning %q", warnHandler.warning, "oat-boom"))
				}

				cleanup()

				o.Expect(errors.Join(errs...)).NotTo(o.HaveOccurred())
			}
		}
	})
})

// TODO: scope to operation
func createVap(oc *exutil.CLI, gvr schema.GroupVersionResource) error {
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
								APIGroups:   []string{gvr.Group},
								APIVersions: []string{gvr.Version},
								Resources:   []string{gvr.Resource},
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
	if err != nil {
		return err
	}

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
	if err != nil {
		return err
	}

	return nil
}

func cleanupVap(oc *exutil.CLI) error {
	err := oc.AdminKubeClient().AdmissionregistrationV1().ValidatingAdmissionPolicies().Delete(context.TODO(), "openshift-admission-testing.openshift.io", metav1.DeleteOptions{})
	if err != nil && !k8serrors.IsNotFound(err) {
		return err
	}

	err = oc.AdminKubeClient().AdmissionregistrationV1().ValidatingAdmissionPolicyBindings().Delete(context.TODO(), "openshift-admission-testing-binding.openshift.io", metav1.DeleteOptions{})
	if err != nil && !k8serrors.IsNotFound(err) {
		return err
	}

	return nil
}

func prepareForAction(client dynamic.Interface, action OperationAction) error {
	for depGVR, dependentResource := range action.DependentResources {
		for _, depRes := range dependentResource {
			_, err := client.Resource(depGVR).Create(context.TODO(), &depRes, metav1.CreateOptions{})
			if err != nil && !k8serrors.IsAlreadyExists(err) {
				return err
			}
		}
	}

	return nil
}

func cleanupAction(client dynamic.Interface, action OperationAction) error {
	for depGVR, dependentResource := range action.DependentResources {
		for _, depRes := range dependentResource {
			err := client.Resource(depGVR).Delete(context.TODO(), depRes.GetName(), metav1.DeleteOptions{})
			if err != nil && !k8serrors.IsNotFound(err) {
				return err
			}
		}
	}

	return nil
}

type warningHandler struct {
	warning string
}

func (w *warningHandler) HandleWarningHeader(code int, agent string, text string) {
	if code != 299 || len(text) == 0 {
		return
	}
	w.warning = text
}

func (w *warningHandler) HandleWarningHeaderWithContext(_ context.Context, code int, agent string, text string) {
	if code != 299 || len(text) == 0 {
		return
	}
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
