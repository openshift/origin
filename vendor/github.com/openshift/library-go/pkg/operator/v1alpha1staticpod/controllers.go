package v1alpha1staticpod

import (
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"

	"github.com/openshift/library-go/pkg/operator/events"
	"github.com/openshift/library-go/pkg/operator/v1alpha1staticpod/controller/backingresource"
	"github.com/openshift/library-go/pkg/operator/v1alpha1staticpod/controller/common"
	"github.com/openshift/library-go/pkg/operator/v1alpha1staticpod/controller/deployment"
	"github.com/openshift/library-go/pkg/operator/v1alpha1staticpod/controller/installer"
	"github.com/openshift/library-go/pkg/operator/v1alpha1staticpod/controller/node"
)

type staticPodOperatorControllers struct {
	deploymentController     *deployment.DeploymentController
	installerController      *installer.InstallerController
	nodeController           *node.NodeController
	serviceAccountController *backingresource.BackingResourceController
}

// NewControllers provides all control loops needed to run a static pod based operator. That includes:
// 1. DeploymentController - this watches multiple resources for "latest" input that has changed from the most current deploymentID.
//    When a change is found, it creates a new deployment by copying resources and adding the deploymentID suffix to the names
//    to make a theoretically immutable set of deployment data.  It then bumps the latestDeploymentID and starts watching again.
// 2. InstallerController - this watches the latestDeploymentID and the list of kubeletStatus (alpha-sorted list).  When a latestDeploymentID
//    appears that doesn't match the current latest for first kubeletStatus and the first kubeletStatus isn't already transitioning,
//    it kicks off an installer pod.  If the next kubeletStatus doesn't match the immediate prior one, it kicks off that transition.
// 3. NodeController - watches nodes for master nodes and keeps the operator status up to date
func NewControllers(targetNamespaceName, staticPodName string, command, deploymentConfigMaps, deploymentSecrets []string,
	staticPodOperatorClient common.OperatorClient, kubeClient kubernetes.Interface, kubeInformersNamespaceScoped,
	kubeInformersClusterScoped informers.SharedInformerFactory, eventRecorder events.Recorder) *staticPodOperatorControllers {
	controller := &staticPodOperatorControllers{}

	controller.deploymentController = deployment.NewDeploymentController(
		targetNamespaceName,
		deploymentConfigMaps,
		deploymentSecrets,
		kubeInformersNamespaceScoped,
		staticPodOperatorClient,
		kubeClient,
		eventRecorder,
	)

	controller.installerController = installer.NewInstallerController(
		targetNamespaceName,
		staticPodName,

		deploymentConfigMaps,
		deploymentSecrets,
		command,
		kubeInformersNamespaceScoped,
		staticPodOperatorClient,
		kubeClient,
	)

	controller.nodeController = node.NewNodeController(
		staticPodOperatorClient,
		kubeInformersClusterScoped,
	)

	controller.serviceAccountController = backingresource.NewBackingResourceController(
		targetNamespaceName,
		staticPodOperatorClient,
		kubeInformersNamespaceScoped,
		kubeClient,
		eventRecorder,
	)

	return controller
}

func (o *staticPodOperatorControllers) Run(stopCh <-chan struct{}) {
	go o.serviceAccountController.Run(1, stopCh)
	go o.deploymentController.Run(1, stopCh)
	go o.installerController.Run(1, stopCh)
	go o.nodeController.Run(1, stopCh)

	<-stopCh
}
