package network

import (
	"errors"
	"fmt"
	"io"
	"reflect"
	"strings"
	"time"

	"github.com/spf13/cobra"

	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/meta"
	kclient "k8s.io/kubernetes/pkg/client/unversioned"
	"k8s.io/kubernetes/pkg/kubectl/resource"
	"k8s.io/kubernetes/pkg/labels"
	"k8s.io/kubernetes/pkg/runtime"
	kerrors "k8s.io/kubernetes/pkg/util/errors"
	"k8s.io/kubernetes/pkg/util/sets"
	"k8s.io/kubernetes/pkg/util/wait"

	osclient "github.com/openshift/origin/pkg/client"
	"github.com/openshift/origin/pkg/cmd/util/clientcmd"
	"github.com/openshift/origin/pkg/project/api"
	sdnapi "github.com/openshift/origin/pkg/sdn/api"
)

type ProjectOptions struct {
	DefaultNamespace string
	Oclient          *osclient.Client
	Kclient          *kclient.Client
	Out              io.Writer

	Mapper            meta.RESTMapper
	Typer             runtime.ObjectTyper
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
	mapper, typer := f.Object(false)

	p.DefaultNamespace = defaultNamespace
	p.Oclient = oc
	p.Kclient = kc
	p.Out = out
	p.Mapper = mapper
	p.Typer = typer
	p.RESTClientFactory = f.Factory.ClientForMapping
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

	// TODO: Validate if the openshift master is running with mutitenant network plugin
	return kerrors.NewAggregate(errList)
}

func (p *ProjectOptions) GetProjects() ([]*api.Project, error) {
	nameArgs := []string{"projects"}
	if len(p.ProjectNames) != 0 {
		nameArgs = append(nameArgs, p.ProjectNames...)
	}

	r := resource.NewBuilder(p.Mapper, p.Typer, resource.ClientMapperFunc(p.RESTClientFactory), kapi.Codecs.UniversalDecoder()).
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
	projectList := []*api.Project{}
	_ = r.Visit(func(info *resource.Info, err error) error {
		if err != nil {
			return err
		}
		project, ok := info.Object.(*api.Project)
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
	netns, err := p.Oclient.NetNamespaces().Get(nsName)
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
		updatedNetNs, err := p.Oclient.NetNamespaces().Get(netns.NetName)
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
