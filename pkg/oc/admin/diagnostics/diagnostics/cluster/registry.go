package cluster

import (
	"bufio"
	"fmt"
	"reflect"
	"regexp"
	"strings"

	kerrs "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/kubernetes/pkg/apis/authorization"
	kapi "k8s.io/kubernetes/pkg/apis/core"
	kclientset "k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset"

	osapi "github.com/openshift/origin/pkg/image/apis/image"
	imagetypedclient "github.com/openshift/origin/pkg/image/generated/internalclientset/typed/image/internalversion"
	"github.com/openshift/origin/pkg/oc/admin/diagnostics/diagnostics/types"
	"github.com/openshift/origin/pkg/oc/admin/diagnostics/diagnostics/util"
)

// ClusterRegistry is a Diagnostic to check that there is a working Docker registry.
type ClusterRegistry struct {
	KubeClient          kclientset.Interface
	ImageStreamClient   imagetypedclient.ImageStreamsGetter
	PreventModification bool
}

const (
	ClusterRegistryName = "ClusterRegistry"

	registryName   = "docker-registry"
	registryVolume = "registry-storage"
	clGetRegNone   = `
There is no "%s" service in project "%s". This is not strictly required to
be present; however, it is required for builds, and its absence probably
indicates an incomplete installation.

Please consult the documentation and use the 'oc adm registry' command
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

	clRegMultiCustomCfg = `
The "%s" service has multiple associated pods each mounted with
ephemeral storage, but also has a custom config %s
mounted; assuming storage config is as desired.`

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

	clRegSelinuxErr = `
The pod logs for the "%s" pod belonging to
the "%s" service indicated the registry is unable to write to disk.
This may indicate an SELinux denial, or problems with volume
ownership/permissions.

For volume permission problems please consult the Persistent Storage section
of the Administrator's Guide.

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
Diagnostics created a test ImageStream and compared the registry
it received to the registry IP and host available via the %[1]s service.

%[1]s by IP: %[2]s
%[1]s by host: %[3]s
ImageStream registry : %[4]s

Neither matches, which could mean that the master has cached an old
service; possibly an administrator re-created the %[1]s service with
a different IP address. Builds or deployments that use ImageStreams
with the wrong %[1]s IP will fail under this condition. If this is the
case, restarting the master (to clear the cache) should resolve the
issue. Existing ImageStreams may need to be re-created.`
)

func (d *ClusterRegistry) Name() string {
	return ClusterRegistryName
}

func (d *ClusterRegistry) Description() string {
	return "Check that there is a working Docker registry"
}

func (d *ClusterRegistry) Requirements() (client bool, host bool) {
	return true, false
}

func (d *ClusterRegistry) CanRun() (bool, error) {
	if d.ImageStreamClient == nil || d.KubeClient == nil {
		return false, fmt.Errorf("must have kube and os clients")
	}
	return util.UserCan(d.KubeClient.Authorization(), &authorization.ResourceAttributes{
		Namespace: metav1.NamespaceDefault,
		Verb:      "get",
		Group:     kapi.GroupName,
		Resource:  "services",
		Name:      registryName,
	})
}

func (d *ClusterRegistry) Check() types.DiagnosticResult {
	r := types.NewDiagnosticResult(ClusterRegistryName)
	if service := d.getRegistryService(r); service != nil {
		// Check that it actually has pod(s) selected and running
		if runningPods := d.getRegistryPods(service, r); len(runningPods) == 0 {
			// not reporting an error here, if there are no running pods an error
			// is reported by getRegistryPods
			return r
		} else if d.checkRegistryEndpoints(runningPods, r) { // Check that matching endpoint exists on the service
			// attempt to create an imagestream and see if it gets the same registry service IP from the service cache
			d.verifyRegistryImageStream(service, r)
		}
	}
	return r
}

func (d *ClusterRegistry) getRegistryService(r types.DiagnosticResult) *kapi.Service {
	service, err := d.KubeClient.Core().Services(metav1.NamespaceDefault).Get(registryName, metav1.GetOptions{})
	if err != nil && reflect.TypeOf(err) == reflect.TypeOf(&kerrs.StatusError{}) {
		r.Warn("DClu1002", err, fmt.Sprintf(clGetRegNone, registryName, metav1.NamespaceDefault))
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
	pods, err := d.KubeClient.Core().Pods(metav1.NamespaceDefault).List(metav1.ListOptions{LabelSelector: labels.SelectorFromSet(service.Spec.Selector).String()})
	if err != nil {
		r.Error("DClu1005", err, fmt.Sprintf("Finding pods for '%s' service failed. This should never happen. Error: (%T) %[2]v", registryName, err))
		return runningPods
	} else if len(pods.Items) < 1 {
		r.Error("DClu1006", nil, fmt.Sprintf(clRegNoPods, registryName))
		return runningPods
	} else if len(pods.Items) > 1 {
		emptyDir := false     // multiple registry pods using EmptyDir will be inconsistent
		customConfig := false // ... unless the user has configured them for e.g. S3
		configPath := "/config.yml"
		// look through the pod volumes to see if that might be a problem
		podSpec := pods.Items[0].Spec
		container := podSpec.Containers[0]
		for _, volume := range podSpec.Volumes {
			if volume.Name == registryVolume && volume.EmptyDir != nil {
				emptyDir = true
			}
		}
		for _, env := range container.Env {
			if env.Name == "REGISTRY_CONFIGURATION_PATH" {
				configPath = env.Value // look for custom config here
			}
		}
		for _, vmount := range container.VolumeMounts {
			if strings.HasPrefix(configPath, vmount.MountPath) {
				customConfig = true // if something's mounted there, assume custom config.
			}
		}
		if emptyDir {
			if customConfig { // assume they know what they're doing
				r.Info("DClu1020", fmt.Sprintf(clRegMultiCustomCfg, registryName, configPath))
			} else { // assume they scaled up with ephemeral storage
				r.Error("DClu1007", nil, fmt.Sprintf(clRegMultiPods, registryName))
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
	readCloser, err := d.KubeClient.Core().RESTClient().Get().
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

	// Indicator that selinux is blocking the registry from writing to disk:
	selinuxErrorRegex, _ := regexp.Compile(".*level=error.*mkdir.*permission denied.*")
	// If seen after the above error regex, we know the problem has since been fixed:
	selinuxSuccessRegex, _ := regexp.Compile(".*level=info.*response completed.*http.request.method=PUT.*")

	clientError := ""
	registryError := ""
	selinuxError := ""

	scanner := bufio.NewScanner(readCloser)
	for scanner.Scan() {
		logLine := scanner.Text()
		// TODO: once the logging API gets "since" and "tail" and "limit", limit to more recent log entries
		// https://github.com/kubernetes/kubernetes/issues/12447
		if strings.Contains(logLine, `level=error msg="client error:`) {
			clientError = logLine // end up showing only the most recent client error
		} else if selinuxErrorRegex.MatchString(logLine) {
			selinuxError = logLine
		} else if selinuxSuccessRegex.MatchString(logLine) {
			// Check for a successful registry push, if this occurs after a selinux error
			// we can safely clear it, the problem has already been fixed.
			selinuxError = ""
		} else if strings.Contains(logLine, "level=error msg=") {
			registryError += "\n" + logLine // gather generic errors
		}
	}
	if clientError != "" {
		r.Error("DClu1011", nil, fmt.Sprintf(clRegPodConn, pod.ObjectMeta.Name, registryName, clientError))
	}
	if selinuxError != "" {
		r.Error("DClu1020", nil, fmt.Sprintf(clRegSelinuxErr, pod.ObjectMeta.Name, registryName, selinuxError))
	}
	if registryError != "" {
		r.Warn("DClu1012", nil, fmt.Sprintf(clRegPodErr, pod.ObjectMeta.Name, registryName, registryError))
	}
}

func (d *ClusterRegistry) checkRegistryEndpoints(pods []*kapi.Pod, r types.DiagnosticResult) bool {
	endPoint, err := d.KubeClient.Core().Endpoints(metav1.NamespaceDefault).Get(registryName, metav1.GetOptions{})
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
	if d.PreventModification {
		r.Info("DClu1021", "Skipping creating an ImageStream to test registry service address, because you requested no API modifications.")
		return
	}
	imgStream, err := d.ImageStreamClient.ImageStreams(metav1.NamespaceDefault).Create(&osapi.ImageStream{ObjectMeta: metav1.ObjectMeta{GenerateName: "diagnostic-test"}})
	if err != nil {
		r.Error("DClu1015", err, fmt.Sprintf("Creating test ImageStream failed. Error: (%T) %[1]v", err))
		return
	}
	defer func() { // delete what we created, or notify that we couldn't
		if err := d.ImageStreamClient.ImageStreams(metav1.NamespaceDefault).Delete(imgStream.ObjectMeta.Name, nil); err != nil {
			r.Warn("DClu1016", err, fmt.Sprintf(clRegISDelFail, imgStream.ObjectMeta.Name, fmt.Sprintf("(%T) %[1]s", err)))
		}
	}()
	imgStream, err = d.ImageStreamClient.ImageStreams(metav1.NamespaceDefault).Get(imgStream.ObjectMeta.Name, metav1.GetOptions{}) // status is filled in post-create
	if err != nil {
		r.Error("DClu1017", err, fmt.Sprintf("Getting created test ImageStream failed. Error: (%T) %[1]v", err))
		return
	}
	r.Debug("DClu1018", fmt.Sprintf("Created test ImageStream: %[1]v", imgStream))
	cacheHost := strings.SplitN(imgStream.Status.DockerImageRepository, "/", 2)[0]
	// the registry for imagestreams was previously recorded as an IP, which could change if the registry service were re-created.
	// Now it is a cluster hostname, which should be unchanging even if re-created. Just ensure it is the right hostname.
	serviceIpPort := fmt.Sprintf("%s:%d", service.Spec.ClusterIP, service.Spec.Ports[0].Port)
	serviceHostPort := fmt.Sprintf("%s.%s.svc:%d", registryName, service.ObjectMeta.Namespace, service.Spec.Ports[0].Port)
	if cacheHost != serviceIpPort && cacheHost != serviceHostPort {
		r.Error("DClu1019", nil, fmt.Sprintf(clRegISMismatch, registryName, serviceIpPort, serviceHostPort, cacheHost))
	}
}
