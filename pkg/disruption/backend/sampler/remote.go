package sampler

import (
	"bytes"
	"context"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/kubernetes/test/e2e/framework"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"

	projv1 "github.com/openshift/api/project/v1"
	configclient "github.com/openshift/client-go/config/clientset/versioned"
	projectclient "github.com/openshift/client-go/project/clientset/versioned"

	"github.com/openshift/origin/pkg/monitor/monitorapi"
	monitorserialization "github.com/openshift/origin/pkg/monitor/serialization"
)

const (
	namespace            = "e2e-disruption-monitor"
	serviceAccountName   = "disruption-monitor-sa"
	varLogPath           = "/var/log"
	disruptionDataFolder = "disruption-data"
	disruptionTypeEnvVar = "DISRUPTION_TYPE_LABEL"
	inClusterEventsFile  = "junit/AdditionalEvents__in_cluster_disruption.json"
)

var disruptionDataPath = fmt.Sprintf("%s/%s", varLogPath, disruptionDataFolder)

func TearDownInClusterMonitors(config *rest.Config) error {
	ctx := context.Background()
	kubeClient, err := kubernetes.NewForConfig(config)
	if err != nil {
		return err
	}
	projectClient, err := projectclient.NewForConfig(config)
	if err != nil {
		return err
	}

	deleteTestBed(ctx, projectClient, kubeClient)

	nodes, err := kubeClient.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	if err != nil {
		return err
	}
	var events monitorapi.Intervals
	var errs []error
	for _, node := range nodes.Items {
		nodeEvents, err := fetchNodeInClusterEvents(ctx, &node)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		framework.Logf("in-cluster monitors: found %d events for node %s", len(events), node.Name)
		events = append(events, nodeEvents...)
	}
	if len(errs) > 0 {
		framework.Logf("found errors fetching in-cluster data: %v", errs)
	}
	artifactPath := filepath.Join(os.Getenv("ARTIFACT_DIR"), inClusterEventsFile)
	return monitorserialization.EventsToFile(artifactPath, events)
}

func StartInClusterMonitors(ctx context.Context, config *rest.Config) error {
	kubeClient, err := kubernetes.NewForConfig(config)
	if err != nil {
		return err
	}
	configClient, err := configclient.NewForConfig(config)
	if err != nil {
		return err
	}
	infra, err := configClient.ConfigV1().Infrastructures().Get(context.Background(), "cluster", metav1.GetOptions{})
	if err != nil {
		return err
	}

	internalAPI, err := url.Parse(infra.Status.APIServerInternalURL)
	if err != nil {
		return err
	}
	apiIntHost := internalAPI.Hostname()

	projectClient, err := projectclient.NewForConfig(config)
	if err != nil {
		return err
	}

	err = callProject(ctx, projectClient, true)
	if err != nil {
		return err
	}
	err = callServiceAccount(ctx, kubeClient, true)
	if err != nil {
		return err
	}
	err = callRBACClusterAdmin(ctx, kubeClient, true)
	if err != nil {
		return err
	}
	err = callRBACHostaccess(ctx, kubeClient, false)
	if err != nil {
		return err
	}
	return callMonitorDaemonset(ctx, kubeClient, apiIntHost, true)
}

func deleteTestBed(ctx context.Context, projectClient *projectclient.Clientset, kubeClient *kubernetes.Clientset) error {
	// Stop daemonsets first so that test stop before serviceaccount is removed
	// and permission issues from apiserver are not recorded as disruption
	err := callMonitorDaemonset(ctx, kubeClient, "", false)
	if err != nil {
		return err
	}

	err = callRBACClusterAdmin(ctx, kubeClient, false)
	if err != nil {
		return err
	}
	err = callRBACHostaccess(ctx, kubeClient, false)
	if err != nil {
		return err
	}
	err = callServiceAccount(ctx, kubeClient, false)
	if err != nil {
		return err
	}
	return callProject(ctx, projectClient, false)
}

func createDaemonset(ctx context.Context, clientset *kubernetes.Clientset, create bool, obj *appsv1.DaemonSet) error {
	client := clientset.AppsV1().DaemonSets(namespace)
	var err error
	if create {
		_, err = client.Create(ctx, obj, metav1.CreateOptions{})
		if apierrors.IsAlreadyExists(err) {
			return nil
		}
		if err != nil {
			return err
		}
		if err := wait.PollUntilContextTimeout(ctx, 10*time.Second, 5*time.Minute, true, func(context.Context) (bool, error) {
			ds, err := client.Get(context.Background(), obj.Name, metav1.GetOptions{})
			if err != nil {
				framework.Logf("error getting daemonsets %v", err)
				return false, nil
			}
			return ds.Status.NumberReady > 0, nil
		}); err != nil {
			return fmt.Errorf("daemonset %s didn't roll out: %v", obj.Name, err)
		}

	} else {
		err = client.Delete(ctx, obj.Name, metav1.DeleteOptions{})
		if err != nil {
			return err
		}
		if err := wait.PollUntilContextTimeout(ctx, 10*time.Second, 5*time.Minute, true, func(context.Context) (bool, error) {
			_, err := client.Get(context.Background(), obj.Name, metav1.GetOptions{})
			if err != nil {
				if apierrors.IsNotFound(err) {
					return true, nil
				}
				framework.Logf("Error getting daemonset %s: %v", err)
				return false, nil
			}
			return false, nil
		}); err != nil {
			return fmt.Errorf("daemonset %s didn't roll out: %v", obj.Name, err)
		}
	}

	return err
}

func callMonitorDaemonset(ctx context.Context, clientset *kubernetes.Clientset, apiIntHost string, create bool) error {
	name := "pod-monitor"
	labels := map[string]string{
		"app": name,
	}
	bTrue := true
	obj := &appsv1.DaemonSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: appsv1.DaemonSetSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: labels,
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: labels,
				},
				Spec: corev1.PodSpec{
					Volumes: []corev1.Volume{
						{
							Name: "artifacts",
							VolumeSource: corev1.VolumeSource{
								HostPath: &corev1.HostPathVolumeSource{
									Path: disruptionDataPath,
								},
							},
						},
					},
					Containers: []corev1.Container{
						{
							Name:  "internal-lb",
							Image: "image-registry.openshift-image-registry.svc:5000/openshift/tests:latest",
							Env: []corev1.EnvVar{
								{
									Name:  "KUBERNETES_SERVICE_HOST",
									Value: apiIntHost,
								},
								{
									Name:  "KUBERNETES_SERVICE_PORT",
									Value: "6443",
								},
								{
									Name: "EXTRA_MESSAGE",
									ValueFrom: &corev1.EnvVarSource{
										FieldRef: &corev1.ObjectFieldSelector{
											FieldPath: "spec.nodeName",
										},
									},
								},
								{
									Name:  "API_DISRUPTION_ONLY",
									Value: "true",
								},
								{
									Name:  "LOAD_BALANCER_TYPE",
									Value: "internal-lb",
								},
							},
							Command: []string{
								"openshift-tests",
								"run-monitor",
								"--artifact-dir",
								disruptionDataPath,
							},
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      "artifacts",
									MountPath: disruptionDataPath,
								},
							},
						},
						{
							Name:  "service-network",
							Image: "image-registry.openshift-image-registry.svc:5000/openshift/tests:latest",
							Command: []string{
								"openshift-tests",
								"run-monitor",
								"--artifact-dir",
								disruptionDataPath,
							},
							Env: []corev1.EnvVar{
								{
									Name: "EXTRA_MESSAGE",
									ValueFrom: &corev1.EnvVarSource{
										FieldRef: &corev1.ObjectFieldSelector{
											FieldPath: "spec.nodeName",
										},
									},
								},
								{
									Name:  "API_DISRUPTION_ONLY",
									Value: "true",
								},
								{
									Name:  "LOAD_BALANCER_TYPE",
									Value: "service-network",
								},
							},
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      "artifacts",
									MountPath: disruptionDataPath,
								},
							},
							SecurityContext: &corev1.SecurityContext{
								Privileged: &bTrue,
							},
						},
					},
					ServiceAccountName: serviceAccountName,
					Tolerations: []corev1.Toleration{
						{
							Key:    "node-role.kubernetes.io/master",
							Effect: corev1.TaintEffectNoSchedule,
						},
					},
				},
			},
		},
	}

	return createDaemonset(ctx, clientset, create, obj)
}

func callRBACClusterAdmin(ctx context.Context, clientset *kubernetes.Clientset, create bool) error {
	obj := &rbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-admin", serviceAccountName),
			Namespace: namespace,
		},
		RoleRef: rbacv1.RoleRef{
			Kind: "ClusterRole",
			Name: "cluster-admin",
		},
		Subjects: []rbacv1.Subject{
			{
				Kind:      rbacv1.ServiceAccountKind,
				Name:      serviceAccountName,
				Namespace: namespace,
			},
		},
	}

	client := clientset.RbacV1().ClusterRoleBindings()
	var err error
	if create {
		_, err = client.Create(ctx, obj, metav1.CreateOptions{})
	} else {
		err = client.Delete(ctx, obj.Name, metav1.DeleteOptions{})
	}

	if apierrors.IsAlreadyExists(err) || apierrors.IsNotFound(err) {
		return nil
	}
	return err
}

func callRBACHostaccess(ctx context.Context, clientset *kubernetes.Clientset, create bool) error {
	obj := &rbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-privileged", serviceAccountName),
			Namespace: namespace,
		},
		RoleRef: rbacv1.RoleRef{
			Kind: "ClusterRole",
			Name: "system:openshift:scc:privileged",
		},
		Subjects: []rbacv1.Subject{
			{
				Kind:      rbacv1.ServiceAccountKind,
				Name:      serviceAccountName,
				Namespace: namespace,
			},
		},
	}

	client := clientset.RbacV1().ClusterRoleBindings()
	var err error
	if create {
		_, err = client.Create(ctx, obj, metav1.CreateOptions{})
	} else {
		err = client.Delete(ctx, obj.Name, metav1.DeleteOptions{})
	}

	if apierrors.IsAlreadyExists(err) || apierrors.IsNotFound(err) {
		return nil
	}
	return err
}

func callServiceAccount(ctx context.Context, clientset *kubernetes.Clientset, create bool) error {
	obj := &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      serviceAccountName,
			Namespace: namespace,
		},
	}

	client := clientset.CoreV1().ServiceAccounts(namespace)
	var err error
	if create {
		_, err = client.Create(ctx, obj, metav1.CreateOptions{})
	} else {
		err = client.Delete(ctx, obj.Name, metav1.DeleteOptions{})
	}

	if apierrors.IsAlreadyExists(err) || apierrors.IsNotFound(err) {
		return nil
	}
	return err
}

func callProject(ctx context.Context, clientset *projectclient.Clientset, create bool) error {
	obj := &projv1.Project{
		ObjectMeta: metav1.ObjectMeta{
			Name: namespace,
			Labels: map[string]string{
				"pod-security.kubernetes.io/enforce": "privileged",
				"pod-security.kubernetes.io/audit":   "privileged",
				"pod-security.kubernetes.io/warn":    "privileged",
			},
		},
	}

	client := clientset.ProjectV1().Projects()
	var err error
	if create {
		_, err = client.Create(ctx, obj, metav1.CreateOptions{})
	} else {
		err = client.Delete(ctx, obj.Name, metav1.DeleteOptions{})
	}

	if apierrors.IsAlreadyExists(err) || apierrors.IsNotFound(err) {
		return nil
	}
	return err
}

func fetchFileViaOC(ctx context.Context, nodeName string, filePath string) (string, error) {
	args := []string{"adm", "node-logs", nodeName, fmt.Sprintf("--path=%s", filePath)}
	cmd := exec.CommandContext(ctx, "oc", args...)
	var outb, errb bytes.Buffer
	cmd.Stdout = &outb
	cmd.Stderr = &errb
	framework.Logf("in-cluster monitors: running 'oc %v'", args)
	if err := cmd.Run(); err != nil {
		// exit early if errors has "404 page not found" - no events were logged
		if strings.HasSuffix(errb.String(), "404 page not found\n") {
			return "", nil
		}
		return "", fmt.Errorf("failed to run oc %s on node %s: %v, stdout: %s, stderr: %s", args, nodeName, err, outb.String(), errb.String())
	}
	return outb.String(), nil
}

func fetchNodeInClusterEvents(ctx context.Context, node *corev1.Node) (monitorapi.Intervals, error) {
	var events monitorapi.Intervals
	var errs []error

	// Fetch a list of e2e data files
	basePath := fmt.Sprintf("/%s/%s", disruptionDataFolder, monitorapi.EventDir)
	fileListOutput, err := fetchFileViaOC(ctx, node.Name, basePath)
	if err != nil {
		return events, fmt.Errorf("failed to list files in disruption event folder on node %s: %v", node.Name, err)
	}
	fileList := strings.Split(strings.Trim(fileListOutput, "\n"), "\n")
	for _, fileName := range fileList {
		if len(fileName) == 0 {
			continue
		}
		framework.Logf("Found events file %s on node %s", fileName, node.Name)
		filePath := fmt.Sprintf("%s/%s", basePath, fileName)
		fileEvents, err := fetchEventsFromFileOnNode(ctx, filePath, node.Name)
		if err != nil {
			errs = append(errs, err)
		}
		events = append(events, fileEvents...)
	}

	if len(errs) > 0 {
		return events, fmt.Errorf("failed to process files on node %s: %v", node.Name, errs)
	}

	return events, nil
}

func fetchEventsFromFileOnNode(ctx context.Context, filePath string, nodeName string) (monitorapi.Intervals, error) {
	var filteredEvents monitorapi.Intervals
	var err error

	eventsJson, err := fetchFileViaOC(ctx, nodeName, filePath)
	if err != nil {
		return filteredEvents, fmt.Errorf("failed to fetch file %s on node %s: %v", filePath, nodeName, err)
	}

	allEvents, err := monitorserialization.EventsFromJSON([]byte(eventsJson))
	if err != nil {
		return nil, fmt.Errorf("failed to convert file %s from node %s to intervals: %v", filePath, nodeName, err)
	}
	fmt.Fprintf(os.Stdout, "Fetched %d events from node %s\n", len(allEvents), nodeName)
	// Keep only disruption events
	for _, event := range allEvents {
		if !strings.HasPrefix(event.Locator, "disruption/") {
			continue
		}
		filteredEvents = append(filteredEvents, event)
	}
	framework.Logf("Fetched %d events from node %s", len(filteredEvents), nodeName)
	return filteredEvents, err
}
