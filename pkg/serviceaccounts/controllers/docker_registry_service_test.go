package controllers

import (
	"encoding/json"
	"fmt"
	"reflect"
	"testing"
	"time"

	"k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/watch"
	informers "k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes/fake"
	clientgotesting "k8s.io/client-go/testing"
	"k8s.io/kubernetes/pkg/controller"
	"k8s.io/kubernetes/pkg/credentialprovider"
)

const (
	registryNamespace = "default"
	registryName      = "docker-registry"
)

var (
	registryService = &v1.Service{
		ObjectMeta: metav1.ObjectMeta{Name: registryName, Namespace: registryNamespace},
		Spec: v1.ServiceSpec{
			ClusterIP: "172.16.123.123",
			Ports:     []v1.ServicePort{{Port: 1235}},
		},
	}
)

func controllerSetup(startingObjects []runtime.Object, t *testing.T, stopCh <-chan struct{}) (*fake.Clientset, *watch.FakeWatcher, *DockerRegistryServiceController, informers.SharedInformerFactory) {
	kubeclient := fake.NewSimpleClientset(startingObjects...)
	fakeWatch := watch.NewFake()
	kubeclient.PrependReactor("create", "*", func(action clientgotesting.Action) (handled bool, ret runtime.Object, err error) {
		return true, action.(clientgotesting.CreateAction).GetObject(), nil
	})
	kubeclient.PrependReactor("update", "*", func(action clientgotesting.Action) (handled bool, ret runtime.Object, err error) {
		return true, action.(clientgotesting.UpdateAction).GetObject(), nil
	})
	kubeclient.PrependWatchReactor("services",
		func(action clientgotesting.Action) (handled bool, ret watch.Interface, err error) {
			return true, fakeWatch, nil
		})

	informerFactory := informers.NewSharedInformerFactory(kubeclient, controller.NoResyncPeriodFunc())

	controller := NewDockerRegistryServiceController(
		informerFactory.Core().V1().Secrets(),
		informerFactory.Core().V1().Services(),
		kubeclient,
		DockerRegistryServiceControllerOptions{
			Resync:                10 * time.Minute,
			DockercfgController:   &DockercfgController{},
			DockerURLsInitialized: make(chan struct{}),
		},
	)
	controller.initialSecretsCheckDone = true
	controller.secretsSynced = func() bool { return true }
	return kubeclient, fakeWatch, controller, informerFactory
}

func wrapHandler(indicator chan bool, handler func() error, t *testing.T) func() error {
	return func() error {
		defer func() { indicator <- true }()

		err := handler()
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		return err
	}
}

func wrapStringHandler(indicator chan bool, handler func(string) error, t *testing.T) func(string) error {
	return func(key string) error {
		defer func() { indicator <- true }()

		err := handler(key)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		return err
	}
}

func TestNoChangeNoOp(t *testing.T) {
	stopChannel := make(chan struct{})
	defer close(stopChannel)
	received := make(chan bool)

	kubeclient, fakeWatch, controller, informerFactory := controllerSetup([]runtime.Object{registryService}, t, stopChannel)
	kubeclient.PrependReactor("update", "secrets", func(action clientgotesting.Action) (handled bool, ret runtime.Object, err error) {
		return true, &v1.Secret{}, fmt.Errorf("%v unexpected", action)
	})
	kubeclient.PrependReactor("create", "secrets", func(action clientgotesting.Action) (handled bool, ret runtime.Object, err error) {
		return true, &v1.Secret{}, fmt.Errorf("%v unexpected", action)
	})
	controller.syncRegistryLocationHandler = wrapHandler(received, controller.syncRegistryLocationChange, t)
	informerFactory.Start(stopChannel)
	go controller.Run(5, stopChannel)

	t.Log("Waiting for ready")
	select {
	case <-controller.dockerURLsInitialized:
	case <-time.After(time.Duration(45 * time.Second)):
		t.Fatalf("failed to become ready")
	}

	fakeWatch.Modify(registryService)

	t.Log("Waiting to reach syncRegistryLocationHandler")
	select {
	case <-received:
	case <-time.After(time.Duration(45 * time.Second)):
		t.Fatalf("failed to call into syncRegistryLocationHandler")
	}
}

func TestUpdateNewStyleSecretAndDNSSuffixAndAdditionalURLs(t *testing.T) {
	stopChannel := make(chan struct{})
	defer close(stopChannel)
	received := make(chan bool)
	updatedSecret := make(chan bool)

	newStyleDockercfgSecret := &v1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name: "secret-name", Namespace: registryNamespace,
			Annotations: map[string]string{
				ServiceAccountTokenValueAnnotation: "the-token",
				ServiceAccountTokenSecretNameKey:   "sa-token-secret",
			},
		},
		Type: v1.SecretTypeDockercfg,
		Data: map[string][]byte{v1.DockerConfigKey: []byte("{}")},
	}

	kubeclient, fakeWatch, controller, informerFactory := controllerSetup([]runtime.Object{newStyleDockercfgSecret}, t, stopChannel)
	controller.clusterDNSSuffix = "something.else"
	// this bit also tests the additional registryURL options
	controller.additionalRegistryURLs = []string{"foo.bar.com"}
	controller.syncRegistryLocationHandler = wrapHandler(received, controller.syncRegistryLocationChange, t)
	controller.syncSecretHandler = wrapStringHandler(updatedSecret, controller.syncSecretUpdate, t)
	controller.initialSecretsCheckDone = false
	informerFactory.Start(stopChannel)
	go controller.Run(5, stopChannel)

	t.Log("Waiting for ready")
	select {
	case <-controller.dockerURLsInitialized:
	case <-time.After(time.Duration(45 * time.Second)):
		t.Fatalf("failed to become ready")
	}
	if controller.initialSecretsCheckDone != false {
		t.Fatalf("initialSecretsCheckDone should be false")
	}

	fakeWatch.Modify(registryService)
	t.Log("Waiting to reach syncRegistryLocationHandler")
	select {
	case <-received:
	case <-time.After(time.Duration(45 * time.Second)):
		t.Fatalf("failed to call into syncRegistryLocationHandler")
	}

	// after this point the secrets should be added to the queue and initial check should be done.
	if controller.initialSecretsCheckDone != true {
		t.Fatalf("initialSecretsCheckDone should be true")
	}

	t.Log("Waiting to update secret")
	select {
	case <-updatedSecret:
	case <-time.After(time.Duration(45 * time.Second)):
		t.Fatalf("failed to call into syncSecret")
	}

	expectedDockercfgMap := credentialprovider.DockerConfig{}
	for _, key := range []string{"foo.bar.com", "172.16.123.123:1235", "docker-registry.default.svc:1235", "docker-registry.default.svc.something.else:1235"} {
		expectedDockercfgMap[key] = credentialprovider.DockerConfigEntry{
			Username: "serviceaccount",
			Password: newStyleDockercfgSecret.Annotations[ServiceAccountTokenValueAnnotation],
			Email:    "serviceaccount@example.org",
		}
	}

	foundSecret := false
	for _, action := range kubeclient.Actions() {
		switch {
		case action.Matches("update", "secrets"):
			updateService := action.(clientgotesting.UpdateAction)
			secret := updateService.GetObject().(*v1.Secret)
			actualDockercfg := &credentialprovider.DockerConfig{}
			if err := json.Unmarshal(secret.Data[v1.DockerConfigKey], actualDockercfg); err != nil {
				t.Errorf("unexpected err %v", err)
				continue
			}
			if !reflect.DeepEqual(*actualDockercfg, expectedDockercfgMap) {
				t.Errorf("expected %v, got %v", expectedDockercfgMap, *actualDockercfg)
				continue
			}
			foundSecret = true
		}
	}

	if !foundSecret {
		t.Errorf("secret wasn't updated.  Got %v\n", kubeclient.Actions())
	}
}

func TestUpdateOldStyleSecretWithKey(t *testing.T) {
	stopChannel := make(chan struct{})
	defer close(stopChannel)
	received := make(chan bool)
	updatedSecret := make(chan bool)

	existingDockercfgMap := credentialprovider.DockerConfig{}
	for _, key := range []string{"somekey"} {
		existingDockercfgMap[key] = credentialprovider.DockerConfigEntry{
			Username: "serviceaccount",
			Password: "token-value",
			Email:    "serviceaccount@example.org",
		}
	}
	dockercfgContent, err := json.Marshal(&existingDockercfgMap)
	if err != nil {
		t.Fatalf("unexpected err %v", err)
	}
	oldStyleDockercfgSecret := &v1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name: "secret-name", Namespace: registryNamespace,
			Annotations: map[string]string{
				ServiceAccountTokenSecretNameKey: "sa-token-secret",
			},
		},
		Type: v1.SecretTypeDockercfg,
		Data: map[string][]byte{v1.DockerConfigKey: dockercfgContent},
	}

	kubeclient, _, controller, informerFactory := controllerSetup([]runtime.Object{registryService, oldStyleDockercfgSecret}, t, stopChannel)
	controller.syncRegistryLocationHandler = wrapHandler(received, controller.syncRegistryLocationChange, t)
	controller.syncSecretHandler = wrapStringHandler(updatedSecret, controller.syncSecretUpdate, t)
	controller.initialSecretsCheckDone = false
	informerFactory.Start(stopChannel)
	go controller.Run(5, stopChannel)

	t.Log("Waiting for ready")
	select {
	case <-controller.dockerURLsInitialized:
	case <-time.After(time.Duration(45 * time.Second)):
		t.Fatalf("failed to become ready")
	}

	t.Log("Waiting to reach syncRegistryLocationHandler")
	select {
	case <-received:
	case <-time.After(time.Duration(45 * time.Second)):
		t.Fatalf("failed to call into syncRegistryLocationHandler")
	}
	t.Log("Waiting to update secret")
	select {
	case <-updatedSecret:
	case <-time.After(time.Duration(45 * time.Second)):
		t.Fatalf("failed to call into syncSecret")
	}

	expectedDockercfgMap := credentialprovider.DockerConfig{}
	for _, key := range []string{"172.16.123.123:1235", "docker-registry.default.svc:1235"} {
		expectedDockercfgMap[key] = credentialprovider.DockerConfigEntry{
			Username: "serviceaccount",
			Password: "token-value",
			Email:    "serviceaccount@example.org",
		}
	}

	foundSecret := false
	for _, action := range kubeclient.Actions() {
		switch {
		case action.Matches("update", "secrets"):
			updateService := action.(clientgotesting.UpdateAction)
			secret := updateService.GetObject().(*v1.Secret)
			actualDockercfg := &credentialprovider.DockerConfig{}
			if err := json.Unmarshal(secret.Data[v1.DockerConfigKey], actualDockercfg); err != nil {
				t.Errorf("unexpected err %v", err)
				continue
			}
			if !reflect.DeepEqual(*actualDockercfg, expectedDockercfgMap) {
				t.Errorf("expected %v, got %v", expectedDockercfgMap, *actualDockercfg)
				continue
			}
			foundSecret = true
		}
	}

	if !foundSecret {
		t.Errorf("secret wasn't updated.  Got %v\n", kubeclient.Actions())
	}
}

func TestUpdateOldStyleSecretWithoutKey(t *testing.T) {
	stopChannel := make(chan struct{})
	defer close(stopChannel)
	received := make(chan bool)
	updatedSecret := make(chan bool)

	oldStyleDockercfgSecret := &v1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name: "secret-name", Namespace: registryNamespace,
			Annotations: map[string]string{
				ServiceAccountTokenSecretNameKey: "sa-token-secret",
			},
		},
		Type: v1.SecretTypeDockercfg,
		Data: map[string][]byte{v1.DockerConfigKey: []byte("{}")},
	}
	tokenSecret := &v1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name: "sa-token-secret", Namespace: registryNamespace,
			Annotations: map[string]string{
				ServiceAccountTokenSecretNameKey: "sa-token-secret",
			},
		},
		Type: v1.SecretTypeServiceAccountToken,
		Data: map[string][]byte{v1.ServiceAccountTokenKey: []byte("the-sa-bearer-token")},
	}

	kubeclient, fakeWatch, controller, informerFactory := controllerSetup([]runtime.Object{tokenSecret, oldStyleDockercfgSecret}, t, stopChannel)
	kubeclient.PrependReactor("get", "secrets", func(action clientgotesting.Action) (handled bool, ret runtime.Object, err error) {
		return true, tokenSecret, nil
	})
	controller.syncRegistryLocationHandler = wrapHandler(received, controller.syncRegistryLocationChange, t)
	controller.syncSecretHandler = wrapStringHandler(updatedSecret, controller.syncSecretUpdate, t)
	informerFactory.Start(stopChannel)
	go controller.Run(5, stopChannel)

	t.Log("Waiting for ready")
	select {
	case <-controller.dockerURLsInitialized:
	case <-time.After(time.Duration(45 * time.Second)):
		t.Fatalf("failed to become ready")
	}

	fakeWatch.Modify(registryService)

	t.Log("Waiting to reach syncRegistryLocationHandler")
	select {
	case <-received:
	case <-time.After(time.Duration(45 * time.Second)):
		t.Fatalf("failed to call into syncRegistryLocationHandler")
	}
	t.Log("Waiting to update secret")
	select {
	case <-updatedSecret:
	case <-time.After(time.Duration(45 * time.Second)):
		t.Fatalf("failed to call into syncSecret")
	}

	expectedDockercfgMap := credentialprovider.DockerConfig{}
	for _, key := range []string{"172.16.123.123:1235", "docker-registry.default.svc:1235"} {
		expectedDockercfgMap[key] = credentialprovider.DockerConfigEntry{
			Username: "serviceaccount",
			Password: "the-sa-bearer-token",
			Email:    "serviceaccount@example.org",
		}
	}

	foundSecret := false
	for _, action := range kubeclient.Actions() {
		switch {
		case action.Matches("update", "secrets"):
			updateService := action.(clientgotesting.UpdateAction)
			secret := updateService.GetObject().(*v1.Secret)
			actualDockercfg := &credentialprovider.DockerConfig{}
			if err := json.Unmarshal(secret.Data[v1.DockerConfigKey], actualDockercfg); err != nil {
				t.Errorf("unexpected err %v", err)
				continue
			}
			if !reflect.DeepEqual(*actualDockercfg, expectedDockercfgMap) {
				t.Errorf("expected %v, got %v", expectedDockercfgMap, *actualDockercfg)
				continue
			}
			foundSecret = true
		}
	}

	if !foundSecret {
		t.Errorf("secret wasn't updated.  Got %v\n", kubeclient.Actions())
	}
}

func TestClearSecretAndRecreate(t *testing.T) {
	stopChannel := make(chan struct{})
	defer close(stopChannel)
	received := make(chan bool)
	updatedSecret := make(chan bool)

	existingDockercfgMap := credentialprovider.DockerConfig{}
	for _, key := range []string{"somekey"} {
		existingDockercfgMap[key] = credentialprovider.DockerConfigEntry{
			Username: "serviceaccount",
			Password: "token-value",
			Email:    "serviceaccount@example.org",
		}
	}
	dockercfgContent, err := json.Marshal(&existingDockercfgMap)
	if err != nil {
		t.Fatalf("unexpected err %v", err)
	}
	oldStyleDockercfgSecret := &v1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name: "secret-name", Namespace: registryNamespace,
			Annotations: map[string]string{
				ServiceAccountTokenValueAnnotation: "the-token",
				ServiceAccountTokenSecretNameKey:   "sa-token-secret",
			},
		},
		Type: v1.SecretTypeDockercfg,
		Data: map[string][]byte{v1.DockerConfigKey: dockercfgContent},
	}

	kubeclient, fakeWatch, controller, informerFactory := controllerSetup([]runtime.Object{registryService, oldStyleDockercfgSecret}, t, stopChannel)
	controller.syncRegistryLocationHandler = wrapHandler(received, controller.syncRegistryLocationChange, t)
	controller.syncSecretHandler = wrapStringHandler(updatedSecret, controller.syncSecretUpdate, t)
	informerFactory.Start(stopChannel)
	go controller.Run(5, stopChannel)

	t.Log("Waiting for ready")
	select {
	case <-controller.dockerURLsInitialized:
	case <-time.After(time.Duration(45 * time.Second)):
		t.Fatalf("failed waiting for dockerURLsInitialized")
	}

	t.Logf("deleting %s service", registryService.Name)
	fakeWatch.Delete(registryService)

	t.Log("Waiting for first update")
	select {
	case <-received:
	case <-time.After(time.Duration(45 * time.Second)):
		t.Fatalf("failed to call into syncRegistryLocationHandler")
	}

	t.Log("Waiting to update secret")
	select {
	case <-updatedSecret:
	case <-time.After(time.Duration(45 * time.Second)):
		t.Fatalf("failed to call into syncSecret")
	}

	clearedSecret := false
	for _, action := range kubeclient.Actions() {
		switch {
		case action.Matches("update", "secrets"):
			updateService := action.(clientgotesting.UpdateAction)
			secret := updateService.GetObject().(*v1.Secret)
			actualDockercfg := &credentialprovider.DockerConfig{}
			if err := json.Unmarshal(secret.Data[v1.DockerConfigKey], actualDockercfg); err != nil {
				t.Errorf("unexpected err %v", err)
				continue
			}
			if !reflect.DeepEqual(*actualDockercfg, credentialprovider.DockerConfig{}) {
				t.Errorf("expected %v, got %v", credentialprovider.DockerConfig{}, *actualDockercfg)
				continue
			}
			clearedSecret = true
		}
	}
	if !clearedSecret {
		t.Errorf("secret wasn't updated.  Got %v\n", kubeclient.Actions())
	}

	kubeclient.ClearActions()

	t.Logf("adding %s service", registryService.Name)
	fakeWatch.Add(registryService)

	t.Log("Waiting for second update")
	select {
	case <-received:
	case <-time.After(time.Duration(45 * time.Second)):
		t.Fatalf("failed to call into syncRegistryLocationHandler")
	}

	t.Log("Waiting to update secret")
	select {
	case <-updatedSecret:
	case <-time.After(time.Duration(45 * time.Second)):
		t.Fatalf("failed to call into syncSecret")
	}

	expectedDockercfgMap := credentialprovider.DockerConfig{}
	for _, key := range []string{"172.16.123.123:1235", "docker-registry.default.svc:1235"} {
		expectedDockercfgMap[key] = credentialprovider.DockerConfigEntry{
			Username: "serviceaccount",
			Password: "the-token",
			Email:    "serviceaccount@example.org",
		}
	}

	foundSecret := false
	for _, action := range kubeclient.Actions() {
		switch {
		case action.Matches("update", "secrets"):
			updateService := action.(clientgotesting.UpdateAction)
			secret := updateService.GetObject().(*v1.Secret)
			actualDockercfg := &credentialprovider.DockerConfig{}
			if err := json.Unmarshal(secret.Data[v1.DockerConfigKey], actualDockercfg); err != nil {
				t.Errorf("unexpected err %v", err)
				continue
			}
			if !reflect.DeepEqual(*actualDockercfg, expectedDockercfgMap) {
				t.Errorf("expected %v, got %v", expectedDockercfgMap, *actualDockercfg)
				continue
			}
			foundSecret = true
		}
	}

	if !foundSecret {
		t.Errorf("secret wasn't updated.  Got %v\n", kubeclient.Actions())
	}
}
