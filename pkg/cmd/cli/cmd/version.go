package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"

	etcdversion "github.com/coreos/etcd/version"

	kapierrors "k8s.io/kubernetes/pkg/api/errors"
	kclient "k8s.io/kubernetes/pkg/client/unversioned"
	kclientcmd "k8s.io/kubernetes/pkg/client/unversioned/clientcmd"
	kcmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"
	kubeversion "k8s.io/kubernetes/pkg/version"

	"github.com/openshift/origin/pkg/client"
	"github.com/openshift/origin/pkg/cmd/util/clientcmd"
	"github.com/openshift/origin/pkg/cmd/util/tokencmd"
	"github.com/openshift/origin/pkg/version"

	"github.com/spf13/cobra"
)

const (
	versionLong = `
Display client and server versions.`
)

type VersionOptions struct {
	BaseName string
	Out      io.Writer

	ClientConfig kclientcmd.ClientConfig
	Clients      func() (*client.Client, *kclient.Client, error)

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

			if err := options.Complete(f, out); err != nil {
				kcmdutil.CheckErr(kcmdutil.UsageError(cmd, err.Error()))
			}

			if err := options.RunVersion(); err != nil {
				kcmdutil.CheckErr(err)
			}
		},
	}

	return cmd
}

func (o *VersionOptions) Complete(f *clientcmd.Factory, out io.Writer) error {
	o.Out = out

	if f == nil {
		return nil
	}

	o.Clients = f.Clients
	o.ClientConfig = f.OpenShiftClientConfig
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

	// max amount of time we want to wait for server to respond
	timeout := 10 * time.Second

	done := make(chan error)
	oVersion := ""
	kVersion := ""
	versionHost := ""

	// start goroutine to fetch openshift / kubernetes server version
	go func() {
		defer close(done)

		// confirm config exists before makig request to server
		var err error
		clientConfig, err := o.ClientConfig.ClientConfig()
		if err != nil {
			done <- err
			return
		}
		versionHost = clientConfig.Host

		oClient, kClient, err := o.Clients()
		if err != nil {
			done <- err
			return
		}

		ocVersionBody, err := oClient.Get().AbsPath("/version/openshift").Do().Raw()
		if kapierrors.IsNotFound(err) || kapierrors.IsUnauthorized(err) || kapierrors.IsForbidden(err) {
			return
		}
		if err != nil {
			done <- err
			return
		}
		var ocServerInfo version.Info
		err = json.Unmarshal(ocVersionBody, &ocServerInfo)
		if err != nil && len(ocVersionBody) > 0 {
			done <- err
			return
		}
		oVersion = fmt.Sprintf("%v", ocServerInfo)

		kubeVersionBody, err := kClient.Get().AbsPath("/version").Do().Raw()
		if kapierrors.IsNotFound(err) || kapierrors.IsUnauthorized(err) || kapierrors.IsForbidden(err) {
			return
		}
		if err != nil {
			done <- err
			return
		}
		var kubeServerInfo kubeversion.Info
		err = json.Unmarshal(kubeVersionBody, &kubeServerInfo)
		if err != nil && len(kubeVersionBody) > 0 {
			done <- err
			return
		}
		kVersion = fmt.Sprintf("%v", kubeServerInfo)

	}()

	select {
	case err, closed := <-done:
		if strings.HasSuffix(fmt.Sprintf("%v", err), "connection refused") || clientcmd.IsConfigurationMissing(err) || kclientcmd.IsConfigurationInvalid(err) {
			return nil
		}
		if closed && err != nil {
			return err
		}
	case <-time.After(timeout):
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
