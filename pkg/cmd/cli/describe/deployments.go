package describe

import (
	"fmt"
	"io"
	"strconv"
	"strings"
	"text/tabwriter"

	"github.com/openshift/origin/pkg/api/graph"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	kerrors "github.com/GoogleCloudPlatform/kubernetes/pkg/api/errors"
	kclient "github.com/GoogleCloudPlatform/kubernetes/pkg/client"
	kctl "github.com/GoogleCloudPlatform/kubernetes/pkg/kubectl"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/labels"

	"github.com/openshift/origin/pkg/client"
	deployapi "github.com/openshift/origin/pkg/deploy/api"
	deployutil "github.com/openshift/origin/pkg/deploy/util"
)

// DeploymentConfigDescriber generates information about a DeploymentConfig
type DeploymentConfigDescriber struct {
	client deploymentDescriberClient
}

type deploymentDescriberClient interface {
	getDeploymentConfig(namespace, name string) (*deployapi.DeploymentConfig, error)
	getDeployment(namespace, name string) (*kapi.ReplicationController, error)
	listPods(namespace string, selector labels.Selector) (*kapi.PodList, error)
	listEvents(deploymentConfig *deployapi.DeploymentConfig) (*kapi.EventList, error)
}

type genericDeploymentDescriberClient struct {
	getDeploymentConfigFunc func(namespace, name string) (*deployapi.DeploymentConfig, error)
	getDeploymentFunc       func(namespace, name string) (*kapi.ReplicationController, error)
	listPodsFunc            func(namespace string, selector labels.Selector) (*kapi.PodList, error)
	listEventsFunc          func(deploymentConfig *deployapi.DeploymentConfig) (*kapi.EventList, error)
}

func (c *genericDeploymentDescriberClient) getDeploymentConfig(namespace, name string) (*deployapi.DeploymentConfig, error) {
	return c.getDeploymentConfigFunc(namespace, name)
}

func (c *genericDeploymentDescriberClient) getDeployment(namespace, name string) (*kapi.ReplicationController, error) {
	return c.getDeploymentFunc(namespace, name)
}

func (c *genericDeploymentDescriberClient) listPods(namespace string, selector labels.Selector) (*kapi.PodList, error) {
	return c.listPodsFunc(namespace, selector)
}

func (c *genericDeploymentDescriberClient) listEvents(deploymentConfig *deployapi.DeploymentConfig) (*kapi.EventList, error) {
	return c.listEventsFunc(deploymentConfig)
}

// NewDeploymentConfigDescriberForConfig returns a new DeploymentConfigDescriber
// for a DeploymentConfig
func NewDeploymentConfigDescriberForConfig(config *deployapi.DeploymentConfig) *DeploymentConfigDescriber {
	return &DeploymentConfigDescriber{
		client: &genericDeploymentDescriberClient{
			getDeploymentConfigFunc: func(namespace, name string) (*deployapi.DeploymentConfig, error) {
				return config, nil
			},
			getDeploymentFunc: func(namespace, name string) (*kapi.ReplicationController, error) {
				return nil, kerrors.NewNotFound("ReplicatonController", name)
			},
			listPodsFunc: func(namespace string, selector labels.Selector) (*kapi.PodList, error) {
				return nil, kerrors.NewNotFound("PodList", fmt.Sprintf("%v", selector))
			},
		},
	}
}

// NewDeploymentConfigDescriber returns a new DeploymentConfigDescriber
func NewDeploymentConfigDescriber(client client.Interface, kclient kclient.Interface) *DeploymentConfigDescriber {
	return &DeploymentConfigDescriber{
		client: &genericDeploymentDescriberClient{
			getDeploymentConfigFunc: func(namespace, name string) (*deployapi.DeploymentConfig, error) {
				return client.DeploymentConfigs(namespace).Get(name)
			},
			getDeploymentFunc: func(namespace, name string) (*kapi.ReplicationController, error) {
				return kclient.ReplicationControllers(namespace).Get(name)
			},
			listPodsFunc: func(namespace string, selector labels.Selector) (*kapi.PodList, error) {
				return kclient.Pods(namespace).List(selector)
			},
			listEventsFunc: func(deploymentConfig *deployapi.DeploymentConfig) (*kapi.EventList, error) {
				return kclient.Events(deploymentConfig.Namespace).Search(deploymentConfig)
			},
		},
	}
}

// Describe returns a description of a DeploymentConfigDescriber
func (d *DeploymentConfigDescriber) Describe(namespace, name string) (string, error) {
	deploymentConfig, err := d.client.getDeploymentConfig(namespace, name)
	if err != nil {
		return "", err
	}
	events, err := d.client.listEvents(deploymentConfig)
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

		printTriggers(deploymentConfig.Triggers, out)

		formatString(out, "Strategy", deploymentConfig.Template.Strategy.Type)
		printStrategy(deploymentConfig.Template.Strategy, out)
		printReplicationControllerSpec(deploymentConfig.Template.ControllerTemplate, out)

		deploymentName := deployutil.LatestDeploymentNameForConfig(deploymentConfig)
		deployment, err := d.client.getDeployment(namespace, deploymentName)
		if err != nil {
			if kerrors.IsNotFound(err) {
				formatString(out, "Latest Deployment", "<none>")
			} else {
				formatString(out, "Latest Deployment", fmt.Sprintf("error: %v", err))
			}
		} else {
			printDeploymentRc(deployment, d.client, out)
		}

		if events != nil {
			kctl.DescribeEvents(events, out)
		}
		return nil
	})
}

func printStrategy(strategy deployapi.DeploymentStrategy, w *tabwriter.Writer) {
	switch strategy.Type {
	case deployapi.DeploymentStrategyTypeRecreate:
		if strategy.RecreateParams != nil {
			pre := strategy.RecreateParams.Pre
			post := strategy.RecreateParams.Post
			if pre != nil {
				printHook("Pre-deployment", pre, w)
			}
			if post != nil {
				printHook("Post-deployment", post, w)
			}
		}
	case deployapi.DeploymentStrategyTypeCustom:
		fmt.Fprintf(w, "\t  Image:\t%s\n", strategy.CustomParams.Image)

		if len(strategy.CustomParams.Environment) > 0 {
			fmt.Fprintf(w, "\t  Environment:\t%s\n", formatLabels(convertEnv(strategy.CustomParams.Environment)))
		}

		if len(strategy.CustomParams.Command) > 0 {
			fmt.Fprintf(w, "\t  Command:\t%v\n", strings.Join(strategy.CustomParams.Command, " "))
		}
	}
}

func printHook(prefix string, hook *deployapi.LifecycleHook, w io.Writer) {
	if hook.ExecNewPod != nil {
		fmt.Fprintf(w, "\t  %s hook (pod type, failure policy: %s)\n", prefix, hook.FailurePolicy)
		fmt.Fprintf(w, "\t    Container:\t%s\n", hook.ExecNewPod.ContainerName)
		fmt.Fprintf(w, "\t    Command:\t%v\n", strings.Join(hook.ExecNewPod.Command, " "))
		fmt.Fprintf(w, "\t    Env:\t%s\n", formatLabels(convertEnv(hook.ExecNewPod.Env)))
	}
}

func printTriggers(triggers []deployapi.DeploymentTriggerPolicy, w *tabwriter.Writer) {
	if len(triggers) == 0 {
		formatString(w, "Triggers", "<none>")
		return
	}

	labels := []string{}

	for _, t := range triggers {
		switch t.Type {
		case deployapi.DeploymentTriggerOnConfigChange:
			labels = append(labels, "Config")
		case deployapi.DeploymentTriggerOnImageChange:
			if len(t.ImageChangeParams.RepositoryName) > 0 {
				labels = append(labels, fmt.Sprintf("Image(%s@%s, auto=%v)", t.ImageChangeParams.RepositoryName, t.ImageChangeParams.Tag, t.ImageChangeParams.Automatic))
			} else if len(t.ImageChangeParams.From.Name) > 0 {
				labels = append(labels, fmt.Sprintf("Image(%s@%s, auto=%v)", t.ImageChangeParams.From.Name, t.ImageChangeParams.Tag, t.ImageChangeParams.Automatic))
			}
		}
	}

	desc := strings.Join(labels, ", ")
	formatString(w, "Triggers", desc)
}

func printReplicationControllerSpec(spec kapi.ReplicationControllerSpec, w io.Writer) error {
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

func printDeploymentRc(deployment *kapi.ReplicationController, client deploymentDescriberClient, w io.Writer) error {
	running, waiting, succeeded, failed, err := getPodStatusForDeployment(deployment, client)
	if err != nil {
		return err
	}

	fmt.Fprint(w, "Latest Deployment:\n")
	fmt.Fprintf(w, "\tName:\t%s\n", deployment.Name)
	fmt.Fprintf(w, "\tStatus:\t%s\n", deployment.Annotations[deployapi.DeploymentStatusAnnotation])
	fmt.Fprintf(w, "\tSelector:\t%s\n", formatLabels(deployment.Spec.Selector))
	fmt.Fprintf(w, "\tLabels:\t%s\n", formatLabels(deployment.Labels))
	fmt.Fprintf(w, "\tReplicas:\t%d current / %d desired\n", deployment.Status.Replicas, deployment.Spec.Replicas)
	fmt.Fprintf(w, "\tPods Status:\t%d Running / %d Waiting / %d Succeeded / %d Failed\n", running, waiting, succeeded, failed)

	return nil
}

func getPodStatusForDeployment(deployment *kapi.ReplicationController, client deploymentDescriberClient) (running, waiting, succeeded, failed int, err error) {
	rcPods, err := client.listPods(deployment.Namespace, labels.SelectorFromSet(deployment.Spec.Selector))
	if err != nil {
		return
	}
	for _, pod := range rcPods.Items {
		switch pod.Status.Phase {
		case kapi.PodRunning:
			running++
		case kapi.PodPending:
			waiting++
		case kapi.PodSucceeded:
			succeeded++
		case kapi.PodFailed:
			failed++
		}
	}
	return
}

type LatestDeploymentDescriber struct {
	client deploymentDescriberClient
}

func NewLatestDeploymentDescriber(client client.Interface, kclient kclient.Interface) *LatestDeploymentDescriber {
	return &LatestDeploymentDescriber{
		client: &genericDeploymentDescriberClient{
			getDeploymentConfigFunc: func(namespace, name string) (*deployapi.DeploymentConfig, error) {
				return client.DeploymentConfigs(namespace).Get(name)
			},
			getDeploymentFunc: func(namespace, name string) (*kapi.ReplicationController, error) {
				return kclient.ReplicationControllers(namespace).Get(name)
			},
			listPodsFunc: func(namespace string, selector labels.Selector) (*kapi.PodList, error) {
				return kclient.Pods(namespace).List(selector)
			},
			listEventsFunc: func(deploymentConfig *deployapi.DeploymentConfig) (*kapi.EventList, error) {
				return kclient.Events(deploymentConfig.Namespace).Search(deploymentConfig)
			},
		},
	}
}

func (d *LatestDeploymentDescriber) Describe(namespace, name string) (string, error) {
	config, err := d.client.getDeploymentConfig(namespace, name)
	if err != nil {
		return "", err
	}

	deploymentName := deployutil.LatestDeploymentNameForConfig(config)
	deployment, err := d.client.getDeployment(config.Namespace, deploymentName)
	if err != nil && !kerrors.IsNotFound(err) {
		return "", err
	}

	g := graph.New()
	deploy := graph.DeploymentConfig(g, config)
	if deployment != nil {
		graph.JoinDeployments(deploy.(*graph.DeploymentConfigNode), []kapi.ReplicationController{*deployment})
	}

	return tabbedString(func(out *tabwriter.Writer) error {
		indent := "  "
		fmt.Fprintf(out, "Latest deployment for %s/%s:\n", name, namespace)
		printLines(out, indent, 1, d.describeDeployment(deploy.(*graph.DeploymentConfigNode))...)
		return nil
	})
}

func (d *LatestDeploymentDescriber) describeDeployment(node *graph.DeploymentConfigNode) []string {
	if node == nil {
		return nil
	}
	out := []string{}

	if node.ActiveDeployment == nil {
		on, auto := describeDeploymentConfigTriggers(node.DeploymentConfig)
		if node.DeploymentConfig.LatestVersion == 0 {
			out = append(out, fmt.Sprintf("#1 waiting %s. Run osc deploy --latest to deploy now.", on))
		} else if auto {
			out = append(out, fmt.Sprintf("#%d pending %s. Run osc deploy --latest to deploy now.", node.DeploymentConfig.LatestVersion, on))
		}
		// TODO: detect new image available?
	} else {
		out = append(out, d.describeDeploymentStatus(node.ActiveDeployment))
	}
	return out
}

func (d *LatestDeploymentDescriber) describeDeploymentStatus(deploy *kapi.ReplicationController) string {
	timeAt := strings.ToLower(formatRelativeTime(deploy.CreationTimestamp.Time))
	switch s := deploy.Annotations[deployapi.DeploymentStatusAnnotation]; deployapi.DeploymentStatus(s) {
	case deployapi.DeploymentStatusFailed:
		// TODO: encode fail time in the rc
		return fmt.Sprintf("#%s failed %s ago. You can restart this deployment with osc deploy --retry.", deploy.Annotations[deployapi.DeploymentVersionAnnotation], timeAt)
	case deployapi.DeploymentStatusComplete:
		// TODO: pod status output
		return fmt.Sprintf("#%s deployed %s ago", deploy.Annotations[deployapi.DeploymentVersionAnnotation], timeAt)
	default:
		return fmt.Sprintf("#%s deployment %s %s ago", deploy.Annotations[deployapi.DeploymentVersionAnnotation], strings.ToLower(s), timeAt)
	}
}

// DeploymentDescriber generates information about a deployment
// DEPRECATED.
type DeploymentDescriber struct {
	client.Interface
}

// Describe returns a description of a DeploymentDescriber
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
