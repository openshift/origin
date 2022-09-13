package ephemeralimageregistry

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"time"

	"github.com/google/uuid"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/apimachinery/pkg/util/wait"
	e2e "k8s.io/kubernetes/test/e2e/framework"

	exutil "github.com/openshift/origin/test/extended/util"
	"github.com/openshift/origin/test/extended/util/image"
)

// Option allows to configure an ephemeral image registry.
type Option interface {
	Mutate(runtime.Object) error
}

// createRegistryCASecret creates a Secret containing a self signed certificate and key.
func createRegistryCASecret(oc *exutil.CLI) (*corev1.Secret, error) {
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, err
	}

	template := x509.Certificate{
		SerialNumber:          big.NewInt(1),
		NotBefore:             time.Now(),
		NotAfter:              time.Now().Add(time.Hour),
		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
		Subject: pkix.Name{
			Organization: []string{"RedHat"},
		},
	}

	derBytes, err := x509.CreateCertificate(
		rand.Reader, &template, &template, &priv.PublicKey, priv,
	)
	if err != nil {
		return nil, err
	}

	crt := &bytes.Buffer{}
	pem.Encode(crt, &pem.Block{
		Type:  "CERTIFICATE",
		Bytes: derBytes,
	})

	key := &bytes.Buffer{}
	pem.Encode(key, &pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(priv),
	})

	sec, err := oc.AdminKubeClient().CoreV1().Secrets(oc.Namespace()).Create(
		context.Background(),
		&corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name: fmt.Sprintf("registry-ca-%s", uuid.New().String()),
			},
			Data: map[string][]byte{
				"domain.crt": crt.Bytes(),
				"domain.key": key.Bytes(),
			},
		},
		metav1.CreateOptions{},
	)
	if err != nil {
		return nil, err
	}
	return sec, nil
}

// secureOption configures the registry deployment to use TLS.
type secureOption struct {
	tlsSecretName string
	err           error
}

// Secure enables TLS for the ephemeral image registry.
func Secure(oc *exutil.CLI) Option {
	sec, err := createRegistryCASecret(oc)
	return secureOption{
		tlsSecretName: sec.Name,
		err:           err,
	}
}

func (s secureOption) Mutate(obj runtime.Object) error {
	if s.err != nil {
		return s.err
	}

	deploy, ok := obj.(*appsv1.Deployment)
	if !ok {
		return nil
	}

	deploy.Spec.Template.Spec.Volumes = append(
		deploy.Spec.Template.Spec.Volumes,
		corev1.Volume{
			Name: "tls",
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					SecretName: s.tlsSecretName,
				},
			},
		},
	)

	deploy.Spec.Template.Spec.Containers[0].VolumeMounts = append(
		deploy.Spec.Template.Spec.Containers[0].VolumeMounts,
		corev1.VolumeMount{
			Name:      "tls",
			MountPath: "/certs",
		},
	)

	deploy.Spec.Template.Spec.Containers[0].Env = append(
		deploy.Spec.Template.Spec.Containers[0].Env,
		corev1.EnvVar{
			Name:  "REGISTRY_HTTP_TLS_CERTIFICATE",
			Value: "/certs/domain.crt",
		},
		corev1.EnvVar{
			Name:  "REGISTRY_HTTP_TLS_KEY",
			Value: "/certs/domain.key",
		},
	)

	return nil
}

// createRegistryService creates a service pointing to deployed ephemeral image registry.
func createRegistryService(oc *exutil.CLI, selector map[string]string) error {
	if _, err := oc.AdminKubeClient().CoreV1().Services(oc.Namespace()).Create(
		context.Background(),
		&corev1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Name: "image-registry",
			},
			Spec: corev1.ServiceSpec{
				Ports: []corev1.ServicePort{
					{
						Port:       5000,
						TargetPort: intstr.FromInt(5000),
						Protocol:   "TCP",
					},
				},
				Selector: selector,
			},
		},
		metav1.CreateOptions{},
	); err != nil {
		return err
	}

	return wait.Poll(5*time.Second, 5*time.Minute, func() (stop bool, err error) {
		_, err = oc.AdminKubeClient().CoreV1().Endpoints(oc.Namespace()).Get(
			context.Background(), "image-registry", metav1.GetOptions{},
		)
		switch {
		case err == nil:
			return true, nil
		case errors.IsNotFound(err):
			e2e.Logf("endpoint for image registry service not found, retrying")
			return false, nil
		default:
			return true, fmt.Errorf("error getting registry service endpoint: %s", err)
		}
	})
}

// Deploy deploys an ephemeral image registry.
func Deploy(oc *exutil.CLI, opts ...Option) error {
	e2e.Logf("Deploying ephemeral image registry...")

	var replicas int32 = 1
	deploy := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name: "image-registry",
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"app": "image-registry"},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{"app": "image-registry"},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  "registry",
							Image: image.LocationFor("docker.io/library/registry:2.8.0-beta.1"),
							Ports: []corev1.ContainerPort{
								{
									ContainerPort: 5000,
									Protocol:      "TCP",
								},
							},
							ReadinessProbe: &corev1.Probe{
								PeriodSeconds:       5,
								InitialDelaySeconds: 5,
								FailureThreshold:    3,
								SuccessThreshold:    3,
								ProbeHandler: corev1.ProbeHandler{
									TCPSocket: &corev1.TCPSocketAction{
										Port: intstr.FromInt(5000),
									},
								},
							},
						},
					},
				},
			},
		},
	}
	for _, opt := range opts {
		if err := opt.Mutate(deploy); err != nil {
			return err
		}
	}

	deploy, err := oc.AdminKubeClient().AppsV1().Deployments(oc.Namespace()).Create(
		context.Background(),
		deploy,
		metav1.CreateOptions{},
	)
	if err != nil {
		return fmt.Errorf("error creating registry deploy: %s", err)
	}

	e2e.Logf("Awaiting for registry deployment to rollout...")
	if err := wait.Poll(5*time.Second, 5*time.Minute, func() (stop bool, err error) {
		deploy, err := oc.AdminKubeClient().AppsV1().Deployments(oc.Namespace()).Get(
			context.Background(), deploy.Name, metav1.GetOptions{},
		)
		if err != nil {
			return false, err
		}
		succeed := deploy.Status.AvailableReplicas == replicas
		if !succeed {
			e2e.Logf("Registry deployment not ready yet, status: %+v", deploy.Status)
		}
		return succeed, nil
	}); err != nil {
		return fmt.Errorf("error awaiting registry deploy: %s", err)
	}
	e2e.Logf("Registry deployment available, moving on")

	return createRegistryService(oc, map[string]string{"app": "image-registry"})
}
