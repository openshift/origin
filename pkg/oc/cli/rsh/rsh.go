package rsh

import (
	"fmt"
	"sort"
	"time"

	"github.com/spf13/cobra"

	"k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	corev1client "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/kubernetes/pkg/apis/apps"
	"k8s.io/kubernetes/pkg/apis/batch"
	kapi "k8s.io/kubernetes/pkg/apis/core"
	"k8s.io/kubernetes/pkg/apis/extensions"
	kclientset "k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset"
	"k8s.io/kubernetes/pkg/controller"
	kubecmd "k8s.io/kubernetes/pkg/kubectl/cmd"
	"k8s.io/kubernetes/pkg/kubectl/cmd/templates"
	kcmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"
	"k8s.io/kubernetes/pkg/kubectl/genericclioptions"
	"k8s.io/kubernetes/pkg/kubectl/polymorphichelpers"
	"k8s.io/kubernetes/pkg/kubectl/util/term"

	oapps "github.com/openshift/api/apps"
	appstypedclient "github.com/openshift/client-go/apps/clientset/versioned/typed/apps/v1"
	appsutil "github.com/openshift/origin/pkg/apps/util"
	"github.com/openshift/origin/pkg/cmd/util"
)

const (
	RshRecommendedName = "rsh"
	DefaultShell       = "/bin/sh"
)

var (
	rshLong = templates.LongDesc(`
		Open a remote shell session to a container

		This command will attempt to start a shell session in a pod for the specified resource.
		It works with pods, deployment configs, deployments, jobs, daemon sets, replication controllers
		and replica sets.
		Any of the aforementioned resources (apart from pods) will be resolved to a ready pod.
		It will default to the first container if none is specified, and will attempt to use
		'/bin/sh' as the default shell. You may pass any flags supported by this command before
		the resource name, and an optional command after the resource name, which will be executed
		instead of a login shell. A TTY will be automatically allocated if standard input is
		interactive - use -t and -T to override. A TERM variable is sent to the environment where
		the shell (or command) will be executed. By default its value is the same as the TERM
		variable from the local environment; if not set, 'xterm' is used.

		Note, some containers may not include a shell - use '%[1]s exec' if you need to run commands
		directly.`)

	rshExample = templates.Examples(`
	  # Open a shell session on the first container in pod 'foo'
	  %[1]s foo

	  # Run the command 'cat /etc/resolv.conf' inside pod 'foo'
	  %[1]s foo cat /etc/resolv.conf

	  # See the configuration of your internal registry
	  %[1]s dc/docker-registry cat config.yml

	  # Open a shell session on the container named 'index' inside a pod of your job
	  # %[1]s -c index job/sheduled`)
)

// RshOptions declare the arguments accepted by the Rsh command
type RshOptions struct {
	ForceTTY   bool
	DisableTTY bool
	Executable string
	Timeout    int
	*kubecmd.ExecOptions
}

func NewRshOptions(parent string, streams genericclioptions.IOStreams) *RshOptions {
	return &RshOptions{
		ForceTTY:   false,
		DisableTTY: false,
		Timeout:    10,
		ExecOptions: &kubecmd.ExecOptions{
			StreamOptions: kubecmd.StreamOptions{
				IOStreams: streams,
				TTY:       true,
				Stdin:     true,
			},

			FullCmdName: parent,
			Executor:    &kubecmd.DefaultRemoteExecutor{},
		},
	}
}

// NewCmdRsh returns a command that attempts to open a shell session to the server.
func NewCmdRsh(name string, parent string, f kcmdutil.Factory, streams genericclioptions.IOStreams) *cobra.Command {
	options := NewRshOptions(parent, streams)

	cmd := &cobra.Command{
		Use:     fmt.Sprintf("%s [flags] POD [COMMAND]", name),
		Short:   "Start a shell session in a pod",
		Long:    fmt.Sprintf(rshLong, parent),
		Example: fmt.Sprintf(rshExample, parent+" "+name),
		Run: func(cmd *cobra.Command, args []string) {
			kcmdutil.CheckErr(options.Complete(f, cmd, args))
			kcmdutil.CheckErr(options.Validate())
			kcmdutil.CheckErr(options.Run())
		},
	}
	cmd.Flags().BoolVarP(&options.ForceTTY, "tty", "t", false, "Force a pseudo-terminal to be allocated")
	cmd.Flags().BoolVarP(&options.DisableTTY, "no-tty", "T", false, "Disable pseudo-terminal allocation")
	cmd.Flags().StringVar(&options.Executable, "shell", DefaultShell, "Path to the shell command")
	cmd.Flags().IntVar(&options.Timeout, "timeout", 10, "Request timeout for obtaining a pod from the server; defaults to 10 seconds")
	cmd.Flags().StringVarP(&options.ContainerName, "container", "c", "", "Container name; defaults to first container")
	cmd.Flags().SetInterspersed(false)
	return cmd
}

// Complete applies the command environment to RshOptions
func (o *RshOptions) Complete(f kcmdutil.Factory, cmd *cobra.Command, args []string) error {
	switch {
	case o.ForceTTY && o.DisableTTY:
		return kcmdutil.UsageErrorf(cmd, "you may not specify -t and -T together")
	case o.ForceTTY:
		o.TTY = true
	case o.DisableTTY:
		o.TTY = false
	default:
		o.TTY = term.IsTerminal(o.In)
	}

	if len(args) < 1 {
		return kcmdutil.UsageErrorf(cmd, "rsh requires a single Pod to connect to")
	}
	resource := args[0]
	args = args[1:]
	if len(args) > 0 {
		o.Command = args
	} else {
		o.Command = []string{o.Executable}
	}

	namespace, _, err := f.ToRawKubeConfigLoader().Namespace()
	if err != nil {
		return err
	}
	o.Namespace = namespace

	config, err := f.ToRESTConfig()
	if err != nil {
		return err
	}
	o.Config = config

	client, err := kclientset.NewForConfig(config)
	if err != nil {
		return err
	}
	o.PodClient = client.Core()

	o.PodName, err = podForResource(f, resource, time.Duration(o.Timeout)*time.Second)

	fullCmdName := ""
	cmdParent := cmd.Parent()
	if cmdParent != nil {
		fullCmdName = cmdParent.CommandPath()
	}
	if len(fullCmdName) > 0 && kcmdutil.IsSiblingCommandExists(cmd, "describe") {
		o.ExecOptions.SuggestedCmdUsage = fmt.Sprintf("Use '%s describe pod/%s -n %s' to see all of the containers in this pod.", fullCmdName, o.PodName, o.Namespace)
	}
	return err
}

// Validate ensures that RshOptions are valid
func (o *RshOptions) Validate() error {
	return o.ExecOptions.Validate()
}

// Run starts a remote shell session on the server
func (o *RshOptions) Run() error {
	// Insert the TERM into the command to be run
	if len(o.Command) == 1 && o.Command[0] == DefaultShell {
		termsh := fmt.Sprintf("TERM=%q %s", util.Env("TERM", "xterm"), DefaultShell)
		o.Command = append(o.Command, "-c", termsh)
	}
	return o.ExecOptions.Run()
}

func podForResource(f kcmdutil.Factory, resource string, timeout time.Duration) (string, error) {
	sortBy := func(pods []*v1.Pod) sort.Interface { return sort.Reverse(controller.ActivePods(pods)) }
	namespace, _, err := f.ToRawKubeConfigLoader().Namespace()
	if err != nil {
		return "", err
	}
	mapper, err := f.ToRESTMapper()
	if err != nil {
		return "", err
	}
	resourceType, name, err := util.ResolveResource(kapi.Resource("pods"), resource, mapper)
	if err != nil {
		return "", err
	}
	clientConfig, err := f.ToRESTConfig()
	if err != nil {
		return "", err
	}

	switch resourceType {
	case kapi.Resource("pods"):
		return name, nil
	case kapi.Resource("replicationcontrollers"):
		config, err := f.ToRESTConfig()
		if err != nil {
			return "", err
		}
		kc, err := corev1client.NewForConfig(config)
		if err != nil {
			return "", err
		}
		rc, err := kc.ReplicationControllers(namespace).Get(name, metav1.GetOptions{})
		if err != nil {
			return "", err
		}
		selector := labels.SelectorFromSet(rc.Spec.Selector)
		pod, _, err := polymorphichelpers.GetFirstPod(kc, namespace, selector.String(), timeout, sortBy)
		if err != nil {
			return "", err
		}
		return pod.Name, nil
	case oapps.Resource("deploymentconfigs"):
		appsClient, err := appstypedclient.NewForConfig(clientConfig)
		if err != nil {
			return "", err
		}
		dc, err := appsClient.DeploymentConfigs(namespace).Get(name, metav1.GetOptions{})
		if err != nil {
			return "", err
		}
		return podForResource(f, fmt.Sprintf("rc/%s", appsutil.LatestDeploymentNameForConfig(dc)), timeout)
	case extensions.Resource("daemonsets"):
		kc, err := f.ClientSet()
		if err != nil {
			return "", err
		}
		ds, err := kc.Extensions().DaemonSets(namespace).Get(name, metav1.GetOptions{})
		if err != nil {
			return "", err
		}
		selector, err := metav1.LabelSelectorAsSelector(ds.Spec.Selector)
		if err != nil {
			return "", err
		}
		config, err := f.ToRESTConfig()
		if err != nil {
			return "", err
		}
		coreclient, err := corev1client.NewForConfig(config)
		if err != nil {
			return "", err
		}

		pod, _, err := polymorphichelpers.GetFirstPod(coreclient, namespace, selector.String(), timeout, sortBy)
		if err != nil {
			return "", err
		}
		return pod.Name, nil
	case extensions.Resource("deployments"):
		kc, err := f.ClientSet()
		if err != nil {
			return "", err
		}
		d, err := kc.Extensions().Deployments(namespace).Get(name, metav1.GetOptions{})
		if err != nil {
			return "", err
		}
		selector, err := metav1.LabelSelectorAsSelector(d.Spec.Selector)
		if err != nil {
			return "", err
		}
		config, err := f.ToRESTConfig()
		if err != nil {
			return "", err
		}
		coreclient, err := corev1client.NewForConfig(config)
		if err != nil {
			return "", err
		}
		pod, _, err := polymorphichelpers.GetFirstPod(coreclient, namespace, selector.String(), timeout, sortBy)
		if err != nil {
			return "", err
		}
		return pod.Name, nil
	case apps.Resource("statefulsets"):
		kc, err := f.ClientSet()
		if err != nil {
			return "", err
		}
		s, err := kc.Apps().StatefulSets(namespace).Get(name, metav1.GetOptions{})
		if err != nil {
			return "", err
		}
		selector, err := metav1.LabelSelectorAsSelector(s.Spec.Selector)
		if err != nil {
			return "", err
		}
		config, err := f.ToRESTConfig()
		if err != nil {
			return "", err
		}
		coreclient, err := corev1client.NewForConfig(config)
		if err != nil {
			return "", err
		}
		pod, _, err := polymorphichelpers.GetFirstPod(coreclient, namespace, selector.String(), timeout, sortBy)
		if err != nil {
			return "", err
		}
		return pod.Name, nil
	case extensions.Resource("replicasets"):
		kc, err := f.ClientSet()
		if err != nil {
			return "", err
		}
		rs, err := kc.Extensions().ReplicaSets(namespace).Get(name, metav1.GetOptions{})
		if err != nil {
			return "", err
		}
		selector, err := metav1.LabelSelectorAsSelector(rs.Spec.Selector)
		if err != nil {
			return "", err
		}
		config, err := f.ToRESTConfig()
		if err != nil {
			return "", err
		}
		coreclient, err := corev1client.NewForConfig(config)
		if err != nil {
			return "", err
		}
		pod, _, err := polymorphichelpers.GetFirstPod(coreclient, namespace, selector.String(), timeout, sortBy)
		if err != nil {
			return "", err
		}
		return pod.Name, nil
	case batch.Resource("jobs"):
		kc, err := f.ClientSet()
		if err != nil {
			return "", err
		}
		job, err := kc.Batch().Jobs(namespace).Get(name, metav1.GetOptions{})
		if err != nil {
			return "", err
		}
		config, err := f.ToRESTConfig()
		if err != nil {
			return "", err
		}
		coreclient, err := corev1client.NewForConfig(config)
		if err != nil {
			return "", err
		}

		return podNameForJob(job, coreclient, timeout, sortBy)
	default:
		return "", fmt.Errorf("remote shell for %s is not supported", resourceType)
	}
}

func podNameForJob(job *batch.Job, kc corev1client.CoreV1Interface, timeout time.Duration, sortBy func(pods []*v1.Pod) sort.Interface) (string, error) {
	selector, err := metav1.LabelSelectorAsSelector(job.Spec.Selector)
	if err != nil {
		return "", err
	}

	pod, _, err := polymorphichelpers.GetFirstPod(kc, job.Namespace, selector.String(), timeout, sortBy)
	if err != nil {
		return "", err
	}
	return pod.Name, nil
}
