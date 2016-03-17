package remove

import (
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/golang/glog"
	"github.com/spf13/cobra"

	"k8s.io/kubernetes/pkg/api"
	kerrors "k8s.io/kubernetes/pkg/api/errors"
	kubeclient "k8s.io/kubernetes/pkg/client/unversioned"
	kcmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"
	"k8s.io/kubernetes/pkg/util/sets"

	"github.com/openshift/origin/pkg/cmd/server/start"
	"github.com/openshift/origin/pkg/cmd/util/clientcmd"
	projectapi "github.com/openshift/origin/pkg/project/api"
)

// RemoveRecommendedCommandName is the recommended command name
const RemoveRecommendedCommandName = "remove"

const (
	removeLong = `
Removes OpenShift on a Kubernetes cluster.

It does the following:
 * delete the namespace that had OpenShift
 * remove all OpenShift finalizer tokens from any namespace in Kubernetes`
)

// NewCmdRemove removes OpenShift from a Kubernetes cluster.
func NewCmdRemove(name, fullName string, f *clientcmd.Factory, out io.Writer) *cobra.Command {
	options := NewDefaultOptions(out)
	cmd := &cobra.Command{
		Use:   name,
		Short: "Remove OpenShift from a Kubernetes cluster.",
		Long:  removeLong,
		Run: func(c *cobra.Command, args []string) {
			kcmdutil.CheckErr(options.Complete(f, c, args, out))
			kcmdutil.CheckErr(options.Validate(args))
			if err := options.Remove(); err != nil {
				if kerrors.IsInvalid(err) {
					if details := err.(*kerrors.StatusError).ErrStatus.Details; details != nil {
						fmt.Fprintf(c.Out(), "error: Invalid %s %s\n", details.Kind, details.Name)
						for _, cause := range details.Causes {
							fmt.Fprintf(c.Out(), "  %s: %s\n", cause.Field, cause.Message)
						}
						os.Exit(255)
					}
				}
				glog.Fatalf("Removal of OpenShift could not complete: %v", err)
			}
		},
	}
	flags := cmd.Flags()
	start.BindKubeConnectionArgs(options.KubeConnectionArgs, flags, "")
	return cmd
}

// Options describes how OpenShift is removed from Kubernetes.
type Options struct {
	// Location to output results from command
	Output io.Writer
	// How to connect to the kubernetes cluster
	KubeConnectionArgs *start.KubeConnectionArgs
	// Namespace to delete.
	Namespace string
}

// NewDefaultOptions creates an options object with default values.
func NewDefaultOptions(out io.Writer) *Options {
	return &Options{
		Output:             out,
		KubeConnectionArgs: start.NewDefaultKubeConnectionArgs(),
	}
}

// Validate validates install options.
func (o *Options) Validate(args []string) error {
	if len(args) != 0 {
		return errors.New("no arguments are supported for install")
	}
	if len(o.Namespace) == 0 {
		return fmt.Errorf("namespace must be known")
	}
	return nil
}

// Complete finishes configuration of install options.
func (o *Options) Complete(f *clientcmd.Factory, cmd *cobra.Command, args []string, out io.Writer) error {
	namespace, _, err := f.DefaultNamespace()
	if err != nil {
		return err
	}
	o.Namespace = namespace
	return nil
}

// buildKubernetesClient returns a Kubernetes client.
func (o *Options) buildKubernetesClient() (kubeclient.Interface, error) {
	config, err := o.KubeConnectionArgs.ClientConfig.ClientConfig()
	if err != nil {
		return nil, err
	}
	client, err := kubeclient.New(config)
	if err != nil {
		return nil, err
	}
	return client, nil
}

// Remove removes OpenShift from Kubernetes by deleting the current namespace.
// In addition, it removes the openshift finalizer from each namespace in cluster.
func (o *Options) Remove() error {
	kubeClient, err := o.buildKubernetesClient()
	if err != nil {
		return err
	}
	// clean-up
	nsToDelete := []string{o.Namespace, "openshift", "openshift-infra"}
	for _, ns := range nsToDelete {
		err = kubeClient.Namespaces().Delete(ns)
		if err != nil && !kerrors.IsNotFound(err) {
			return err
		}
	}
	// remove our finalizer tokens on anything
	nsList, err := kubeClient.Namespaces().List(api.ListOptions{})
	if err != nil {
		return err
	}
	for i := range nsList.Items {
		_, err = finalizeNamespace(kubeClient, &nsList.Items[i], projectapi.FinalizerOrigin)
		if err != nil {
			return err
		}
	}
	return nil
}

// finalizeNamespace removes the specified finalizerToken and finalizes the namespace
// TODO: make this public in namespace controller utils upstream...
func finalizeNamespace(kubeClient kubeclient.Interface, namespace *api.Namespace, finalizerToken api.FinalizerName) (*api.Namespace, error) {
	namespaceFinalize := api.Namespace{}
	namespaceFinalize.ObjectMeta = namespace.ObjectMeta
	namespaceFinalize.Spec = namespace.Spec
	finalizerSet := sets.NewString()
	for i := range namespace.Spec.Finalizers {
		if namespace.Spec.Finalizers[i] != finalizerToken {
			finalizerSet.Insert(string(namespace.Spec.Finalizers[i]))
		}
	}
	namespaceFinalize.Spec.Finalizers = make([]api.FinalizerName, 0, len(finalizerSet))
	for _, value := range finalizerSet.List() {
		namespaceFinalize.Spec.Finalizers = append(namespaceFinalize.Spec.Finalizers, api.FinalizerName(value))
	}
	namespace, err := kubeClient.Namespaces().Finalize(&namespaceFinalize)
	if err != nil {
		// it was removed already, so life is good
		if kerrors.IsNotFound(err) {
			return namespace, nil
		}
	}
	return namespace, err
}
