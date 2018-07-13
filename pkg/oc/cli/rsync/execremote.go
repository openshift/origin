package rsync

import (
	"io"
	"strings"

	"github.com/golang/glog"
	"k8s.io/kubernetes/pkg/kubectl/genericclioptions"

	restclient "k8s.io/client-go/rest"
	kclientset "k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset"
	kubecmd "k8s.io/kubernetes/pkg/kubectl/cmd"
	kcmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"
)

// remoteExecutor will execute commands on a given pod/container by using the kube Exec command
type remoteExecutor struct {
	Namespace         string
	PodName           string
	ContainerName     string
	SuggestedCmdUsage string
	Client            kclientset.Interface
	Config            *restclient.Config
}

// Ensure it implements the executor interface
var _ executor = &remoteExecutor{}

// Execute will run a command in a pod
func (e *remoteExecutor) Execute(command []string, in io.Reader, out, errOut io.Writer) error {
	glog.V(3).Infof("Remote executor running command: %s", strings.Join(command, " "))
	execOptions := &kubecmd.ExecOptions{
		StreamOptions: kubecmd.StreamOptions{
			Namespace:     e.Namespace,
			PodName:       e.PodName,
			ContainerName: e.ContainerName,
			IOStreams: genericclioptions.IOStreams{
				In:     in,
				Out:    out,
				ErrOut: errOut,
			},
			Stdin: in != nil,
		},
		SuggestedCmdUsage: e.SuggestedCmdUsage,
		Executor:          &kubecmd.DefaultRemoteExecutor{},
		PodClient:         e.Client.Core(),
		Config:            e.Config,
		Command:           command,
	}
	err := execOptions.Validate()
	if err != nil {
		glog.V(4).Infof("Error from remote command validation: %v", err)
		return err
	}
	err = execOptions.Run()
	if err != nil {
		glog.V(4).Infof("Error from remote execution: %v", err)
	}
	return err
}

func newRemoteExecutor(f kcmdutil.Factory, o *RsyncOptions) (executor, error) {
	config, err := f.ToRESTConfig()
	if err != nil {
		return nil, err
	}
	client, err := kclientset.NewForConfig(config)
	if err != nil {
		return nil, err
	}

	return &remoteExecutor{
		Namespace:         o.Namespace,
		PodName:           o.PodName(),
		ContainerName:     o.ContainerName,
		SuggestedCmdUsage: o.SuggestedCmdUsage,
		Config:            config,
		Client:            client,
	}, nil
}
