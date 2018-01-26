package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"

	etcdversion "github.com/coreos/etcd/version"

	kapierrors "k8s.io/apimachinery/pkg/api/errors"
	kubeversiontypes "k8s.io/apimachinery/pkg/version"
	kclientcmd "k8s.io/client-go/tools/clientcmd"
	kclientset "k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset"
	"k8s.io/kubernetes/pkg/kubectl/cmd/templates"
	kcmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"
	kubeversion "k8s.io/kubernetes/pkg/version"

	"github.com/openshift/origin/pkg/oc/cli/util/clientcmd"
	"github.com/openshift/origin/pkg/oc/util/tokencmd"
	"github.com/openshift/origin/pkg/version"

	"github.com/spf13/cobra"
)

var (
	versionLong = templates.LongDesc(`Display client and server versions.`)
)

type VersionOptions struct {
	BaseName string
	Out      io.Writer

	ClientConfig kclientcmd.ClientConfig
	Clients      func() (kclientset.Interface, error)

	Timeout time.Duration

	IsServer            bool
	PrintEtcdVersion    bool
	PrintClientFeatures bool
}

// NewCmdVersion creates a command for displaying the version of this binary
func NewCmdVersion(fullName string, f *clientcmd.Factory, out io.Writer, options VersionOptions) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "version",
		Short: "Display client and server versions",
		Long:  versionLong,
		Run: func(cmd *cobra.Command, args []string) {
			options.BaseName = fullName

			if err := options.Complete(cmd, f, out); err != nil {
				kcmdutil.CheckErr(kcmdutil.UsageErrorf(cmd, err.Error()))
			}

			if err := options.RunVersion(); err != nil {
				kcmdutil.CheckErr(err)
			}
		},
	}

	return cmd
}

func (o *VersionOptions) Complete(cmd *cobra.Command, f *clientcmd.Factory, out io.Writer) error {
	o.Out = out

	if f == nil {
		return nil
	}

	o.Clients = f.ClientSet
	o.ClientConfig = f.OpenShiftClientConfig()

	if !o.IsServer {
		// retrieve config timeout and set cmd option
		// use this instead of getting value from global
		// flag, as flag value would have to be parsed
		// from a string potentially not formatted as
		// a valid time.Duration value
		config, err := o.ClientConfig.ClientConfig()
		if err == nil {
			o.Timeout = config.Timeout
		}
	}
	if o.Timeout == 0 {
		o.Timeout = time.Duration(10 * time.Second)
	}
	return nil
}

// RunVersion attempts to display client and server versions for Kubernetes and OpenShift
func (o VersionOptions) RunVersion() error {
	fmt.Fprintf(o.Out, "%s %v\n", o.BaseName, version.Get())
	fmt.Fprintf(o.Out, "kubernetes %v\n", kubeversion.Get())
	if o.PrintEtcdVersion {
		fmt.Fprintf(o.Out, "etcd %v\n", etcdversion.Version)
	}

	if o.PrintClientFeatures {
		features := []string{}
		if tokencmd.BasicEnabled() {
			features = append(features, "Basic-Auth")
		}
		if tokencmd.GSSAPIEnabled() {
			features = append(features, "GSSAPI")
			features = append(features, "Kerberos") // GSSAPI or SSPI
			features = append(features, "SPNEGO")   // GSSAPI or SSPI
		}
		fmt.Printf("features: %s\n", strings.Join(features, " "))
	}

	// do not attempt to print server info if already running cmd as the server
	// or if no client config is present
	if o.ClientConfig == nil || o.IsServer {
		return nil
	}

	done := make(chan error)
	oVersion := ""
	kVersion := ""
	versionHost := ""

	// start goroutine to fetch openshift / kubernetes server version
	go func() {
		defer close(done)

		// confirm config exists before making request to server
		var err error
		clientConfig, err := o.ClientConfig.ClientConfig()
		if err != nil {
			done <- err
			return
		}
		versionHost = clientConfig.Host

		kClient, err := o.Clients()
		if err != nil {
			done <- err
			return
		}

		kRESTClient := kClient.Core().RESTClient()
		kubeVersionBody, err := kRESTClient.Get().AbsPath("/version").Do().Raw()
		switch {
		case err == nil:
			var kubeServerInfo kubeversiontypes.Info
			err = json.Unmarshal(kubeVersionBody, &kubeServerInfo)
			if err != nil && len(kubeVersionBody) > 0 {
				done <- err
				return
			}
			kVersion = fmt.Sprintf("%v", kubeServerInfo)
		case kapierrors.IsNotFound(err) || kapierrors.IsUnauthorized(err) || kapierrors.IsForbidden(err):
		default:
			done <- err
			return
		}

		ocVersionBody, err := kClient.Discovery().RESTClient().Get().AbsPath("/version/openshift").Do().Raw()
		switch {
		case err == nil:
			var ocServerInfo version.Info
			err = json.Unmarshal(ocVersionBody, &ocServerInfo)
			if err != nil && len(ocVersionBody) > 0 {
				done <- err
				return
			}
			oVersion = fmt.Sprintf("%v", ocServerInfo)
		case kapierrors.IsNotFound(err) || kapierrors.IsUnauthorized(err) || kapierrors.IsForbidden(err):
		default:
			done <- err
			return
		}
	}()

	select {
	case err, closed := <-done:
		if strings.HasSuffix(fmt.Sprintf("%v", err), "connection refused") || clientcmd.IsConfigurationMissing(err) || kclientcmd.IsConfigurationInvalid(err) {
			return nil
		}
		if closed && err != nil {
			return err
		}
	case <-time.After(o.Timeout):
		return fmt.Errorf("%s", "error: server took too long to respond with version information.")
	}

	if oVersion != "" || kVersion != "" {
		fmt.Fprintf(o.Out, "\n%s%s\n", "Server ", versionHost)
	}
	if oVersion != "" {
		fmt.Fprintf(o.Out, "openshift %s\n", oVersion)
	}
	if kVersion != "" {
		fmt.Fprintf(o.Out, "kubernetes %s\n", kVersion)
	}

	return nil
}
