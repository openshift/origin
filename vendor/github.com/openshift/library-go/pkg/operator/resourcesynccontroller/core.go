package resourcesynccontroller

import (
	"crypto/x509"
	"fmt"
	"reflect"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	corev1client "k8s.io/client-go/kubernetes/typed/core/v1"
	corev1listers "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/util/cert"

	"github.com/openshift/library-go/pkg/crypto"
	"github.com/openshift/library-go/pkg/operator/events"
)

func CombineCABundleConfigMaps(destinationConfigMap ResourceLocation, lister corev1listers.ConfigMapLister, client corev1client.ConfigMapsGetter, recorder events.Recorder, inputConfigMaps ...ResourceLocation) (*corev1.ConfigMap, error) {
	certificates := []*x509.Certificate{}
	for _, input := range inputConfigMaps {
		inputConfigMap, err := lister.ConfigMaps(input.Namespace).Get(input.Name)
		if apierrors.IsNotFound(err) {
			continue
		}
		if err != nil {
			return nil, err
		}

		// configmaps must conform to this
		inputContent := inputConfigMap.Data["ca-bundle.crt"]
		if len(inputContent) == 0 {
			continue
		}
		inputCerts, err := cert.ParseCertsPEM([]byte(inputContent))
		if err != nil {
			return nil, fmt.Errorf("configmap/%s in %q is malformed: %v", input.Name, input.Namespace, err)
		}
		certificates = append(certificates, inputCerts...)
	}

	certificates = crypto.FilterExpiredCerts(certificates...)
	finalCertificates := []*x509.Certificate{}
	// now check for duplicates. n^2, but super simple
	for i := range certificates {
		found := false
		for j := range finalCertificates {
			if reflect.DeepEqual(certificates[i].Raw, finalCertificates[j].Raw) {
				found = true
				break
			}
		}
		if !found {
			finalCertificates = append(finalCertificates, certificates[i])
		}
	}

	caBytes, err := crypto.EncodeCertificates(finalCertificates...)
	if err != nil {
		return nil, err
	}

	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Namespace: destinationConfigMap.Namespace, Name: destinationConfigMap.Name},
		Data: map[string]string{
			"ca-bundle.crt": string(caBytes),
		},
	}, nil
}
