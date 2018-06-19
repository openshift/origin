package info

import (
	"fmt"
	"net/http"
	"net/url"

	"github.com/spf13/cobra"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	apirequest "k8s.io/apiserver/pkg/endpoints/request"
	"k8s.io/kubernetes/pkg/kubectl/cmd/templates"
	kcmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"

	imageclient "github.com/openshift/client-go/image/clientset/versioned"
	imageapi "github.com/openshift/origin/pkg/image/apis/image"
	"github.com/openshift/origin/pkg/image/registryclient"
	"k8s.io/kubernetes/pkg/kubectl/genericclioptions"
)

var (
	desc = templates.LongDesc(`
		Display information about the integrated registry

		This command exposes information about the integrated registry, if configured.
		Use --check to verify your local client can access the registry. If the adminstrator
		has not configured a public hostname for the registry then this command may fail when
		run outside of the server.

		Experimental: This command is under active development and may change without notice.`)

	example = templates.Examples(`
# Display information about the integrated registry
%[1]s		
`)
)

type Options struct {
	Check        bool
	Quiet        bool
	ShowInternal bool
	ShowPublic   bool

	Namespaces []string
	Client     imageclient.Interface

	genericclioptions.IOStreams
}

func NewRegistryInfoOptions(streams genericclioptions.IOStreams) *Options {
	return &Options{
		IOStreams: streams,
	}
}

// New creates a command that displays info about the registry.
func NewRegistryInfoCmd(name string, f kcmdutil.Factory, streams genericclioptions.IOStreams) *cobra.Command {
	o := NewRegistryInfoOptions(streams)

	cmd := &cobra.Command{
		Use:     "info ",
		Short:   "Print info about the integrated registry",
		Long:    desc,
		Example: fmt.Sprintf(example, name+" login"),
		Run: func(c *cobra.Command, args []string) {
			kcmdutil.CheckErr(o.Complete(f, args))
			kcmdutil.CheckErr(o.Validate())
			kcmdutil.CheckErr(o.Run())
		},
	}

	flag := cmd.Flags()
	flag.BoolVar(&o.Check, "check", o.Check, "Attempt to contact the integrated registry.")
	flag.BoolVarP(&o.Check, "quiet", "q", o.Quiet, "Suppress normal output and only print status.")
	flag.BoolVar(&o.ShowInternal, "internal", o.ShowInternal, "Only check the internal registry hostname.")
	flag.BoolVar(&o.ShowPublic, "public", o.ShowPublic, "Only check the public registry hostname.")

	return cmd
}

func (o *Options) Complete(f kcmdutil.Factory, args []string) error {
	cfg, err := f.ToRESTConfig()
	if err != nil {
		return err
	}
	client, err := imageclient.NewForConfig(cfg)
	if err != nil {
		return err
	}
	o.Client = client

	ns, _, err := f.ToRawKubeConfigLoader().Namespace()
	if err != nil {
		return err
	}
	o.Namespaces = []string{ns, "openshift"}

	return nil
}

type RegistryInfo struct {
	Public   string
	Internal string
}

func (i *RegistryInfo) Installed() bool {
	return len(i.Public) > 0 || len(i.Internal) > 0
}

func (i *RegistryInfo) HostPort() (string, bool) {
	if len(i.Public) > 0 {
		return i.Public, true
	}
	return i.Internal, false
}

func findRegistryInfo(client imageclient.Interface, namespaces ...string) (*RegistryInfo, error) {
	for _, ns := range namespaces {
		imageStreams, err := client.Image().ImageStreams(ns).List(metav1.ListOptions{})
		if err != nil || len(imageStreams.Items) == 0 {
			continue
		}
		is := imageStreams.Items[0]

		info := &RegistryInfo{}
		if value := is.Status.PublicDockerImageRepository; len(value) > 0 {
			ref, err := imageapi.ParseDockerImageReference(value)
			if err != nil {
				return nil, fmt.Errorf("unable to parse public registry info from the server")
			}
			info.Public = ref.Registry
		}
		if value := is.Status.DockerImageRepository; len(value) > 0 {
			ref, err := imageapi.ParseDockerImageReference(value)
			if err != nil {
				return nil, fmt.Errorf("unable to parse internal registry info from the server")
			}
			info.Internal = ref.Registry
		}
		if !info.Installed() {
			return nil, fmt.Errorf("the integrated registry has not been configured")
		}
		return info, nil
	}
	return nil, fmt.Errorf("no image streams could be located to retrieve registry info, please specify a namespace with image streams")
}

func (o *Options) Validate() error {
	if o.ShowInternal && o.ShowPublic {
		return fmt.Errorf("only one of --internal or --public may be specified at a time")
	}
	return nil
}

func (o *Options) Run() error {
	info, err := findRegistryInfo(o.Client, o.Namespaces...)
	if err != nil {
		return err
	}

	var host string
	var public bool
	switch {
	case o.ShowPublic:
		host = info.Public
		if len(host) == 0 {
			return fmt.Errorf("registry does not have public hostname")
		}
	case o.ShowInternal:
		host = info.Internal
		public = false
		if len(host) == 0 {
			return fmt.Errorf("registry does not have an internal hostname")
		}
	default:
		host, public = info.HostPort()
	}

	if o.Check {
		ctx := apirequest.NewContext()
		if !public && !o.ShowInternal {
			fmt.Fprintf(o.ErrOut, "info: Registry does not have a public hostname\n")
		}
		url := &url.URL{Host: host}
		c := registryclient.NewContext(http.DefaultTransport, http.DefaultTransport)
		_, src, err := c.Ping(ctx, url, false)
		if err != nil {
			return fmt.Errorf("registry could not be contacted at %s: %v", url.Host, err)
		}
		host = src.Host
	}

	if !o.Quiet {
		fmt.Fprintf(o.Out, "%s\n", host)
	}
	return nil
}
