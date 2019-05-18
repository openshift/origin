package resourcesynccontroller

import (
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"k8s.io/apimachinery/pkg/runtime"
	ktesting "k8s.io/client-go/testing"
	"k8s.io/client-go/tools/cache"

	"github.com/openshift/library-go/pkg/operator/events/eventstesting"
	"github.com/openshift/library-go/pkg/operator/v1helpers"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes/fake"

	operatorv1 "github.com/openshift/api/operator/v1"

	"github.com/openshift/library-go/pkg/operator/events"
)

func TestSyncSecret(t *testing.T) {
	kubeClient := fake.NewSimpleClientset(
		&corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{Namespace: "config", Name: "foo"},
		},
		&corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{Namespace: "operator", Name: "to-remove"},
		},
	)

	destinationSecretCreated := make(chan struct{})
	destinationSecretBarChecked := make(chan struct{})
	destinationSecretEmptySourceChecked := make(chan struct{})

	kubeClient.PrependReactor("create", "secrets", func(action ktesting.Action) (bool, runtime.Object, error) {
		actual, isCreate := action.(ktesting.CreateAction)
		if !isCreate {
			return false, nil, nil
		}
		secret, isSecret := actual.GetObject().(*corev1.Secret)
		if !isSecret {
			return false, nil, nil
		}
		if secret.Name == "foo" && secret.Namespace == "operator" {
			close(destinationSecretCreated)
		}
		return false, nil, nil
	})

	deleteSecretCounterMutex := sync.Mutex{}
	deleteSecretCounter := 0

	kubeClient.PrependReactor("delete", "secrets", func(action ktesting.Action) (bool, runtime.Object, error) {
		deleteSecretCounterMutex.Lock()
		defer deleteSecretCounterMutex.Unlock()
		deleteSecretCounter++
		return false, nil, nil
	})

	kubeClient.PrependReactor("get", "secrets", func(action ktesting.Action) (bool, runtime.Object, error) {
		actual, isGet := action.(ktesting.GetAction)
		if !isGet {
			return false, nil, nil
		}
		if actual.GetNamespace() == "operator" {
			switch actual.GetName() {
			case "bar":
				close(destinationSecretBarChecked)
			case "empty-source":
				close(destinationSecretEmptySourceChecked)
			}
		}
		return false, nil, nil
	})

	secretInformers := informers.NewSharedInformerFactoryWithOptions(kubeClient, 1*time.Minute, informers.WithNamespace("config"))
	operatorInformers := informers.NewSharedInformerFactoryWithOptions(kubeClient, 1*time.Minute, informers.WithNamespace("operator"))
	fakeStaticPodOperatorClient := v1helpers.NewFakeOperatorClient(
		&operatorv1.OperatorSpec{
			ManagementState: operatorv1.Managed,
		},
		&operatorv1.OperatorStatus{},
		nil,
	)
	eventRecorder := eventstesting.NewTestingEventRecorder(t)
	c := NewResourceSyncController(
		fakeStaticPodOperatorClient,
		v1helpers.NewFakeKubeInformersForNamespaces(map[string]informers.SharedInformerFactory{
			"config":   secretInformers,
			"operator": operatorInformers,
		}),
		kubeClient.CoreV1(),
		kubeClient.CoreV1(),
		eventRecorder,
	)
	c.cachesToSync = []cache.InformerSynced{
		secretInformers.Core().V1().Secrets().Informer().HasSynced,
	}
	c.configMapGetter = kubeClient.CoreV1()
	c.secretGetter = kubeClient.CoreV1()

	stopCh := make(chan struct{})
	defer close(stopCh)

	go secretInformers.Start(stopCh)
	go c.Run(1, stopCh)

	// The source secret was removed (404) but the destination exists. This should increase the "deleteSecretCounter"
	if err := c.SyncSecret(ResourceLocation{Namespace: "operator", Name: "to-remove"}, ResourceLocation{Namespace: "config", Name: "removed"}); err != nil {
		t.Fatal(err)
	}

	// The source secret exists, but the destination does not. This should close the "destinationSecretCreated" channel
	if err := c.SyncSecret(ResourceLocation{Namespace: "operator", Name: "foo"}, ResourceLocation{Namespace: "config", Name: "foo"}); err != nil {
		t.Fatal(err)
	}

	// The source secret does not exists nor the destination secret. This should close the "destinationSecretBarChecked" and should not increase
	// the deleteSecretCounter (we don't issue Delete() call when Get() returns 404)
	if err := c.SyncSecret(ResourceLocation{Namespace: "operator", Name: "bar"}, ResourceLocation{Namespace: "config", Name: "bar"}); err != nil {
		t.Fatal(err)
	}

	// The source resource location is not set and the destination does not exists. This should close the "destinationSecretEmptySourceChecked" and
	// should not increase the deleteSecretCounter (this is special case in resource sync controller.
	if err := c.SyncSecret(ResourceLocation{Namespace: "operator", Name: "empty-source"}, ResourceLocation{}); err != nil {
		t.Fatal(err)
	}

	select {
	case <-destinationSecretCreated:
	case <-time.After(10 * time.Second):
		t.Fatal("timeout while waiting for destination secret to be created")
	}

	select {
	case <-destinationSecretBarChecked:
	case <-time.After(10 * time.Second):
		t.Fatal("timeout while waiting for destination secret 'bar' to be checked for existence")
	}

	select {
	case <-destinationSecretEmptySourceChecked:
	case <-time.After(10 * time.Second):
		t.Fatal("timeout while waiting for destination secret 'empty-source' to be checked for existence")
	}

	deleteSecretCounterMutex.Lock()
	defer deleteSecretCounterMutex.Unlock()
	if deleteSecretCounter != 1 {
		t.Fatalf("expected exactly 1 delete call for this test, got %d", deleteSecretCounter)
	}
}

func TestSyncConfigMap(t *testing.T) {
	kubeClient := fake.NewSimpleClientset(
		&corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{Namespace: "other", Name: "foo"},
		},
		&corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{Namespace: "other", Name: "foo"},
		},
		&corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{Namespace: "config", Name: "bar"},
		},
		&corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{Namespace: "config-managed", Name: "pear"},
		},
	)

	configInformers := informers.NewSharedInformerFactoryWithOptions(kubeClient, 1*time.Minute, informers.WithNamespace("config"))
	configManagedInformers := informers.NewSharedInformerFactoryWithOptions(kubeClient, 1*time.Minute, informers.WithNamespace("config-managed"))
	operatorInformers := informers.NewSharedInformerFactoryWithOptions(kubeClient, 1*time.Minute, informers.WithNamespace("operator"))

	fakeStaticPodOperatorClient := v1helpers.NewFakeOperatorClient(
		&operatorv1.OperatorSpec{
			ManagementState: operatorv1.Managed,
		},
		&operatorv1.OperatorStatus{},
		nil,
	)
	eventRecorder := events.NewRecorder(kubeClient.CoreV1().Events("test"), "test-operator", &corev1.ObjectReference{})

	kubeInformersForNamespaces := v1helpers.NewFakeKubeInformersForNamespaces(map[string]informers.SharedInformerFactory{"other": configInformers})

	c := NewResourceSyncController(
		fakeStaticPodOperatorClient,
		v1helpers.NewFakeKubeInformersForNamespaces(map[string]informers.SharedInformerFactory{
			"config":         configInformers,
			"config-managed": configManagedInformers,
			"operator":       operatorInformers,
		}),
		v1helpers.CachedSecretGetter(kubeClient.CoreV1(), kubeInformersForNamespaces),
		v1helpers.CachedConfigMapGetter(kubeClient.CoreV1(), kubeInformersForNamespaces),
		eventRecorder,
	)
	c.configMapGetter = kubeClient.CoreV1()
	c.secretGetter = kubeClient.CoreV1()

	// sync ones for namespaces we don't have
	if err := c.SyncSecret(ResourceLocation{Namespace: "other", Name: "foo"}, ResourceLocation{Namespace: "operator", Name: "foo"}); err == nil || err.Error() != `not watching namespace "other"` {
		t.Error(err)
	}
	if err := c.SyncSecret(ResourceLocation{Namespace: "config", Name: "foo"}, ResourceLocation{Namespace: "other", Name: "foo"}); err == nil || err.Error() != `not watching namespace "other"` {
		t.Error(err)
	}
	if err := c.SyncConfigMap(ResourceLocation{Namespace: "other", Name: "foo"}, ResourceLocation{Namespace: "operator", Name: "foo"}); err == nil || err.Error() != `not watching namespace "other"` {
		t.Error(err)
	}
	if err := c.SyncConfigMap(ResourceLocation{Namespace: "config", Name: "foo"}, ResourceLocation{Namespace: "other", Name: "foo"}); err == nil || err.Error() != `not watching namespace "other"` {
		t.Error(err)
	}

	// register
	kubeClient.ClearActions()
	if err := c.SyncSecret(ResourceLocation{Namespace: "operator", Name: "foo"}, ResourceLocation{Namespace: "config", Name: "bar"}); err != nil {
		t.Fatal(err)
	}
	if err := c.SyncConfigMap(ResourceLocation{Namespace: "operator", Name: "apple"}, ResourceLocation{Namespace: "config-managed", Name: "pear"}); err != nil {
		t.Fatal(err)
	}
	if err := c.sync(); err != nil {
		t.Fatal(err)
	}
	if _, err := kubeClient.CoreV1().Secrets("operator").Get("foo", metav1.GetOptions{}); err != nil {
		t.Error(err)
	}
	if _, err := kubeClient.CoreV1().ConfigMaps("operator").Get("apple", metav1.GetOptions{}); err != nil {
		t.Error(err)
	}

	// clear
	kubeClient.ClearActions()
	if err := c.SyncSecret(ResourceLocation{Namespace: "operator", Name: "foo"}, ResourceLocation{}); err != nil {
		t.Fatal(err)
	}
	if err := c.SyncConfigMap(ResourceLocation{Namespace: "operator", Name: "apple"}, ResourceLocation{}); err != nil {
		t.Fatal(err)
	}
	if err := c.sync(); err != nil {
		t.Fatal(err)
	}
	if _, err := kubeClient.CoreV1().Secrets("operator").Get("foo", metav1.GetOptions{}); !apierrors.IsNotFound(err) {
		t.Error(err)
	}
	if _, err := kubeClient.CoreV1().ConfigMaps("operator").Get("apple", metav1.GetOptions{}); !apierrors.IsNotFound(err) {
		t.Error(err)
	}
}

func TestServeHTTP(t *testing.T) {
	c := &ResourceSyncController{
		secretSyncRules: map[ResourceLocation]ResourceLocation{
			{Namespace: "foo", Name: "cat"}:  {Namespace: "bar", Name: "cat"},
			{Namespace: "test", Name: "dog"}: {Namespace: "othertest", Name: "dog"},
			{Namespace: "foo", Name: "dog"}:  {Namespace: "bar", Name: "dog"},
		},
		configMapSyncRules: map[ResourceLocation]ResourceLocation{
			{Namespace: "a", Name: "b"}:   {Namespace: "foo", Name: "bar"},
			{Namespace: "a", Name: "c"}:   {Namespace: "foo", Name: "barc"},
			{Namespace: "bar", Name: "b"}: {Namespace: "foo", Name: "baz"},
		},
	}

	expected := `{"secrets":[` +
		`{"source":{"namespace":"foo","name":"cat"},"destination":{"namespace":"bar","name":"cat"}},` +
		`{"source":{"namespace":"foo","name":"dog"},"destination":{"namespace":"bar","name":"dog"}},` +
		`{"source":{"namespace":"test","name":"dog"},"destination":{"namespace":"othertest","name":"dog"}}],` +
		`"configs":[` +
		`{"source":{"namespace":"a","name":"b"},"destination":{"namespace":"foo","name":"bar"}},` +
		`{"source":{"namespace":"a","name":"c"},"destination":{"namespace":"foo","name":"barc"}},` +
		`{"source":{"namespace":"bar","name":"b"},"destination":{"namespace":"foo","name":"baz"}}]}`

	handler := NewDebugHandler(c)
	writer := httptest.NewRecorder()
	handler.ServeHTTP(writer, &http.Request{})
	if writer.Body == nil {
		t.Fatal("expected a body")
	}
	response := writer.Body.String()
	if response != expected {
		t.Errorf("Expected:%+v\n Got: %+v\n", expected, response)
	}
}
