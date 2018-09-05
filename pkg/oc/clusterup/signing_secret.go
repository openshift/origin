package clusterup

import (
	"io/ioutil"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	coreclientv1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/rest"
)

const (
	serviceCANamespace  = "openshift-service-cert-signer"
	serviceCASecretName = "service-serving-cert-signer-signing-key"
)

func createServiceCASigningSecret(clientConfig *rest.Config, certFile, keyFile string) error {
	cert, err := ioutil.ReadFile(certFile)
	if err != nil {
		return err
	}

	key, err := ioutil.ReadFile(keyFile)
	if err != nil {
		return err
	}

	client, err := coreclientv1.NewForConfig(clientConfig)
	if err != nil {
		return err
	}

	ns := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: serviceCANamespace,
			Labels: map[string]string{
				"openshift.io/run-level": "1",
			},
			Annotations: nil,
		},
	}
	_, err = client.Namespaces().Create(ns)
	if err != nil && !errors.IsAlreadyExists(err) {
		return err
	}

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: serviceCANamespace,
			Name:      serviceCASecretName,
		},
		Type: "kubernetes.io/tls",
		Data: map[string][]byte{
			"tls.crt": cert,
			"tls.key": key,
		},
	}

	_, err = client.Secrets(serviceCANamespace).Create(secret)
	if errors.IsAlreadyExists(err) {
		return nil
	}
	return err
}
