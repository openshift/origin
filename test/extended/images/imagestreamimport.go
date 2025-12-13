package images

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
	"strings"
	"time"

	"github.com/google/uuid"
	g "github.com/onsi/ginkgo/v2"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/util/retry"
	admissionapi "k8s.io/pod-security-admission/api"

	configv1 "github.com/openshift/api/config/v1"
	imagev1 "github.com/openshift/api/image/v1"

	exutil "github.com/openshift/origin/test/extended/util"
	"github.com/openshift/origin/test/extended/util/image"
)

var _ = g.Describe("[sig-imageregistry][Feature:ImageStreamImport][Serial][Slow] ImageStream API [apigroup:config.openshift.io]", func() {
	defer g.GinkgoRecover()
	oc := exutil.NewCLIWithPodSecurityLevel("imagestream-api", admissionapi.LevelBaseline)
	g.BeforeEach(func() {
		if err := deployEphemeralImageRegistry(oc); err != nil {
			g.GinkgoT().Fatalf("error deploying ephemeral registry: %s", err)
		}
	})

	g.AfterEach(func() {
		// awaits until all cluster operators are available
		if err := wait.PollImmediate(30*time.Second, 10*time.Minute, func() (bool, error) {
			coList, err := oc.AdminConfigClient().ConfigV1().ClusterOperators().List(
				context.Background(), metav1.ListOptions{},
			)
			if err != nil {
				g.GinkgoT().Error("error fetching list of cluster operators: %v", err)
				return false, nil
			}
			for _, operator := range coList.Items {
				for _, cond := range operator.Status.Conditions {
					stable := true
					switch cond.Type {
					case configv1.OperatorAvailable:
						stable = cond.Status == configv1.ConditionTrue
					case configv1.OperatorProgressing:
						stable = cond.Status == configv1.ConditionFalse
					case configv1.OperatorDegraded:
						stable = cond.Status == configv1.ConditionFalse
					}
					if !stable {
						g.GinkgoT().Logf("operator %s not stable, condition: %v", operator.Name, cond)
						return false, nil
					}
				}
			}
			return true, nil
		}); err != nil {
			g.GinkgoT().Error("error waiting for operators: %v")
		}
	})

	g.It("TestImportImageFromInsecureRegistry [apigroup:image.openshift.io]", g.Label("Size:M"), func() {
		TestImportImageFromInsecureRegistry(oc)
	})
	g.It("TestImportImageFromBlockedRegistry [apigroup:image.openshift.io]", g.Label("Size:M"), func() {
		TestImportImageFromBlockedRegistry(oc)
	})
	g.It("TestImportRepositoryFromInsecureRegistry [apigroup:image.openshift.io]", g.Label("Size:M"), func() {
		TestImportRepositoryFromInsecureRegistry(oc)
	})
	g.It("TestImportRepositoryFromBlockedRegistry [apigroup:image.openshift.io]", g.Label("Size:M"), func() {
		TestImportRepositoryFromBlockedRegistry(oc)
	})
})

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

// createRegistryService creates a service pointing to deployed ephemeral image registry.
func createRegistryService(oc *exutil.CLI, selector map[string]string) error {
	t := g.GinkgoT()
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
			t.Log("endpoint for image registry service not found, retrying")
			return false, nil
		default:
			return true, fmt.Errorf("error getting registry service endpoint: %s", err)
		}
	})
}

// deployEphemeralImageRegistry deploys an ephemeral image registry instance using self signed
// certificates, a service is created pointing to image registry. This function awaits until
// the deployment is complete. Registry is configured without authentication.
func deployEphemeralImageRegistry(oc *exutil.CLI) error {
	var replicas int32 = 1

	t := g.GinkgoT()
	secret, err := createRegistryCASecret(oc)
	if err != nil {
		return fmt.Errorf("error creating registry secret: %v", err)
	}

	volumeProjection := corev1.VolumeProjection{
		Secret: &corev1.SecretProjection{
			LocalObjectReference: corev1.LocalObjectReference{
				Name: secret.Name,
			},
		},
	}

	volumes := []corev1.Volume{
		{
			Name: "tls",
			VolumeSource: corev1.VolumeSource{
				Projected: &corev1.ProjectedVolumeSource{
					Sources: []corev1.VolumeProjection{volumeProjection},
				},
			},
		},
	}

	containers := []corev1.Container{
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
			VolumeMounts: []corev1.VolumeMount{
				{
					Name:      "tls",
					MountPath: "/certs",
				},
			},
			Env: []corev1.EnvVar{
				{
					Name:  "REGISTRY_HTTP_TLS_CERTIFICATE",
					Value: "/certs/domain.crt",
				},
				{
					Name:  "REGISTRY_HTTP_TLS_KEY",
					Value: "/certs/domain.key",
				},
			},
		},
	}

	deploy, err := oc.AdminKubeClient().AppsV1().Deployments(oc.Namespace()).Create(
		context.Background(),
		&appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{
				Name:   "image-registry",
				Labels: map[string]string{"app": "image-registry"},
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
						Containers: containers,
						Volumes:    volumes,
					},
				},
			},
		},
		metav1.CreateOptions{},
	)
	if err != nil {
		return fmt.Errorf("error creating registry deploy: %s", err)
	}

	t.Log("awaiting for registry deployment to rollout")
	if err := wait.Poll(5*time.Second, 5*time.Minute, func() (stop bool, err error) {
		deploy, err := oc.AdminKubeClient().AppsV1().Deployments(oc.Namespace()).Get(
			context.Background(), deploy.Name, metav1.GetOptions{},
		)
		if err != nil {
			return false, err
		}
		succeed := deploy.Status.AvailableReplicas == replicas
		if !succeed {
			t.Logf("registry deployment not ready yet, status: %+v", deploy.Status)
		}
		return succeed, nil
	}); err != nil {
		return fmt.Errorf("error awaiting registry deploy: %s", err)
	}
	t.Log("registry deployment available, moving on")

	return createRegistryService(oc, map[string]string{"app": "image-registry"})
}

// TestImportImageFromInsecureRegistry verifies api capability of importing images from insecure
// remote image registries. We support two ways of setting a registry as inscure: by setting it
// as insecure directly on an ImageStreamImport request or by setting it as insecure globally by
// adding it to InsecureRegistry on images.config.openshift.io/cluster.
func TestImportImageFromInsecureRegistry(oc *exutil.CLI) {
	t := g.GinkgoT()

	// at this stage our ephemeral registry is available at image-registry.project:5000,
	// as it uses a self signed certificate if we request a stream import from it API should
	// fail with a certificate error.
	ephemeralRegistry := fmt.Sprintf("image-registry.%s:5000", oc.Namespace())
	imageOnRegistry := fmt.Sprintf("%s/invalid/invalid", ephemeralRegistry)
	baseISI := &imagev1.ImageStreamImport{
		ObjectMeta: metav1.ObjectMeta{
			Name: fmt.Sprintf("stream-import-test-%s", uuid.New().String()),
		},
		Spec: imagev1.ImageStreamImportSpec{
			Import: true,
			Images: []imagev1.ImageImportSpec{
				{
					IncludeManifest: true,
					From: corev1.ObjectReference{
						Kind: "DockerImage",
						Name: imageOnRegistry,
					},
				},
			},
		},
	}
	isi, err := oc.AdminImageClient().ImageV1().ImageStreamImports(oc.Namespace()).Create(
		context.Background(), baseISI, metav1.CreateOptions{},
	)
	if err != nil {
		t.Fatalf("error creating image stream import: %v", err)
	}

	// check if we fail with certificate error as expected.
	imgImportStatus := isi.Status.Images[0].Status
	if imgImportStatus.Status != "Failure" {
		t.Errorf("wrong status for insecure import: %s", imgImportStatus.Status)
	}
	if !strings.Contains(imgImportStatus.Message, "certificate is not valid") {
		t.Errorf("wrong message for insecure import: %s", imgImportStatus.Message)
	}

	// test now by setting the remote registry as "insecure" on ImageStreamImport.
	baseISI.Name = fmt.Sprintf("stream-import-test-%s", uuid.New().String())
	baseISI.Spec.Images[0].ImportPolicy.Insecure = true
	isi, err = oc.AdminImageClient().ImageV1().ImageStreamImports(oc.Namespace()).Create(
		context.Background(), baseISI, metav1.CreateOptions{},
	)
	if err != nil {
		t.Fatalf("error creating image stream import: %v", err)
	}

	// we also expect a failure here but now it should not be related to certificates but
	// NotFound instead (the ephemeral registry does not know our invalid image).
	imgImportStatus = isi.Status.Images[0].Status
	if imgImportStatus.Status != "Failure" {
		t.Errorf("wrong status for insecure import: %s", imgImportStatus.Status)
	}
	if imgImportStatus.Reason != "NotFound" {
		t.Errorf("invalid reason for insecure import: %s", imgImportStatus.Reason)
	}

	// finally we add our ephemeral registry as insecure globally.
	if err := retry.RetryOnConflict(retry.DefaultBackoff, func() error {
		imageConfig, err := oc.AdminConfigClient().ConfigV1().Images().Get(
			context.Background(), "cluster", metav1.GetOptions{},
		)
		if err != nil {
			return err
		}
		imageConfig.Spec.RegistrySources.InsecureRegistries = []string{ephemeralRegistry}
		_, err = oc.AdminConfigClient().ConfigV1().Images().Update(
			context.Background(), imageConfig, metav1.UpdateOptions{},
		)
		return err
	}); err != nil {
		t.Errorf("error adding registry to insecure: %v", err)
	}
	defer func() {
		// remove our ephemeral registry as "insecure" globally.
		if err := retry.RetryOnConflict(retry.DefaultBackoff, func() error {
			imageConfig, err := oc.AdminConfigClient().ConfigV1().Images().Get(
				context.Background(), "cluster", metav1.GetOptions{},
			)
			if err != nil {
				return err
			}
			imageConfig.Spec.RegistrySources.InsecureRegistries = []string{}
			_, err = oc.AdminConfigClient().ConfigV1().Images().Update(
				context.Background(), imageConfig, metav1.UpdateOptions{},
			)
			return err
		}); err != nil {
			t.Errorf("error removing registry from insecure: %v", err)
		}
	}()

	// test one more time, now with the registry configured as insecure globally.
	baseISI.Name = fmt.Sprintf("stream-import-test-%s", uuid.New().String())
	baseISI.Spec.Images[0].ImportPolicy.Insecure = false
	isi, err = oc.AdminImageClient().ImageV1().ImageStreamImports(oc.Namespace()).Create(
		context.Background(), baseISI, metav1.CreateOptions{},
	)
	if err != nil {
		t.Fatalf("error creating image stream import: %v", err)
	}

	// we also expect a failure here but now it should not be related to certificates but
	// NotFound instead (the ephemeral registry does not know our invalid image).
	imgImportStatus = isi.Status.Images[0].Status
	if imgImportStatus.Status != "Failure" {
		t.Errorf("wrong status for insecure import: %s", imgImportStatus.Status)
	}
	if imgImportStatus.Reason != "NotFound" {
		t.Errorf("invalid reason for insecure import: %s", imgImportStatus.Reason)
	}
}

// TestImportImageFromBlockedRegistry verifies users can't import images from a registry that
// is configured as blocked through images.config.openshift.io/cluster.
func TestImportImageFromBlockedRegistry(oc *exutil.CLI) {
	t := g.GinkgoT()

	// at this stage our ephemeral registry is available at image-registry.project:5000,
	// as it uses a self signed certificate if we request a stream import from it API should
	// fail with a certificate error.
	ephemeralRegistry := fmt.Sprintf("image-registry.%s:5000", oc.Namespace())
	imageOnRegistry := fmt.Sprintf("%s/invalid/invalid", ephemeralRegistry)
	baseISI := &imagev1.ImageStreamImport{
		ObjectMeta: metav1.ObjectMeta{
			Name: fmt.Sprintf("stream-import-test-%s", uuid.New().String()),
		},
		Spec: imagev1.ImageStreamImportSpec{
			Import: true,
			Images: []imagev1.ImageImportSpec{
				{
					IncludeManifest: true,
					ImportPolicy: imagev1.TagImportPolicy{
						Insecure: true,
					},
					From: corev1.ObjectReference{
						Kind: "DockerImage",
						Name: imageOnRegistry,
					},
				},
			},
		},
	}
	isi, err := oc.AdminImageClient().ImageV1().ImageStreamImports(oc.Namespace()).Create(
		context.Background(), baseISI, metav1.CreateOptions{},
	)
	if err != nil {
		t.Fatalf("error creating image stream import: %v", err)
	}

	// we expect it to fail as ephemeral registry does not contain the image.
	imgImportStatus := isi.Status.Images[0].Status
	if imgImportStatus.Status != "Failure" {
		t.Errorf("wrong status for insecure import: %s", imgImportStatus.Status)
	}
	if imgImportStatus.Reason != "NotFound" {
		t.Errorf("invalid reason for insecure import: %s", imgImportStatus.Reason)
	}

	// add our ephemeral registry as blocked globally.
	if err := retry.RetryOnConflict(retry.DefaultBackoff, func() error {
		imageConfig, err := oc.AdminConfigClient().ConfigV1().Images().Get(
			context.Background(), "cluster", metav1.GetOptions{},
		)
		if err != nil {
			return err
		}
		imageConfig.Spec.RegistrySources.BlockedRegistries = []string{ephemeralRegistry}
		_, err = oc.AdminConfigClient().ConfigV1().Images().Update(
			context.Background(), imageConfig, metav1.UpdateOptions{},
		)
		return err
	}); err != nil {
		t.Errorf("error adding registry to insecure: %v", err)
	}
	defer func() {
		// remove our ephemeral registry as blocked.
		if err := retry.RetryOnConflict(retry.DefaultBackoff, func() error {
			imageConfig, err := oc.AdminConfigClient().ConfigV1().Images().Get(
				context.Background(), "cluster", metav1.GetOptions{},
			)
			if err != nil {
				return err
			}
			imageConfig.Spec.RegistrySources.BlockedRegistries = []string{}
			_, err = oc.AdminConfigClient().ConfigV1().Images().Update(
				context.Background(), imageConfig, metav1.UpdateOptions{},
			)
			return err
		}); err != nil {
			t.Errorf("error removing registry from insecure: %v", err)
		}
	}()

	// test one more time, now with the registry configured as blocked.
	baseISI.Name = fmt.Sprintf("stream-import-test-%s", uuid.New().String())
	isi, err = oc.AdminImageClient().ImageV1().ImageStreamImports(oc.Namespace()).Create(
		context.Background(), baseISI, metav1.CreateOptions{},
	)
	if err != nil {
		t.Fatalf("error creating image stream import: %v", err)
	}

	// we also expect a failure here but now it should not be related to the image not
	// existing on the ephemeral registry but Forbidden instead (the ephemeral registry
	// is blocked).
	imgImportStatus = isi.Status.Images[0].Status
	if imgImportStatus.Status != "Failure" {
		t.Errorf("wrong status for insecure import: %s", imgImportStatus.Status)
	}
	if imgImportStatus.Reason != "Forbidden" {
		t.Errorf("invalid reason for insecure import: %s", imgImportStatus.Reason)
	}
}

// TestImportRepositoryFromBlockedRegistry verifies users can't import repositories from a
// registry that is configured as blocked through images.config.openshift.io/cluster.
func TestImportRepositoryFromBlockedRegistry(oc *exutil.CLI) {
	t := g.GinkgoT()

	// at this stage our ephemeral registry is available at image-registry.project:5000,
	// as it uses a self signed certificate if we request a stream import from it API should
	// fail with a certificate error.
	ephemeralRegistry := fmt.Sprintf("image-registry.%s:5000", oc.Namespace())
	imageOnRegistry := fmt.Sprintf("%s/invalid/invalid", ephemeralRegistry)
	baseISI := &imagev1.ImageStreamImport{
		ObjectMeta: metav1.ObjectMeta{
			Name: fmt.Sprintf("stream-import-test-%s", uuid.New().String()),
		},
		Spec: imagev1.ImageStreamImportSpec{
			Import: true,
			Repository: &imagev1.RepositoryImportSpec{
				IncludeManifest: true,
				ImportPolicy: imagev1.TagImportPolicy{
					Insecure: true,
				},
				From: corev1.ObjectReference{
					Kind: "DockerImage",
					Name: imageOnRegistry,
				},
			},
		},
	}
	isi, err := oc.AdminImageClient().ImageV1().ImageStreamImports(oc.Namespace()).Create(
		context.Background(), baseISI, metav1.CreateOptions{},
	)
	if err != nil {
		t.Fatalf("error creating image stream import: %v", err)
	}

	// we expect it to fail as ephemeral registry does not contain the repository.
	repoImportStatus := isi.Status.Repository.Status
	if repoImportStatus.Status != "Failure" {
		t.Errorf("wrong status for insecure import: %s", repoImportStatus.Status)
	}
	if repoImportStatus.Reason != "NotFound" {
		t.Errorf("invalid reason for insecure import: %s", repoImportStatus.Reason)
	}

	// add our ephemeral registry as blocked globally.
	if err := retry.RetryOnConflict(retry.DefaultBackoff, func() error {
		imageConfig, err := oc.AdminConfigClient().ConfigV1().Images().Get(
			context.Background(), "cluster", metav1.GetOptions{},
		)
		if err != nil {
			return err
		}
		imageConfig.Spec.RegistrySources.BlockedRegistries = []string{ephemeralRegistry}
		_, err = oc.AdminConfigClient().ConfigV1().Images().Update(
			context.Background(), imageConfig, metav1.UpdateOptions{},
		)
		return err
	}); err != nil {
		t.Errorf("error adding registry to insecure: %v", err)
	}
	defer func() {
		// remove our ephemeral registry as blocked.
		if err := retry.RetryOnConflict(retry.DefaultBackoff, func() error {
			imageConfig, err := oc.AdminConfigClient().ConfigV1().Images().Get(
				context.Background(), "cluster", metav1.GetOptions{},
			)
			if err != nil {
				return err
			}
			imageConfig.Spec.RegistrySources.BlockedRegistries = []string{}
			_, err = oc.AdminConfigClient().ConfigV1().Images().Update(
				context.Background(), imageConfig, metav1.UpdateOptions{},
			)
			return err
		}); err != nil {
			t.Errorf("error removing registry from insecure: %v", err)
		}
	}()

	// test one more time, now with the registry configured as blocked.
	baseISI.Name = fmt.Sprintf("stream-import-test-%s", uuid.New().String())
	isi, err = oc.AdminImageClient().ImageV1().ImageStreamImports(oc.Namespace()).Create(
		context.Background(), baseISI, metav1.CreateOptions{},
	)
	if err != nil {
		t.Fatalf("error creating image stream import: %v", err)
	}

	// we also expect a failure here but now it should not be related to the repository not
	// existing on the ephemeral registry but Forbidden instead (the ephemeral registry is
	// blocked).
	repoImportStatus = isi.Status.Repository.Status
	if repoImportStatus.Status != "Failure" {
		t.Errorf("wrong status for insecure import: %s", repoImportStatus.Status)
	}
	if repoImportStatus.Reason != "Forbidden" {
		t.Errorf("invalid reason for insecure import: %s", repoImportStatus.Reason)
	}
}

// TestImportRepositoryFromInsecureRegistry verifies api capability of importing a repository from
// insecure remote registries. We support two ways of setting a registry as insecure: by setting
// it as insecure directly on an ImageStreamImport request or by setting it as insecure globally
// by adding it to InsecureRegistry config on images.config.openshift.io/cluster.
func TestImportRepositoryFromInsecureRegistry(oc *exutil.CLI) {
	t := g.GinkgoT()

	// at this stage our ephemeral registry is available at image-registry.project:5000,
	// as it uses a self signed certificate if we request a stream import from it API should
	// fail with a certificate error.
	ephemeralRegistry := fmt.Sprintf("image-registry.%s:5000", oc.Namespace())
	imageOnRegistry := fmt.Sprintf("%s/invalid/invalid", ephemeralRegistry)
	baseISI := &imagev1.ImageStreamImport{
		ObjectMeta: metav1.ObjectMeta{
			Name: fmt.Sprintf("stream-import-test-%s", uuid.New().String()),
		},
		Spec: imagev1.ImageStreamImportSpec{
			Import: true,
			Repository: &imagev1.RepositoryImportSpec{
				From: corev1.ObjectReference{
					Kind: "DockerImage",
					Name: imageOnRegistry,
				},
			},
		},
	}
	isi, err := oc.AdminImageClient().ImageV1().ImageStreamImports(oc.Namespace()).Create(
		context.Background(), baseISI, metav1.CreateOptions{},
	)
	if err != nil {
		t.Fatalf("error creating image stream import: %v", err)
	}

	// check if we fail with certificate error as expected.
	repoImportStatus := isi.Status.Repository.Status
	if repoImportStatus.Status != "Failure" {
		t.Errorf("wrong status for insecure import: %s", repoImportStatus.Status)
	}
	if !strings.Contains(repoImportStatus.Message, "certificate is not valid") {
		t.Errorf("wrong message for insecure import: %s", repoImportStatus.Message)
	}

	// test now by setting the remote registry as "insecure" on ImageStreamImport.
	baseISI.Name = fmt.Sprintf("stream-import-test-%s", uuid.New().String())
	baseISI.Spec.Repository.ImportPolicy.Insecure = true
	isi, err = oc.AdminImageClient().ImageV1().ImageStreamImports(oc.Namespace()).Create(
		context.Background(), baseISI, metav1.CreateOptions{},
	)
	if err != nil {
		t.Fatalf("error creating image stream import: %v", err)
	}

	// we also expect a failure here but now it should not be related to certificates but
	// NotFound instead (the ephemeral registry does not know our invalid repository).
	repoImportStatus = isi.Status.Repository.Status
	if repoImportStatus.Status != "Failure" {
		t.Errorf("wrong status for insecure import: %s", repoImportStatus.Status)
	}
	if repoImportStatus.Reason != "NotFound" {
		t.Errorf("invalid reason for insecure import: %s", repoImportStatus.Reason)
	}

	// finally we add our ephemeral registry as insecure globally.
	if err := retry.RetryOnConflict(retry.DefaultBackoff, func() error {
		imageConfig, err := oc.AdminConfigClient().ConfigV1().Images().Get(
			context.Background(), "cluster", metav1.GetOptions{},
		)
		if err != nil {
			return err
		}
		imageConfig.Spec.RegistrySources.InsecureRegistries = []string{ephemeralRegistry}
		_, err = oc.AdminConfigClient().ConfigV1().Images().Update(
			context.Background(), imageConfig, metav1.UpdateOptions{},
		)
		return err
	}); err != nil {
		t.Errorf("error adding registry to insecure: %v", err)
	}
	defer func() {
		// remove our ephemeral registry as "insecure" globally.
		if err := retry.RetryOnConflict(retry.DefaultBackoff, func() error {
			imageConfig, err := oc.AdminConfigClient().ConfigV1().Images().Get(
				context.Background(), "cluster", metav1.GetOptions{},
			)
			if err != nil {
				return err
			}
			imageConfig.Spec.RegistrySources.InsecureRegistries = []string{}
			_, err = oc.AdminConfigClient().ConfigV1().Images().Update(
				context.Background(), imageConfig, metav1.UpdateOptions{},
			)
			return err
		}); err != nil {
			t.Errorf("error removing registry from insecure: %v", err)
		}
	}()

	// test one more time, now with the registry configured as insecure globally.
	baseISI.Name = fmt.Sprintf("stream-import-test-%s", uuid.New().String())
	baseISI.Spec.Repository.ImportPolicy.Insecure = false
	isi, err = oc.AdminImageClient().ImageV1().ImageStreamImports(oc.Namespace()).Create(
		context.Background(), baseISI, metav1.CreateOptions{},
	)
	if err != nil {
		t.Fatalf("error creating image stream import: %v", err)
	}

	// we also expect a failure here but now it should not be related to certificates but
	// NotFound instead (the ephemeral registry does not know our invalid repository).
	repoImportStatus = isi.Status.Repository.Status
	if repoImportStatus.Status != "Failure" {
		t.Errorf("wrong status for insecure import: %s", repoImportStatus.Status)
	}
	if repoImportStatus.Reason != "NotFound" {
		t.Errorf("invalid reason for insecure import: %s", repoImportStatus.Reason)
	}
}
