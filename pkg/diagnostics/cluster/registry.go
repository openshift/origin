package cluster

import (
	"bufio"
	"fmt"
	"reflect"
	"strings"

	kapi "k8s.io/kubernetes/pkg/api"
	kerrs "k8s.io/kubernetes/pkg/api/errors"
	kclient "k8s.io/kubernetes/pkg/client/unversioned"
	"k8s.io/kubernetes/pkg/fields"
	"k8s.io/kubernetes/pkg/labels"

	authorizationapi "github.com/openshift/origin/pkg/authorization/api"
	osclient "github.com/openshift/origin/pkg/client"
	"github.com/openshift/origin/pkg/diagnostics/types"
	osapi "github.com/openshift/origin/pkg/image/api"
)

// ClusterRegistry is a Diagnostic to check that there is a working Docker registry.
type ClusterRegistry struct {
	KubeClient *kclient.Client
	OsClient   *osclient.Client
}

const (
	ClusterRegistryName = "ClusterRegistry"

	registryName   = "docker-registry"
	registryVolume = "registry-storage"
	clGetRegNone   = `
There is no "%s" service in project "%s". This is not strictly required to
be present; however, it is required for builds, and its absence probably
indicates an incomplete installation.

Please consult the documentation and use the 'oadm registry' command
to create a Docker registry.`

	clGetRegFailed = `
Client error while retrieving registry service. Client retrieved records
during discovery, so this is likely to be a transient error. Try running
diagnostics again. If this message persists, there may be a permissions
problem with getting records. The error was:

(%T) %[1]v `

	clRegNoPods = `
The "%s" service exists but has no associated pods, so it
is not available. Builds and deployments that use the registry will fail.`

	clRegNoRunningPods = `
The "%s" service exists but no pods currently running, so it
is not available. Builds and deployments that use the registry will fail.`

	clRegMultiPods = `
The "%s" service has multiple associated pods each using
ephemeral storage. These are likely to have inconsistent stores of
images. Builds and deployments that use images from the registry may
fail sporadically. Use a single registry or add a shared storage volume
to the registries.`

	clRegPodDown = `
The "%s" pod for the "%s" service is not running.
This may be transient, a scheduling error, or something else.`
	clRegPodLog = `
Failed to read the logs for the "%s" pod belonging to
the "%s" service. This is not a problem by itself but
prevents diagnostics from looking for errors in those logs. The
error encountered was:
%s`

	clRegPodConn = `
The pod logs for the "%s" pod belonging to
the "%s" service indicated a problem connecting to the
master to notify it about a new image. This typically results in builds
succeeding but not triggering deployments (as they wait on notifications
to the ImageStream from the build).

There are many reasons for this step to fail, including invalid
credentials, master outages, DNS failures, network errors, and so on. It
can be temporary or ongoing. Check the most recent error message from the
registry pod logs to determine the nature of the problem:

%s`

	clRegPodErr = `
The pod logs for the "%s" pod belonging to
the "%s" service indicated unknown errors.
This could result in problems with builds or deployments.
Please examine the log entries to determine if there might be
any related problems:
%s`

	clRegNoEP = `
The "%[1]s" service exists with %d associated pod(s), but there
are %d endpoints in the "%[1]s" service.
This mismatch likely indicates a system bug, and builds and
deployments that require the registry may fail sporadically.`

	clRegISDelFail = `
The diagnostics created an ImageStream named "%[1]s"
for test purposes and then attempted to delete it, which failed. This
should be an unusual, transient occurrence. The error encountered in
deleting it was:

%s

This message is just to notify you that this object exists.
You ought to be able to delete this object with:

oc delete imagestream/%[1]s -n default
`

	clRegISMismatch = `
Diagnostics created a test ImageStream and compared the registry IP
it received to the registry IP available via the %[1]s service.

%[1]s      : %[2]s
ImageStream registry : %[3]s

They do not match, which probably means that an administrator re-created
the %[1]s service but the master has cached the old service
IP address. Builds or deployments that use ImageStreams with the wrong
%[1]s IP will fail under this condition.

To resolve this issue, restarting the master (to clear the cache) should
be sufficient. Existing ImageStreams may need to be re-created.`
)

func (d *ClusterRegistry) Name() string {
	return ClusterRegistryName
}

func (d *ClusterRegistry) Description() string {
	return "Check that there is a working Docker registry"
}

func (d *ClusterRegistry) CanRun() (bool, error) {
	if d.OsClient == nil || d.KubeClient == nil {
		return false, fmt.Errorf("must have kube and os clients")
	}
	return userCan(d.OsClient, authorizationapi.AuthorizationAttributes{
		Namespace:    kapi.NamespaceDefault,
		Verb:         "get",
		Resource:     "services",
		ResourceName: registryName,
	})
}

func (d *ClusterRegistry) Check() types.DiagnosticResult {
	r := types.NewDiagnosticResult(ClusterRegistryName)
	if service := d.getRegistryService(r); service != nil {
		// Check that it actually has pod(s) selected and running
		if runningPods := d.getRegistryPods(service, r); len(runningPods) == 0 {
			r.Error("DClu1001", nil, fmt.Sprintf(clRegNoRunningPods, registryName))
			return r
		} else if d.checkRegistryEndpoints(runningPods, r) { // Check that matching endpoint exists on the service
			// attempt to create an imagestream and see if it gets the same registry service IP from the service cache
			d.verifyRegistryImageStream(service, r)
		}
	}
	return r
}

func (d *ClusterRegistry) getRegistryService(r types.DiagnosticResult) *kapi.Service {
	service, err := d.KubeClient.Services(kapi.NamespaceDefault).Get(registryName)
	if err != nil && reflect.TypeOf(err) == reflect.TypeOf(&kerrs.StatusError{}) {
		r.Warn("DClu1002", err, fmt.Sprintf(clGetRegNone, registryName, kapi.NamespaceDefault))
		return nil
	} else if err != nil {
		r.Error("DClu1003", err, fmt.Sprintf(clGetRegFailed, err))
		return nil
	}
	r.Debug("DClu1004", fmt.Sprintf("Found %s service with ports %v", registryName, service.Spec.Ports))
	return service
}

func (d *ClusterRegistry) getRegistryPods(service *kapi.Service, r types.DiagnosticResult) []*kapi.Pod {
	runningPods := []*kapi.Pod{}
	pods, err := d.KubeClient.Pods(kapi.NamespaceDefault).List(labels.SelectorFromSet(service.Spec.Selector), fields.Everything())
	if err != nil {
		r.Error("DClu1005", err, fmt.Sprintf("Finding pods for '%s' service failed. This should never happen. Error: (%T) %[2]v", registryName, err))
		return runningPods
	} else if len(pods.Items) < 1 {
		r.Error("DClu1006", nil, fmt.Sprintf(clRegNoPods, registryName))
		return runningPods
	} else if len(pods.Items) > 1 {
		// multiple registry pods using EmptyDir will be inconsistent
		for _, volume := range pods.Items[0].Spec.Volumes {
			if volume.Name == registryVolume && volume.EmptyDir != nil {
				r.Error("DClu1007", nil, fmt.Sprintf(clRegMultiPods, registryName))
				break
			}
		}
	}
	for _, pod := range pods.Items {
		r.Debug("DClu1008", fmt.Sprintf("Found %s pod with name %s", registryName, pod.ObjectMeta.Name))
		if pod.Status.Phase != kapi.PodRunning {
			r.Warn("DClu1009", nil, fmt.Sprintf(clRegPodDown, pod.ObjectMeta.Name, registryName))
		} else {
			runningPods = append(runningPods, &pod)
			// Check the logs for that pod for common issues (credentials, DNS resolution failure)
			d.checkRegistryLogs(&pod, r)
		}
	}
	return runningPods
}

func (d *ClusterRegistry) checkRegistryLogs(pod *kapi.Pod, r types.DiagnosticResult) {
	// pull out logs from the pod
	readCloser, err := d.KubeClient.RESTClient.Get().
		Namespace("default").Name(pod.ObjectMeta.Name).
		Resource("pods").SubResource("log").
		Param("follow", "false").
		Param("container", pod.Spec.Containers[0].Name).
		Stream()
	if err != nil {
		r.Warn("DClu1010", nil, fmt.Sprintf(clRegPodLog, pod.ObjectMeta.Name, registryName, fmt.Sprintf("(%T) %[1]v", err)))
		return
	}
	defer readCloser.Close()

	clientError := ""
	registryError := ""
	scanner := bufio.NewScanner(readCloser)
	for scanner.Scan() {
		logLine := scanner.Text()
		// TODO: once the logging API gets "since" and "tail" and "limit", limit to more recent log entries
		// https://github.com/kubernetes/kubernetes/issues/12447
		if strings.Contains(logLine, `level=error msg="client error:`) {
			clientError = logLine // end up showing only the most recent client error
		} else if strings.Contains(logLine, "level=error msg=") {
			registryError += "\n" + logLine // gather generic errors
		}
	}
	if clientError != "" {
		r.Error("DClu1011", nil, fmt.Sprintf(clRegPodConn, pod.ObjectMeta.Name, registryName, clientError))
	}
	if registryError != "" {
		r.Warn("DClu1012", nil, fmt.Sprintf(clRegPodErr, pod.ObjectMeta.Name, registryName, registryError))
	}

}

func (d *ClusterRegistry) checkRegistryEndpoints(pods []*kapi.Pod, r types.DiagnosticResult) bool {
	endPoint, err := d.KubeClient.Endpoints(kapi.NamespaceDefault).Get(registryName)
	if err != nil {
		r.Error("DClu1013", err, fmt.Sprintf(`Finding endpoints for "%s" service failed. This should never happen. Error: (%[2]T) %[2]v`, registryName, err))
		return false
	}
	numEP := 0
	for _, subs := range endPoint.Subsets {
		numEP += len(subs.Addresses)
	}
	if numEP != len(pods) {
		r.Warn("DClu1014", nil, fmt.Sprintf(clRegNoEP, registryName, len(pods), numEP))
		return false
	}
	return true
}

func (d *ClusterRegistry) verifyRegistryImageStream(service *kapi.Service, r types.DiagnosticResult) {
	imgStream, err := d.OsClient.ImageStreams(kapi.NamespaceDefault).Create(&osapi.ImageStream{ObjectMeta: kapi.ObjectMeta{GenerateName: "diagnostic-test"}})
	if err != nil {
		r.Error("DClu1015", err, fmt.Sprintf("Creating test ImageStream failed. Error: (%T) %[1]v", err))
		return
	}
	defer func() { // delete what we created, or notify that we couldn't
		if err := d.OsClient.ImageStreams(kapi.NamespaceDefault).Delete(imgStream.ObjectMeta.Name); err != nil {
			r.Warn("DClu1016", err, fmt.Sprintf(clRegISDelFail, imgStream.ObjectMeta.Name, fmt.Sprintf("(%T) %[1]s", err)))
		}
	}()
	imgStream, err = d.OsClient.ImageStreams(kapi.NamespaceDefault).Get(imgStream.ObjectMeta.Name) // status is filled in post-create
	if err != nil {
		r.Error("DClu1017", err, fmt.Sprintf("Getting created test ImageStream failed. Error: (%T) %[1]v", err))
		return
	}
	r.Debug("DClu1018", fmt.Sprintf("Created test ImageStream: %[1]v", imgStream))
	cacheHost := strings.SplitN(imgStream.Status.DockerImageRepository, "/", 2)[0]
	serviceHost := fmt.Sprintf("%s:%d", service.Spec.ClusterIP, service.Spec.Ports[0].Port)
	if cacheHost != serviceHost {
		r.Error("DClu1019", nil, fmt.Sprintf(clRegISMismatch, registryName, serviceHost, cacheHost))
	}
}
