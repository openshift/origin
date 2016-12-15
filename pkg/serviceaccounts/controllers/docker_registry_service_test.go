package controllers

import (
	"encoding/json"
	"fmt"
	"reflect"
	"testing"
	"time"

	"github.com/openshift/origin/pkg/client/testclient"
	routeapi "github.com/openshift/origin/pkg/route/api"
	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset/fake"
	"k8s.io/kubernetes/pkg/client/testing/core"
	ktestclient "k8s.io/kubernetes/pkg/client/unversioned/testclient"
	"k8s.io/kubernetes/pkg/credentialprovider"
	"k8s.io/kubernetes/pkg/runtime"
	"k8s.io/kubernetes/pkg/watch"
)

const (
	registryNamespace = "ns"
	registryName      = "registry"
)

var (
	registryService = &kapi.Service{
		ObjectMeta: kapi.ObjectMeta{Name: registryName, Namespace: registryNamespace},
		Spec: kapi.ServiceSpec{
			ClusterIP: "172.16.123.123",
			Ports:     []kapi.ServicePort{{Port: 1235}},
		},
	}
	registryRoute = &routeapi.Route{
		ObjectMeta: kapi.ObjectMeta{Name: registryName, Namespace: registryNamespace},
		Spec: routeapi.RouteSpec{
			Host: "registry.local",
			To:   routeapi.RouteTargetReference{Name: registryName},
		},
	}
)

func controllerSetup(startingObjects []runtime.Object, startingRoute runtime.Object, t *testing.T) (*fake.Clientset, *watch.FakeWatcher, *watch.FakeWatcher, *DockerRegistryServiceController) {
	kubeclient := fake.NewSimpleClientset(startingObjects...)
	fakeWatch := watch.NewFake()
	fakeRouteWatch := watch.NewFake()
	kubeclient.PrependReactor("create", "*", func(action core.Action) (handled bool, ret runtime.Object, err error) {
		return true, action.(core.CreateAction).GetObject(), nil
	})
	kubeclient.PrependReactor("update", "*", func(action core.Action) (handled bool, ret runtime.Object, err error) {
		return true, action.(core.UpdateAction).GetObject(), nil
	})
	kubeclient.PrependWatchReactor("services", core.DefaultWatchReactor(fakeWatch, nil))

	var routeClient *testclient.Fake
	if startingRoute != nil {
		routeClient = testclient.NewSimpleFake(startingRoute)
	} else {
		routeClient = testclient.NewSimpleFake()
	}
	routeClient.AddWatchReactor("routes", func(action ktestclient.Action) (bool, watch.Interface, error) {
		return true, fakeRouteWatch, nil
	})

	controller := NewDockerRegistryServiceController(kubeclient, routeClient, DockerRegistryServiceControllerOptions{
		Resync:               10 * time.Minute,
		RegistryNamespace:    registryNamespace,
		RegistryServiceName:  registryName,
		DockercfgController:  &DockercfgController{},
		DockerURLsIntialized: make(chan struct{}),
	})

	return kubeclient, fakeWatch, fakeRouteWatch, controller
}

func wrapHandler(indicator chan bool, handler func(string) error, t *testing.T) func(string) error {
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

	kubeclient, fakeWatch, _, controller := controllerSetup([]runtime.Object{registryService}, nil, t)
	kubeclient.PrependReactor("update", "secrets", func(action core.Action) (handled bool, ret runtime.Object, err error) {
		return true, &kapi.Secret{}, fmt.Errorf("%v unexpected", action)
	})
	kubeclient.PrependReactor("create", "secrets", func(action core.Action) (handled bool, ret runtime.Object, err error) {
		return true, &kapi.Secret{}, fmt.Errorf("%v unexpected", action)
	})
	controller.syncRegistryLocationHandler = wrapHandler(received, controller.syncRegistryLocationChange, t)
	go controller.Run(5, stopChannel)

	t.Log("Waiting for ready")
	select {
	case <-controller.dockerURLsIntialized:
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

func TestUpdateNewStyleSecret(t *testing.T) {
	stopChannel := make(chan struct{})
	defer close(stopChannel)
	received := make(chan bool)
	updatedSecret := make(chan bool)

	newStyleDockercfgSecret := &kapi.Secret{
		ObjectMeta: kapi.ObjectMeta{
			Name: "secret-name", Namespace: registryNamespace,
			Annotations: map[string]string{
				ServiceAccountTokenValueAnnotation: "the-token",
				ServiceAccountTokenSecretNameKey:   "sa-token-secret",
			},
		},
		Type: kapi.SecretTypeDockercfg,
		Data: map[string][]byte{kapi.DockerConfigKey: []byte("{}")},
	}

	kubeclient, fakeWatch, routeFakeWatch, controller := controllerSetup([]runtime.Object{newStyleDockercfgSecret}, nil, t)
	controller.syncRegistryLocationHandler = wrapHandler(received, controller.syncRegistryLocationChange, t)
	controller.syncSecretHandler = wrapHandler(updatedSecret, controller.syncSecretUpdate, t)
	go controller.Run(5, stopChannel)

	t.Log("Waiting for ready")
	select {
	case <-controller.dockerURLsIntialized:
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
	for _, key := range []string{"172.16.123.123:1235", "registry.ns.svc:1235"} {
		expectedDockercfgMap[key] = credentialprovider.DockerConfigEntry{
			Username: "serviceaccount",
			Password: newStyleDockercfgSecret.Annotations[ServiceAccountTokenValueAnnotation],
			Email:    "serviceaccount@example.org",
		}
	}

	checkUpdatedSecrets := func(expected credentialprovider.DockerConfig) {
		foundSecret := false
		defer kubeclient.ClearActions()
		for _, action := range kubeclient.Actions() {
			switch {
			case action.Matches("update", "secrets"):
				updateService := action.(core.UpdateAction)
				secret := updateService.GetObject().(*kapi.Secret)
				actualDockercfg := &credentialprovider.DockerConfig{}
				if err := json.Unmarshal(secret.Data[kapi.DockerConfigKey], actualDockercfg); err != nil {
					t.Errorf("unexpected err %v", err)
					continue
				}
				if !reflect.DeepEqual(*actualDockercfg, expected) {
					t.Errorf("expected %v, got %v", expected, *actualDockercfg)
					continue
				}
				foundSecret = true
			}
		}

		if !foundSecret {
			t.Errorf("secret wasn't updated.  Got %v\n", kubeclient.Actions())
		}
	}

	checkUpdatedSecrets(expectedDockercfgMap)

	t.Log("Adding registry.local route to registry service")
	routeFakeWatch.Add(registryRoute)
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

	expectedDockercfgMap["registry.local"] = expectedDockercfgMap["172.16.123.123:1235"]
	checkUpdatedSecrets(expectedDockercfgMap)

	t.Log("Changing registry.local route to new-registry.local route")
	modifiedRoute := registryRoute
	modifiedRoute.Spec.Host = "new-registry.local"
	routeFakeWatch.Modify(modifiedRoute)
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

	delete(expectedDockercfgMap, "registry.local")
	expectedDockercfgMap["new-registry.local"] = expectedDockercfgMap["172.16.123.123:1235"]
	checkUpdatedSecrets(expectedDockercfgMap)

	t.Log("Using the OPENSHIFT_DEFAULT_REGISTRY")
	controller.registryDefaultHost = "new-registry.local:5000"
	routeFakeWatch.Modify(modifiedRoute)
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

	delete(expectedDockercfgMap, "new-registry.local")
	expectedDockercfgMap["new-registry.local:5000"] = expectedDockercfgMap["172.16.123.123:1235"]
	checkUpdatedSecrets(expectedDockercfgMap)
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
	oldStyleDockercfgSecret := &kapi.Secret{
		ObjectMeta: kapi.ObjectMeta{
			Name: "secret-name", Namespace: registryNamespace,
			Annotations: map[string]string{
				ServiceAccountTokenSecretNameKey: "sa-token-secret",
			},
		},
		Type: kapi.SecretTypeDockercfg,
		Data: map[string][]byte{kapi.DockerConfigKey: dockercfgContent},
	}

	kubeclient, fakeWatch, _, controller := controllerSetup([]runtime.Object{oldStyleDockercfgSecret}, nil, t)
	controller.syncRegistryLocationHandler = wrapHandler(received, controller.syncRegistryLocationChange, t)
	controller.syncSecretHandler = wrapHandler(updatedSecret, controller.syncSecretUpdate, t)
	go controller.Run(5, stopChannel)

	t.Log("Waiting for ready")
	select {
	case <-controller.dockerURLsIntialized:
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
	for _, key := range []string{"172.16.123.123:1235", "registry.ns.svc:1235"} {
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
			updateService := action.(core.UpdateAction)
			secret := updateService.GetObject().(*kapi.Secret)
			actualDockercfg := &credentialprovider.DockerConfig{}
			if err := json.Unmarshal(secret.Data[kapi.DockerConfigKey], actualDockercfg); err != nil {
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

	oldStyleDockercfgSecret := &kapi.Secret{
		ObjectMeta: kapi.ObjectMeta{
			Name: "secret-name", Namespace: registryNamespace,
			Annotations: map[string]string{
				ServiceAccountTokenSecretNameKey: "sa-token-secret",
			},
		},
		Type: kapi.SecretTypeDockercfg,
		Data: map[string][]byte{kapi.DockerConfigKey: []byte("{}")},
	}
	tokenSecret := &kapi.Secret{
		ObjectMeta: kapi.ObjectMeta{
			Name: "sa-token-secret", Namespace: registryNamespace,
			Annotations: map[string]string{
				ServiceAccountTokenSecretNameKey: "sa-token-secret",
			},
		},
		Type: kapi.SecretTypeServiceAccountToken,
		Data: map[string][]byte{kapi.ServiceAccountTokenKey: []byte("the-sa-bearer-token")},
	}

	kubeclient, fakeWatch, _, controller := controllerSetup([]runtime.Object{tokenSecret, oldStyleDockercfgSecret}, nil, t)
	kubeclient.PrependReactor("get", "secrets", func(action core.Action) (handled bool, ret runtime.Object, err error) {
		return true, tokenSecret, nil
	})
	controller.syncRegistryLocationHandler = wrapHandler(received, controller.syncRegistryLocationChange, t)
	controller.syncSecretHandler = wrapHandler(updatedSecret, controller.syncSecretUpdate, t)
	go controller.Run(5, stopChannel)

	t.Log("Waiting for ready")
	select {
	case <-controller.dockerURLsIntialized:
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
	for _, key := range []string{"172.16.123.123:1235", "registry.ns.svc:1235"} {
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
			updateService := action.(core.UpdateAction)
			secret := updateService.GetObject().(*kapi.Secret)
			actualDockercfg := &credentialprovider.DockerConfig{}
			if err := json.Unmarshal(secret.Data[kapi.DockerConfigKey], actualDockercfg); err != nil {
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
	oldStyleDockercfgSecret := &kapi.Secret{
		ObjectMeta: kapi.ObjectMeta{
			Name: "secret-name", Namespace: registryNamespace,
			Annotations: map[string]string{
				ServiceAccountTokenValueAnnotation: "the-token",
				ServiceAccountTokenSecretNameKey:   "sa-token-secret",
			},
		},
		Type: kapi.SecretTypeDockercfg,
		Data: map[string][]byte{kapi.DockerConfigKey: dockercfgContent},
	}

	kubeclient, fakeWatch, _, controller := controllerSetup([]runtime.Object{registryService, oldStyleDockercfgSecret}, nil, t)
	controller.syncRegistryLocationHandler = wrapHandler(received, controller.syncRegistryLocationChange, t)
	controller.syncSecretHandler = wrapHandler(updatedSecret, controller.syncSecretUpdate, t)
	go controller.Run(5, stopChannel)

	t.Log("Waiting for ready")
	select {
	case <-controller.dockerURLsIntialized:
	case <-time.After(time.Duration(45 * time.Second)):
		t.Fatalf("failed to become ready")
	}

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
			updateService := action.(core.UpdateAction)
			secret := updateService.GetObject().(*kapi.Secret)
			actualDockercfg := &credentialprovider.DockerConfig{}
			if err := json.Unmarshal(secret.Data[kapi.DockerConfigKey], actualDockercfg); err != nil {
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
	for _, key := range []string{"172.16.123.123:1235", "registry.ns.svc:1235"} {
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
			updateService := action.(core.UpdateAction)
			secret := updateService.GetObject().(*kapi.Secret)
			actualDockercfg := &credentialprovider.DockerConfig{}
			if err := json.Unmarshal(secret.Data[kapi.DockerConfigKey], actualDockercfg); err != nil {
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
