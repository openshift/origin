package orchestration_operator

import (
	"fmt"
	"io"
	"os"

	"github.com/golang/glog"
	"github.com/spf13/cobra"
	"k8s.io/kubernetes/pkg/client/leaderelectionconfig"

	"k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/util/uuid"
	"k8s.io/client-go/kubernetes"
	v1core "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/leaderelection"
	"k8s.io/client-go/tools/leaderelection/resourcelock"
	"k8s.io/client-go/tools/record"
	aggregatorinstall "k8s.io/kube-aggregator/pkg/apis/apiregistration/install"
	"k8s.io/kubernetes/pkg/api/legacyscheme"
	"k8s.io/kubernetes/pkg/kubectl/cmd/templates"
	kcmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"

	"github.com/openshift/origin/pkg/cmd/openshift-operators/orchestration-operator/controller"
	"github.com/openshift/origin/pkg/cmd/server/origin"
)

const (
	RecommendedOrchestrationOperatorName = "openshift-orchestration-operator"
)

type OrchestrationOperator struct {
	Output io.Writer
}

var longDescription = templates.LongDesc(`
	Install the OpenShift orchestrations`)

func NewOrchestrationOperatorCommand(name string, out, errout io.Writer) *cobra.Command {
	options := &OrchestrationOperator{Output: out}

	cmd := &cobra.Command{
		Use:   name,
		Short: "Install the OpenShift orchestrations",
		Long:  longDescription,
		Run: func(c *cobra.Command, args []string) {
			// TODO: register our own scheme
			aggregatorinstall.Install(legacyscheme.GroupFactoryRegistry, legacyscheme.Registry, legacyscheme.Scheme)

			kcmdutil.CheckErr(options.Validate())

			origin.StartProfiler()

			if err := options.RunOrchestrationOperator(); err != nil {
				if kerrors.IsInvalid(err) {
					if details := err.(*kerrors.StatusError).ErrStatus.Details; details != nil {
						fmt.Fprintf(errout, "Invalid %s %s\n", details.Kind, details.Name)
						for _, cause := range details.Causes {
							fmt.Fprintf(errout, "  %s: %s\n", cause.Field, cause.Message)
						}
						os.Exit(255)
					}
				}
				glog.Fatal(err)
			}
		},
	}

	return cmd
}

func (o *OrchestrationOperator) Validate() error {
	return nil
}

func (o *OrchestrationOperator) RunOrchestrationOperator() error {
	clientConfig, err := rest.InClusterConfig()
	if err != nil {
		return err
	}
	kubeClient, err := kubernetes.NewForConfig(clientConfig)
	if err != nil {
		return err
	}

	operator := &controller.OrchestrationOperatorStarter{
		ClientConfig: clientConfig,
	}

	eventBroadcaster := record.NewBroadcaster()
	eventBroadcaster.StartLogging(glog.Infof)
	eventBroadcaster.StartRecordingToSink(&v1core.EventSinkImpl{Interface: v1core.New(kubeClient.CoreV1().RESTClient()).Events("")})
	eventRecorder := eventBroadcaster.NewRecorder(legacyscheme.Scheme, v1.EventSource{Component: "openshift-orchestration"})
	rl, err := resourcelock.New(
		resourcelock.ConfigMapsResourceLock,
		"openshift-core-operators",
		RecommendedOrchestrationOperatorName,
		kubeClient.CoreV1(),
		resourcelock.ResourceLockConfig{
			Identity:      string(uuid.NewUUID()),
			EventRecorder: eventRecorder,
		})
	if err != nil {
		return err
	}
	leaderelection.RunOrDie(leaderelection.LeaderElectionConfig{
		Lock:          rl,
		LeaseDuration: leaderelectionconfig.DefaultLeaseDuration,
		RenewDeadline: leaderelectionconfig.DefaultRenewDeadline,
		RetryPeriod:   leaderelectionconfig.DefaultRetryPeriod,
		Callbacks: leaderelection.LeaderCallbacks{
			OnStartedLeading: operator.Run,
			OnStoppedLeading: func() {
				glog.Fatalf("leaderelection lost")
			},
		},
	})

	return fmt.Errorf("exiting")
}
