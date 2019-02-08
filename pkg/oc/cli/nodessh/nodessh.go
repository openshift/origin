package nodessh

import (
	"fmt"
	"golang.org/x/crypto/ssh"
	"io/ioutil"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"k8s.io/cli-runtime/pkg/genericclioptions"
	corev1client "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/kubernetes/pkg/kubectl/cmd/templates"
	kcmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"
)

const (
	NodesshRecommendedName = "node-ssh"
	User                   = "core"
)

var (
	nodesshLong = templates.LongDesc(`
		Open an ssh session to a node on the cluster
		`)

	nodesshExample = templates.Examples(`
	  # Open an ssh session to node 'foo'
	  %[1]s foo

	  # Run the command 'cat /etc/resolv.conf' inside node 'foo'
	  %[1]s foo cat /etc/resolv.conf
	  `)
)

// NodesshOptions declare the arguments accepted by the Nodessh command
type NodesshOptions struct {
	Timeout       int
	KeyFile       string
	NodeName      string
	Command       []string
	Bastion       string
	ServiceClient corev1client.ServicesGetter
}

func NewNodesshOptions(parent string, streams genericclioptions.IOStreams) *NodesshOptions {
	return &NodesshOptions{
		Timeout: 60,
	}
}

// NewCmdNodessh returns a command that attempts to open a shell session to the server.
func NewCmdNodessh(name string, parent string, f kcmdutil.Factory, streams genericclioptions.IOStreams) *cobra.Command {
	options := NewNodesshOptions(parent, streams)

	cmd := &cobra.Command{
		Use:     fmt.Sprintf("%s [flags] NODE [COMMAND]", name),
		Short:   "ssh to a node in the cluster",
		Long:    fmt.Sprintf(nodesshLong, parent),
		Example: fmt.Sprintf(nodesshExample, parent+" "+name),
		Run: func(cmd *cobra.Command, args []string) {
			kcmdutil.CheckErr(options.Complete(f, cmd, args))
			kcmdutil.CheckErr(options.Validate())
			kcmdutil.CheckErr(options.Run())
		},
	}
	cmd.Flags().IntVar(&options.Timeout, "timeout", 60, "Request timeout for obtaining a pod from the server; defaults to 10 seconds")
	cmd.Flags().StringVarP(&options.KeyFile, "identity-file", "i", "~/.ssh/id_rsa", "Path to private key file")
	cmd.Flags().SetInterspersed(false)
	return cmd
}

// Complete applies the command environment to NodesshOptions
func (o *NodesshOptions) Complete(f kcmdutil.Factory, cmd *cobra.Command, args []string) error {
	if len(args) < 1 {
		return kcmdutil.UsageErrorf(cmd, "nodessh requires a single Node to connect to")
	}
	o.NodeName = args[0]
	args = args[1:]
	if len(args) > 0 {
		o.Command = args
	}

	config, err := f.ToRESTConfig()
	if err != nil {
		return err
	}
	o.ServiceClient, err = corev1client.NewForConfig(config)

	return err
}

// Validate ensures that NodesshOptions are valid
func (o *NodesshOptions) Validate() error {
	return nil
}

// Run starts a remote shell session on the server
func (o *NodesshOptions) Run() error {
	bastion, err := o.getBastion()
	if err != nil {
		return err
	}
	o.Bastion = strings.Trim(bastion, "\"\n")
	return o.Connect()
}

// TODO: use o.ServiceClient
func (o *NodesshOptions) getBastion() (string, error) {
	// example of bastion node: "a0f26c3ec2b3911e9b04806746b5f062-105421878.us-west-1.elb.amazonaws.com"
	out, err := exec.Command("oc", "get", "service", "-n", "openshift-ssh-bastion", "ssh-bastion", "-o", "jsonpath=\"{.status.loadBalancer.ingress[0].hostname}\"").CombinedOutput()
	return string(out), err
}

func (o *NodesshOptions) Config() (*ssh.ClientConfig, error) {
	pemBytes, err := ioutil.ReadFile(o.KeyFile)
	if err != nil {
		return nil, err
	}
	signer, err := ssh.ParsePrivateKey(pemBytes)
	if err != nil {
		return nil, fmt.Errorf("failed to parse private key:%v", err)
	}
	config := &ssh.ClientConfig{
		User:            User,
		Auth:            []ssh.AuthMethod{ssh.PublicKeys(signer)},
		Timeout:         time.Duration(o.Timeout * int(time.Second)),
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
	}
	return config, err
}

func (o *NodesshOptions) Connect() error {
	config, err := o.Config()
	if err != nil {
		return fmt.Errorf("Error in creating ssh config: %v", err)
	}
	bastion := o.Bastion + ":22"
	bClient, err := ssh.Dial("tcp", bastion, config)
	if err != nil {
		return fmt.Errorf("dial bastion error:", err)
	}
	// Dial a connection to the service host, from the bastion
	conn, err := bClient.Dial("tcp", o.NodeName+":22")
	if err != nil {
		return fmt.Errorf("dial target error", err)
	}
	ncc, chans, reqs, err := ssh.NewClientConn(conn, o.NodeName+":22", config)
	if err != nil {
		return fmt.Errorf("new target conn error:", err)
	}

	targetClient := ssh.NewClient(ncc, chans, reqs)
	if err != nil {
		return fmt.Errorf("target ssh error:%v", err)
	}

	session, err := targetClient.NewSession()

	if err != nil {
		return fmt.Errorf("session failed:%v", err)
	}
	defer session.Close()

	// Set up terminal modes
	modes := ssh.TerminalModes{
		ssh.ECHO:          0,     // disable echoing
		ssh.TTY_OP_ISPEED: 14400, // input speed = 14.4kbaud
		ssh.TTY_OP_OSPEED: 14400, // output speed = 14.4kbaud
	}
	// Request pseudo terminal
	if err := session.RequestPty("xterm", 40, 80, modes); err != nil {
		return fmt.Errorf("request for pseudo terminal failed: ", err)
	}

	session.Stdout = os.Stdout
	session.Stdin = os.Stdin
	session.Stderr = os.Stderr

	// Start remote shell
	if err := session.Shell(); err != nil {
		return fmt.Errorf("failed to start shell: ", err)
	}

	return session.Wait()
}
