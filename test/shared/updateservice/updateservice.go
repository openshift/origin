// Package updateservice deploys a fake Cincinnati update service (a pod
// serving a hand-built update graph over HTTP) into a test namespace, so
// tests can point ClusterVersion.spec.upstream at it and have the CVO
// reconcile arbitrary graph contents into status.availableUpdates and
// status.conditionalUpdates.
//
// The package is deliberately framework-agnostic and dependency-restricted
// so that external openshift-tests-extension (OTE) suites can import it
// without dragging in origin's test framework: it must only use the standard
// library and the k8s.io/{api,apimachinery,client-go} modules, and must not
// import ginkgo/gomega, origin's exutil, or anything that transitively
// reaches k8s.io/kubernetes.
package updateservice

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/url"
	"strconv"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
)

// DefaultShellImage is a pull spec any pod on an OpenShift cluster can pull
// and that contains python3, which the update service serves the graph with.
const DefaultShellImage = "image-registry.openshift-image-registry.svc:5000/openshift/tools:latest"

const (
	deploymentLabel = "test-update-service"
	containerName   = "update-service"
	containerPort   = 8000
	graphPath       = "graph"
	graphVolumeName = "graph"
	graphMountPath  = "/var/lib/updateservice"

	pollInterval = 2 * time.Second
	readyTimeout = 200 * time.Second
)

// UpdateService is a fake Cincinnati update service deployed by Run.
type UpdateService struct {
	// Name is the shared name of the backing Deployment and Service.
	Name string
	// URL is the in-cluster URL of the graph endpoint, suitable as a
	// ClusterVersion spec.upstream value.
	URL *url.URL

	client        kubernetes.Interface
	namespace     string
	configMapName string
}

// Run creates a ConfigMap holding the marshaled graph, a Deployment serving
// it, and a Service in namespace, and waits for the deployment to become
// ready. shellImage must contain python3; pass "" to use DefaultShellImage.
// If Run fails after creating resources, it deletes them before returning
// and joins any cleanup error into the returned error. Cleanup on the
// returned UpdateService removes the created resources; deleting the
// namespace also works.
func Run(ctx context.Context, client kubernetes.Interface, namespace, shellImage string, graph Graph) (_ *UpdateService, err error) {
	if shellImage == "" {
		shellImage = DefaultShellImage
	}
	payload, err := json.Marshal(graph)
	if err != nil {
		return nil, fmt.Errorf("marshaling update graph: %w", err)
	}

	// The graph is delivered as a ConfigMap volume rather than interpolated
	// into the container command, so arbitrary graph contents (including
	// shell metacharacters) cannot break the serving script.
	configMap, err := client.CoreV1().ConfigMaps(namespace).Create(ctx, &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: "test-update-service-",
		},
		Data: map[string]string{
			graphPath: string(payload),
		},
	}, metav1.CreateOptions{})
	if err != nil {
		return nil, fmt.Errorf("creating update-service configmap: %w", err)
	}

	// deployment tracks how far Run got for the deferred cleanup below;
	// the Service shares the Deployment's name.
	var deployment *appsv1.Deployment

	// If Run fails after the ConfigMap is created, synchronously delete
	// everything created so far so failures do not leak resources into
	// the namespace. Cleanup runs with a context that is not cancelled
	// with Run's, because Run commonly fails by cancellation or timeout.
	// The owner references on the Deployment and Service are a safety net
	// in case this cleanup itself fails or never runs.
	defer func() {
		if err == nil {
			return
		}
		cleanupCtx := context.WithoutCancel(ctx)
		var cleanupErrs []error
		if deployment != nil {
			cleanupErrs = append(cleanupErrs,
				ignoreNotFound(client.AppsV1().Deployments(namespace).Delete(cleanupCtx, deployment.Name, metav1.DeleteOptions{})),
				ignoreNotFound(client.CoreV1().Services(namespace).Delete(cleanupCtx, deployment.Name, metav1.DeleteOptions{})),
			)
		}
		cleanupErrs = append(cleanupErrs, ignoreNotFound(client.CoreV1().ConfigMaps(namespace).Delete(cleanupCtx, configMap.Name, metav1.DeleteOptions{})))
		if cleanupErr := errors.Join(cleanupErrs...); cleanupErr != nil {
			err = errors.Join(err, fmt.Errorf("cleaning up after failed Run: %w", cleanupErr))
		}
	}()

	deployment, err = client.AppsV1().Deployments(namespace).Create(ctx, &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName:    "test-update-service-",
			OwnerReferences: configMapOwner(configMap),
		},
		Spec: appsv1.DeploymentSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app": deploymentLabel,
				},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"app": deploymentLabel,
					},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{{
						Name:  containerName,
						Image: shellImage,
						Args: []string{
							"/bin/sh",
							"-c",
							fmt.Sprintf(`cd %[1]s &&
python3 -m http.server --bind ::
`, graphMountPath),
						},
						Ports: []corev1.ContainerPort{{
							Name:          containerName,
							ContainerPort: containerPort,
						}},
						Resources: corev1.ResourceRequirements{
							Requests: corev1.ResourceList{
								corev1.ResourceCPU:    resource.MustParse("10m"),
								corev1.ResourceMemory: resource.MustParse("20Mi"),
							},
						},
						VolumeMounts: []corev1.VolumeMount{{
							Name:      graphVolumeName,
							MountPath: graphMountPath,
							ReadOnly:  true,
						}},
					}},
					Volumes: []corev1.Volume{{
						Name: graphVolumeName,
						VolumeSource: corev1.VolumeSource{
							ConfigMap: &corev1.ConfigMapVolumeSource{
								LocalObjectReference: corev1.LocalObjectReference{
									Name: configMap.Name,
								},
							},
						},
					}},
				},
			},
		},
	}, metav1.CreateOptions{})
	if err != nil {
		return nil, fmt.Errorf("creating update-service deployment: %w", err)
	}

	service, err := client.CoreV1().Services(namespace).Create(ctx, &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:            deployment.Name,
			OwnerReferences: configMapOwner(configMap),
		},
		Spec: corev1.ServiceSpec{
			Selector: deployment.Spec.Template.Labels,
			Ports: []corev1.ServicePort{{
				Name: containerName,
				Port: containerPort,
			}},
		},
	}, metav1.CreateOptions{})
	if err != nil {
		return nil, fmt.Errorf("creating update-service service: %w", err)
	}

	if err := waitForDeploymentReady(ctx, client, namespace, deployment.Name); err != nil {
		return nil, err
	}

	return &UpdateService{
		Name: deployment.Name,
		URL: &url.URL{
			Scheme: "http",
			Host:   net.JoinHostPort(service.Spec.ClusterIP, strconv.Itoa(int(service.Spec.Ports[0].Port))),
			Path:   graphPath,
		},
		client:        client,
		namespace:     namespace,
		configMapName: configMap.Name,
	}, nil
}

// Cleanup deletes the ConfigMap, Deployment, and Service created by Run.
func (s *UpdateService) Cleanup(ctx context.Context) error {
	return errors.Join(
		ignoreNotFound(s.client.AppsV1().Deployments(s.namespace).Delete(ctx, s.Name, metav1.DeleteOptions{})),
		ignoreNotFound(s.client.CoreV1().Services(s.namespace).Delete(ctx, s.Name, metav1.DeleteOptions{})),
		ignoreNotFound(s.client.CoreV1().ConfigMaps(s.namespace).Delete(ctx, s.configMapName, metav1.DeleteOptions{})),
	)
}

// configMapOwner returns an owner reference making configMap the owner of
// the Deployment and Service created by Run, so that deleting the ConfigMap
// (directly or through namespace deletion) garbage-collects them even if
// Cleanup never runs.
func configMapOwner(configMap *corev1.ConfigMap) []metav1.OwnerReference {
	return []metav1.OwnerReference{{
		APIVersion: "v1",
		Kind:       "ConfigMap",
		Name:       configMap.Name,
		UID:        configMap.UID,
	}}
}

// ignoreNotFound returns nil for API NotFound errors, which are expected
// when a resource is already gone, for example because the namespace was
// deleted first.
func ignoreNotFound(err error) error {
	if apierrors.IsNotFound(err) {
		return nil
	}
	return err
}

func waitForDeploymentReady(ctx context.Context, client kubernetes.Interface, namespace, name string) error {
	var lastErr error
	err := wait.PollUntilContextTimeout(ctx, pollInterval, readyTimeout, true, func(ctx context.Context) (bool, error) {
		deployment, err := client.AppsV1().Deployments(namespace).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			// keep polling through transient errors until the timeout
			lastErr = err
			return false, nil
		}
		replicas := int32(1)
		if deployment.Spec.Replicas != nil {
			replicas = *deployment.Spec.Replicas
		}
		return deployment.Status.AvailableReplicas == replicas, nil
	})
	if err != nil {
		if lastErr != nil {
			err = fmt.Errorf("%w; last error: %v", err, lastErr)
		}
		return fmt.Errorf("waiting for update-service deployment %s/%s to become ready: %w", namespace, name, err)
	}
	return nil
}
