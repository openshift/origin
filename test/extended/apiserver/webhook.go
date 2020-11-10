package apiserver

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	g "github.com/onsi/ginkgo"

	exetcd "github.com/openshift/origin/test/extended/etcd"
	exutil "github.com/openshift/origin/test/extended/util"

	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/apimachinery/pkg/util/uuid"
	"k8s.io/apimachinery/pkg/util/wait"
	discocache "k8s.io/client-go/discovery/cached"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/restmapper"
	e2e "k8s.io/kubernetes/test/e2e/framework"
	e2edeployment "k8s.io/kubernetes/test/e2e/framework/deployment"
	imageutils "k8s.io/kubernetes/test/utils/image"
	"k8s.io/utils/pointer"
)

var _ = g.Describe("[sig-api-machinery][Feature:AdmissionWebhook]", func() {
	defer g.GinkgoRecover()
	oc := exutil.NewCLI("apiserver")

	g.It("Validating Admission Webhook should be called when accessing OpenShift APIs", func() {
		targetNamespace := oc.Namespace()
		kubeClient := oc.AdminKubeClient()
		servicePort := int32(8443)
		containerPort := int32(8444)
		mapper := restmapper.NewDeferredDiscoveryRESTMapper(discocache.NewMemCacheClient(kubeClient.Discovery()))
		adminRestConfig := oc.AdminConfig()

		annotateNamespace(kubeClient, targetNamespace, "webhook-marker")
		createService(kubeClient, targetNamespace, "openshift-test-webhook", "openshift-test-webhook", servicePort, containerPort)
		deployWebhookServer(kubeClient, imageutils.GetE2EImage(imageutils.Agnhost), targetNamespace, "openshift-test-webhook", "openshift-test-webhook", "openshift-test-webhook", containerPort)
		removeWebhookConfiguration := createValidatingWebhook(kubeClient, targetNamespace, "webhook-marker", "webhook-marker", "openshift-test-webhook", "openshift-test-webhook", servicePort)
		defer removeWebhookConfiguration()

		storageData := exetcd.OpenshiftEtcdStorageData
		for key := range storageData {
			gvr := key
			data := storageData[gvr]

			// apply for core types is already well-tested, so skip
			// openshift types that are just aliases.
			aliasToCoreType := data.ExpectedGVK != nil
			if aliasToCoreType {
				continue
			}

			g.By(fmt.Sprintf("validating %v API", gvr), func() {
				for _, prerequisite := range data.Prerequisites {
					// the etcd storage test for oauthclientauthorizations needs to
					// manually create a service account secret but that is not
					// necessary (or possible) when interacting with an apiserver.
					// The service account secret will be created by the controller
					// manager.
					if gvr.Resource == "oauthclientauthorizations" && prerequisite.GvrData.Resource == "secrets" {
						continue
					}
					resourceClient, unstructuredObj, err := createRes(adminRestConfig, mapper, prerequisite.GvrData, false, prerequisite.Stub, targetNamespace)
					e2e.ExpectNoError(err, fmt.Sprintf("failed to create %s", gvr))
					defer deleteRes(resourceClient, unstructuredObj.GetName())
				}

				var (
					resourceClient  dynamic.ResourceInterface
					unstructuredObj *unstructured.Unstructured
					err             error
				)
				createResourceWrapper := func() error {
					resourceClient, unstructuredObj, err = createRes(adminRestConfig, mapper, gvr, true, data.Stub, targetNamespace)
					return err
				}
				deleteResourceWrapper := func() {
					if err == nil && resourceClient != nil && unstructuredObj != nil {
						deleteRes(resourceClient, unstructuredObj.GetName())
					}
				}

				err = hitValidatingWebhook(createResourceWrapper, deleteResourceWrapper)
				e2e.ExpectNoError(err, fmt.Sprintf("failed to create %s, expected to get admission deny error", gvr))
			})
		}
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

	var (
		cm       *v1.ConfigMap
		cmClient = client.CoreV1().ConfigMaps(namespace)
	)

	createResourceFn := func() error {
		cm = &v1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name: string(uuid.NewUUID()),
				Labels: map[string]string{
					objectSelector: "true",
				},
			},
		}
		_, err := cmClient.Create(context.Background(), cm, metav1.CreateOptions{})
		return err
	}
	deleteResourceFn := func() {
		err = cmClient.Delete(context.Background(), cm.GetName(), metav1.DeleteOptions{})
		e2e.ExpectNoError(err, "deleting config map %s in namespace %s", cm.Name, namespace)
	}

	err = hitValidatingWebhook(createResourceFn, deleteResourceFn)
	if err != nil {
		removeWebhookConfiguration()
		e2e.ExpectNoError(err, "calling validating admission webhook")
	}
	return removeWebhookConfiguration
}

// hitValidatingWebhook tries to create a resource and expect it to fail (deny)
// it tries to send a few request before giving up because registering a webhook is not instant and might take up to a few seconds
func hitValidatingWebhook(createResourceFn func() error, deleteResourceFn func()) error {
	return wait.PollImmediate(100*time.Millisecond, 30*time.Second, func() (bool, error) {
		err := createResourceFn()
		if err != nil {
			// the always-deny webhook does not provide a reason, so check for the error string we expect
			if strings.Contains(err.Error(), "denied") {
				return true, nil
			}
			return false, err
		}
		deleteResourceFn()
		e2e.Logf("Calling webhook succeeded but it should fail trying one more time...")
		return false, nil
	})
}

func createRes(restConfig *rest.Config, mapper *restmapper.DeferredDiscoveryRESTMapper, gvr schema.GroupVersionResource, withWebhookObjectSelector bool, stub, namespace string) (dynamic.ResourceInterface, *unstructured.Unstructured, error) {
	gvk, err := mapper.KindFor(gvr)
	if err != nil {
		return nil, nil, err
	}

	// supply a value for namespace if the scope requires
	mapping, err := mapper.RESTMapping(gvk.GroupKind())
	if err != nil {
		return nil, nil, err
	}

	// ensure that any stub embedding the etcd test namespace
	// is updated to use local test namespace instead.
	stub = strings.Replace(stub, exetcd.TestNamespace, namespace, -1)

	// create unstructured object from stub and set labels
	unstructuredObj := unstructured.Unstructured{}
	err = json.Unmarshal([]byte(stub), &unstructuredObj.Object)
	if err != nil {
		return nil, nil, err
	}
	unstructuredObj.SetGroupVersionKind(gvk)
	if withWebhookObjectSelector {
		unstructuredObj.SetLabels(map[string]string{"webhook-marker": "true"})
	}

	dynamicClient, err := dynamic.NewForConfig(restConfig)
	if err != nil {
		return nil, nil, err
	}

	// if the resource we are about to create is a cluster-wide skipp the namespace
	var resourceClient dynamic.ResourceInterface
	if mapping.Scope.Name() == meta.RESTScopeNameNamespace {
		resourceClient = dynamicClient.Resource(gvr).Namespace(namespace)
	} else {
		resourceClient = dynamicClient.Resource(gvr).Namespace("")
	}

	_, err = resourceClient.Create(context.Background(), &unstructuredObj, metav1.CreateOptions{})
	return resourceClient, &unstructuredObj, err
}

func deleteRes(resourceClient dynamic.ResourceInterface, name string) {
	err := resourceClient.Delete(context.Background(), name, metav1.DeleteOptions{})
	e2e.ExpectNoError(err, "Unexpected error deleting resource: %v")
}

func strPtr(s string) *string { return &s }
