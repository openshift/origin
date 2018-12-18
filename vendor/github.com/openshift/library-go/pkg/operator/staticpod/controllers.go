package staticpod

import (
	"github.com/openshift/library-go/pkg/operator/events"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"

	"github.com/openshift/library-go/pkg/operator/staticpod/controller/backingresource"
	"github.com/openshift/library-go/pkg/operator/staticpod/controller/common"
	"github.com/openshift/library-go/pkg/operator/staticpod/controller/installer"
	"github.com/openshift/library-go/pkg/operator/staticpod/controller/node"
	"github.com/openshift/library-go/pkg/operator/staticpod/controller/revision"
)

type staticPodOperatorControllers struct {
	revisionController       *revision.RevisionController
	installerController      *installer.InstallerController
	nodeController           *node.NodeController
	serviceAccountController *backingresource.BackingResourceController
}

// NewControllers provides all control loops needed to run a static pod based operator. That includes:
// 1. RevisionController - this watches multiple resources for "latest" input that has changed from the most current revision.
//    When a change is found, it creates a new revision by copying resources and adding the revision suffix to the names
//    to make a theoretically immutable set of revision data.  It then bumps the latestRevision and starts watching again.
// 2. InstallerController - this watches the latestRevision and the list of kubeletStatus (alpha-sorted list).  When a latestRevision
//    appears that doesn't match the current latest for first kubeletStatus and the first kubeletStatus isn't already transitioning,
//    it kicks off an installer pod.  If the next kubeletStatus doesn't match the immediate prior one, it kicks off that transition.
// 3. NodeController - watches nodes for master nodes and keeps the operator status up to date
func NewControllers(targetNamespaceName, staticPodName string, command, revisionConfigMaps, revisionSecrets []string,
	staticPodOperatorClient common.OperatorClient, kubeClient kubernetes.Interface, kubeInformersNamespaceScoped,
	kubeInformersClusterScoped informers.SharedInformerFactory, eventRecorder events.Recorder) *staticPodOperatorControllers {
	controller := &staticPodOperatorControllers{}

	controller.revisionController = revision.NewRevisionController(
		targetNamespaceName,
		revisionConfigMaps,
		revisionSecrets,
		kubeInformersNamespaceScoped,
		staticPodOperatorClient,
		kubeClient,
		eventRecorder,
	)

	controller.installerController = installer.NewInstallerController(
		targetNamespaceName,
		staticPodName,
		revisionConfigMaps,
		revisionSecrets,
		command,
		kubeInformersNamespaceScoped,
		staticPodOperatorClient,
		kubeClient,
		eventRecorder,
	)

	controller.nodeController = node.NewNodeController(
		staticPodOperatorClient,
		kubeInformersClusterScoped,
		eventRecorder,
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
	go o.revisionController.Run(1, stopCh)
	go o.installerController.Run(1, stopCh)
	go o.nodeController.Run(1, stopCh)

	<-stopCh
}
