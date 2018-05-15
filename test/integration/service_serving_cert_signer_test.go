package integration

import (
	"reflect"
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
	kapi "k8s.io/kubernetes/pkg/apis/core"

	testutil "github.com/openshift/origin/test/util"
	testserver "github.com/openshift/origin/test/util/server"

	"github.com/openshift/service-serving-cert-signer/pkg/controller/servingcert"
)

func TestServiceServingCertSigner(t *testing.T) {
	ns := "service-serving-cert-signer"

	masterConfig, clusterAdminKubeConfig, err := testserver.StartTestMaster()
	if err != nil {
		t.Fatal(err)
	}
	defer testserver.CleanupMasterEtcd(t, masterConfig)
	clusterAdminConfig, err := testutil.GetClusterAdminClientConfig(clusterAdminKubeConfig)
	if err != nil {
		t.Fatal(err)
	}
	clusterAdminKubeClientset, err := testutil.GetClusterAdminKubeClient(clusterAdminKubeConfig)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := testserver.CreateNewProject(clusterAdminConfig, "service-serving-cert-signer", "deads"); err != nil {
		t.Fatal(err)
	}

	service := &kapi.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name: "my-svc",
			Annotations: map[string]string{
				servingcert.ServingCertSecretAnnotation: "my-secret",
			},
		},
		Spec: kapi.ServiceSpec{
			Ports: []kapi.ServicePort{
				{Port: 80},
			},
		},
	}
	actualService, err := clusterAdminKubeClientset.Core().Services(ns).Create(service)
	if err != nil {
		t.Fatal(err)
	}

	var actualFirstSecret *kapi.Secret
	secretWatcher1, err := clusterAdminKubeClientset.Core().Secrets(ns).Watch(metav1.ListOptions{ResourceVersion: actualService.ResourceVersion})
	if err != nil {
		t.Fatal(err)
	}
	_, err = watch.Until(30*time.Second, secretWatcher1, func(event watch.Event) (bool, error) {
		if event.Type != watch.Added {
			return false, nil
		}
		secret := event.Object.(*kapi.Secret)
		if secret.Name == "my-secret" {
			actualFirstSecret = secret
			return true, nil
		}
		return false, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	secretWatcher1.Stop()

	// now check to make sure that regeneration works.  First, remove the annotation entirely, this simulates
	// the "old data" case where the expiry didn't exist
	delete(actualFirstSecret.Annotations, servingcert.ServingCertExpiryAnnotation)
	actualSecondSecret, err := clusterAdminKubeClientset.Core().Secrets(ns).Update(actualFirstSecret)
	if err != nil {
		t.Fatal(err)
	}

	var actualThirdSecret *kapi.Secret
	secretWatcher2, err := clusterAdminKubeClientset.Core().Secrets(ns).Watch(metav1.ListOptions{ResourceVersion: actualSecondSecret.ResourceVersion})
	if err != nil {
		t.Fatal(err)
	}
	_, err = watch.Until(30*time.Second, secretWatcher2, func(event watch.Event) (bool, error) {
		if event.Type != watch.Modified {
			return false, nil
		}
		secret := event.Object.(*kapi.Secret)
		if secret.Name == "my-secret" {
			actualThirdSecret = secret
			return true, nil
		}
		return false, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	secretWatcher2.Stop()

	if _, ok := actualThirdSecret.Annotations[servingcert.ServingCertExpiryAnnotation]; !ok {
		t.Fatalf("missing annotation: %#v", actualThirdSecret)
	}
	if reflect.DeepEqual(actualThirdSecret.Data, actualSecondSecret.Data) {
		t.Fatalf("didn't update secret content: %#v", actualThirdSecret)
	}

	// now change the annotation to indicate that we're about to expire.  The controller should regenerate.
	actualThirdSecret.Annotations[servingcert.ServingCertExpiryAnnotation] = time.Now().Add(10 * time.Second).Format(time.RFC3339)
	actualFourthSecret, err := clusterAdminKubeClientset.Core().Secrets(ns).Update(actualThirdSecret)
	if err != nil {
		t.Fatal(err)
	}

	var actualFifthSecret *kapi.Secret
	secretWatcher3, err := clusterAdminKubeClientset.Core().Secrets(ns).Watch(metav1.ListOptions{ResourceVersion: actualFourthSecret.ResourceVersion})
	if err != nil {
		t.Fatal(err)
	}
	_, err = watch.Until(30*time.Second, secretWatcher3, func(event watch.Event) (bool, error) {
		if event.Type != watch.Modified {
			return false, nil
		}
		secret := event.Object.(*kapi.Secret)
		if secret.Name == "my-secret" {
			actualFifthSecret = secret
			return true, nil
		}
		return false, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	secretWatcher3.Stop()

	if reflect.DeepEqual(actualFourthSecret.Data, actualFifthSecret.Data) {
		t.Fatalf("didn't update secret content: %#v", actualFifthSecret)
	}
}
