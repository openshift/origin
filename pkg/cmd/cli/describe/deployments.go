package describe

import (
	"fmt"
	"io"
	"strconv"
	"strings"
	"text/tabwriter"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"

	"github.com/openshift/origin/pkg/client"
	deployapi "github.com/openshift/origin/pkg/deploy/api"
)

// DeploymentConfigDescriber generates information about a DeploymentConfig
type DeploymentConfigDescriber struct {
	client deploymentDescriberClient
}

type deploymentDescriberClient interface {
	GetDeploymentConfig(namespace, name string) (*deployapi.DeploymentConfig, error)
}

type genericDeploymentDescriberClient struct {
	getDeploymentConfig func(namespace, name string) (*deployapi.DeploymentConfig, error)
}

func (c *genericDeploymentDescriberClient) GetDeploymentConfig(namespace, name string) (*deployapi.DeploymentConfig, error) {
	return c.getDeploymentConfig(namespace, name)
}

func NewDeploymentConfigDescriberForConfig(config *deployapi.DeploymentConfig) *DeploymentConfigDescriber {
	return &DeploymentConfigDescriber{
		client: &genericDeploymentDescriberClient{
			getDeploymentConfig: func(namespace, name string) (*deployapi.DeploymentConfig, error) {
				return config, nil
			},
		},
	}
}

func NewDeploymentConfigDescriber(client client.Interface) *DeploymentConfigDescriber {
	return &DeploymentConfigDescriber{
		client: &genericDeploymentDescriberClient{
			getDeploymentConfig: func(namespace, name string) (*deployapi.DeploymentConfig, error) {
				return client.DeploymentConfigs(namespace).Get(name)
			},
		},
	}
}

func (d *DeploymentConfigDescriber) Describe(namespace, name string) (string, error) {
	deploymentConfig, err := d.client.GetDeploymentConfig(namespace, name)
	if err != nil {
		return "", err
	}

	return tabbedString(func(out *tabwriter.Writer) error {
		formatMeta(out, deploymentConfig.ObjectMeta)

		if deploymentConfig.LatestVersion == 0 {
			formatString(out, "Latest Version", "Not deployed")
		} else {
			formatString(out, "Latest Version", strconv.Itoa(deploymentConfig.LatestVersion))
		}

		printStrategy(deploymentConfig.Template.Strategy, out)
		printTriggers(deploymentConfig.Triggers, out)
		printReplicationController(deploymentConfig.Template.ControllerTemplate, out)

		return nil
	})
}

func printStrategy(strategy deployapi.DeploymentStrategy, w io.Writer) {
	fmt.Fprintf(w, "Strategy:\t%s\n", strategy.Type)
	switch strategy.Type {
	case deployapi.DeploymentStrategyTypeRecreate:
	case deployapi.DeploymentStrategyTypeCustom:
		fmt.Fprintf(w, "\t- Image:\t%s\n", strategy.CustomParams.Image)

		if len(strategy.CustomParams.Environment) > 0 {
			fmt.Fprintf(w, "\t- Environment:\t%s\n", formatLabels(convertEnv(strategy.CustomParams.Environment)))
		}

		if len(strategy.CustomParams.Command) > 0 {
			fmt.Fprintf(w, "\t- Command:\t%v\n", strings.Join(strategy.CustomParams.Command, " "))
		}
	}
}

func printTriggers(triggers []deployapi.DeploymentTriggerPolicy, w io.Writer) {
	if len(triggers) == 0 {
		fmt.Fprint(w, "No triggers.")
		return
	}

	fmt.Fprint(w, "Triggers:\n")
	for _, t := range triggers {
		fmt.Fprintf(w, "\t- %s\n", t.Type)
		switch t.Type {
		case deployapi.DeploymentTriggerOnConfigChange:
			fmt.Fprintf(w, "\t\t<no options>\n")
		case deployapi.DeploymentTriggerOnImageChange:
			fmt.Fprintf(w, "\t\tAutomatic:\t%v\n\t\tRepository:\t%s\n\t\tTag:\t%s\n",
				t.ImageChangeParams.Automatic,
				t.ImageChangeParams.RepositoryName,
				t.ImageChangeParams.Tag,
			)
		default:
			fmt.Fprint(w, "unknown\n")
		}
	}
}

func printReplicationController(spec kapi.ReplicationControllerSpec, w io.Writer) error {
	fmt.Fprint(w, "Template:\n")

	fmt.Fprintf(w, "\tSelector:\t%s\n\tReplicas:\t%d\n",
		formatLabels(spec.Selector),
		spec.Replicas)

	fmt.Fprintf(w, "\tContainers:\n\t\tNAME\tIMAGE\tENV\n")
	for _, container := range spec.Template.Spec.Containers {
		fmt.Fprintf(w, "\t\t%s\t%s\t%s\n",
			container.Name,
			container.Image,
			formatLabels(convertEnv(container.Env)))
	}
	return nil
}

// DeploymentDescriber generates information about a deployment
// DEPRECATED.
type DeploymentDescriber struct {
	client.Interface
}

func (d *DeploymentDescriber) Describe(namespace, name string) (string, error) {
	c := d.Deployments(namespace)
	deployment, err := c.Get(name)
	if err != nil {
		return "", err
	}

	return tabbedString(func(out *tabwriter.Writer) error {
		formatMeta(out, deployment.ObjectMeta)
		formatString(out, "Status", bold(deployment.Status))
		formatString(out, "Strategy", deployment.Strategy.Type)
		causes := []string{}
		if deployment.Details != nil {
			for _, c := range deployment.Details.Causes {
				causes = append(causes, string(c.Type))
			}
		}
		formatString(out, "Causes", strings.Join(causes, ","))
		return nil
	})
}
