package webconsole_operator

import (
	"fmt"
	"io"

	"github.com/golang/glog"
	"github.com/spf13/cobra"
	"k8s.io/kubernetes/pkg/client/leaderelectionconfig"

	"k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/uuid"
	"k8s.io/client-go/kubernetes"
	v1core "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/leaderelection"
	"k8s.io/client-go/tools/leaderelection/resourcelock"
	"k8s.io/client-go/tools/record"
	"k8s.io/kubernetes/pkg/api/legacyscheme"
	"k8s.io/kubernetes/pkg/kubectl/cmd/templates"
	kcmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"
)

const (
	RecommendedWebConsoleOperatorName = "openshift-webconsole-operator"
)

type WebConsoleOperatorCommandOptions struct {
	Output io.Writer
}

var longDescription = templates.LongDesc(`
	Install the OpenShift webconsole`)

func NewWebConsoleOperatorCommand(name string, out, errout io.Writer) *cobra.Command {
	options := &WebConsoleOperatorCommandOptions{Output: out}

	cmd := &cobra.Command{
		Use:   name,
		Short: "Install the OpenShift webconsole",
		Long:  longDescription,
		RunE: func(c *cobra.Command, args []string) error {
			kcmdutil.CheckErr(options.Validate())

			return options.RunWebConsoleOperator()
		},
	}

	return cmd
}

func (o *WebConsoleOperatorCommandOptions) Validate() error {
	return nil
}

func (o *WebConsoleOperatorCommandOptions) RunWebConsoleOperator() error {
	clientConfig, err := rest.InClusterConfig()
	if err != nil {
		return err
	}
	kubeClient, err := kubernetes.NewForConfig(clientConfig)
	if err != nil {
		return err
	}

	operator := &WebConsoleOperatorStarter{
		ClientConfig: clientConfig,
	}

	eventBroadcaster := record.NewBroadcaster()
	eventBroadcaster.StartLogging(glog.Infof)
	eventBroadcaster.StartRecordingToSink(&v1core.EventSinkImpl{Interface: v1core.New(kubeClient.CoreV1().RESTClient()).Events("")})
	eventRecorder := eventBroadcaster.NewRecorder(legacyscheme.Scheme, v1.EventSource{Component: "openshift-webconsole"})
	rl, err := resourcelock.New(
		resourcelock.ConfigMapsResourceLock,
		"openshift-core-operators",
		RecommendedWebConsoleOperatorName,
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
