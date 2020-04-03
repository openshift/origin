package deployer

import (
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/labels"
	appsv1informers "k8s.io/client-go/informers/apps/v1"
	corev1informers "k8s.io/client-go/informers/core/v1"
	appsv1listers "k8s.io/client-go/listers/apps/v1"
	"k8s.io/client-go/tools/cache"

	operatorv1helpers "github.com/openshift/library-go/pkg/operator/v1helpers"
)

// DeploymentNodeProvider returns the node list from nodes matching the node selector of a Deployment
type DeploymentNodeProvider struct {
	targetNamespaceDeploymentInformer appsv1informers.DeploymentInformer
	targetNamespaceDeploymentLister   appsv1listers.DeploymentNamespaceLister
	nodeInformer                      corev1informers.NodeInformer
}

var (
	_ MasterNodeProvider = &DeploymentNodeProvider{}
)

func NewDeploymentNodeProvider(targetNamespace string, kubeInformersForNamespaces operatorv1helpers.KubeInformersForNamespaces) *DeploymentNodeProvider {
	return &DeploymentNodeProvider{
		targetNamespaceDeploymentInformer: kubeInformersForNamespaces.InformersFor(targetNamespace).Apps().V1().Deployments(),
		targetNamespaceDeploymentLister:   kubeInformersForNamespaces.InformersFor(targetNamespace).Apps().V1().Deployments().Lister().Deployments(targetNamespace),
		nodeInformer:                      kubeInformersForNamespaces.InformersFor("").Core().V1().Nodes(),
	}
}

func (p DeploymentNodeProvider) MasterNodeNames() ([]string, error) {
	deploy, err := p.targetNamespaceDeploymentLister.Get("apiserver")
	if err != nil && errors.IsNotFound(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	nodes, err := p.nodeInformer.Lister().List(labels.SelectorFromSet(deploy.Spec.Template.Spec.NodeSelector))
	if err != nil {
		return nil, err
	}

	ret := make([]string, 0, len(nodes))
	for _, n := range nodes {
		ret = append(ret, n.Name)
	}

	return ret, nil
}

func (p DeploymentNodeProvider) AddEventHandler(handler cache.ResourceEventHandler) []cache.InformerSynced {
	p.targetNamespaceDeploymentInformer.Informer().AddEventHandler(handler)
	p.nodeInformer.Informer().AddEventHandler(handler)

	return []cache.InformerSynced{
		p.targetNamespaceDeploymentInformer.Informer().HasSynced,
		p.nodeInformer.Informer().HasSynced,
	}
}
