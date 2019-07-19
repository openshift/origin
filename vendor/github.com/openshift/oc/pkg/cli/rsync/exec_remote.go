package rsync

import (
	"io"
	"strings"

	"k8s.io/klog"

	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/kubernetes"
	restclient "k8s.io/client-go/rest"
	kexec "k8s.io/kubernetes/pkg/kubectl/cmd/exec"
)

// remoteExecutor will execute commands on a given pod/container by using the kube Exec command
type remoteExecutor struct {
	Namespace         string
	PodName           string
	ContainerName     string
	SuggestedCmdUsage string
	Client            kubernetes.Interface
	Config            *restclient.Config
}

// Ensure it implements the executor interface
var _ executor = &remoteExecutor{}

// Execute will run a command in a pod
func (e *remoteExecutor) Execute(command []string, in io.Reader, out, errOut io.Writer) error {
	klog.V(3).Infof("Remote executor running command: %s", strings.Join(command, " "))
	execOptions := &kexec.ExecOptions{
		StreamOptions: kexec.StreamOptions{
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
		Executor:          &kexec.DefaultRemoteExecutor{},
		PodClient:         e.Client.CoreV1(),
		Config:            e.Config,
		Command:           command,
	}
	err := execOptions.Validate()
	if err != nil {
		klog.V(4).Infof("Error from remote command validation: %v", err)
		return err
	}
	err = execOptions.Run()
	if err != nil {
		klog.V(4).Infof("Error from remote execution: %v", err)
	}
	return err
}

func newRemoteExecutor(o *RsyncOptions) executor {
	return &remoteExecutor{
		Namespace:         o.Namespace,
		PodName:           o.PodName(),
		ContainerName:     o.ContainerName,
		SuggestedCmdUsage: o.SuggestedCmdUsage,
		Config:            o.Config,
		Client:            o.Client,
	}
}
