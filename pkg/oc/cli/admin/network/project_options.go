package network

import (
	"errors"
	"fmt"
	"reflect"
	"strings"
	"time"

	"github.com/spf13/cobra"

	kapierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	kerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	kcmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"
	"k8s.io/kubernetes/pkg/kubectl/genericclioptions"
	"k8s.io/kubernetes/pkg/kubectl/genericclioptions/resource"
	"k8s.io/kubernetes/pkg/kubectl/scheme"

	networkv1 "github.com/openshift/api/network/v1"
	projectv1 "github.com/openshift/api/project/v1"
	networkv1typedclient "github.com/openshift/client-go/network/clientset/versioned/typed/network/v1"
	"github.com/openshift/library-go/pkg/network/networkapihelpers"
	"github.com/openshift/origin/pkg/network"
)

type ProjectOptions struct {
	DefaultNamespace string
	NetClient        networkv1typedclient.NetworkV1Interface
	KubeClient       kubernetes.Interface

	Builder *resource.Builder

	ProjectNames []string

	// Common optional params
	Selector      string
	CheckSelector bool

	genericclioptions.IOStreams
}

func NewProjectOptions(streams genericclioptions.IOStreams) *ProjectOptions {
	return &ProjectOptions{
		IOStreams: streams,
	}
}

func (p *ProjectOptions) Complete(f kcmdutil.Factory, c *cobra.Command, args []string) error {
	var err error
	p.DefaultNamespace, _, err = f.ToRawKubeConfigLoader().Namespace()
	if err != nil {
		return err
	}
	clientConfig, err := f.ToRESTConfig()
	if err != nil {
		return err
	}
	p.KubeClient, err = kubernetes.NewForConfig(clientConfig)
	if err != nil {
		return err
	}
	p.NetClient, err = networkv1typedclient.NewForConfig(clientConfig)
	if err != nil {
		return err
	}

	p.Builder = f.NewBuilder()
	p.ProjectNames = []string{}
	if len(args) != 0 {
		p.ProjectNames = append(p.ProjectNames, args...)
	}
	return nil
}

// Common validations
func (p *ProjectOptions) Validate() error {
	errList := []error{}
	if p.CheckSelector {
		if len(p.Selector) > 0 {
			if _, err := labels.Parse(p.Selector); err != nil {
				errList = append(errList, errors.New("--selector=<project_selector> must be a valid label selector"))
			}
		}
		if len(p.ProjectNames) != 0 {
			errList = append(errList, errors.New("either specify --selector=<project_selector> or projects but not both"))
		}
	} else if len(p.ProjectNames) == 0 {
		errList = append(errList, errors.New("must provide --selector=<project_selector> or projects"))
	}

	clusterNetwork, err := p.NetClient.ClusterNetworks().Get(networkv1.ClusterNetworkDefault, metav1.GetOptions{})
	if err != nil {
		if kapierrors.IsNotFound(err) {
			errList = append(errList, errors.New("managing pod network is only supported for openshift multitenant network plugin"))
		} else {
			errList = append(errList, errors.New("failed to fetch current network plugin info"))
		}
	} else if !network.IsOpenShiftMultitenantNetworkPlugin(clusterNetwork.PluginName) {
		errList = append(errList, fmt.Errorf("using plugin: %q, managing pod network is only supported for openshift multitenant network plugin", clusterNetwork.PluginName))
	}

	return kerrors.NewAggregate(errList)
}

func (p *ProjectOptions) GetProjects() ([]*projectv1.Project, error) {
	nameArgs := []string{"projects"}
	if len(p.ProjectNames) != 0 {
		nameArgs = append(nameArgs, p.ProjectNames...)
	}

	r := p.Builder.
		WithScheme(scheme.Scheme, scheme.Scheme.PrioritizedVersionsAllGroups()...).
		ContinueOnError().
		NamespaceParam(p.DefaultNamespace).
		LabelSelectorParam(p.Selector).
		ResourceTypeOrNameArgs(true, nameArgs...).
		Flatten().
		Do()
	if r.Err() != nil {
		return nil, r.Err()
	}

	errList := []error{}
	projectList := []*projectv1.Project{}
	_ = r.Visit(func(info *resource.Info, err error) error {
		if err != nil {
			return err
		}
		project, ok := info.Object.(*projectv1.Project)
		if !ok {
			err := fmt.Errorf("cannot convert input to Project: %v", reflect.TypeOf(info.Object))
			errList = append(errList, err)
			// Don't bail out if one project fails
			return nil
		}
		projectList = append(projectList, project)
		return nil
	})
	if len(errList) != 0 {
		return projectList, kerrors.NewAggregate(errList)
	}

	if len(projectList) == 0 {
		return projectList, fmt.Errorf("no projects found")
	} else {
		givenProjectNames := sets.NewString(p.ProjectNames...)
		foundProjectNames := sets.String{}
		for _, project := range projectList {
			foundProjectNames.Insert(project.ObjectMeta.Name)
		}
		skippedProjectNames := givenProjectNames.Difference(foundProjectNames)
		if skippedProjectNames.Len() > 0 {
			return projectList, fmt.Errorf("projects %v not found", strings.Join(skippedProjectNames.List(), ", "))
		}
	}
	return projectList, nil
}

func (p *ProjectOptions) UpdatePodNetwork(nsName string, action networkapihelpers.PodNetworkAction, args string) error {
	// Get corresponding NetNamespace for given namespace
	netns, err := p.NetClient.NetNamespaces().Get(nsName, metav1.GetOptions{})
	if err != nil {
		return err
	}

	// Apply pod network change intent
	networkapihelpers.SetChangePodNetworkAnnotation(netns, action, args)

	// Update NetNamespace object
	_, err = p.NetClient.NetNamespaces().Update(netns)
	if err != nil {
		return err
	}

	// Validate SDN controller applied or rejected the intent
	backoff := wait.Backoff{
		Steps:    15,
		Duration: 500 * time.Millisecond,
		Factor:   1.1,
	}
	return wait.ExponentialBackoff(backoff, func() (bool, error) {
		updatedNetNs, err := p.NetClient.NetNamespaces().Get(netns.NetName, metav1.GetOptions{})
		if err != nil {
			return false, err
		}

		if _, _, err = networkapihelpers.GetChangePodNetworkAnnotation(updatedNetNs); err == networkapihelpers.ErrorPodNetworkAnnotationNotFound {
			return true, nil
		}
		// Pod network change not applied yet
		return false, nil
	})
}
