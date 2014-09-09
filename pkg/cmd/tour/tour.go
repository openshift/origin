package tour

import (
	"bufio"
	"fmt"
	"os"
	"time"

	kubeclient "github.com/GoogleCloudPlatform/kubernetes/pkg/client"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/util/wait"
	"github.com/golang/glog"
	osclient "github.com/openshift/origin/pkg/client"
	"github.com/openshift/origin/pkg/cmd/util/docker"
	"github.com/spf13/cobra"
)

const longDescription = `
OpenShift Tour

The tour is an interactive walkthrough that helps you get familiar with how OpenShift works.
`

const tourDockerClientErr = `
OpenShift uses the Docker container runtime. We were unable to detect Docker running on this
system. Please see our getting started guide at:

  https://github.com/openshift/origin/CONTRIBUTING.adoc

for more information about configuring Docker properly.  You won't be able to complete the
tour without correcting this.

%s: %v
`
const tourDockerPingErr = tourDockerClientErr

const tourOne = `
First Steps
-----------

OpenShift is a platform for developing, building, and deploying networked applications. It
focuses on streamlining and automating the everyday tasks that developers and operators
depend on to create and maintain web applications, databases, message brokers, and other
critical pieces of their hobbies, projects, and businesses. It leverages Docker to package
and deliver software, and is built on top of Kubernetes, the open source cluster management
tool.

The first step is to start up and run the all-in-one OpenShift server, which as the name
might indicate includes everything you need to try out OpenShift. The server consists of
the OpenShift and Kubernetes APIs, the backend store etcd, and the agent that runs the
containers for your applications.

  OpenShift

    Clients:
      Browser, Mobile, or CLI -> API

    Masters:
      API -> Data-store (etcd) -> Scheduling -> Agents

    Nodes:
      Agent -> Docker -> Containers running your code

In an OpenShift deployment, you have a few servers called "masters" that are the API and
the data-store code. The other servers are known as "nodes" and are where containers are
run. In the all-in-one, your local system is client, master, and node.

NOTE: Before continuing, you'll want to open a new terminal window in the same directory,
      so you can try things side by side.
`

const tourOneRunning = `
It looks like you already have OpenShift running at %s
`

const tourOneTryLater = `
The server didn't start in time. If you're having troubles running the start command
please contact us on IRC in #openshift-dev on Freenode or open an issue on our GitHub
site:

  https://github.com/openshift/origin
`

const tourOneStart = `
In a new terminal window, start the all-in-one server by running:

  $ %s start

The tour will wait until it detects the server is running ...
`

const tourOneStarted = `
OpenShift was started at %s.
`

const tourTwo = `
Running Applications
--------------------

In OpenShift, application code runs in Docker containers on the hosts.  Every container
is part of a "pod" - a group of one or more containers that shares an IP address, some
filesystem directories, and are created or deleted together. Put multiple containers in
a pod when they need to access the same disk data (like a web server and a log analyzer).

If you want to have high availability or handle extra traffic, you'd create multiple
copies of the same pod. Let's start by creating a single pod that serves a simple "Hello,
OpenShift" web page.

The 'kube' subcommand is the Kubernetes client and has all of the core administrative
commands available. You can see the help at any time by running:

  $ %[1]s kube
`

const tourTwoCreate = `
From a new terminal window, run the following command to create a new pod:

  $ %[1]s kube -c https://raw.githubusercontent.com/openshift/origin/master/examples/hello-openshift/hello-pod.json create pods

The tour will wait until it detects the pod is created ...
`

const tourTwoCreated = `
Your pod has been created. You can see information about all of your pods by running

  $ %[1]s kube list pods

The 'Status' column tells you when your pod has started running on this system. 'Waiting'
means that the pod has been assigned to a node (the 'Host' column) but hasn't started up
yet. Once the pod is listed as 'Running' you should be able to see it in the list of
Docker containers on the system.

  $ docker ps

OpenShift (through Kubernetes) assigns a long name to each container that maps back to
the pod that created the container. You'll also see one 'kubernetes/pause' container for
each pod - this container is created first and holds on to the network and volume settings
for your pod, even if individual containers are restarted.

You should be able to connect to your container by curling the pod's host port:

  $ curl http://localhost:6061

Hello, OpenShift!
`

const tourHavingTrouble = `
%s

Please contact us on IRC in #openshift-dev on Freenode or open an issue on our GitHub
site if you continue to have problems:

  https://github.com/openshift/origin
`

const tourThree = `
This concludes the tour.  More coming soon!
`

const defaultServerAddr = "127.0.0.1:8080"
const maxWait = time.Minute * 10

func NewCommandTour(name string) *cobra.Command {
	dockerHelper := docker.NewHelper()
	cmd := &cobra.Command{
		Use:   name,
		Short: "An interactive OpenShift tour",
		Long:  longDescription,
		Run: func(c *cobra.Command, args []string) {
			tour(dockerHelper)
		},
	}
	dockerHelper.InstallFlags(cmd.Flags())
	return cmd
}

func tour(dockerHelper *docker.Helper) {
	cmd := os.Args[0]

	_, kubeClient := clients()

	// check Docker
	dockerClient, addr, err := dockerHelper.GetClient()
	if err != nil {
		fmt.Printf(tourDockerClientErr, addr, err)
		os.Exit(1)
	}
	if err := dockerClient.Ping(); err != nil {
		fmt.Printf(tourDockerPingErr, addr, err)
		//os.Exit(1)
		continueTour()
	}

	fmt.Printf(tourOne)
	continueTour()

	// check for server start
	if serverRunning(kubeClient) {
		fmt.Printf(tourOneRunning, defaultServerAddr)

	} else {
		fmt.Printf(tourOneStart, cmd)

		if err := wait.Poll(time.Second, maxWait, func() (bool, error) {
			return serverRunning(kubeClient), nil
		}); err == wait.ErrWaitTimeout {
			fmt.Printf(tourHavingTrouble, "The server didn't seem to start in time.")
			os.Exit(1)
		}

		fmt.Printf(tourOneStarted, defaultServerAddr)
	}
	continueTour()

	// create a pod
	fmt.Printf(tourTwo, cmd)
	continueTour()

	fmt.Printf(tourTwoCreate, cmd)
	if err := wait.Poll(time.Second, maxWait, waitForPod(kubeClient, "hello-openshift")); err == wait.ErrWaitTimeout {
		fmt.Printf(tourHavingTrouble, "The pod didn't seem to get created in time.")
		os.Exit(1)
	}

	// info about pod creation
	fmt.Printf(tourTwoCreated, cmd)
	continueTour()

	// more to come
	fmt.Printf(tourThree)
}

func continueTour() {
	fmt.Printf("\nHit ENTER to continue ... ")
	bio := bufio.NewReader(os.Stdin)
	for {
		_, hasMoreInLine, err := bio.ReadLine()
		if err != nil {
			glog.Fatalf("Unable to read newline: %v", err)
		}
		if !hasMoreInLine {
			break
		}
	}
}

func clients() (*osclient.Client, *kubeclient.Client) {
	kubeClient, err := kubeclient.New(defaultServerAddr, nil)
	if err != nil {
		glog.Fatalf("Unable to start tour: %v", err)
	}

	client, err := osclient.New(defaultServerAddr, nil)
	if err != nil {
		glog.Fatalf("Unable to start tour: %v", err)
	}
	return client, kubeClient
}

func serverRunning(client *kubeclient.Client) bool {
	if _, err := client.ServerVersion(); err != nil {
		return false
	}
	return true
}

func waitForPod(client *kubeclient.Client, id string) wait.ConditionFunc {
	return func() (bool, error) {
		_, err := client.GetPod(id)
		return (err == nil), nil
	}
}
