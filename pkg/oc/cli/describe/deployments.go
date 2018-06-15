package describe

import (
	"fmt"
	"io"
	"sort"
	"strconv"
	"strings"
	"text/tabwriter"

	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/kubernetes/pkg/api/legacyscheme"
	"k8s.io/kubernetes/pkg/apis/autoscaling"
	kapi "k8s.io/kubernetes/pkg/apis/core"
	kclientset "k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset"
	kprinters "k8s.io/kubernetes/pkg/printers"
	kinternalprinters "k8s.io/kubernetes/pkg/printers/internalversion"

	appsapi "github.com/openshift/origin/pkg/apps/apis/apps"
	appsinternalversion "github.com/openshift/origin/pkg/apps/generated/internalclientset/typed/apps/internalversion"
	appsutil "github.com/openshift/origin/pkg/apps/util"
	imageapi "github.com/openshift/origin/pkg/image/apis/image"
	appsedges "github.com/openshift/origin/pkg/oc/graph/appsgraph"
	appsgraph "github.com/openshift/origin/pkg/oc/graph/appsgraph/nodes"
	"github.com/openshift/origin/pkg/oc/graph/genericgraph"
	kubegraph "github.com/openshift/origin/pkg/oc/graph/kubegraph/nodes"
)

const (
	// maxDisplayDeployments is the number of deployments to show when describing
	// deployment configuration.
	maxDisplayDeployments = 3

	// maxDisplayDeploymentsEvents is the number of events to display when
	// describing the deployment configuration.
	// TODO: Make the estimation of this number more sophisticated and make this
	// number configurable via DescriberSettings
	maxDisplayDeploymentsEvents = 8
)

// DeploymentConfigDescriber generates information about a DeploymentConfig
type DeploymentConfigDescriber struct {
	appsClient appsinternalversion.AppsInterface
	kubeClient kclientset.Interface

	config *appsapi.DeploymentConfig
}

// NewDeploymentConfigDescriber returns a new DeploymentConfigDescriber
func NewDeploymentConfigDescriber(client appsinternalversion.AppsInterface, kclient kclientset.Interface, config *appsapi.DeploymentConfig) *DeploymentConfigDescriber {
	return &DeploymentConfigDescriber{
		appsClient: client,
		kubeClient: kclient,
		config:     config,
	}
}

// Describe returns the description of a DeploymentConfig
func (d *DeploymentConfigDescriber) Describe(namespace, name string, settings kprinters.DescriberSettings) (string, error) {
	var deploymentConfig *appsapi.DeploymentConfig
	if d.config != nil {
		// If a deployment config is already provided use that.
		// This is used by `oc rollback --dry-run`.
		deploymentConfig = d.config
	} else {
		var err error
		deploymentConfig, err = d.appsClient.DeploymentConfigs(namespace).Get(name, metav1.GetOptions{})
		if err != nil {
			return "", err
		}
	}

	return tabbedString(func(out *tabwriter.Writer) error {
		formatMeta(out, deploymentConfig.ObjectMeta)
		var (
			deploymentsHistory   []*kapi.ReplicationController
			activeDeploymentName string
		)

		if d.config == nil {
			if rcs, err := d.kubeClient.Core().ReplicationControllers(namespace).List(metav1.ListOptions{LabelSelector: appsutil.ConfigSelector(deploymentConfig.Name).String()}); err == nil {
				deploymentsHistory = make([]*kapi.ReplicationController, 0, len(rcs.Items))
				for i := range rcs.Items {
					deploymentsHistory = append(deploymentsHistory, &rcs.Items[i])
				}
			}
		}

		if deploymentConfig.Status.LatestVersion == 0 {
			formatString(out, "Latest Version", "Not deployed")
		} else {
			formatString(out, "Latest Version", strconv.FormatInt(deploymentConfig.Status.LatestVersion, 10))
		}

		printDeploymentConfigSpec(d.kubeClient, *deploymentConfig, out)
		fmt.Fprintln(out)

		latestDeploymentName := appsutil.LatestDeploymentNameForConfig(deploymentConfig)
		if activeDeployment := appsutil.ActiveDeployment(deploymentsHistory); activeDeployment != nil {
			activeDeploymentName = activeDeployment.Name
		}

		var deployment *kapi.ReplicationController
		isNotDeployed := len(deploymentsHistory) == 0
		for _, item := range deploymentsHistory {
			if item.Name == latestDeploymentName {
				deployment = item
			}
		}
		if deployment == nil {
			isNotDeployed = true
		}

		if isNotDeployed {
			formatString(out, "Latest Deployment", "<none>")
		} else {
			header := fmt.Sprintf("Deployment #%d (latest)", appsutil.DeploymentVersionFor(deployment))
			// Show details if the current deployment is the active one or it is the
			// initial deployment.
			printDeploymentRc(deployment, d.kubeClient, out, header, (deployment.Name == activeDeploymentName) || len(deploymentsHistory) == 1)
		}

		// We don't show the deployment history when running `oc rollback --dry-run`.
		if d.config == nil && !isNotDeployed {
			var sorted []*kapi.ReplicationController
			// TODO(rebase-1.6): we should really convert the describer to use a versioned clientset
			for i := range deploymentsHistory {
				sorted = append(sorted, deploymentsHistory[i])
			}
			sort.Sort(sort.Reverse(OverlappingControllers(sorted)))
			counter := 1
			for _, item := range sorted {
				if item.Name != latestDeploymentName && deploymentConfig.Name == appsutil.DeploymentConfigNameFor(item) {
					header := fmt.Sprintf("Deployment #%d", appsutil.DeploymentVersionFor(item))
					printDeploymentRc(item, d.kubeClient, out, header, item.Name == activeDeploymentName)
					counter++
				}
				if counter == maxDisplayDeployments {
					break
				}
			}
		}

		if settings.ShowEvents {
			// Events
			if events, err := d.kubeClient.Core().Events(deploymentConfig.Namespace).Search(legacyscheme.Scheme, deploymentConfig); err == nil && events != nil {
				latestDeploymentEvents := &kapi.EventList{Items: []kapi.Event{}}
				for i := len(events.Items); i != 0 && i > len(events.Items)-maxDisplayDeploymentsEvents; i-- {
					latestDeploymentEvents.Items = append(latestDeploymentEvents.Items, events.Items[i-1])
				}
				fmt.Fprintln(out)
				pw := kinternalprinters.NewPrefixWriter(out)
				kinternalprinters.DescribeEvents(latestDeploymentEvents, pw)
			}
		}
		return nil
	})
}

// OverlappingControllers sorts a list of controllers by creation timestamp, using their names as a tie breaker.
// From
// https://github.com/kubernetes/kubernetes/blob/9eab226947d73a77cbf8474188f216cd64cd5fef/pkg/controller/replication/replication_controller_utils.go#L81-L92
// and modified to use internal instead of versioned objects.
type OverlappingControllers []*kapi.ReplicationController

func (o OverlappingControllers) Len() int      { return len(o) }
func (o OverlappingControllers) Swap(i, j int) { o[i], o[j] = o[j], o[i] }

func (o OverlappingControllers) Less(i, j int) bool {
	if o[i].CreationTimestamp.Equal(&o[j].CreationTimestamp) {
		return o[i].Name < o[j].Name
	}
	return o[i].CreationTimestamp.Before(&o[j].CreationTimestamp)
}

func multilineStringArray(sep, indent string, args ...string) string {
	for i, s := range args {
		if strings.HasSuffix(s, "\n") {
			s = strings.TrimSuffix(s, "\n")
		}
		if strings.Contains(s, "\n") {
			s = "\n" + indent + strings.Join(strings.Split(s, "\n"), "\n"+indent)
		}
		args[i] = s
	}
	strings.TrimRight(args[len(args)-1], "\n ")
	return strings.Join(args, " ")
}

func printStrategy(strategy appsapi.DeploymentStrategy, indent string, w *tabwriter.Writer) {
	if strategy.CustomParams != nil {
		if len(strategy.CustomParams.Image) == 0 {
			fmt.Fprintf(w, "%sImage:\t%s\n", indent, "<default>")
		} else {
			fmt.Fprintf(w, "%sImage:\t%s\n", indent, strategy.CustomParams.Image)
		}

		if len(strategy.CustomParams.Environment) > 0 {
			fmt.Fprintf(w, "%sEnvironment:\t%s\n", indent, formatLabels(convertEnv(strategy.CustomParams.Environment)))
		}

		if len(strategy.CustomParams.Command) > 0 {
			fmt.Fprintf(w, "%sCommand:\t%v\n", indent, multilineStringArray(" ", "\t  ", strategy.CustomParams.Command...))
		}
	}

	if strategy.RecreateParams != nil {
		pre := strategy.RecreateParams.Pre
		mid := strategy.RecreateParams.Mid
		post := strategy.RecreateParams.Post
		if pre != nil {
			printHook("Pre-deployment", pre, indent, w)
		}
		if mid != nil {
			printHook("Mid-deployment", mid, indent, w)
		}
		if post != nil {
			printHook("Post-deployment", post, indent, w)
		}
	}

	if strategy.RollingParams != nil {
		pre := strategy.RollingParams.Pre
		post := strategy.RollingParams.Post
		if pre != nil {
			printHook("Pre-deployment", pre, indent, w)
		}
		if post != nil {
			printHook("Post-deployment", post, indent, w)
		}
	}
}

func printHook(prefix string, hook *appsapi.LifecycleHook, indent string, w io.Writer) {
	if hook.ExecNewPod != nil {
		fmt.Fprintf(w, "%s%s hook (pod type, failure policy: %s):\n", indent, prefix, hook.FailurePolicy)
		fmt.Fprintf(w, "%s  Container:\t%s\n", indent, hook.ExecNewPod.ContainerName)
		fmt.Fprintf(w, "%s  Command:\t%v\n", indent, multilineStringArray(" ", "\t  ", hook.ExecNewPod.Command...))
		if len(hook.ExecNewPod.Env) > 0 {
			fmt.Fprintf(w, "%s  Env:\t%s\n", indent, formatLabels(convertEnv(hook.ExecNewPod.Env)))
		}
	}
	if len(hook.TagImages) > 0 {
		fmt.Fprintf(w, "%s%s hook (tag images, failure policy: %s):\n", indent, prefix, hook.FailurePolicy)
		for _, image := range hook.TagImages {
			fmt.Fprintf(w, "%s  Tag:\tcontainer %s to %s %s %s\n", indent, image.ContainerName, image.To.Kind, image.To.Name, image.To.Namespace)
		}
	}
}

func printTriggers(triggers []appsapi.DeploymentTriggerPolicy, w *tabwriter.Writer) {
	if len(triggers) == 0 {
		formatString(w, "Triggers", "<none>")
		return
	}

	labels := []string{}

	for _, t := range triggers {
		switch t.Type {
		case appsapi.DeploymentTriggerOnConfigChange:
			labels = append(labels, "Config")
		case appsapi.DeploymentTriggerOnImageChange:
			if len(t.ImageChangeParams.From.Name) > 0 {
				name, tag, _ := imageapi.SplitImageStreamTag(t.ImageChangeParams.From.Name)
				labels = append(labels, fmt.Sprintf("Image(%s@%s, auto=%v)", name, tag, t.ImageChangeParams.Automatic))
			}
		}
	}

	desc := strings.Join(labels, ", ")
	formatString(w, "Triggers", desc)
}

func printDeploymentConfigSpec(kc kclientset.Interface, dc appsapi.DeploymentConfig, w *tabwriter.Writer) error {
	spec := dc.Spec
	// Selector
	formatString(w, "Selector", formatLabels(spec.Selector))

	// Replicas
	test := ""
	if spec.Test {
		test = " (test, will be scaled down between deployments)"
	}
	formatString(w, "Replicas", fmt.Sprintf("%d%s", spec.Replicas, test))

	if spec.Paused {
		formatString(w, "Paused", "yes")
	}

	// Autoscaling info
	// FIXME: The CrossVersionObjectReference should specify the Group
	printAutoscalingInfo(
		[]schema.GroupResource{
			appsapi.Resource("DeploymentConfig"),
			// this needs to remain as long as HPA supports putting in the "wrong" DC scheme
			appsapi.LegacyResource("DeploymentConfig"),
		},
		dc.Namespace, dc.Name, kc, w)

	// Triggers
	printTriggers(spec.Triggers, w)

	// Strategy
	formatString(w, "Strategy", spec.Strategy.Type)
	printStrategy(spec.Strategy, "  ", w)

	if dc.Spec.MinReadySeconds > 0 {
		formatString(w, "MinReadySeconds", fmt.Sprintf("%d", spec.MinReadySeconds))
	}

	// Pod template
	fmt.Fprintf(w, "Template:\n")
	kinternalprinters.DescribePodTemplate(spec.Template, kinternalprinters.NewPrefixWriter(w))

	return nil
}

// TODO: Move this upstream
func printAutoscalingInfo(res []schema.GroupResource, namespace, name string, kclient kclientset.Interface, w *tabwriter.Writer) {
	hpaList, err := kclient.Autoscaling().HorizontalPodAutoscalers(namespace).List(metav1.ListOptions{LabelSelector: labels.Everything().String()})
	if err != nil {
		return
	}

	scaledBy := []autoscaling.HorizontalPodAutoscaler{}
	for _, hpa := range hpaList.Items {
		for _, r := range res {
			if hpa.Spec.ScaleTargetRef.Name == name && hpa.Spec.ScaleTargetRef.Kind == r.String() {
				scaledBy = append(scaledBy, hpa)
			}
		}
	}

	for _, hpa := range scaledBy {
		fmt.Fprintf(w, "Autoscaling:\tbetween %d and %d replicas", *hpa.Spec.MinReplicas, hpa.Spec.MaxReplicas)

		targetDescriptions := formatHPATargets(&hpa)
		if len(targetDescriptions) == 1 {
			fmt.Fprintf(w, " targeting %s\n", targetDescriptions[0])
		} else {
			fmt.Fprintf(w, "\n")
			for _, description := range targetDescriptions {
				// NB(directxman12): we should *not* use the wording "triggered at" here.
				// The HPA is *not* threshold-based.  Rather, it "aims" for a particular load,
				// quasi-constantly scaling the replica count by the ratio of current to target.
				fmt.Fprintf(w, "\t  targeting %s\n", description)
			}
		}
		// TODO: Print a warning in case of multiple hpas.
		// Related oc status PR: https://github.com/openshift/origin/pull/7799
		break
	}
}

// formatHPATargets formats a list of HPA targets in human readable form.  It functions similarly to the
// upstream describer and printer, except that it doesn't include status information, so it's more compact.
func formatHPATargets(hpa *autoscaling.HorizontalPodAutoscaler) []string {
	descriptions := make([]string, len(hpa.Spec.Metrics))
	for i, metricSpec := range hpa.Spec.Metrics {
		switch metricSpec.Type {
		case autoscaling.PodsMetricSourceType:
			descriptions[i] = fmt.Sprintf("%s %s average per pod", metricSpec.Pods.TargetAverageValue.String(), metricSpec.Pods.MetricName)
		case autoscaling.ObjectMetricSourceType:
			// TODO: it'd probably be more accurate if we put the group in here too,
			// but it might be a bit to verbose to read at a glance
			// TODO: we might want to use the resource name here instead of the kind?
			targetObjDesc := fmt.Sprintf("%s %s", metricSpec.Object.Target.Kind, metricSpec.Object.Target.Name)
			descriptions[i] = fmt.Sprintf("%s %s on %s", metricSpec.Object.TargetValue.String(), metricSpec.Object.MetricName, targetObjDesc)
		case autoscaling.ResourceMetricSourceType:
			if metricSpec.Resource.TargetAverageValue != nil {
				descriptions[i] = fmt.Sprintf("%s %s average per pod", metricSpec.Resource.TargetAverageValue.String(), metricSpec.Resource.Name)
			} else if metricSpec.Resource.TargetAverageUtilization != nil {
				descriptions[i] = fmt.Sprintf("%d%% %s average per pod", *metricSpec.Resource.TargetAverageUtilization, metricSpec.Resource.Name)
			} else {
				descriptions[i] = "<unset resource metric>"
			}
		default:
			descriptions[i] = "<unknown metric type>"
		}
	}

	return descriptions
}

func printDeploymentRc(deployment *kapi.ReplicationController, kubeClient kclientset.Interface, w io.Writer, header string, verbose bool) error {
	if len(header) > 0 {
		fmt.Fprintf(w, "%v:\n", header)
	}

	if verbose {
		fmt.Fprintf(w, "\tName:\t%s\n", deployment.Name)
	}
	timeAt := strings.ToLower(formatRelativeTime(deployment.CreationTimestamp.Time))
	fmt.Fprintf(w, "\tCreated:\t%s ago\n", timeAt)
	fmt.Fprintf(w, "\tStatus:\t%s\n", appsutil.DeploymentStatusFor(deployment))
	fmt.Fprintf(w, "\tReplicas:\t%d current / %d desired\n", deployment.Status.Replicas, deployment.Spec.Replicas)

	if verbose {
		fmt.Fprintf(w, "\tSelector:\t%s\n", formatLabels(deployment.Spec.Selector))
		fmt.Fprintf(w, "\tLabels:\t%s\n", formatLabels(deployment.Labels))
		running, waiting, succeeded, failed, err := getPodStatusForDeployment(deployment, kubeClient)
		if err != nil {
			return err
		}
		fmt.Fprintf(w, "\tPods Status:\t%d Running / %d Waiting / %d Succeeded / %d Failed\n", running, waiting, succeeded, failed)
	}

	return nil
}

func getPodStatusForDeployment(deployment *kapi.ReplicationController, kubeClient kclientset.Interface) (running, waiting, succeeded, failed int, err error) {
	rcPods, err := kubeClient.Core().Pods(deployment.Namespace).List(metav1.ListOptions{LabelSelector: labels.Set(deployment.Spec.Selector).AsSelector().String()})
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

type LatestDeploymentsDescriber struct {
	count      int
	appsClient appsinternalversion.AppsInterface
	kubeClient kclientset.Interface
}

// NewLatestDeploymentsDescriber lists the latest deployments limited to "count". In case count == -1, list back to the last successful.
func NewLatestDeploymentsDescriber(client appsinternalversion.AppsInterface, kclient kclientset.Interface, count int) *LatestDeploymentsDescriber {
	return &LatestDeploymentsDescriber{
		count:      count,
		appsClient: client,
		kubeClient: kclient,
	}
}

// Describe returns the description of the latest deployments for a config
func (d *LatestDeploymentsDescriber) Describe(namespace, name string) (string, error) {
	var f formatter

	config, err := d.appsClient.DeploymentConfigs(namespace).Get(name, metav1.GetOptions{})
	if err != nil {
		return "", err
	}

	var deployments []kapi.ReplicationController
	if d.count == -1 || d.count > 1 {
		list, err := d.kubeClient.Core().ReplicationControllers(namespace).List(metav1.ListOptions{LabelSelector: appsutil.ConfigSelector(name).String()})
		if err != nil && !kerrors.IsNotFound(err) {
			return "", err
		}
		deployments = list.Items
	} else {
		deploymentName := appsutil.LatestDeploymentNameForConfig(config)
		deployment, err := d.kubeClient.Core().ReplicationControllers(config.Namespace).Get(deploymentName, metav1.GetOptions{})
		if err != nil && !kerrors.IsNotFound(err) {
			return "", err
		}
		if deployment != nil {
			deployments = []kapi.ReplicationController{*deployment}
		}
	}

	g := genericgraph.New()
	dcNode := appsgraph.EnsureDeploymentConfigNode(g, config)
	for i := range deployments {
		kubegraph.EnsureReplicationControllerNode(g, &deployments[i])
	}
	appsedges.AddTriggerDeploymentConfigsEdges(g, dcNode)
	appsedges.AddDeploymentConfigsDeploymentEdges(g, dcNode)
	activeDeployment, inactiveDeployments := appsedges.RelevantDeployments(g, dcNode)

	return tabbedString(func(out *tabwriter.Writer) error {
		descriptions := describeDeploymentConfigDeployments(f, dcNode, activeDeployment, inactiveDeployments, nil, d.count)
		for i, description := range descriptions {
			descriptions[i] = fmt.Sprintf("%v %v", name, description)
		}
		printLines(out, "", 0, descriptions...)
		return nil
	})
}
