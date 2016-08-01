package rsync

import (
	"io"
	"strings"

	"github.com/golang/glog"
	"k8s.io/kubernetes/pkg/client/restclient"
	kclient "k8s.io/kubernetes/pkg/client/unversioned"
	kubecmd "k8s.io/kubernetes/pkg/kubectl/cmd"

	"github.com/openshift/origin/pkg/cmd/util/clientcmd"
)

// remoteExecutor will execute commands on a given pod/container by using the kube Exec command
type remoteExecutor struct {
	Namespace     string
	PodName       string
	ContainerName string
	Client        *kclient.Client
	Config        *restclient.Config
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
			In:            in,
			Out:           out,
			Err:           errOut,
			Stdin:         in != nil,
		},
		Executor: &kubecmd.DefaultRemoteExecutor{},
		Client:   e.Client,
		Config:   e.Config,
		Command:  command,
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

func newRemoteExecutor(f *clientcmd.Factory, o *RsyncOptions) (executor, error) {
	config, err := f.ClientConfig()
	if err != nil {
		return nil, err
	}

	client, err := f.Client()
	if err != nil {
		return nil, err
	}

	return &remoteExecutor{
		Namespace:     o.Namespace,
		PodName:       o.PodName(),
		ContainerName: o.ContainerName,
		Config:        config,
		Client:        client,
	}, nil
}
