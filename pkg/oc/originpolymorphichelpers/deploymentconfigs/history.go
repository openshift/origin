package deploymentconfigs

import (
	"bytes"
	"fmt"
	"sort"
	"text/tabwriter"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kapi "k8s.io/kubernetes/pkg/apis/core"
	kclientset "k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset"
	kcoreclient "k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset/typed/core/internalversion"
	"k8s.io/kubernetes/pkg/kubectl"
	kinternalprinters "k8s.io/kubernetes/pkg/printers/internalversion"

	appsapi "github.com/openshift/origin/pkg/apps/apis/apps"
	appsutil "github.com/openshift/origin/pkg/apps/util"
)

func NewDeploymentConfigHistoryViewer(kc kclientset.Interface) kubectl.HistoryViewer {
	return &DeploymentConfigHistoryViewer{rn: kc.Core()}
}

// DeploymentConfigHistoryViewer is an implementation of the kubectl HistoryViewer interface
// for deployment configs.
type DeploymentConfigHistoryViewer struct {
	rn kcoreclient.ReplicationControllersGetter
}

var _ kubectl.HistoryViewer = &DeploymentConfigHistoryViewer{}

// ViewHistory returns a description of all the history it can find for a deployment config.
func (h *DeploymentConfigHistoryViewer) ViewHistory(namespace, name string, revision int64) (string, error) {
	opts := metav1.ListOptions{LabelSelector: appsutil.ConfigSelector(name).String()}
	deploymentList, err := h.rn.ReplicationControllers(namespace).List(opts)
	if err != nil {
		return "", err
	}

	if len(deploymentList.Items) == 0 {
		return "No rollout history found.", nil
	}

	items := deploymentList.Items
	history := make([]*kapi.ReplicationController, 0, len(items))
	for i := range items {
		history = append(history, &items[i])
	}

	// Print details of a specific revision
	if revision > 0 {
		var desired *kapi.PodTemplateSpec
		// We could use a binary search here but brute-force is always faster to write
		for i := range history {
			rc := history[i]

			if appsutil.DeploymentVersionFor(rc) == revision {
				desired = rc.Spec.Template
				break
			}
		}

		if desired == nil {
			return "", fmt.Errorf("unable to find the specified revision")
		}

		buf := bytes.NewBuffer([]byte{})
		kinternalprinters.DescribePodTemplate(desired, kinternalprinters.NewPrefixWriter(buf))
		return buf.String(), nil
	}

	sort.Sort(appsutil.ByLatestVersionAsc(history))

	return tabbedString(func(out *tabwriter.Writer) error {
		fmt.Fprintf(out, "REVISION\tSTATUS\tCAUSE\n")
		for i := range history {
			rc := history[i]

			rev := appsutil.DeploymentVersionFor(rc)
			status := appsutil.DeploymentStatusFor(rc)
			cause := rc.Annotations[appsapi.DeploymentStatusReasonAnnotation]
			if len(cause) == 0 {
				cause = "<unknown>"
			}
			fmt.Fprintf(out, "%d\t%s\t%s\n", rev, status, cause)
		}
		return nil
	})
}

// TODO: Re-use from an utility package
func tabbedString(f func(*tabwriter.Writer) error) (string, error) {
	out := new(tabwriter.Writer)
	buf := &bytes.Buffer{}
	out.Init(buf, 0, 8, 1, '\t', 0)

	err := f(out)
	if err != nil {
		return "", err
	}

	out.Flush()
	str := string(buf.String())
	return str, nil
}
