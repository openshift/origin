package e2e

import (
	"math/rand"
	"os"
	"strings"
	"testing"
	"time"

	"k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/openshift/service-serving-cert-signer/pkg/controller/api"
)

const (
	serviceCAOperatorNamespace   = "openshift-core-operators"
	serviceCAControllerNamespace = "openshift-service-cert-signer"
	serviceCAOperatorPodPrefix   = "openshift-service-cert-signer-operator"
	apiInjectorPodPrefix         = "apiservice-cabundle-injector"
	configMapInjectorPodPrefix   = "configmap-cabundle-injector"
	caControllerPodPrefix        = "service-serving-cert-signer"
)

func hasPodWithPrefixName(client *kubernetes.Clientset, name, namespace string) bool {
	if client == nil || len(name) == 0 || len(namespace) == 0 {
		return false
	}
	pods, err := client.CoreV1().Pods(namespace).List(metav1.ListOptions{})
	if err != nil {
		return false
	}
	for _, pod := range pods.Items {
		if strings.HasPrefix(pod.GetName(), name) {
			return true
		}
	}
	return false
}

func createTestNamespace(client *kubernetes.Clientset, namespaceName string) (*v1.Namespace, error) {
	ns, err := client.CoreV1().Namespaces().Create(&v1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: namespaceName,
		},
	})
	return ns, err
}

// on success returns serviceName, secretName, nil
func createServingCertAnnotatedService(client *kubernetes.Clientset, secretName, serviceName, namespace string) error {
	_, err := client.CoreV1().Services(namespace).Create(&v1.Service{
		TypeMeta: metav1.TypeMeta{},
		ObjectMeta: metav1.ObjectMeta{
			Name: serviceName,
			Annotations: map[string]string{
				api.ServingCertSecretAnnotation: secretName,
			},
		},
		Spec: v1.ServiceSpec{
			Ports: []v1.ServicePort{
				{
					Name: "tests",
					Port: 8443,
				},
			},
		},
	})
	return err
}

func pollForServiceServingSecret(client *kubernetes.Clientset, secretName, namespace string) error {
	return wait.PollImmediate(time.Second, 10*time.Second, func() (bool, error) {
		_, err := client.CoreV1().Secrets(namespace).Get(secretName, metav1.GetOptions{})
		if err != nil && errors.IsNotFound(err) {
			return false, nil
		}
		if err != nil {
			return false, err
		}
		return true, nil
	})
}

func cleanupServiceSignerTestObjects(client *kubernetes.Clientset, secretName, serviceName, namespace string) {
	client.CoreV1().Secrets(namespace).Delete(secretName, &metav1.DeleteOptions{})
	client.CoreV1().Services(namespace).Delete(serviceName, &metav1.DeleteOptions{})
	client.CoreV1().Namespaces().Delete(namespace, &metav1.DeleteOptions{})
	// TODO this should just delete the namespace and wait for it to be gone
	// it should probably fail the test if the namespace gets stuck
}

func TestE2E(t *testing.T) {
	// use /tmp/admin.conf (placed by ci-operator) or KUBECONFIG env
	confPath := "/tmp/admin.conf"
	if conf := os.Getenv("KUBECONFIG"); conf != "" {
		confPath = conf
	}

	// load client
	client, err := clientcmd.LoadFromFile(confPath)
	if err != nil {
		t.Fatalf("error loading config: %v", err)
	}
	adminConfig, err := clientcmd.NewDefaultClientConfig(*client, &clientcmd.ConfigOverrides{}).ClientConfig()
	if err != nil {
		t.Fatalf("error loading admin config: %v", err)
	}
	adminClient, err := kubernetes.NewForConfig(adminConfig)
	if err != nil {
		t.Fatalf("error getting admin client: %v", err)
	}

	// the service-serving-cert-operator and controllers should be running as a stock OpenShift component. our first test is to
	// verify that all of the components are running.
	if !hasPodWithPrefixName(adminClient, serviceCAOperatorPodPrefix, serviceCAOperatorNamespace) {
		t.Fatalf("%s not running in %s namespace", serviceCAOperatorPodPrefix, serviceCAOperatorNamespace)
	}
	if !hasPodWithPrefixName(adminClient, apiInjectorPodPrefix, serviceCAControllerNamespace) {
		t.Fatalf("%s not running in %s namespace", apiInjectorPodPrefix, serviceCAControllerNamespace)
	}
	if !hasPodWithPrefixName(adminClient, configMapInjectorPodPrefix, serviceCAControllerNamespace) {
		t.Fatalf("%s not running in %s namespace", configMapInjectorPodPrefix, serviceCAControllerNamespace)
	}
	if !hasPodWithPrefixName(adminClient, caControllerPodPrefix, serviceCAControllerNamespace) {
		t.Fatalf("%s not running in %s namespace", caControllerPodPrefix, serviceCAControllerNamespace)
	}

	// test the main feature. annotate service -> created secret
	t.Run("serving-cert-annotation", func(t *testing.T) {
		ns, err := createTestNamespace(adminClient, "test-"+randSeq(5))
		if err != nil {
			t.Fatalf("could not create test namespace: %v", err)
		}
		testServiceName := "test-service-" + randSeq(5)
		testSecretName := "test-secret-" + randSeq(5)
		defer cleanupServiceSignerTestObjects(adminClient, testSecretName, testServiceName, ns.Name)

		err = createServingCertAnnotatedService(adminClient, testSecretName, testServiceName, ns.Name)
		if err != nil {
			t.Fatalf("error creating annotated service: %v", err)
		}

		err = pollForServiceServingSecret(adminClient, testSecretName, ns.Name)
		if err != nil {
			t.Fatalf("error fetching created serving cert secret: %v", err)
		}
	})

	// TODO: additional tests
	// - configmap CA bundle injection
	// - API service CA bundle injection
	// - regenerate serving cert
}

func init() {
	rand.Seed(time.Now().UnixNano())
}

var characters = []rune("abcdefghijklmnopqrstuvwxyz0123456789")

// TODO drop this and just use generate name
// used for random suffix
func randSeq(n int) string {
	b := make([]rune, n)
	for i := range b {
		b[i] = characters[rand.Intn(len(characters))]
	}
	return string(b)
}
