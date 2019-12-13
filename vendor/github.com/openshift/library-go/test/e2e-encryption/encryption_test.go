package e2e

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	apiextensionsv1beta1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1beta1"
	"k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset/typed/apiextensions/v1beta1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	apiserverv1 "k8s.io/apiserver/pkg/apis/config/v1"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	corev1client "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/retry"
	"sigs.k8s.io/yaml"

	configv1 "github.com/openshift/api/config/v1"
	operatorv1 "github.com/openshift/api/operator/v1"
	configv1clientfake "github.com/openshift/client-go/config/clientset/versioned/fake"
	configv1informers "github.com/openshift/client-go/config/informers/externalversions"

	"github.com/openshift/library-go/pkg/operator/encryption"
	"github.com/openshift/library-go/pkg/operator/encryption/secrets"
	"github.com/openshift/library-go/pkg/operator/events"
	"github.com/openshift/library-go/pkg/operator/genericoperatorclient"
	"github.com/openshift/library-go/pkg/operator/v1helpers"
	"github.com/openshift/library-go/test/library"
)

func TestEncryptionIntegration(tt *testing.T) {
	// in terminal print logs immediately
	var t T = tt
	fi, _ := os.Stdin.Stat()
	if (fi.Mode() & os.ModeCharDevice) != 0 {
		t = fmtLogger{tt}
	}

	stopCh := make(chan struct{})
	defer close(stopCh)

	component := strings.ToLower(library.GenerateNameForTest(tt, ""))

	kubeConfig, err := library.NewClientConfigForTest()
	require.NoError(t, err)

	// kube clients
	kubeClient, err := kubernetes.NewForConfig(kubeConfig)
	require.NoError(t, err)
	kubeInformers := v1helpers.NewKubeInformersForNamespaces(kubeClient, "openshift-config-managed")
	apiextensionsClient, err := v1beta1.NewForConfig(kubeConfig)
	require.NoError(t, err)

	// create ExtensionTest operator CRD
	var operatorCRD apiextensionsv1beta1.CustomResourceDefinition
	require.NoError(t, yaml.Unmarshal([]byte(encryptionTestOperatorCRD), &operatorCRD))
	crd, err := apiextensionsClient.CustomResourceDefinitions().Create(&operatorCRD)
	if errors.IsAlreadyExists(err) {
		t.Logf("CRD %s already existing, ignoring error", operatorCRD.Name)
	} else {
		require.NoError(t, err)
	}
	defer apiextensionsClient.CustomResourceDefinitions().Delete(crd.Name, &metav1.DeleteOptions{})

	// create operator client and create instance with ManagementState="Managed"
	operatorGVR := schema.GroupVersionResource{Group: operatorCRD.Spec.Group, Version: "v1", Resource: operatorCRD.Spec.Names.Plural}
	operatorv1.GroupVersion.WithResource("encryptiontests")
	operatorClient, operatorInformer, err := genericoperatorclient.NewClusterScopedOperatorClient(kubeConfig, operatorGVR)
	dynamicClient, err := dynamic.NewForConfig(kubeConfig)
	require.NoError(t, err)
	err = wait.PollImmediate(time.Second, wait.ForeverTestTimeout, func() (bool, error) {
		_, err := dynamicClient.Resource(operatorGVR).Create(&unstructured.Unstructured{
			Object: map[string]interface{}{
				"apiVersion": "operator.openshift.io/v1",
				"kind":       "EncryptionTest",
				"metadata": map[string]interface{}{
					"name": "cluster",
				},
				"spec": map[string]interface{}{
					"managementState": "Managed",
				},
			},
		}, metav1.CreateOptions{})
		if err != nil && !errors.IsAlreadyExists(err) {
			t.Logf("failed to create APIServer object: %v", err)
			return false, nil
		}
		return true, nil
	})
	require.NoError(t, err)

	// create APIServer clients
	fakeConfigClient := configv1clientfake.NewSimpleClientset(&configv1.APIServer{ObjectMeta: metav1.ObjectMeta{Name: "cluster"}})
	fakeConfigInformer := configv1informers.NewSharedInformerFactory(fakeConfigClient, 10*time.Minute)
	fakeApiServerClient := fakeConfigClient.ConfigV1().APIServers()

	// create controllers
	eventRecorder := events.NewLoggingEventRecorder(component)
	deployer := NewInstantDeployer(t, stopCh, kubeInformers, kubeClient.CoreV1(), fmt.Sprintf("encryption-config-%s", component))

	controllers, err := encryption.NewControllers(
		component,
		deployer,
		operatorClient,
		fakeApiServerClient,
		fakeConfigInformer.Config().V1().APIServers(),
		kubeInformers,
		deployer, // secret client wrapping kubeClient with encryption-config revision counting
		kubeClient.Discovery(),
		eventRecorder,
		dynamicClient,
		// some random low-cardinality GVRs:
		schema.GroupResource{Group: "operator.openshift.io", Resource: "kubeapiservers"},
		schema.GroupResource{Group: "operator.openshift.io", Resource: "kubeschedulers"},
	)
	require.NoError(t, err)

	// launch controllers
	fakeConfigInformer.Start(stopCh)
	kubeInformers.Start(stopCh)
	operatorInformer.Start(stopCh)
	go controllers.Run(stopCh)

	waitForConfigEventuallyCond := func(cond func(s string) bool) {
		t.Helper()
		stopCh := time.After(wait.ForeverTestTimeout)
		for {
			c, err := deployer.WaitUntil(stopCh)
			require.NoError(t, err)
			err = deployer.Deploy()
			require.NoError(t, err)

			got := toString(c)
			t.Logf("Observed %s", got)
			if cond(got) {
				return
			}
		}
	}
	waitForConfigEventually := func(expected string) {
		t.Helper()
		waitForConfigEventuallyCond(func(got string) bool {
			return expected == got
		})
	}
	waitForConfigs := func(ss ...string) {
		t.Helper()
		for _, expected := range ss {
			c, err := deployer.Wait()
			require.NoError(t, err)
			got := toString(c)
			t.Logf("Observed %s", got)
			if expected != "*" && got != expected {
				t.Fatalf("wrong EncryptionConfig:\n  expected: %s\n  got:      %s", expected, got)
			}

			err = deployer.Deploy()
			require.NoError(t, err)
		}
	}
	conditionStatus := func(condType string) operatorv1.ConditionStatus {
		_, status, _, err := operatorClient.GetOperatorState()
		require.NoError(t, err)

		for _, c := range status.Conditions {
			if c.Type != condType {
				continue
			}
			return c.Status
		}
		return operatorv1.ConditionUnknown
	}
	requireConditionStatus := func(condType string, expected operatorv1.ConditionStatus) {
		t.Helper()
		if status := conditionStatus(condType); status != expected {
			t.Errorf("expected condition %s of status %s, found: %q", condType, expected, status)
		}
	}
	waitForConditionStatus := func(condType string, expected operatorv1.ConditionStatus) {
		t.Helper()
		err := wait.PollImmediate(time.Millisecond*100, wait.ForeverTestTimeout, func() (bool, error) {
			return conditionStatus(condType) == expected, nil
		})
		require.NoError(t, err)
	}
	waitForMigration := func(key string) {
		t.Helper()
		err := wait.PollImmediate(time.Millisecond*100, wait.ForeverTestTimeout, func() (bool, error) {
			s, err := kubeClient.CoreV1().Secrets("openshift-config-managed").Get(fmt.Sprintf("encryption-key-%s-%s", component, key), metav1.GetOptions{})
			require.NoError(t, err)

			ks, err := secrets.ToKeyState(s)
			require.NoError(t, err)
			return len(ks.Migrated.Resources) == 2, nil
		})
		require.NoError(t, err)
	}

	t.Logf("Wait for initial Encrypted condition")
	waitForConditionStatus("Encrypted", operatorv1.ConditionFalse)

	t.Logf("Enable encryption, mode aescbc")
	_, err = fakeApiServerClient.Patch("cluster", types.MergePatchType, []byte(`{"spec":{"encryption":{"type":"aescbc"}}}`))
	require.NoError(t, err)

	t.Logf("Waiting for key to show up")
	keySecretsLabel := fmt.Sprintf("%s=%s", secrets.EncryptionKeySecretsLabel, component)
	waitForKeys := func(n int) {
		t.Helper()
		err := wait.PollImmediate(time.Second, wait.ForeverTestTimeout, func() (bool, error) {
			l, err := kubeClient.CoreV1().Secrets("openshift-config-managed").List(metav1.ListOptions{LabelSelector: keySecretsLabel})
			if err != nil {
				return false, err
			}
			if len(l.Items) == n {
				return true, nil
			}
			t.Logf("Seeing %d secrets, waiting for %d", len(l.Items), n)
			return false, nil
		})
		require.NoError(t, err)
	}
	waitForKeys(1)
	waitForConfigs(
		"kubeapiservers.operator.openshift.io=identity,aescbc:1;kubeschedulers.operator.openshift.io=identity,aescbc:1",
		"kubeapiservers.operator.openshift.io=aescbc:1,identity;kubeschedulers.operator.openshift.io=aescbc:1,identity",
	)
	waitForMigration("1")
	requireConditionStatus("Encrypted", operatorv1.ConditionTrue)

	t.Logf("Switch to identity")
	_, err = fakeApiServerClient.Patch("cluster", types.MergePatchType, []byte(`{"spec":{"encryption":{"type":"identity"}}}`))
	require.NoError(t, err)
	waitForKeys(2)
	waitForConfigs(
		"kubeapiservers.operator.openshift.io=aescbc:1,identity,aesgcm:2;kubeschedulers.operator.openshift.io=aescbc:1,identity,aesgcm:2",
		"kubeapiservers.operator.openshift.io=identity,aescbc:1,aesgcm:2;kubeschedulers.operator.openshift.io=identity,aescbc:1,aesgcm:2",
	)
	requireConditionStatus("Encrypted", operatorv1.ConditionFalse)

	t.Logf("Switch to empty mode")
	_, err = fakeApiServerClient.Patch("cluster", types.MergePatchType, []byte(`{"spec":{"encryption":{"type":""}}}`))
	require.NoError(t, err)
	time.Sleep(5 * time.Second) // give controller time to create keys (it shouldn't)
	waitForKeys(2)
	requireConditionStatus("Encrypted", operatorv1.ConditionFalse)

	t.Logf("Switch to aescbc again")
	_, err = fakeApiServerClient.Patch("cluster", types.MergePatchType, []byte(`{"spec":{"encryption":{"type":"aescbc"}}}`))
	require.NoError(t, err)
	waitForKeys(3)
	waitForConfigs(
		"kubeapiservers.operator.openshift.io=identity,aescbc:3,aescbc:1,aesgcm:2;kubeschedulers.operator.openshift.io=identity,aescbc:3,aescbc:1,aesgcm:2",
		"kubeapiservers.operator.openshift.io=aescbc:3,aescbc:1,identity,aesgcm:2;kubeschedulers.operator.openshift.io=aescbc:3,aescbc:1,identity,aesgcm:2",
		"kubeapiservers.operator.openshift.io=aescbc:3,identity,aesgcm:2;kubeschedulers.operator.openshift.io=aescbc:3,identity,aesgcm:2",
	)
	waitForConditionStatus("Encrypted", operatorv1.ConditionTrue)

	t.Logf("Setting external reason")
	setExternalReason := func(reason string) {
		t.Helper()
		err = retry.RetryOnConflict(retry.DefaultBackoff, func() error {
			spec, _, rv, err := operatorClient.GetOperatorState()
			if err != nil {
				return err
			}
			spec.UnsupportedConfigOverrides.Raw = []byte(fmt.Sprintf(`{"encryption":{"reason":%q}}`, reason))
			_, _, err = operatorClient.UpdateOperatorSpec(rv, spec)
			return err
		})
		require.NoError(t, err)
	}
	setExternalReason("a")
	waitForKeys(4)
	waitForConfigs(
		"kubeapiservers.operator.openshift.io=aescbc:3,aescbc:4,identity,aesgcm:2;kubeschedulers.operator.openshift.io=aescbc:3,aescbc:4,identity,aesgcm:2",
		"kubeapiservers.operator.openshift.io=aescbc:4,aescbc:3,identity,aesgcm:2;kubeschedulers.operator.openshift.io=aescbc:4,aescbc:3,identity,aesgcm:2",
		"kubeapiservers.operator.openshift.io=aescbc:4,aescbc:3,identity;kubeschedulers.operator.openshift.io=aescbc:4,aescbc:3,identity",
	)

	t.Logf("Setting another external reason")
	setExternalReason("b")
	waitForKeys(5)
	waitForConfigs(
		"kubeapiservers.operator.openshift.io=aescbc:4,aescbc:5,aescbc:3,identity;kubeschedulers.operator.openshift.io=aescbc:4,aescbc:5,aescbc:3,identity",
		"kubeapiservers.operator.openshift.io=aescbc:5,aescbc:4,aescbc:3,identity;kubeschedulers.operator.openshift.io=aescbc:5,aescbc:4,aescbc:3,identity",
		"kubeapiservers.operator.openshift.io=aescbc:5,aescbc:4,identity;kubeschedulers.operator.openshift.io=aescbc:5,aescbc:4,identity",
	)

	t.Logf("Expire the last key")
	_, err = kubeClient.CoreV1().Secrets("openshift-config-managed").Patch(fmt.Sprintf("encryption-key-%s-5", component), types.MergePatchType, []byte(`{"metadata":{"annotations":{"encryption.apiserver.operator.openshift.io/migrated-timestamp":"2010-10-17T14:14:52+02:00"}}}`))
	require.NoError(t, err)
	waitForKeys(6)
	waitForConfigs(
		"kubeapiservers.operator.openshift.io=aescbc:5,aescbc:6,aescbc:4,identity;kubeschedulers.operator.openshift.io=aescbc:5,aescbc:6,aescbc:4,identity",
		"kubeapiservers.operator.openshift.io=aescbc:6,aescbc:5,aescbc:4,identity;kubeschedulers.operator.openshift.io=aescbc:6,aescbc:5,aescbc:4,identity",
		"kubeapiservers.operator.openshift.io=aescbc:6,aescbc:5,identity;kubeschedulers.operator.openshift.io=aescbc:6,aescbc:5,identity",
	)
	waitForConditionStatus("Encrypted", operatorv1.ConditionTrue)

	t.Logf("Delete the last key")
	_, err = kubeClient.CoreV1().Secrets("openshift-config-managed").Patch(fmt.Sprintf("encryption-key-%s-6", component), types.JSONPatchType, []byte(`[{"op":"remove","path":"/metadata/finalizers"}]`))
	require.NoError(t, err)
	err = kubeClient.CoreV1().Secrets("openshift-config-managed").Delete(fmt.Sprintf("encryption-key-%s-6", component), nil)
	require.NoError(t, err)
	err = wait.PollImmediate(time.Second, wait.ForeverTestTimeout, func() (bool, error) {
		_, err := kubeClient.CoreV1().Secrets("openshift-config-managed").Get(fmt.Sprintf("encryption-key-%s-7", component), metav1.GetOptions{})
		if errors.IsNotFound(err) {
			return false, nil
		}
		return err == nil, nil
	})
	require.NoError(t, err)
	// here we see potentially also the following if the key controller is slower than the state controller:
	//   kubeapiservers.operator.openshift.io=aescbc:6,aescbc:5,identity;kubeschedulers.operator.openshift.io=aescbc:6,aescbc:5,identity
	// but eventually we get the following:
	waitForConfigEventually(
		// 6 as preserved, unbacked config key, 7 as newly created key, and 5 as fully migrated key
		"kubeapiservers.operator.openshift.io=aescbc:6,aescbc:7,aescbc:5,aescbc:4,identity;kubeschedulers.operator.openshift.io=aescbc:6,aescbc:7,aescbc:5,aescbc:4,identity",
	)
	waitForConfigs(
		// 7 is promoted
		"kubeapiservers.operator.openshift.io=aescbc:7,aescbc:6,aescbc:5,aescbc:4,identity;kubeschedulers.operator.openshift.io=aescbc:7,aescbc:6,aescbc:5,aescbc:4,identity",
		// 7 is migrated, plus one more backed key, which is 5 (6 is deleted)
		"kubeapiservers.operator.openshift.io=aescbc:7,aescbc:6,aescbc:5,identity;kubeschedulers.operator.openshift.io=aescbc:7,aescbc:6,aescbc:5,identity",
	)
	waitForConditionStatus("Encrypted", operatorv1.ConditionTrue)

	t.Logf("Delete the openshift-config-managed config")
	_, err = kubeClient.CoreV1().Secrets("openshift-config-managed").Patch(fmt.Sprintf("encryption-config-%s", component), types.JSONPatchType, []byte(`[{"op":"remove","path":"/metadata/finalizers"}]`))
	require.NoError(t, err)
	err = kubeClient.CoreV1().Secrets("openshift-config-managed").Delete(fmt.Sprintf("encryption-config-%s", component), nil)
	require.NoError(t, err)
	waitForConfigs(
		// one migrated read-key (7) and one more backed key (5), and everything in between (6)
		"kubeapiservers.operator.openshift.io=aescbc:7,aescbc:6,aescbc:5,identity;kubeschedulers.operator.openshift.io=aescbc:7,aescbc:6,aescbc:5,identity",
	)
	waitForConditionStatus("Encrypted", operatorv1.ConditionTrue)

	t.Logf("Delete the openshift-config-managed config")
	deployer.DeleteOperandConfig()
	waitForConfigs(
		// 7 is migrated and hence only one needed, but we rotate through identity
		"kubeapiservers.operator.openshift.io=identity,aescbc:7;kubeschedulers.operator.openshift.io=identity,aescbc:7",
		// 7 is migrated, plus one backed key (5). 6 is deleted, and therefore is not preserved (would be if the operand config was not deleted)
		"kubeapiservers.operator.openshift.io=aescbc:7,aescbc:5,identity;kubeschedulers.operator.openshift.io=aescbc:7,aescbc:5,identity",
	)
	waitForConditionStatus("Encrypted", operatorv1.ConditionTrue)
}

const encryptionTestOperatorCRD = `
apiVersion: apiextensions.k8s.io/v1beta1
kind: CustomResourceDefinition
metadata:
  name: encryptiontests.operator.openshift.io
spec:
  group: operator.openshift.io
  names:
    kind: EncryptionTest
    listKind: EncryptionTestList
    plural: encryptiontests
    singular: encryptiontest
  scope: Cluster
  subresources:
    status: {}
  versions:
  - name: v1
    served: true
    storage: true
`

func toString(c *apiserverv1.EncryptionConfiguration) string {
	rs := make([]string, 0, len(c.Resources))
	for _, r := range c.Resources {
		ps := make([]string, 0, len(r.Providers))
		for _, p := range r.Providers {
			var s string
			switch {
			case p.AESCBC != nil:
				s = "aescbc:" + p.AESCBC.Keys[0].Name
			case p.AESGCM != nil:
				s = "aesgcm:" + p.AESGCM.Keys[0].Name
			case p.Identity != nil:
				s = "identity"
			}
			ps = append(ps, s)
		}
		rs = append(rs, fmt.Sprintf("%s=%s", strings.Join(r.Resources, ","), strings.Join(ps, ",")))
	}
	return strings.Join(rs, ";")
}

func NewInstantDeployer(t T, stopCh <-chan struct{}, kubeInformers v1helpers.KubeInformersForNamespaces, secretsClient corev1client.SecretsGetter,
	secretName string) *lockStepDeployer {
	return &lockStepDeployer{
		kubeInformers: kubeInformers,
		secretsClient: secretsClient,
		stopCh:        stopCh,
		configManagedSecretsClient: secretInterceptor{
			t:               t,
			output:          make(chan *corev1.Secret),
			SecretInterface: secretsClient.Secrets("openshift-config-managed"),
			secretName:      secretName,
		},
	}
}

// lockStepDeployer mirrors the encryption-config each time Deploy() is called.
// After Deploy() a call to Wait() is necessary.
type lockStepDeployer struct {
	stopCh <-chan struct{}

	kubeInformers              v1helpers.KubeInformersForNamespaces
	secretsClient              corev1client.SecretsGetter
	configManagedSecretsClient secretInterceptor

	lock     sync.Mutex
	next     *corev1.Secret
	current  *corev1.Secret
	handlers []cache.ResourceEventHandler
}

func (d *lockStepDeployer) Wait() (*apiserverv1.EncryptionConfiguration, error) {
	return d.WaitUntil(nil)
}

func (d *lockStepDeployer) WaitUntil(stopCh <-chan time.Time) (*apiserverv1.EncryptionConfiguration, error) {
	d.lock.Lock()
	if d.next != nil {
		d.lock.Unlock()
		return nil, fmt.Errorf("next secret already set. Forgotten Deploy call?")
	}
	d.lock.Unlock()

	select {
	case s := <-d.configManagedSecretsClient.output:
		var c apiserverv1.EncryptionConfiguration
		if err := json.Unmarshal(s.Data["encryption-config"], &c); err != nil {
			return nil, fmt.Errorf("failed to unmarshal encryption secret: %v", err)
		}

		d.lock.Lock()
		defer d.lock.Unlock()
		d.next = s

		return &c, nil
	case <-stopCh:
		return nil, fmt.Errorf("timeout")
	case <-d.stopCh:
		return nil, fmt.Errorf("terminating")
	}
}

func (d *lockStepDeployer) Deploy() error {
	d.lock.Lock()

	if d.next == nil {
		d.lock.Unlock()
		return fmt.Errorf("no next secret available")
	}

	old := d.current
	d.current = d.next
	d.next = nil

	handlers := make([]cache.ResourceEventHandler, len(d.handlers))
	copy(handlers, d.handlers)

	d.lock.Unlock()

	for _, h := range handlers {
		if old == nil {
			h.OnAdd(d.current)
		} else {
			h.OnUpdate(old, d.current)
		}
	}

	return nil
}

func (d *lockStepDeployer) Secrets(namespace string) corev1client.SecretInterface {
	if namespace == "openshift-config-managed" {
		return &d.configManagedSecretsClient
	}
	return d.secretsClient.Secrets(namespace)
}

type secretInterceptor struct {
	corev1client.SecretInterface

	t          T
	output     chan *corev1.Secret
	secretName string
}

func (c *secretInterceptor) Create(s *corev1.Secret) (*corev1.Secret, error) {
	s, err := c.SecretInterface.Create(s)
	if err != nil {
		return s, err
	}

	c.t.Logf("Create %s", s.Name)
	if s.Name == c.secretName {
		c.output <- s
	}

	return s, nil
}

func (c *secretInterceptor) Update(s *corev1.Secret) (*corev1.Secret, error) {
	s, err := c.SecretInterface.Update(s)
	if err != nil {
		return s, err
	}

	c.t.Logf("Update %s", s.Name)
	if s.Name == c.secretName {
		c.output <- s
	}

	return s, nil
}

func (c *secretInterceptor) Patch(name string, pt types.PatchType, data []byte, subresources ...string) (result *corev1.Secret, err error) {
	s, err := c.SecretInterface.Patch(name, pt, data, subresources...)
	if err != nil {
		return s, err
	}

	c.t.Logf("Patch %s", s.Name)
	if s.Name == c.secretName {
		c.output <- s
	}

	return s, nil
}

func (d *lockStepDeployer) AddEventHandler(handler cache.ResourceEventHandler) []cache.InformerSynced {
	d.lock.Lock()
	defer d.lock.Unlock()

	d.handlers = append(d.handlers, handler)

	return []cache.InformerSynced{}
}

func (d *lockStepDeployer) DeployedEncryptionConfigSecret() (secret *corev1.Secret, converged bool, err error) {
	d.lock.Lock()
	defer d.lock.Unlock()

	return d.current, true, nil
}

func (d *lockStepDeployer) DeleteOperandConfig() {
	d.lock.Lock()
	old := d.current
	d.current = nil
	d.next = nil
	handlers := make([]cache.ResourceEventHandler, len(d.handlers))
	copy(handlers, d.handlers)
	d.lock.Unlock()

	for _, h := range handlers {
		h.OnDelete(old)
	}
}

type T interface {
	require.TestingT
	Logf(format string, args ...interface{})
	Fatalf(format string, args ...interface{})
	Helper()
}

type fmtLogger struct {
	*testing.T
}

func (l fmtLogger) Errorf(format string, args ...interface{}) {
	l.T.Helper()
	fmt.Printf(format+"\n", args...)
	l.T.Errorf(format, args...)
}

func (l fmtLogger) Logf(format string, args ...interface{}) {
	l.T.Helper()
	fmt.Printf("STEP: "+format+"\n", args...)
	l.T.Logf(format, args...)
}
