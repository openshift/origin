package cmd

import (
	"bytes"
	"fmt"
	"io"
	"strings"

	. "github.com/MakeNowJust/heredoc/dot"
	"github.com/spf13/cobra"

	ocutil "github.com/openshift/origin/pkg/cmd/util"
	"github.com/openshift/origin/pkg/cmd/util/clientcmd"
)

type term struct {
	Name         string
	Abbreviation string
	Description  string
}

var concepts = []term{
	{
		"Containers",
		"",
		D(`
      A definition of how to run one or more processes inside of a portable Linux
      environment. Containers are started from an Image and are usually isolated
      from other containers on the same machine.
    `),
	},
	{
		"Image",
		"",
		D(`
      A layered Linux filesystem that contains application code, dependencies,
      and any supporting operating system libraries. An image is identified by
      a name that can be local to the current cluster or point to a remote Docker
      registry (a storage server for images).
    `),
	}, {
		"Pods",
		"pod",
		D(`
      A set of one or more containers that are deployed onto a Node together and
      share a unique IP and Volumes (persistent storage). Pods also define the
      security and runtime policy for each container.
    `),
	}, {
		"Labels",
		"",
		D(`
      Labels are key value pairs that can be assigned to any resource in the
      system for grouping and selection. Many resources use labels to identify
      sets of other resources.
    `),
	}, {
		"Volumes",
		"",
		D(`
      Containers are not persistent by default - on restart their contents are
      cleared. Volumes are mounted filesystems available to Pods and their
      containers which may be backed by a number of host-local or network
      attached storage endpoints. The simplest volume type is EmptyDir, which
      is a temporary directory on a single machine. Administrators may also
      allow you to request a Persistent Volume that is automatically attached
      to your pods.
    `),
	}, {
		"Nodes",
		"node",
		D(`
      Machines set up in the cluster to run containers. Usually managed
      by administrators and not by end users.
    `),
	}, {
		"Services",
		"svc",
		D(`
      A name representing a set of pods (or external servers) that are
      accessed by other pods. The service gets an IP and a DNS name, and can be
      exposed externally to the cluster via a port or a Route. It's also easy
      to consume services from pods because an environment variable with the
      name <SERVICE>_HOST is automatically injected into other pods.
    `),
	}, {
		"Routes",
		"route",
		D(`
      A route is an external DNS entry (either a top level domain or a
      dynamically allocated name) that is created to point to a service so that
      it can be accessed outside the cluster. The administrator may configure
      one or more Routers to handle those routes, typically through an Apache
      or HAProxy load balancer / proxy.
    `),
	},
	{
		"Replication Controllers",
		"rc",
		D(`
      A replication controller maintains a specific number of pods based on a
      template that match a set of labels. If pods are deleted (because the
      node they run on is taken out of service) the controller creates a new
      copy of that pod. A replication controller is most commonly used to
      represent a single deployment of part of an application based on a
      built image.
    `),
	},
	{
		"Deployment Configuration",
		"dc",
		D(`
      Defines the template for a pod and manages deploying new images or 
      configuration changes whenever those change. A single deployment 
      configuration is usually analogous to a single micro-service. Can support 
      many different deployment pattrens, including full restart, customizable 
      rolling updates, and fully custom behaviors, as well as pre- and post- 
      hooks. Each deployment is represented as a replication controller.
    `),
	},
	{
		"Build Configuration",
		"bc",
		D(`
      Contains a description of how to build source code and a base image into a
      new image - the primary method for delivering changes to your application.
      Builds can be source based and use builder images for common languages like
      Java, PHP, Ruby, or Python, or be Docker based and create builds from a
      Dockerfile. Each build configuration has web-hooks and can be triggered
      automatically by changes to their base images.
    `),
	},
	{
		"Image Streams and Image Stream Tags",
		"is,istag",
		D(`
      An image stream groups sets of related images under tags - analogous to a
      branch in a source code repository. Each image stream may have one or
      more tags (the default tag is called "latest") and those tags may point
      at external Docker registries, at other tags in the same stream, or be
      controlled to directly point at known images. In addition, images can be
      pushed to an image stream tag directly via the integrated Docker 
      registry.
    `),
	},
	{
		"Projects",
		"project",
		D(`
      All of the above resources (except Nodes) exist inside of a project.
      Projects have a list of members and their roles, like viewer, editor,
      or admin, as well as a set of security controls on the running pods, and
      limits on how many resources the project can use. The names of each
      resource are unique within a project. Developers may request projects
      be created, but administrators control the resources allocated to
      projects.
    `),
	},
}

func writeTerm(w io.Writer, t term) {
	fmt.Fprintf(w, "* %s", t.Name)
	if len(t.Abbreviation) > 0 {
		fmt.Fprintf(w, " [%s]", t.Abbreviation)
	}
	fmt.Fprintln(w, ":")
	for _, s := range strings.Split(t.Description, "\n") {
		fmt.Fprintf(w, "    %s\n", s)
	}
}

var (
	typesLong = D(`
    OpenShift Concepts and Types

    OpenShift helps developers and operators build, test, and deploy applications in
    a containerized cloud environment. Applications may be composed of all of the
    components below, although most developers will be concerned with Services,
    Deployments, and Builds for delivering changes.

    Concepts:

    %[1]sFor more, see https://docs.openshift.com
  `)

	typesExample = `  // View all projects you have access to
  $ %[1]s projects

  // See a list of all services in the current project
  $ %[1]s get svc

  // Describe a deployment configuration in detail
  $ %[1]s describe dc mydeploymentconfig

  // Show the images tagged into an image stream
  $ %[1]s describe is ruby-centos7`
)

func NewCmdTypes(fullName string, f *clientcmd.Factory, out io.Writer) *cobra.Command {
	buf := &bytes.Buffer{}
	for _, c := range concepts {
		writeTerm(buf, c)
	}
	cmd := &cobra.Command{
		Use:     "types",
		Short:   "An introduction to the concepts and types in OpenShift",
		Long:    fmt.Sprintf(typesLong, buf.String()),
		Example: fmt.Sprintf(typesExample, fullName),
		Run:     ocutil.DefaultSubCommandRun(out),
	}
	return cmd
}
