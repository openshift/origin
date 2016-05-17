package servingcert

import (
	"fmt"
	"io/ioutil"
	"reflect"
	"testing"
	"time"

	kapi "k8s.io/kubernetes/pkg/api"
	kapierrors "k8s.io/kubernetes/pkg/api/errors"
	ktestclient "k8s.io/kubernetes/pkg/client/unversioned/testclient"
	"k8s.io/kubernetes/pkg/runtime"
	"k8s.io/kubernetes/pkg/types"
	"k8s.io/kubernetes/pkg/watch"

	"github.com/openshift/origin/pkg/cmd/server/admin"
)

func controllerSetup(startingObjects []runtime.Object, stopChannel chan struct{}, t *testing.T) ( /*caName*/ string, *ktestclient.Fake, *watch.FakeWatcher, *ServiceServingCertController) {
	certDir, err := ioutil.TempDir("", "serving-cert-unit-")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	caInfo := admin.DefaultServiceSignerCAInfo(certDir)

	caOptions := admin.CreateSignerCertOptions{
		CertFile: caInfo.CertFile,
		KeyFile:  caInfo.KeyFile,
		Name:     admin.DefaultServiceServingCertSignerName(),
		Output:   ioutil.Discard,
	}
	ca, err := caOptions.CreateSignerCert()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	kubeclient := ktestclient.NewSimpleFake(startingObjects...)
	fakeWatch := watch.NewFake()
	kubeclient.PrependReactor("create", "*", func(action ktestclient.Action) (handled bool, ret runtime.Object, err error) {
		return true, action.(ktestclient.CreateAction).GetObject(), nil
	})
	kubeclient.PrependReactor("update", "*", func(action ktestclient.Action) (handled bool, ret runtime.Object, err error) {
		return true, action.(ktestclient.UpdateAction).GetObject(), nil
	})
	kubeclient.PrependWatchReactor("*", ktestclient.DefaultWatchReactor(fakeWatch, nil))

	controller := NewServiceServingCertController(kubeclient, kubeclient, ca, "cluster.local", 10*time.Minute)

	return caOptions.Name, kubeclient, fakeWatch, controller
}

func TestBasicControllerFlow(t *testing.T) {
	stopChannel := make(chan struct{})
	defer close(stopChannel)
	received := make(chan bool)

	caName, kubeclient, fakeWatch, controller := controllerSetup([]runtime.Object{}, stopChannel, t)
	controller.syncHandler = func(serviceKey string) error {
		defer func() { received <- true }()

		err := controller.syncService(serviceKey)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		return err
	}
	go controller.Run(1, stopChannel)

	expectedSecretName := "new-secret"
	serviceName := "svc-name"
	serviceUID := "some-uid"
	expectedServiceAnnotations := map[string]string{ServingCertSecretAnnotation: expectedSecretName, ServingCertCreatedByAnnotation: caName}
	expectedSecretAnnotations := map[string]string{ServiceUIDAnnotation: serviceUID, ServiceNameAnnotation: serviceName}
	namespace := "ns"

	serviceToAdd := &kapi.Service{}
	serviceToAdd.Name = serviceName
	serviceToAdd.Namespace = namespace
	serviceToAdd.UID = types.UID(serviceUID)
	serviceToAdd.Annotations = map[string]string{ServingCertSecretAnnotation: expectedSecretName}
	fakeWatch.Add(serviceToAdd)

	t.Log("waiting to reach syncHandler")
	select {
	case <-received:
	case <-time.After(time.Duration(30 * time.Second)):
		t.Fatalf("failed to call into syncService")
	}

	foundSecret := false
	foundServiceUpdate := false
	for _, action := range kubeclient.Actions() {
		switch {
		case action.Matches("create", "secrets"):
			createSecret := action.(ktestclient.CreateAction)
			newSecret := createSecret.GetObject().(*kapi.Secret)
			if newSecret.Name != expectedSecretName {
				t.Errorf("expected %v, got %v", expectedSecretName, newSecret.Name)
				continue
			}
			if newSecret.Namespace != namespace {
				t.Errorf("expected %v, got %v", namespace, newSecret.Namespace)
				continue
			}
			if !reflect.DeepEqual(newSecret.Annotations, expectedSecretAnnotations) {
				t.Errorf("expected %v, got %v", expectedSecretAnnotations, newSecret.Annotations)
				continue
			}
			foundSecret = true

		case action.Matches("update", "services"):
			updateService := action.(ktestclient.UpdateAction)
			service := updateService.GetObject().(*kapi.Service)
			if !reflect.DeepEqual(service.Annotations, expectedServiceAnnotations) {
				t.Errorf("expected %v, got %v", expectedServiceAnnotations, service.Annotations)
				continue
			}
			foundServiceUpdate = true

		}
	}

	if !foundSecret {
		t.Errorf("secret wasn't created.  Got %v\n", kubeclient.Actions())
	}
	if !foundServiceUpdate {
		t.Errorf("service wasn't updated.  Got %v\n", kubeclient.Actions())
	}
}

func TestAlreadyExistingSecretControllerFlow(t *testing.T) {
	stopChannel := make(chan struct{})
	defer close(stopChannel)
	received := make(chan bool)

	expectedSecretName := "new-secret"
	serviceName := "svc-name"
	serviceUID := "some-uid"
	expectedSecretAnnotations := map[string]string{ServiceUIDAnnotation: serviceUID, ServiceNameAnnotation: serviceName}
	namespace := "ns"

	existingSecret := &kapi.Secret{}
	existingSecret.Name = expectedSecretName
	existingSecret.Namespace = namespace
	existingSecret.Type = kapi.SecretTypeTLS
	existingSecret.Annotations = expectedSecretAnnotations

	caName, kubeclient, fakeWatch, controller := controllerSetup([]runtime.Object{existingSecret}, stopChannel, t)
	kubeclient.PrependReactor("create", "secrets", func(action ktestclient.Action) (handled bool, ret runtime.Object, err error) {
		return true, &kapi.Secret{}, kapierrors.NewAlreadyExists(kapi.Resource("secrets"), "new-secret")
	})
	controller.syncHandler = func(serviceKey string) error {
		defer func() { received <- true }()

		err := controller.syncService(serviceKey)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		return err
	}
	go controller.Run(1, stopChannel)

	expectedServiceAnnotations := map[string]string{ServingCertSecretAnnotation: expectedSecretName, ServingCertCreatedByAnnotation: caName}

	serviceToAdd := &kapi.Service{}
	serviceToAdd.Name = serviceName
	serviceToAdd.Namespace = namespace
	serviceToAdd.UID = types.UID(serviceUID)
	serviceToAdd.Annotations = map[string]string{ServingCertSecretAnnotation: expectedSecretName}
	fakeWatch.Add(serviceToAdd)

	t.Log("waiting to reach syncHandler")
	select {
	case <-received:
	case <-time.After(time.Duration(30 * time.Second)):
		t.Fatalf("failed to call into syncService")
	}

	foundSecret := false
	foundServiceUpdate := false
	for _, action := range kubeclient.Actions() {
		switch {
		case action.Matches("get", "secrets"):
			foundSecret = true

		case action.Matches("update", "services"):
			updateService := action.(ktestclient.UpdateAction)
			service := updateService.GetObject().(*kapi.Service)
			if !reflect.DeepEqual(service.Annotations, expectedServiceAnnotations) {
				t.Errorf("expected %v, got %v", expectedServiceAnnotations, service.Annotations)
				continue
			}
			foundServiceUpdate = true

		}
	}

	if !foundSecret {
		t.Errorf("secret wasn't retrieved.  Got %v\n", kubeclient.Actions())
	}
	if !foundServiceUpdate {
		t.Errorf("service wasn't updated.  Got %v\n", kubeclient.Actions())
	}

}

func TestAlreadyExistingSecretForDifferentUIDControllerFlow(t *testing.T) {
	stopChannel := make(chan struct{})
	defer close(stopChannel)
	received := make(chan bool)

	expectedError := "secret/new-secret references serviceUID wrong-uid, which does not match some-uid"
	expectedSecretName := "new-secret"
	serviceName := "svc-name"
	serviceUID := "some-uid"
	namespace := "ns"

	existingSecret := &kapi.Secret{}
	existingSecret.Name = expectedSecretName
	existingSecret.Namespace = namespace
	existingSecret.Type = kapi.SecretTypeTLS
	existingSecret.Annotations = map[string]string{ServiceUIDAnnotation: "wrong-uid", ServiceNameAnnotation: serviceName}

	_, kubeclient, fakeWatch, controller := controllerSetup([]runtime.Object{existingSecret}, stopChannel, t)
	kubeclient.PrependReactor("create", "secrets", func(action ktestclient.Action) (handled bool, ret runtime.Object, err error) {
		return true, &kapi.Secret{}, kapierrors.NewAlreadyExists(kapi.Resource("secrets"), "new-secret")
	})
	controller.syncHandler = func(serviceKey string) error {
		defer func() { received <- true }()

		err := controller.syncService(serviceKey)
		if err != nil && err.Error() != expectedError {
			t.Errorf("unexpected error: %v", err)
		}

		return err
	}
	go controller.Run(1, stopChannel)

	expectedServiceAnnotations := map[string]string{ServingCertSecretAnnotation: expectedSecretName, ServingCertErrorAnnotation: expectedError, ServingCertErrorNumAnnotation: "1"}

	serviceToAdd := &kapi.Service{}
	serviceToAdd.Name = serviceName
	serviceToAdd.Namespace = namespace
	serviceToAdd.UID = types.UID(serviceUID)
	serviceToAdd.Annotations = map[string]string{ServingCertSecretAnnotation: expectedSecretName}
	fakeWatch.Add(serviceToAdd)

	t.Log("waiting to reach syncHandler")
	select {
	case <-received:
	case <-time.After(time.Duration(30 * time.Second)):
		t.Fatalf("failed to call into syncService")
	}

	foundSecret := false
	foundServiceUpdate := false
	for _, action := range kubeclient.Actions() {
		switch {
		case action.Matches("get", "secrets"):
			foundSecret = true

		case action.Matches("update", "services"):
			updateService := action.(ktestclient.UpdateAction)
			service := updateService.GetObject().(*kapi.Service)
			if !reflect.DeepEqual(service.Annotations, expectedServiceAnnotations) {
				t.Errorf("expected %v, got %v", expectedServiceAnnotations, service.Annotations)
				continue
			}
			foundServiceUpdate = true

		}
	}

	if !foundSecret {
		t.Errorf("secret wasn't retrieved.  Got %v\n", kubeclient.Actions())
	}
	if !foundServiceUpdate {
		t.Errorf("service wasn't updated.  Got %v\n", kubeclient.Actions())
	}
}

func TestSecretCreationErrorControllerFlow(t *testing.T) {
	stopChannel := make(chan struct{})
	defer close(stopChannel)
	received := make(chan bool)

	expectedError := `secrets "new-secret" is forbidden: any reason`
	expectedSecretName := "new-secret"
	serviceName := "svc-name"
	serviceUID := "some-uid"
	namespace := "ns"

	_, kubeclient, fakeWatch, controller := controllerSetup([]runtime.Object{}, stopChannel, t)
	kubeclient.PrependReactor("create", "secrets", func(action ktestclient.Action) (handled bool, ret runtime.Object, err error) {
		return true, &kapi.Secret{}, kapierrors.NewForbidden(kapi.Resource("secrets"), "new-secret", fmt.Errorf("any reason"))
	})
	controller.syncHandler = func(serviceKey string) error {
		defer func() { received <- true }()

		err := controller.syncService(serviceKey)
		if err != nil && err.Error() != expectedError {
			t.Errorf("unexpected error: %v", err)
		}

		return err
	}
	go controller.Run(1, stopChannel)

	expectedServiceAnnotations := map[string]string{ServingCertSecretAnnotation: expectedSecretName, ServingCertErrorAnnotation: expectedError, ServingCertErrorNumAnnotation: "1"}

	serviceToAdd := &kapi.Service{}
	serviceToAdd.Name = serviceName
	serviceToAdd.Namespace = namespace
	serviceToAdd.UID = types.UID(serviceUID)
	serviceToAdd.Annotations = map[string]string{ServingCertSecretAnnotation: expectedSecretName}
	fakeWatch.Add(serviceToAdd)

	t.Log("waiting to reach syncHandler")
	select {
	case <-received:
	case <-time.After(time.Duration(30 * time.Second)):
		t.Fatalf("failed to call into syncService")
	}

	foundServiceUpdate := false
	for _, action := range kubeclient.Actions() {
		switch {
		case action.Matches("update", "services"):
			updateService := action.(ktestclient.UpdateAction)
			service := updateService.GetObject().(*kapi.Service)
			if !reflect.DeepEqual(service.Annotations, expectedServiceAnnotations) {
				t.Errorf("expected %v, got %v", expectedServiceAnnotations, service.Annotations)
				continue
			}
			foundServiceUpdate = true

		}
	}

	if !foundServiceUpdate {
		t.Errorf("service wasn't updated.  Got %v\n", kubeclient.Actions())
	}
}

func TestSkipGenerationControllerFlow(t *testing.T) {
	stopChannel := make(chan struct{})
	defer close(stopChannel)
	received := make(chan bool)

	expectedSecretName := "new-secret"
	serviceName := "svc-name"
	serviceUID := "some-uid"
	namespace := "ns"

	caName, kubeclient, fakeWatch, controller := controllerSetup([]runtime.Object{}, stopChannel, t)
	kubeclient.PrependReactor("update", "service", func(action ktestclient.Action) (handled bool, ret runtime.Object, err error) {
		return true, &kapi.Service{}, kapierrors.NewForbidden(kapi.Resource("fdsa"), "new-service", fmt.Errorf("any service reason"))
	})
	kubeclient.PrependReactor("create", "secret", func(action ktestclient.Action) (handled bool, ret runtime.Object, err error) {
		return true, &kapi.Secret{}, kapierrors.NewForbidden(kapi.Resource("asdf"), "new-secret", fmt.Errorf("any reason"))
	})
	kubeclient.PrependReactor("update", "secret", func(action ktestclient.Action) (handled bool, ret runtime.Object, err error) {
		return true, &kapi.Secret{}, kapierrors.NewForbidden(kapi.Resource("asdf"), "new-secret", fmt.Errorf("any reason"))
	})
	controller.syncHandler = func(serviceKey string) error {
		defer func() { received <- true }()

		err := controller.syncService(serviceKey)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		return err
	}
	go controller.Run(1, stopChannel)

	serviceToAdd := &kapi.Service{}
	serviceToAdd.Name = serviceName
	serviceToAdd.Namespace = namespace
	serviceToAdd.UID = types.UID(serviceUID)
	serviceToAdd.Annotations = map[string]string{ServingCertSecretAnnotation: expectedSecretName, ServingCertErrorAnnotation: "any-error", ServingCertErrorNumAnnotation: "11"}
	fakeWatch.Add(serviceToAdd)

	t.Log("waiting to reach syncHandler")
	select {
	case <-received:
	case <-time.After(time.Duration(30 * time.Second)):
		t.Fatalf("failed to call into syncService")
	}

	for _, action := range kubeclient.Actions() {
		switch action.GetVerb() {
		case "update", "create":
			t.Errorf("no mutation expected, but we got %v", action)
		}
	}

	kubeclient.ClearActions()
	serviceToAdd.Annotations = map[string]string{ServingCertSecretAnnotation: expectedSecretName, ServingCertCreatedByAnnotation: caName}
	fakeWatch.Add(serviceToAdd)

	t.Log("waiting to reach syncHandler")
	select {
	case <-received:
	case <-time.After(time.Duration(30 * time.Second)):
		t.Fatalf("failed to call into syncService")
	}

	for _, action := range kubeclient.Actions() {
		switch action.GetVerb() {
		case "update", "create":
			t.Errorf("no mutation expected, but we got %v", action)
		}
	}
}
