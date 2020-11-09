package apiserver

import (
	"context"
	"fmt"
	"strings"
	"time"

	g "github.com/onsi/ginkgo"

	imagev1 "github.com/openshift/api/image/v1"
	exutil "github.com/openshift/origin/test/extended/util"

	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/apimachinery/pkg/util/uuid"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	e2e "k8s.io/kubernetes/test/e2e/framework"
	e2edeployment "k8s.io/kubernetes/test/e2e/framework/deployment"
	imageutils "k8s.io/kubernetes/test/utils/image"
	"k8s.io/utils/pointer"
)

var _ = g.Describe("[sig-api-machinery][Feature:AdmissionWebhook]", func() {
	defer g.GinkgoRecover()

	oc := exutil.NewCLI("apiserver")

	g.It("should call a validating admission webhook when accessing OpenShift API Server", func() {
		targetNamespace := oc.Namespace()
		kubeClient := oc.AdminKubeClient()
		servicePort := int32(8443)
		containerPort := int32(8444)

		annotateNamespace(kubeClient, targetNamespace, "webhook-marker")
		createService(kubeClient, targetNamespace, "openshift-test-webhook", "openshift-test-webhook", servicePort, containerPort)
		deployWebhookServer(kubeClient, imageutils.GetE2EImage(imageutils.Agnhost), targetNamespace, "openshift-test-webhook", "openshift-test-webhook", "openshift-test-webhook", containerPort)
		removeWebhookConfiguration := createValidatingWebhook(kubeClient, targetNamespace, "webhook-marker", "webhook-marker", "openshift-test-webhook", "openshift-test-webhook", servicePort)
		defer removeWebhookConfiguration()

		g.By(fmt.Sprintf("attempting to create an image stream in %s namespace (should fail)", targetNamespace))
		err := hitValidatingWebhook(func() (func(), error) {
			is := &imagev1.ImageStream{
				ObjectMeta: metav1.ObjectMeta{
					Name:   "openshift-test-webhook",
					Labels: map[string]string{"webhook-marker": "true"},
				},
			}
			_, err := oc.AdminImageClient().ImageV1().ImageStreams(targetNamespace).Create(context.TODO(), is, metav1.CreateOptions{})
			removeImageStream := func() {
				err = oc.AdminImageClient().ImageV1().ImageStreams(targetNamespace).Delete(context.TODO(), is.GetName(), metav1.DeleteOptions{})
				e2e.ExpectNoError(err, "deleting an image stream %s in namespace %s", is.Name, targetNamespace)
			}
			return removeImageStream, err

		})
		e2e.ExpectNoError(err, "failed to create an image stream, expected to get admission deny error")
	})
})

// annotateNamespace adds the label selector so that the webhook is scoped only to this namespace.
func annotateNamespace(client kubernetes.Interface, namespace string, labelSelectorKey string) {
	g.By("annotating the namespace so that the webhook is scoped to this namespace")
	actualNamespace, err := client.CoreV1().Namespaces().Get(context.Background(), namespace, metav1.GetOptions{})
	e2e.ExpectNoError(err, "unable to get namespace %s, err %v", namespace, err)
	if actualNamespace.Labels == nil {
		actualNamespace.Labels = map[string]string{}
	}
	actualNamespace.Labels[labelSelectorKey] = "true"
	_, err = client.CoreV1().Namespaces().Update(context.Background(), actualNamespace, metav1.UpdateOptions{})
	e2e.ExpectNoError(err, "unable to update namespace %s, err %v", namespace, err)
}

// createService creates a service that will route traffic to the webhook server
// note that a signed serving certificate and the key pair will be created automatically
// since the service is annotated with service.beta.openshift.io/serving-cert-secret-name
func createService(client kubernetes.Interface, namespace string, secretName string, serviceName string, servicePort int32, containerPort int32) {
	g.By("creating a service that will send traffic to the webhook server")
	serviceLabels := map[string]string{"webhook": "true"}
	service := &v1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Namespace:   namespace,
			Name:        serviceName,
			Labels:      map[string]string{"test": "webhook"},
			Annotations: map[string]string{"service.beta.openshift.io/serving-cert-secret-name": secretName},
		},
		Spec: v1.ServiceSpec{
			Selector: serviceLabels,
			Ports: []v1.ServicePort{
				{
					Protocol:   "TCP",
					Port:       servicePort,
					TargetPort: intstr.FromInt(int(containerPort)),
				},
			},
		},
	}
	_, err := client.CoreV1().Services(namespace).Create(context.Background(), service, metav1.CreateOptions{})
	e2e.ExpectNoError(err, "creating service %s in namespace %s", serviceName, namespace)
}

// deployWebhookServer deploys the webhook server by creating a deployment and verifying the wiring
func deployWebhookServer(client kubernetes.Interface, image string, namespace string, deploymentName string, secretName string, serviceName string, containerPort int32) {
	g.By("deploying the webhook server")

	podLabels := map[string]string{"app": "sample-webhook", "webhook": "true"}
	replicas := int32(1)
	zero := int64(0)
	mounts := []v1.VolumeMount{
		{
			Name:      "webhook-certs",
			ReadOnly:  true,
			MountPath: "/webhook.local.config/certificates",
		},
	}
	volumes := []v1.Volume{
		{
			Name: "webhook-certs",
			VolumeSource: v1.VolumeSource{
				Secret: &v1.SecretVolumeSource{SecretName: secretName},
			},
		},
	}
	containers := []v1.Container{
		{
			Name:         "sample-webhook",
			VolumeMounts: mounts,
			Args: []string{
				"webhook",
				"--tls-cert-file=/webhook.local.config/certificates/tls.crt",
				"--tls-private-key-file=/webhook.local.config/certificates/tls.key",
				"--alsologtostderr",
				"-v=4",
				// Use a non-default port for containers.
				fmt.Sprintf("--port=%d", containerPort),
			},
			ReadinessProbe: &v1.Probe{
				Handler: v1.Handler{
					HTTPGet: &v1.HTTPGetAction{
						Scheme: v1.URISchemeHTTPS,
						Port:   intstr.FromInt(int(containerPort)),
						Path:   "/readyz",
					},
				},
				PeriodSeconds:    1,
				SuccessThreshold: 1,
				FailureThreshold: 30,
			},
			Image: image,
			Ports: []v1.ContainerPort{{ContainerPort: containerPort}},
		},
	}
	d := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:   deploymentName,
			Labels: podLabels,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: podLabels,
			},
			Strategy: appsv1.DeploymentStrategy{
				Type: appsv1.RollingUpdateDeploymentStrategyType,
			},
			Template: v1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: podLabels,
				},
				Spec: v1.PodSpec{
					TerminationGracePeriodSeconds: &zero,
					Containers:                    containers,
					Volumes:                       volumes,
				},
			},
		},
	}

	deployment, err := client.AppsV1().Deployments(namespace).Create(context.Background(), d, metav1.CreateOptions{})
	e2e.ExpectNoError(err, "creating deployment %s in namespace %s", deploymentName, namespace)
	g.By("waiting for the deployment to be ready")
	err = e2edeployment.WaitForDeploymentRevisionAndImage(client, namespace, deploymentName, "1", image)
	e2e.ExpectNoError(err, "waiting for the deployment of image %s in %s in %s to complete", image, deploymentName, namespace)
	err = e2edeployment.WaitForDeploymentComplete(client, deployment)
	e2e.ExpectNoError(err, "waiting for the deployment status valid", image, deploymentName, namespace)

	g.By("verifying the service has paired with the endpoint")
	err = e2e.WaitForServiceEndpointsNum(client, namespace, serviceName, 1, 1*time.Second, 30*time.Second)
	e2e.ExpectNoError(err, "waiting for service %s/%s have %d endpoint", namespace, serviceName, 1)
}

// createValidatingWebhook registers a ValidatingWebhookConfiguration that points to an always-deny webook for all resources matching objectSelector and namespaceSelector
func createValidatingWebhook(client kubernetes.Interface, namespace string, namespaceSelector string, objectSelector string, configName string, serviceName string, servicePort int32) func() {
	var err error
	g.By("registering an always-deny validating webhook on all resources matching a special selector and namespace")

	failurePolicy := admissionregistrationv1.Fail
	sideEffectsNone := admissionregistrationv1.SideEffectClassNone

	// this webhook denies all requests to Create that match objectSelector and namespaceSelector
	config := &admissionregistrationv1.ValidatingWebhookConfiguration{
		ObjectMeta: metav1.ObjectMeta{
			Name:        configName,
			Annotations: map[string]string{"service.beta.openshift.io/inject-cabundle": "true"},
		},
		Webhooks: []admissionregistrationv1.ValidatingWebhook{
			{
				Name: "deny-all-webhook.openshift.io",
				Rules: []admissionregistrationv1.RuleWithOperations{{
					Operations: []admissionregistrationv1.OperationType{admissionregistrationv1.Create},
					Rule: admissionregistrationv1.Rule{
						APIGroups:   []string{"*"},
						APIVersions: []string{"*"},
						Resources:   []string{"*"},
					},
				}},
				ClientConfig: admissionregistrationv1.WebhookClientConfig{
					Service: &admissionregistrationv1.ServiceReference{
						Namespace: namespace,
						Name:      serviceName,
						Path:      strPtr("/always-deny"),
						Port:      pointer.Int32Ptr(servicePort),
					},
				},
				SideEffects:             &sideEffectsNone,
				AdmissionReviewVersions: []string{"v1", "v1beta1"},
				FailurePolicy:           &failurePolicy,
				NamespaceSelector: &metav1.LabelSelector{
					MatchLabels: map[string]string{namespaceSelector: "true"},
				},
				ObjectSelector: &metav1.LabelSelector{
					MatchLabels: map[string]string{objectSelector: "true"},
				},
			},
		},
	}
	_, err = client.AdmissionregistrationV1().ValidatingWebhookConfigurations().Create(context.Background(), config, metav1.CreateOptions{})
	e2e.ExpectNoError(err, "creating webhook config %s in namespace %s", configName, namespace)

	removeWebhookConfiguration := func() {
		err := client.AdmissionregistrationV1().ValidatingWebhookConfigurations().Delete(context.Background(), configName, metav1.DeleteOptions{})
		e2e.ExpectNoError(err, "deleting webhook config %s in namespace %s", configName, namespace)
	}

	err = hitValidatingWebhook(func() (func(), error) {
		cmClient := client.CoreV1().ConfigMaps(namespace)
		cm := &v1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name: string(uuid.NewUUID()),
				Labels: map[string]string{
					objectSelector: "true",
				},
			},
		}
		_, err := cmClient.Create(context.Background(), cm, metav1.CreateOptions{})
		removeConfigMap := func() {
			err = cmClient.Delete(context.Background(), cm.GetName(), metav1.DeleteOptions{})
			e2e.ExpectNoError(err, "deleting config map %s in namespace %s", cm.Name, namespace)
		}

		return removeConfigMap, err
	})
	if err != nil {
		removeWebhookConfiguration()
		e2e.ExpectNoError(err, "calling validating admission webhook")
	}
	return removeWebhookConfiguration
}

// hitValidatingWebhook tries to create a resource and expect it to fail (deny)
// it tries to send a few request before giving up because registering a webhook is not instant and might take up to a few seconds
func hitValidatingWebhook(callback func() (func(), error)) error {
	return wait.PollImmediate(100*time.Millisecond, 30*time.Second, func() (bool, error) {
		cleanup, err := callback()
		if err != nil {
			// the always-deny webhook does not provide a reason, so check for the error string we expect
			if strings.Contains(err.Error(), "denied") {
				return true, nil
			}
			return false, err
		}
		cleanup()
		e2e.Logf("Calling webhook succeeded but it should fail trying one more time...")
		return false, nil
	})
}

func strPtr(s string) *string { return &s }
