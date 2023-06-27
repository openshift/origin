package sampler

import (
	"bytes"
	"context"
	_ "embed"
	"fmt"
	"net/url"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/openshift/library-go/pkg/operator/resource/resourceread"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
	watchtools "k8s.io/client-go/tools/watch"
	"k8s.io/kubernetes/test/e2e/framework"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/watch"

	configclient "github.com/openshift/client-go/config/clientset/versioned"

	"github.com/openshift/origin/pkg/monitor/monitorapi"
	"github.com/openshift/origin/pkg/monitor/nodedetails"
	monitorserialization "github.com/openshift/origin/pkg/monitor/serialization"
)

const (
	disruptionDataFolder = "disruption-data"
	disruptionTypeEnvVar = "DISRUPTION_TYPE_LABEL"
	inClusterEventsFile  = "junit/AdditionalEvents__in_cluster_disruption.json"
)

var (
	namespace string
	//go:embed manifests/namespace.yaml
	namespaceYaml []byte
	//go:embed manifests/crb-hostaccess.yaml
	rbacPrivilegedYaml []byte
	//go:embed manifests/crb-cluster-reader.yaml
	rbacClusterReaderYaml []byte
	//go:embed manifests/serviceaccount.yaml
	serviceAccountYaml []byte
	//go:embed manifests/daemonset.yaml
	daemonsetYaml            []byte
	rbacPrivilegedCRBName    string
	rbacClusterReaderCRBName string
	daemonsetName            string
)

func TearDownInClusterMonitors(config *rest.Config) error {
	ctx := context.Background()
	client, err := kubernetes.NewForConfig(config)
	if err != nil {
		return err
	}
	deleteTestBed(ctx, client)

	nodes, err := client.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	if err != nil {
		return err
	}
	var events monitorapi.Intervals
	var errs []error
	for _, node := range nodes.Items {
		nodeEvents, err := fetchNodeInClusterEvents(ctx, client, &node)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		fmt.Fprintf(os.Stdout, "in-cluster monitors: found %d events for node %s\n", len(events), node.Name)
		events = append(events, nodeEvents...)
	}
	if len(errs) > 0 {
		fmt.Fprintf(os.Stdout, "found errors fetching in-cluster data: %+v\n", errs)
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

	err = createNamespace(ctx, kubeClient)
	if err != nil {
		return err
	}
	err = createServiceAccount(ctx, kubeClient)
	if err != nil {
		return err
	}
	err = createRBACPrivileged(ctx, kubeClient)
	if err != nil {
		return err
	}
	return createDaemonset(ctx, kubeClient, apiIntHost)
}

func deleteTestBed(ctx context.Context, kubeClient *kubernetes.Clientset) error {
	nsClient := kubeClient.CoreV1().Namespaces()
	err := nsClient.Delete(ctx, namespace, metav1.DeleteOptions{})
	if apierrors.IsNotFound(err) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("error removing namespace %s: %v", namespace, err)
	}

	timeLimitedCtx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()

	if _, watchErr := watchtools.UntilWithSync(timeLimitedCtx,
		cache.NewListWatchFromClient(
			kubeClient.AppsV1().RESTClient(), "daemonsets", namespace, fields.OneTermEqualSelector("metadata.name", daemonsetName)),
		&appsv1.DaemonSet{},
		nil,
		func(event watch.Event) (bool, error) {
			return event.Type == watch.Deleted, nil
		},
	); watchErr != nil {
		return fmt.Errorf("namespace %s didn't get destroyed: %v", namespace, watchErr)
	}

	rbacClient := kubeClient.RbacV1().ClusterRoleBindings()
	err = rbacClient.Delete(ctx, rbacClusterReaderCRBName, metav1.DeleteOptions{})
	if apierrors.IsNotFound(err) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("error removing cluster reader CRB: %v", err)
	}

	err = rbacClient.Delete(ctx, rbacPrivilegedCRBName, metav1.DeleteOptions{})
	if apierrors.IsNotFound(err) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("error removing cluster reader CRB: %v", err)
	}
	return nil
}

func createDaemonset(ctx context.Context, clientset *kubernetes.Clientset, apiIntHost string) error {
	daemonsetObj := resourceread.ReadDaemonSetV1OrDie(daemonsetYaml)
	daemonsetObj.Namespace = namespace
	daemonsetObj.Spec.Template.Spec.Containers[0].Env[0].Value = apiIntHost

	client := clientset.AppsV1().DaemonSets(namespace)
	var err error
	_, err = client.Create(ctx, daemonsetObj, metav1.CreateOptions{})
	if err != nil && !apierrors.IsAlreadyExists(err) {
		return fmt.Errorf("error creating daemonset: %v", err)
	}

	timeLimitedCtx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()

	if _, watchErr := watchtools.UntilWithSync(timeLimitedCtx,
		cache.NewListWatchFromClient(
			clientset.AppsV1().RESTClient(), "daemonsets", namespace, fields.OneTermEqualSelector("metadata.name", daemonsetObj.Name)),
		&appsv1.DaemonSet{},
		nil,
		func(event watch.Event) (bool, error) {
			ds := event.Object.(*appsv1.DaemonSet)
			return ds.Status.NumberReady > 0, nil
		},
	); watchErr != nil {
		return fmt.Errorf("daemonset %s didn't roll out: %v", daemonsetObj.Name, watchErr)
	}
	daemonsetName = daemonsetObj.Name
	return nil
}

func createRBACPrivileged(ctx context.Context, clientset *kubernetes.Clientset) error {
	rbacPrivilegedObj := resourceread.ReadClusterRoleBindingV1OrDie(rbacPrivilegedYaml)
	rbacPrivilegedObj.Subjects[0].Namespace = namespace

	client := clientset.RbacV1().ClusterRoleBindings()
	_, err := client.Create(ctx, rbacPrivilegedObj, metav1.CreateOptions{})
	if err != nil && !apierrors.IsAlreadyExists(err) {
		return fmt.Errorf("error creating privileged SCC CRB: %v", err)
	}
	rbacPrivilegedCRBName = rbacPrivilegedObj.Name
	return nil
}

func createServiceAccount(ctx context.Context, clientset *kubernetes.Clientset) error {
	serviceAccountObj := resourceread.ReadServiceAccountV1OrDie(serviceAccountYaml)
	serviceAccountObj.Namespace = namespace
	client := clientset.CoreV1().ServiceAccounts(namespace)
	_, err := client.Create(ctx, serviceAccountObj, metav1.CreateOptions{})
	if err != nil && !apierrors.IsAlreadyExists(err) {
		return fmt.Errorf("error creating service account: %v", err)
	}
	return nil
}

func createNamespace(ctx context.Context, clientset *kubernetes.Clientset) error {
	namespaceObj := resourceread.ReadNamespaceV1OrDie(namespaceYaml)

	client := clientset.CoreV1().Namespaces()
	actualNamespace, err := client.Create(ctx, namespaceObj, metav1.CreateOptions{})
	if err != nil && !apierrors.IsAlreadyExists(err) {
		return fmt.Errorf("error creating namespace: %v", err)
	}
	namespace = actualNamespace.Name
	return nil
}

func fetchNodeInClusterEvents(ctx context.Context, clientset *kubernetes.Clientset, node *corev1.Node) (monitorapi.Intervals, error) {
	var events monitorapi.Intervals
	var errs []error

	// Fetch a list of e2e data files
	basePath := fmt.Sprintf("/%s/%s", disruptionDataFolder, monitorapi.EventDir)
	allBytes, err := nodedetails.GetNodeLogFile(ctx, clientset, node.Name, basePath)
	if err != nil {
		return events, fmt.Errorf("failed to list files in disruption event folder on node %s: %v", node.Name, err)
	}
	fileList, err := nodedetails.GetDirectoryListing(bytes.NewBuffer(allBytes))
	if err != nil {
		return nil, err
	}
	for _, fileName := range fileList {
		if len(fileName) == 0 {
			continue
		}
		filePath := fmt.Sprintf("%s/%s", basePath, fileName)
		fmt.Fprintf(os.Stdout, "Found events file %s on node %s\n", filePath, node.Name)
		fileEvents, err := fetchEventsFromFileOnNode(ctx, clientset, filePath, node.Name)
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

func fetchEventsFromFileOnNode(ctx context.Context, clientset *kubernetes.Clientset, filePath string, nodeName string) (monitorapi.Intervals, error) {
	var filteredEvents monitorapi.Intervals
	var err error

	allBytes, err := nodedetails.GetNodeLogFile(ctx, clientset, nodeName, filePath)
	if err != nil {
		return filteredEvents, fmt.Errorf("failed to fetch file %s on node %s: %v", filePath, nodeName, err)
	}
	allEvents, err := monitorserialization.EventsFromJSON(allBytes)
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
	fmt.Fprintf(os.Stdout, "Found %d disruption events from node %s\n", len(filteredEvents), nodeName)
	return filteredEvents, err
}
