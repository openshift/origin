package servingcert

import (
	"crypto/x509"
	"encoding/asn1"
	"encoding/pem"
	"fmt"
	"io/ioutil"
	"reflect"
	"testing"
	"time"

	kapi "k8s.io/kubernetes/pkg/api"
	kapierrors "k8s.io/kubernetes/pkg/api/errors"
	"k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset/fake"
	"k8s.io/kubernetes/pkg/client/testing/core"
	"k8s.io/kubernetes/pkg/runtime"
	"k8s.io/kubernetes/pkg/types"
	"k8s.io/kubernetes/pkg/watch"

	"github.com/openshift/origin/pkg/cmd/server/admin"
	"github.com/openshift/origin/pkg/cmd/server/crypto/extensions"
)

func controllerSetup(startingObjects []runtime.Object, stopChannel chan struct{}, t *testing.T) ( /*caName*/ string, *fake.Clientset, *watch.FakeWatcher, *watch.FakeWatcher, *ServiceServingCertController) {
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

	kubeclient := fake.NewSimpleClientset(startingObjects...)
	fakeWatch := watch.NewFake()
	fakeSecretWatch := watch.NewFake()
	kubeclient.PrependReactor("create", "*", func(action core.Action) (handled bool, ret runtime.Object, err error) {
		return true, action.(core.CreateAction).GetObject(), nil
	})
	kubeclient.PrependReactor("update", "*", func(action core.Action) (handled bool, ret runtime.Object, err error) {
		return true, action.(core.UpdateAction).GetObject(), nil
	})
	kubeclient.PrependWatchReactor("services", core.DefaultWatchReactor(fakeWatch, nil))
	kubeclient.PrependWatchReactor("secrets", core.DefaultWatchReactor(fakeSecretWatch, nil))

	controller := NewServiceServingCertController(kubeclient.Core(), kubeclient.Core(), ca, "cluster.local", 10*time.Minute)
	controller.serviceHasSynced = func() bool { return true }
	controller.secretHasSynced = func() bool { return true }

	return caOptions.Name, kubeclient, fakeWatch, fakeSecretWatch, controller
}

func checkGeneratedCertificate(t *testing.T, certData []byte, service *kapi.Service) {
	block, _ := pem.Decode(certData)
	if block == nil {
		t.Errorf("PEM block not found in secret")
		return
	}
	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		t.Errorf("expected valid certificate in first position: %v", err)
		return
	}

	if len(cert.DNSNames) != 2 {
		t.Errorf("unexpected DNSNames: %v", cert.DNSNames)
	}
	for _, s := range cert.DNSNames {
		switch s {
		case fmt.Sprintf("%s.%s.svc", service.Name, service.Namespace),
			fmt.Sprintf("%s.%s.svc.cluster.local", service.Name, service.Namespace):
		default:
			t.Errorf("unexpected DNSNames: %v", cert.DNSNames)
		}
	}

	found := true
	for _, ext := range cert.Extensions {
		if extensions.OpenShiftServerSigningServiceUIDOID.Equal(ext.Id) {
			var value string
			if _, err := asn1.Unmarshal(ext.Value, &value); err != nil {
				t.Errorf("unable to parse certificate extension: %v", ext.Value)
				continue
			}
			if value != string(service.UID) {
				t.Errorf("unexpected extension value: %v", value)
				continue
			}
			found = true
			break
		}
	}
	if !found {
		t.Errorf("unable to find service UID certificate extension in cert: %#v", cert)
	}
}

func TestBasicControllerFlow(t *testing.T) {
	stopChannel := make(chan struct{})
	defer close(stopChannel)
	received := make(chan bool)

	caName, kubeclient, fakeWatch, _, controller := controllerSetup([]runtime.Object{}, stopChannel, t)
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
			createSecret := action.(core.CreateAction)
			newSecret := createSecret.GetObject().(*kapi.Secret)
			if newSecret.Name != expectedSecretName {
				t.Errorf("expected %v, got %v", expectedSecretName, newSecret.Name)
				continue
			}
			if newSecret.Namespace != namespace {
				t.Errorf("expected %v, got %v", namespace, newSecret.Namespace)
				continue
			}
			delete(newSecret.Annotations, ServingCertExpiryAnnotation)
			if !reflect.DeepEqual(newSecret.Annotations, expectedSecretAnnotations) {
				t.Errorf("expected %v, got %v", expectedSecretAnnotations, newSecret.Annotations)
				continue
			}

			checkGeneratedCertificate(t, newSecret.Data["tls.crt"], serviceToAdd)
			foundSecret = true

		case action.Matches("update", "services"):
			updateService := action.(core.UpdateAction)
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

	caName, kubeclient, fakeWatch, _, controller := controllerSetup([]runtime.Object{existingSecret}, stopChannel, t)
	kubeclient.PrependReactor("create", "secrets", func(action core.Action) (handled bool, ret runtime.Object, err error) {
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
			updateService := action.(core.UpdateAction)
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

	_, kubeclient, fakeWatch, _, controller := controllerSetup([]runtime.Object{existingSecret}, stopChannel, t)
	kubeclient.PrependReactor("create", "secrets", func(action core.Action) (handled bool, ret runtime.Object, err error) {
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
			updateService := action.(core.UpdateAction)
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

	_, kubeclient, fakeWatch, _, controller := controllerSetup([]runtime.Object{}, stopChannel, t)
	kubeclient.PrependReactor("create", "secrets", func(action core.Action) (handled bool, ret runtime.Object, err error) {
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
			updateService := action.(core.UpdateAction)
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

	caName, kubeclient, fakeWatch, _, controller := controllerSetup([]runtime.Object{}, stopChannel, t)
	kubeclient.PrependReactor("update", "service", func(action core.Action) (handled bool, ret runtime.Object, err error) {
		return true, &kapi.Service{}, kapierrors.NewForbidden(kapi.Resource("fdsa"), "new-service", fmt.Errorf("any service reason"))
	})
	kubeclient.PrependReactor("create", "secret", func(action core.Action) (handled bool, ret runtime.Object, err error) {
		return true, &kapi.Secret{}, kapierrors.NewForbidden(kapi.Resource("asdf"), "new-secret", fmt.Errorf("any reason"))
	})
	kubeclient.PrependReactor("update", "secret", func(action core.Action) (handled bool, ret runtime.Object, err error) {
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

func TestRecreateSecretControllerFlow(t *testing.T) {
	stopChannel := make(chan struct{})
	defer close(stopChannel)
	received := make(chan bool)

	caName, kubeclient, fakeWatch, fakeSecretWatch, controller := controllerSetup([]runtime.Object{}, stopChannel, t)
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

	secretToDelete := &kapi.Secret{}
	secretToDelete.Name = expectedSecretName
	secretToDelete.Namespace = namespace
	secretToDelete.Annotations = map[string]string{ServiceNameAnnotation: serviceName}

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
			createSecret := action.(core.CreateAction)
			newSecret := createSecret.GetObject().(*kapi.Secret)
			if newSecret.Name != expectedSecretName {
				t.Errorf("expected %v, got %v", expectedSecretName, newSecret.Name)
				continue
			}
			if newSecret.Namespace != namespace {
				t.Errorf("expected %v, got %v", namespace, newSecret.Namespace)
				continue
			}
			delete(newSecret.Annotations, ServingCertExpiryAnnotation)
			if !reflect.DeepEqual(newSecret.Annotations, expectedSecretAnnotations) {
				t.Errorf("expected %v, got %v", expectedSecretAnnotations, newSecret.Annotations)
				continue
			}

			checkGeneratedCertificate(t, newSecret.Data["tls.crt"], serviceToAdd)
			foundSecret = true

		case action.Matches("update", "services"):
			updateService := action.(core.UpdateAction)
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

	kubeclient.ClearActions()
	fakeSecretWatch.Add(secretToDelete)
	fakeSecretWatch.Delete(secretToDelete)

	t.Log("waiting to reach syncHandler")
	select {
	case <-received:
	case <-time.After(time.Duration(30 * time.Second)):
		t.Fatalf("failed to call into syncService")
	}

	for _, action := range kubeclient.Actions() {
		switch {
		case action.Matches("create", "secrets"):
			createSecret := action.(core.CreateAction)
			newSecret := createSecret.GetObject().(*kapi.Secret)
			if newSecret.Name != expectedSecretName {
				t.Errorf("expected %v, got %v", expectedSecretName, newSecret.Name)
				continue
			}
			if newSecret.Namespace != namespace {
				t.Errorf("expected %v, got %v", namespace, newSecret.Namespace)
				continue
			}
			delete(newSecret.Annotations, ServingCertExpiryAnnotation)
			if !reflect.DeepEqual(newSecret.Annotations, expectedSecretAnnotations) {
				t.Errorf("expected %v, got %v", expectedSecretAnnotations, newSecret.Annotations)
				continue
			}

			checkGeneratedCertificate(t, newSecret.Data["tls.crt"], serviceToAdd)
			foundSecret = true

		case action.Matches("update", "services"):
			updateService := action.(core.UpdateAction)
			service := updateService.GetObject().(*kapi.Service)
			if !reflect.DeepEqual(service.Annotations, expectedServiceAnnotations) {
				t.Errorf("expected %v, got %v", expectedServiceAnnotations, service.Annotations)
				continue
			}
			foundServiceUpdate = true

		}
	}
}
