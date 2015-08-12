package cluster

import (
	"bufio"
	"fmt"
	"reflect"
	"regexp"
	"strings"

	kapi "k8s.io/kubernetes/pkg/api"
	kerrs "k8s.io/kubernetes/pkg/api/errors"
	kclient "k8s.io/kubernetes/pkg/client"
	"k8s.io/kubernetes/pkg/fields"
	"k8s.io/kubernetes/pkg/labels"

	authorizationapi "github.com/openshift/origin/pkg/authorization/api"
	osclient "github.com/openshift/origin/pkg/client"
	"github.com/openshift/origin/pkg/diagnostics/log"
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
Failed to read the logs for the "{{.podName}}" pod belonging to
the "{{.registryName}}" service. This is not a problem by itself but
prevents diagnostics from looking for errors in those logs. The
error encountered was:
{{.error}}`

	clRegPodConn = `
The pod logs for the "{{.podName}}" pod belonging to
the "{{.registryName}}" service indicated a problem connecting to the
master to notify it about a new image. This typically results in builds
succeeding but not triggering deployments (as they wait on notifications
to the ImageStream from the build).

There are many reasons for this step to fail, including invalid
credentials, DNS failures, network errors, and so on. Examine the
following error message from the registry pod logs to determine the
problem:

{{.log}}`

	clRegNoEP = `
The "{{.registryName}}" service exists with {{.numPods}} associated pod(s), but there
are {{.numEP}} endpoints in the "{{.registryName}}" service.
This mismatch likely indicates a system bug, and builds and
deployments that require the registry may fail sporadically.`

	clRegISDelFail = `
The diagnostics created an ImageStream named "{{.name}}"
for test purposes and then attempted to delete it, which failed. This
should be an unusual, transient occurrence. The error encountered in
deleting it was:

{{.error}}

This message is just to notify you that this object exists.
You ought to be able to delete this object with:

oc delete imagestream/{{.name}} -n default
`

	clRegISMismatch = `
Diagnostics created a test ImageStream and compared the registry IP
it received to the registry IP available via the {{.registryName}} service.

{{.registryName}}      : {{.serviceHost}}
ImageStream registry : {{.cacheHost}}

They do not match, which probably means that an administrator re-created
the {{.registryName}} service but the master has cached the old service
IP address. Builds or deployments that use ImageStreams with the wrong
{{.registryName}} IP will fail under this condition.

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
	return adminCan(d.OsClient, kapi.NamespaceDefault, &authorizationapi.SubjectAccessReview{
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
			r.Errorf("clRegNoRunningPods ", nil, clRegNoRunningPods, registryName)
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
		r.Warnf("clGetRegNone", err, clGetRegNone, registryName, kapi.NamespaceDefault)
		return nil
	} else if err != nil {
		r.Errorf("clGetRegFailed", err, clGetRegFailed, err)
		return nil
	}
	r.Debugf("clRegFound", "Found %s service with ports %v", registryName, service.Spec.Ports)
	return service
}

func (d *ClusterRegistry) getRegistryPods(service *kapi.Service, r types.DiagnosticResult) []*kapi.Pod {
	runningPods := []*kapi.Pod{}
	pods, err := d.KubeClient.Pods(kapi.NamespaceDefault).List(labels.SelectorFromSet(service.Spec.Selector), fields.Everything())
	if err != nil {
		r.Errorf("clRegListPods", err, "Finding pods for '%s' service failed. This should never happen. Error: (%T) %[2]v", registryName, err)
		return runningPods
	} else if len(pods.Items) < 1 {
		r.Errorf("clRegNoPods", nil, clRegNoPods, registryName)
		return runningPods
	} else if len(pods.Items) > 1 {
		// multiple registry pods using EmptyDir will be inconsistent
		for _, volume := range pods.Items[0].Spec.Volumes {
			if volume.Name == registryVolume && volume.EmptyDir != nil {
				r.Errorf("clRegMultiPods", nil, clRegMultiPods, registryName)
				break
			}
		}
	}
	for _, pod := range pods.Items {
		r.Debugf("clRegPodFound", "Found %s pod with name %s", registryName, pod.ObjectMeta.Name)
		if pod.Status.Phase != kapi.PodRunning {
			r.Warnf("clRegPodDown", nil, clRegPodDown, pod.ObjectMeta.Name, registryName)
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
		r.Warnt("clRegPodLog", nil, clRegPodLog, log.Hash{
			"error":        fmt.Sprintf("(%T) %[1]v", err),
			"podName":      pod.ObjectMeta.Name,
			"registryName": registryName,
		})
		return
	}
	defer readCloser.Close()

	scanner := bufio.NewScanner(readCloser)
	for scanner.Scan() {
		logLine := scanner.Text()
		if regexp.MustCompile(`level=error msg="client error: Post http(\S+)/subjectaccessreviews`).MatchString(logLine) {
			r.Errort("clRegPodConn", nil, clRegPodConn, log.Hash{
				"log":          logLine,
				"podName":      pod.ObjectMeta.Name,
				"registryName": registryName,
			})
			break
		}
	}
}

func (d *ClusterRegistry) checkRegistryEndpoints(pods []*kapi.Pod, r types.DiagnosticResult) bool {
	endPoint, err := d.KubeClient.Endpoints(kapi.NamespaceDefault).Get(registryName)
	if err != nil {
		r.Errorf("clRegGetEP", err, `Finding endpoints for "%s" service failed. This should never happen. Error: (%[2]T) %[2]v`, registryName, err)
		return false
	}
	numEP := 0
	for _, subs := range endPoint.Subsets {
		numEP += len(subs.Addresses)
	}
	if numEP != len(pods) {
		r.Warnt("clRegNoEP", nil, clRegNoEP, log.Hash{"registryName": registryName, "numPods": len(pods), "numEP": numEP})
		return false
	}
	return true
}

func (d *ClusterRegistry) verifyRegistryImageStream(service *kapi.Service, r types.DiagnosticResult) {
	imgStream, err := d.OsClient.ImageStreams(kapi.NamespaceDefault).Create(&osapi.ImageStream{ObjectMeta: kapi.ObjectMeta{GenerateName: "diagnostic-test"}})
	if err != nil {
		r.Errorf("clRegISCFail", err, "Creating test ImageStream failed. Error: (%T) %[1]v", err)
		return
	}
	defer func() { // delete what we created, or notify that we couldn't
		if err := d.OsClient.ImageStreams(kapi.NamespaceDefault).Delete(imgStream.ObjectMeta.Name); err != nil {
			r.Warnt("clRegISDelFail", err, clRegISDelFail, log.Hash{
				"name":  imgStream.ObjectMeta.Name,
				"error": fmt.Sprintf("(%T) %[1]s", err),
			})
		}
	}()
	imgStream, err = d.OsClient.ImageStreams(kapi.NamespaceDefault).Get(imgStream.ObjectMeta.Name) // status is filled in post-create
	if err != nil {
		r.Errorf("clRegISCFail", err, "Getting created test ImageStream failed. Error: (%T) %[1]v", err)
		return
	}
	r.Debugf("clRegISC", "Created test ImageStream: %[1]v", imgStream)
	cacheHost := strings.SplitN(imgStream.Status.DockerImageRepository, "/", 2)[0]
	serviceHost := fmt.Sprintf("%s:%d", service.Spec.ClusterIP, service.Spec.Ports[0].Port)
	if cacheHost != serviceHost {
		r.Errort("clRegISMismatch", nil, clRegISMismatch, log.Hash{
			"serviceHost":  serviceHost,
			"cacheHost":    cacheHost,
			"registryName": registryName,
		})
	}
}
