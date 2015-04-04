package client

import (
	"fmt"
	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	kerrs "github.com/GoogleCloudPlatform/kubernetes/pkg/api/errors"
	client "github.com/GoogleCloudPlatform/kubernetes/pkg/client"
	clientcmdapi "github.com/GoogleCloudPlatform/kubernetes/pkg/client/clientcmd/api"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/fields"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/labels"
	osclient "github.com/openshift/origin/pkg/client"
	"github.com/openshift/origin/pkg/diagnostics/discovery"
	"github.com/openshift/origin/pkg/diagnostics/log"
	"github.com/openshift/origin/pkg/diagnostics/types/diagnostic"
	osapi "github.com/openshift/origin/pkg/image/api"
	"reflect"
	"strings"
)

var Diagnostics = map[string]diagnostic.Diagnostic{
	"NodeDefinitions": {
		Description: "Check node records on master",
		Condition: func(env *discovery.Environment) (skip bool, reason string) {
			if env.ClusterAdminFactory == nil {
				return true, "Client does not have cluster-admin access and cannot see node records"
			}
			return false, ""
		},
		Run: func(env *discovery.Environment) {
			var err error
			var nodes *kapi.NodeList
			if _, kclient, err := env.ClusterAdminFactory.Clients(); err == nil {
				nodes, err = kclient.Nodes().List(labels.LabelSelector{}, fields.Everything())
			}
			if err != nil {
				env.Log.Errorf("clGetNodesFailed", `
Client error while retrieving node records. Client retrieved records
during discovery, so this is likely to be a transient error. Try running
diagnostics again. If this message persists, there may be a permissions
problem with getting node records. The error was:

(%T) %[1]v`, err)
				return
			}
			for _, node := range nodes.Items {
				//pretty.Println("Node record:", node)
				var ready *kapi.NodeCondition
				for i, condition := range node.Status.Conditions {
					switch condition.Type {
					// currently only one... used to be more, may be again
					case kapi.NodeReady:
						ready = &node.Status.Conditions[i]
					}
				}
				//pretty.Println("Node conditions for "+node.Name, ready, schedulable)
				if ready == nil || ready.Status != kapi.ConditionTrue {
					msg := log.Msg{
						"node": node.Name,
						"tmpl": `
Node {{.node}} is defined but is not marked as ready.
Ready status is {{.status}} because "{{.reason}}"
If the node is not intentionally disabled, check that the master can
reach the node hostname for a health check and the node is checking in
to the master with the same hostname.

While in this state, pods should not be scheduled to deploy on the node,
and any existing scheduled pods will be considered failed and removed.
 `,
					}
					if ready == nil {
						msg["status"] = "None"
						msg["reason"] = "There is no readiness record."
					} else {
						msg["status"] = ready.Status
						msg["reason"] = ready.Reason
					}
					env.Log.Warnm("clNodeBroken", msg)
				}
			}
		},
	},

	"ConfigContexts": {
		Description: "Test that client config contexts have no undefined references",
		Condition: func(env *discovery.Environment) (skip bool, reason string) {
			if env.ClientConfigRaw == nil {
				return true, "There is no client config file"
			}
			return false, ""
		},
		Run: func(env *discovery.Environment) {
			cc := env.ClientConfigRaw
			current := cc.CurrentContext
			ccSuccess := false
			var ccResult log.Msg //nil
			for context := range cc.Contexts {
				result, success := TestContext(context, cc)
				msg := log.Msg{"tmpl": "For client config context '{{.context}}':{{.result}}", "context": context, "result": result}
				if context == current {
					ccResult, ccSuccess = msg, success
				} else if success {
					env.Log.Infom("clientCfgSuccess", msg)
				} else {
					env.Log.Warnm("clientCfgWarn", msg)
				}
			}
			if _, exists := cc.Contexts[current]; exists {
				ccResult["tmpl"] = `
The current context from client config is '{{.context}}'
This will be used by default to contact your OpenShift server.
` + ccResult["tmpl"].(string)
				if ccSuccess {
					env.Log.Infom("currentccSuccess", ccResult)
				} else {
					env.Log.Errorm("currentccWarn", ccResult)
				}
			} else { // context does not exist
				env.Log.Errorm("cConUndef", log.Msg{"tmpl": `
Your client config specifies a current context of '{{.context}}'
which is not defined; it is likely that a mistake was introduced while
manually editing your config. If this is a simple typo, you may be
able to fix it manually.
The OpenShift master creates a fresh config when it is started; it may be
useful to use this as a base if available.`, "context": current})
			}
		},
	},

	"ClusterRegistry": {
		Description: "Check there is a working Docker registry",
		Condition: func(env *discovery.Environment) (skip bool, reason string) {
			if env.ClusterAdminFactory == nil {
				return true, "Client does not have cluster-admin access and cannot see registry objects"
			}
			return false, ""
		},
		Run: func(env *discovery.Environment) {
			osClient, kclient, err := env.ClusterAdminFactory.Clients()
			if err != nil {
				env.Log.Errorf("clGetClientFailed", "Constructing clients failed. This should never happen. Error: (%T) %[1]v", err)
				return
			}
			// retrieve the service if it exists
			if service := getRegistryService(kclient, env.Log); service != nil {
				// Check that it actually has a pod selected that's running
				if pod := getRegistryPod(kclient, service, env.Log); pod != nil {
					// Check that an endpoint exists on the service
					if endPoint := getRegistryEndpoint(kclient, env.Log); endPoint != nil {
						// TODO: Check that endpoints on the service match the pod (hasn't been a problem yet though)
						// TODO: Check the logs for that pod for common issues (credentials, DNS resolution failure)
						// attempt to create an imagestream and see if it gets the same registry service IP from the service cache
						testRegistryImageStream(osClient, service, env.Log)
					}
				}
			}

		},
	},
}

func TestContext(contextName string, config *clientcmdapi.Config) (result string, success bool) {
	context, exists := config.Contexts[contextName]
	if !exists {
		return "client config context '" + contextName + "' is not defined.", false
	}
	clusterName := context.Cluster
	cluster, exists := config.Clusters[clusterName]
	if !exists {
		return fmt.Sprintf("client config context '%s' has a cluster '%s' which is not defined.", contextName, clusterName), false
	}
	authName := context.AuthInfo
	if _, exists := config.AuthInfos[authName]; !exists {
		return fmt.Sprintf("client config context '%s' has a user identity '%s' which is not defined.", contextName, authName), false
	}
	project := context.Namespace
	if project == "" {
		project = kapi.NamespaceDefault // OpenShift/k8s fills this in if missing
	}
	// TODO: actually send a request to see if can connect
	return fmt.Sprintf(`
The server URL is '%s'
The user authentication is '%s'
The current project is '%s'`, cluster.Server, authName, project), true
}

func getRegistryService(kclient *client.Client, logger *log.Logger) *kapi.Service {
	service, err := kclient.Services("default").Get("docker-registry")
	if err != nil && reflect.TypeOf(err) == reflect.TypeOf(&kerrs.StatusError{}) {
		logger.Warnf("clGetRegFailed", `
There is no "docker-registry" service. This is not strictly required
to use OpenShift, however it is required for builds and its absence
probably indicates an incomplete installation of OpenShift.

Please use the 'osadm registry' command to create a registry.
				`)
		return nil
	} else if err != nil {
		logger.Errorf("clGetRegFailed", `
Client error while retrieving registry service. Client retrieved records
during discovery, so this is likely to be a transient error. Try running
diagnostics again. If this message persists, there may be a permissions
problem with getting records. The error was:

(%T) %[1]v`, err)
		return nil
	}
	logger.Debugf("clRegFound", "Found docker-registry service with ports %v", service.Spec.Ports)
	return service
}

func getRegistryPod(kclient *client.Client, service *kapi.Service, logger *log.Logger) *kapi.Pod {
	pods, err := kclient.Pods("default").List(labels.SelectorFromSet(service.Spec.Selector), fields.Everything())
	if err != nil {
		logger.Errorf("clRegListPods", "Finding pods for 'docker-registry' service failed. This should never happen. Error: (%T) %[1]v", err)
		return nil
	} else if len(pods.Items) < 1 {
		logger.Error("clRegNoPods", `
The "docker-registry" service exists but has no associated pods, so it
is not available. Builds and deployments that use the registry will fail.`)
		return nil
	} else if len(pods.Items) > 1 {
		logger.Error("clRegNoPods", `
The "docker-registry" service has multiple associated pods. Load-balanced
registries are not yet available, so these are likely to have incomplete
stores of images. Builds and deployments that use the registry will
fail sporadically.`)
		return nil
	}
	pod := &pods.Items[0]
	if pod.Status.Phase != kapi.PodRunning {
		logger.Errorf("clRegPodDown", `
The "%s" pod for the "docker-registry" service is not running.
This may be transient, a scheduling error, or something else.
Builds and deployments that require the registry will fail.`, pod.ObjectMeta.Name)
		return nil
	}
	logger.Debugf("clRegPodFound", "Found docker-registry pod with name %s", pod.ObjectMeta.Name)
	return pod
}

func getRegistryEndpoint(kclient *client.Client, logger *log.Logger) *kapi.Endpoints {
	endPoint, err := kclient.Endpoints("default").Get("docker-registry")
	if err != nil {
		logger.Errorf("clRegGetEP", "Finding endpoints for 'docker-registry' service failed. This should never happen. Error: (%T) %[1]v", err)
		return nil
	} else if len(endPoint.Subsets) != 1 || len(endPoint.Subsets[0].Addresses) != 1 {
		logger.Warn("clRegNoEP", `
The "docker-registry" service exists with one associated pod, but the
number of endpoints in the "docker-registry" endpoint object does not
match. This mismatch probably indicates a bug in OpenShift and it is
likely that builds and deployments that require the registry will fail.`)
		return nil
	}
	logger.Debugf("clRegPodFound", "Found docker-registry endpoint object")
	return endPoint
}

func testRegistryImageStream(client *osclient.Client, service *kapi.Service, logger *log.Logger) {
	imgStream, err := client.ImageStreams("default").Create(&osapi.ImageStream{ObjectMeta: kapi.ObjectMeta{GenerateName: "diagnostic-test-"}})
	if err != nil {
		logger.Errorf("clRegISCFail", "Creating test ImageStream failed. Error: (%T) %[1]v", err)
		return
	}
	defer client.ImageStreams("default").Delete(imgStream.ObjectMeta.Name)         // TODO: report if deleting fails
	imgStream, err = client.ImageStreams("default").Get(imgStream.ObjectMeta.Name) // status is filled in post-create
	if err != nil {
		logger.Errorf("clRegISCFail", "Getting created test ImageStream failed. Error: (%T) %[1]v", err)
		return
	}
	logger.Debugf("clRegISC", "Created test ImageStream: %[1]v", imgStream)
	cacheHost := strings.SplitN(imgStream.Status.DockerImageRepository, "/", 2)[0]
	serviceHost := fmt.Sprintf("%s:%d", service.Spec.PortalIP, service.Spec.Ports[0].Port)
	if cacheHost != serviceHost {
		logger.Errorm("clRegISMismatch", log.Msg{
			"serviceHost": serviceHost,
			"cacheHost":   cacheHost,
			"tmpl": `
Diagnostics created a test ImageStream and compared the registry IP
it received to the registry IP available via the docker-registry service.

docker-registry      : {{.serviceHost}}
ImageStream registry : {{.cacheHost}}

They differ, which probably means that an administrator re-created
the docker-registry service but the master has cached the old service
IP address. Builds or deployments that use ImageStreams with the wrong
docker-registry IP will fail under this condition.

To resolve this issue, restarting the master (to clear the cache) should
be sufficient.
`})
	}
}
