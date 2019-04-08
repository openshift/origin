package integration

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	metav1beta1 "k8s.io/apimachinery/pkg/apis/meta/v1beta1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/gengo/examples/set-gen/sets"
	"k8s.io/kubernetes/pkg/api/legacyscheme"
	"k8s.io/kubernetes/pkg/kubectl/cmd/util"
	"k8s.io/kubernetes/pkg/printers"
	printersinternal "k8s.io/kubernetes/pkg/printers/internalversion"

	testutil "github.com/openshift/origin/test/util"
	testserver "github.com/openshift/origin/test/util/server"
)

var printKindWhiteList = sets.NewString(
	// k8s.io/api/core
	"APIGroup",
	"APIVersions",
	"Binding",
	"DeleteOptions",
	"ExportOptions",
	"GetOptions",
	"ListOptions",
	"CreateOptions",
	"UpdateOptions",
	"NodeProxyOptions",
	"PodAttachOptions",
	"PodExecOptions",
	"PodPortForwardOptions",
	"PodLogOptions",
	"PodProxyOptions",
	"PodStatusResult",
	"RangeAllocation",
	"ServiceProxyOptions",
	"SerializedReference",
	// --

	// k8s.io/api/admission
	"AdmissionReview",
	// --

	// k8s.io/api/admissionregistration
	"InitializerConfiguration",
	// --

	// k8s.io/api/apiextensions
	"ConversionReview",
	// --

	// k8s.io/api/authentication
	"TokenRequest",
	"TokenReview",
	// --

	// k8s.io/api/authorization
	"LocalSubjectAccessReview",
	"SelfSubjectAccessReview",
	"SelfSubjectRulesReview",
	"SubjectAccessReview",
	// --

	// k8s.io/api/autoscaling
	"Scale",
	// --

	// k8s.io/api/apps
	"DeploymentRollback",
	// --

	// k8s.io/api/batch
	"JobTemplate",
	// --

	// k8s.io/api/cloudcontrollermanager
	"CloudControllerManagerConfiguration",
	// --

	// k8s.io/api/extensions
	"ReplicationControllerDummy",
	// --

	// k8s.io/api/imagepolicy
	"ImageReview",
	// --

	// k8s.io/api/policy
	"Eviction",
	// --

	// k8s.io/apimachinery/pkg/apis/meta
	"WatchEvent",
	"Status",
	// --

	// openshift.io/api/apps
	"DeploymentConfigRollback",
	"DeploymentLog",
	"DeploymentLogOptions",
	"DeploymentRequest",
	// --

	// openshift.io/api/authorization
	"IsPersonalSubjectAccessReview",
	"OAuthClient",
	"OAuthClientAuthorization",
	"ResourceAccessReviewResponse",
	// --

	// openshift.io/api/build
	"BinaryBuildRequestOptions",
	"BuildLog",
	"BuildLogOptions",
	"BuildRequest",
	// --

	// openshift.io/api/image
	"DockerImage",
	"ImageSignature",
	"ImageStreamImport",
	"ImageStreamLayers",
	"ImageStreamMapping",
	// --

	// openshift.io/api/oauth
	"OAuthRedirectReference",
	// --

	// openshift.io/api/security
	"LocalResourceAccessReview",
	"PodSecurityPolicyReview",
	"PodSecurityPolicySelfSubjectReview",
	"PodSecurityPolicySubjectReview",
	"ResourceAccessReview",
	"SubjectAccessReviewResponse",
	"SubjectRulesReview",
	// --

	// openshift.io/api/template
	"BrokerTemplateInstance",
	"ProcessedTemplate",
	// --

	// openshift.io/api/quota
	// This should be removed after https://github.com/openshift/origin/pull/22425 merges
	"AppliedClusterResourceQuota",
	"ClusterRoleBinding",
	// --
)

var groupWhiteList = sets.NewString(
	// network.openshift.io has a separate APIServer
	"network.openshift.io",
)

// TODO (soltysh): this list has to go down to 0!
var printMissingHanlders = sets.NewString(
	"APIService",
	"AuditSink",
	"ClusterRole",
	"LimitRange",
	"MutatingWebhookConfiguration",
	"PodPreset",
	"PriorityClass",
	"ResourceQuota",
	"Role",
	"ValidatingWebhookConfiguration",
	"VolumeAttachment",
)

func TestServerSidePrint(t *testing.T) {
	masterConfig, err := testserver.DefaultMasterOptions()
	if err != nil {
		t.Fatalf("error getting master config: %#v", err)
	}
	// enable APIs that are off by default
	masterConfig.KubernetesMasterConfig.APIServerArguments = map[string][]string{
		"runtime-config": {
			"auditregistration.k8s.io/v1alpha1=true",
			"rbac.authorization.k8s.io/v1alpha1=true",
			"scheduling.k8s.io/v1alpha1=true",
			"settings.k8s.io/v1alpha1=true",
			"storage.k8s.io/v1alpha1=true",
			"batch/v2alpha1=true",
		},
	}

	clusterAdminKubeConfig, err := testserver.StartConfiguredMasterAPI(masterConfig)
	if err != nil {
		t.Fatalf("error starting server: %v", err.Error())
	}

	clusterAdminKubeClientset, err := testutil.GetClusterAdminKubeClient(clusterAdminKubeConfig)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	clusterAdminClientConfig, err := testutil.GetClusterAdminClientConfig(clusterAdminKubeConfig)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "server-print"}}
	if _, err := clusterAdminKubeClientset.CoreV1().Namespaces().Create(ns); err != nil {
		t.Fatalf("error creating namespace:%v", err)
	}

	tableParam := fmt.Sprintf("application/json;as=Table;g=%s;v=%s, application/json", metav1beta1.GroupName, metav1beta1.SchemeGroupVersion.Version)
	printer := newFakePrinter(printersinternal.AddHandlers)

	cacheDir, err := ioutil.TempDir(os.TempDir(), "test-integration-apiserver-print")
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	defer func() {
		os.Remove(cacheDir)
	}()

	cachedClient, err := discovery.NewCachedDiscoveryClientForConfig(clusterAdminClientConfig, cacheDir, "", time.Duration(10*time.Minute))
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	data, err := ioutil.ReadFile(clusterAdminKubeConfig)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	clientConfig, err := clientcmd.NewClientConfigFromBytes(data)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	configFlags := genericclioptions.NewTestConfigFlags().
		WithClientConfig(clientConfig).WithDiscoveryClient(cachedClient)

	factory := util.NewFactory(configFlags)
	mapper, err := factory.ToRESTMapper()
	if err != nil {
		t.Errorf("unexpected error getting mapper: %v", err)
		return
	}
	for gvk, apiType := range legacyscheme.Scheme.AllKnownTypes() {
		// we do not care about internal objects or lists // TODO make sure this is always true
		if gvk.Version == runtime.APIVersionInternal || strings.HasSuffix(apiType.Name(), "List") {
			continue
		}
		if printKindWhiteList.Has(gvk.Kind) || printMissingHanlders.Has(gvk.Kind) || groupWhiteList.Has(gvk.Group) {
			continue
		}

		t.Logf("Checking %s", gvk)
		// read table definition as returned by the server
		mapping, err := mapper.RESTMapping(gvk.GroupKind(), gvk.Version)
		if err != nil {
			t.Errorf("unexpected error getting mapping for GVK %s: %v", gvk, err)
			continue
		}
		client, err := factory.ClientForMapping(mapping)
		if err != nil {
			t.Errorf("unexpected error getting client for GVK %s: %v", gvk, err)
			continue
		}
		req := client.Get()
		if mapping.Scope.Name() == meta.RESTScopeNameNamespace {
			req = req.Namespace(ns.Name)
		}
		body, err := req.Resource(mapping.Resource.Resource).SetHeader("Accept", tableParam).Do().Raw()
		if err != nil {
			t.Errorf("unexpected error getting %s: %v", gvk, err)
			continue
		}
		actual, err := decodeIntoTable(body)
		if err != nil {
			t.Errorf("unexpected error decoding %s: %v", gvk, err)
			continue
		}

		// get table definition used in printers
		obj, err := legacyscheme.Scheme.New(gvk)
		if err != nil {
			t.Errorf("unexpected error creating %s: %v", gvk, err)
			continue
		}
		intGV := gvk.GroupKind().WithVersion(runtime.APIVersionInternal).GroupVersion()
		intObj, err := legacyscheme.Scheme.ConvertToVersion(obj, intGV)
		if err != nil {
			t.Errorf("unexpected error converting %s to internal: %v", gvk, err)
			continue
		}
		expectedColumnDefinitions, ok := printer.handlers[reflect.TypeOf(intObj)]
		if !ok {
			t.Errorf("missing handler for type %v", gvk)
			continue
		}

		for _, e := range expectedColumnDefinitions {
			for _, a := range actual.ColumnDefinitions {
				if a.Name == e.Name && !reflect.DeepEqual(a, e) {
					t.Errorf("unexpected difference in column definition %s for %s:\nexpected:\n%#v\nactual:\n%#v\n", e.Name, gvk, e, a)
				}
			}
		}
	}
}

type fakePrinter struct {
	handlers map[reflect.Type][]metav1beta1.TableColumnDefinition
}

var _ printers.PrintHandler = &fakePrinter{}

func (f *fakePrinter) Handler(columns, columnsWithWide []string, printFunc interface{}) error {
	return nil
}

func (f *fakePrinter) TableHandler(columns []metav1beta1.TableColumnDefinition, printFunc interface{}) error {
	printFuncValue := reflect.ValueOf(printFunc)
	objType := printFuncValue.Type().In(0)
	f.handlers[objType] = columns
	return nil
}

func (f *fakePrinter) DefaultTableHandler(columns []metav1beta1.TableColumnDefinition, printFunc interface{}) error {
	return nil
}

func newFakePrinter(fns ...func(printers.PrintHandler)) *fakePrinter {
	handlers := make(map[reflect.Type][]metav1beta1.TableColumnDefinition, len(fns))
	p := &fakePrinter{handlers: handlers}
	for _, fn := range fns {
		fn(p)
	}
	return p
}

func decodeIntoTable(body []byte) (*metav1beta1.Table, error) {
	table := &metav1beta1.Table{}
	err := json.Unmarshal(body, table)
	if err != nil {
		return nil, err
	}
	return table, nil
}
