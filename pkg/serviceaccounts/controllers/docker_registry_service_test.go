package controllers

import (
	"encoding/json"
	"fmt"
	"reflect"
	"testing"
	"time"

	kapi "k8s.io/kubernetes/pkg/api"
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
)

func controllerSetup(startingObjects []runtime.Object, t *testing.T) (*ktestclient.Fake, *watch.FakeWatcher, *DockerRegistryServiceController) {
	kubeclient := ktestclient.NewSimpleFake(startingObjects...)
	fakeWatch := watch.NewFake()
	kubeclient.PrependReactor("create", "*", func(action ktestclient.Action) (handled bool, ret runtime.Object, err error) {
		return true, action.(ktestclient.CreateAction).GetObject(), nil
	})
	kubeclient.PrependReactor("update", "*", func(action ktestclient.Action) (handled bool, ret runtime.Object, err error) {
		return true, action.(ktestclient.UpdateAction).GetObject(), nil
	})
	kubeclient.PrependWatchReactor("*", ktestclient.DefaultWatchReactor(fakeWatch, nil))

	controller := NewDockerRegistryServiceController(kubeclient, DockerRegistryServiceControllerOptions{
		Resync:              10 * time.Minute,
		RegistryNamespace:   registryNamespace,
		RegistryServiceName: registryName,
		DockercfgController: &DockercfgController{},
	})

	return kubeclient, fakeWatch, controller
}

func TestNoChangeNoOp(t *testing.T) {
	stopChannel := make(chan struct{})
	defer close(stopChannel)
	received := make(chan bool)

	kubeclient, fakeWatch, controller := controllerSetup([]runtime.Object{registryService}, t)
	kubeclient.PrependReactor("*", "secrets", func(action ktestclient.Action) (handled bool, ret runtime.Object, err error) {
		return true, &kapi.Secret{}, fmt.Errorf("%v unexpected", action)
	})
	controller.syncHandler = func(serviceKey string) error {
		defer func() { received <- true }()

		err := controller.syncService(serviceKey)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		return err
	}
	go controller.Run(stopChannel)

	t.Log("Waiting for first sync")
	select {
	case <-received:
	case <-time.After(time.Duration(10 * time.Second)):
		t.Fatalf("failed to call into syncService")
	}

	fakeWatch.Modify(registryService)

	t.Log("Waiting to reach syncHandler")
	select {
	case <-received:
	case <-time.After(time.Duration(10 * time.Second)):
		t.Fatalf("failed to call into syncService")
	}
}

func TestUpdateNewStyleSecret(t *testing.T) {
	stopChannel := make(chan struct{})
	defer close(stopChannel)
	received := make(chan bool)

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

	kubeclient, fakeWatch, controller := controllerSetup([]runtime.Object{newStyleDockercfgSecret}, t)
	controller.syncHandler = func(serviceKey string) error {
		defer func() { received <- true }()

		err := controller.syncService(serviceKey)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		return err
	}
	go controller.Run(stopChannel)

	t.Log("Waiting for first sync")
	select {
	case <-received:
	case <-time.After(time.Duration(10 * time.Second)):
		t.Fatalf("failed to call into syncService")
	}

	fakeWatch.Modify(registryService)

	t.Log("Waiting to reach syncHandler")
	select {
	case <-received:
	case <-time.After(time.Duration(10 * time.Second)):
		t.Fatalf("failed to call into syncService")
	}

	expectedDockercfgMap := credentialprovider.DockerConfig{}
	for _, key := range []string{"172.16.123.123:1235", "registry.ns.svc:1235"} {
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
			updateService := action.(ktestclient.UpdateAction)
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

func TestUpdateOldStyleSecretWithKey(t *testing.T) {
	stopChannel := make(chan struct{})
	defer close(stopChannel)
	received := make(chan bool)

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

	kubeclient, fakeWatch, controller := controllerSetup([]runtime.Object{oldStyleDockercfgSecret}, t)
	controller.syncHandler = func(serviceKey string) error {
		defer func() { received <- true }()

		err := controller.syncService(serviceKey)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		return err
	}
	go controller.Run(stopChannel)

	t.Log("Waiting for first sync")
	select {
	case <-received:
	case <-time.After(time.Duration(10 * time.Second)):
		t.Fatalf("failed to call into syncService")
	}

	fakeWatch.Modify(registryService)

	t.Log("Waiting to reach syncHandler")
	select {
	case <-received:
	case <-time.After(time.Duration(10 * time.Second)):
		t.Fatalf("failed to call into syncService")
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
			updateService := action.(ktestclient.UpdateAction)
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

	kubeclient, fakeWatch, controller := controllerSetup([]runtime.Object{tokenSecret, oldStyleDockercfgSecret}, t)
	controller.syncHandler = func(serviceKey string) error {
		defer func() { received <- true }()

		err := controller.syncService(serviceKey)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		return err
	}
	go controller.Run(stopChannel)

	t.Log("Waiting for first sync")
	select {
	case <-received:
	case <-time.After(time.Duration(10 * time.Second)):
		t.Fatalf("failed to call into syncService")
	}

	fakeWatch.Modify(registryService)

	t.Log("Waiting to reach syncHandler")
	select {
	case <-received:
	case <-time.After(time.Duration(10 * time.Second)):
		t.Fatalf("failed to call into syncService")
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
			updateService := action.(ktestclient.UpdateAction)
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

	kubeclient, fakeWatch, controller := controllerSetup([]runtime.Object{registryService, oldStyleDockercfgSecret}, t)
	controller.syncHandler = func(serviceKey string) error {
		defer func() { received <- true }()

		err := controller.syncService(serviceKey)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		return err
	}
	go controller.Run(stopChannel)

	t.Log("Waiting for first sync")
	select {
	case <-received:
	case <-time.After(time.Duration(10 * time.Second)):
		t.Fatalf("failed to call into syncService")
	}

	fakeWatch.Delete(registryService)

	t.Log("Waiting for first update")
	select {
	case <-received:
	case <-time.After(time.Duration(10 * time.Second)):
		t.Fatalf("failed to call into syncService")
	}

	clearedSecret := false
	for _, action := range kubeclient.Actions() {
		switch {
		case action.Matches("update", "secrets"):
			updateService := action.(ktestclient.UpdateAction)
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
	case <-time.After(time.Duration(10 * time.Second)):
		t.Fatalf("failed to call into syncService")
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
			updateService := action.(ktestclient.UpdateAction)
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
