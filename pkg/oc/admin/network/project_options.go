package network

import (
	"errors"
	"fmt"
	"io"
	"reflect"
	"strings"
	"time"

	"github.com/spf13/cobra"

	kapierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	kerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/util/wait"
	kapi "k8s.io/kubernetes/pkg/api"
	kclientset "k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset"
	"k8s.io/kubernetes/pkg/kubectl/resource"

	osclient "github.com/openshift/origin/pkg/client"
	"github.com/openshift/origin/pkg/cmd/util/clientcmd"
	projectapi "github.com/openshift/origin/pkg/project/apis/project"
	sdnapi "github.com/openshift/origin/pkg/sdn/apis/network"
)

type ProjectOptions struct {
	DefaultNamespace string
	Oclient          *osclient.Client
	Kclient          kclientset.Interface
	Out              io.Writer

	Mapper            meta.RESTMapper
	Typer             runtime.ObjectTyper
	CategoryExpander  resource.CategoryExpander
	RESTClientFactory func(mapping *meta.RESTMapping) (resource.RESTClient, error)

	ProjectNames []string

	// Common optional params
	Selector      string
	CheckSelector bool
}

func (p *ProjectOptions) Complete(f *clientcmd.Factory, c *cobra.Command, args []string, out io.Writer) error {
	defaultNamespace, _, err := f.DefaultNamespace()
	if err != nil {
		return err
	}
	oc, kc, err := f.Clients()
	if err != nil {
		return err
	}
	mapper, typer := f.Object()

	p.DefaultNamespace = defaultNamespace
	p.Oclient = oc
	p.Kclient = kc
	p.Out = out
	p.Mapper = mapper
	p.Typer = typer
	p.CategoryExpander = f.CategoryExpander()
	p.RESTClientFactory = f.ClientForMapping
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

	clusterNetwork, err := p.Oclient.ClusterNetwork().Get(sdnapi.ClusterNetworkDefault, metav1.GetOptions{})
	if err != nil {
		if kapierrors.IsNotFound(err) {
			errList = append(errList, errors.New("Managing pod network is only supported for openshift multitenant network plugin"))
		} else {
			errList = append(errList, errors.New("Failed to fetch current network plugin info"))
		}
	} else if !sdnapi.IsOpenShiftMultitenantNetworkPlugin(clusterNetwork.PluginName) {
		errList = append(errList, fmt.Errorf("Using plugin: %q, managing pod network is only supported for openshift multitenant network plugin", clusterNetwork.PluginName))
	}

	return kerrors.NewAggregate(errList)
}

func (p *ProjectOptions) GetProjects() ([]*projectapi.Project, error) {
	nameArgs := []string{"projects"}
	if len(p.ProjectNames) != 0 {
		nameArgs = append(nameArgs, p.ProjectNames...)
	}

	r := resource.NewBuilder(p.Mapper, p.CategoryExpander, p.Typer, resource.ClientMapperFunc(p.RESTClientFactory), kapi.Codecs.UniversalDecoder()).
		ContinueOnError().
		NamespaceParam(p.DefaultNamespace).
		SelectorParam(p.Selector).
		ResourceTypeOrNameArgs(true, nameArgs...).
		Flatten().
		Do()
	if r.Err() != nil {
		return nil, r.Err()
	}

	errList := []error{}
	projectList := []*projectapi.Project{}
	_ = r.Visit(func(info *resource.Info, err error) error {
		if err != nil {
			return err
		}
		project, ok := info.Object.(*projectapi.Project)
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
		return projectList, fmt.Errorf("No projects found")
	} else {
		givenProjectNames := sets.NewString(p.ProjectNames...)
		foundProjectNames := sets.String{}
		for _, project := range projectList {
			foundProjectNames.Insert(project.ObjectMeta.Name)
		}
		skippedProjectNames := givenProjectNames.Difference(foundProjectNames)
		if skippedProjectNames.Len() > 0 {
			return projectList, fmt.Errorf("Projects %v not found", strings.Join(skippedProjectNames.List(), ", "))
		}
	}
	return projectList, nil
}

func (p *ProjectOptions) UpdatePodNetwork(nsName string, action sdnapi.PodNetworkAction, args string) error {
	// Get corresponding NetNamespace for given namespace
	netns, err := p.Oclient.NetNamespaces().Get(nsName, metav1.GetOptions{})
	if err != nil {
		return err
	}

	// Apply pod network change intent
	sdnapi.SetChangePodNetworkAnnotation(netns, action, args)

	// Update NetNamespace object
	_, err = p.Oclient.NetNamespaces().Update(netns)
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
		updatedNetNs, err := p.Oclient.NetNamespaces().Get(netns.NetName, metav1.GetOptions{})
		if err != nil {
			return false, err
		}

		if _, _, err = sdnapi.GetChangePodNetworkAnnotation(updatedNetNs); err == sdnapi.ErrorPodNetworkAnnotationNotFound {
			return true, nil
		}
		// Pod network change not applied yet
		return false, nil
	})
}
