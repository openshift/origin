package clientcmd

import (
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"time"

	"github.com/blang/semver"
	"github.com/emicklei/go-restful/swagger"
	"github.com/spf13/cobra"

	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/meta"
	"k8s.io/kubernetes/pkg/api/unversioned"
	"k8s.io/kubernetes/pkg/api/validation"
	"k8s.io/kubernetes/pkg/client/restclient"
	"k8s.io/kubernetes/pkg/client/typed/dynamic"
	"k8s.io/kubernetes/pkg/controller"
	"k8s.io/kubernetes/pkg/kubectl"
	kcmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"
	"k8s.io/kubernetes/pkg/kubectl/resource"
	"k8s.io/kubernetes/pkg/labels"
	"k8s.io/kubernetes/pkg/runtime"

	"github.com/openshift/origin/pkg/api/latest"
	authorizationapi "github.com/openshift/origin/pkg/authorization/api"
	authorizationreaper "github.com/openshift/origin/pkg/authorization/reaper"
	buildapi "github.com/openshift/origin/pkg/build/api"
	buildcmd "github.com/openshift/origin/pkg/build/cmd"
	buildutil "github.com/openshift/origin/pkg/build/util"
	"github.com/openshift/origin/pkg/client"
	"github.com/openshift/origin/pkg/cmd/cli/describe"
	deployapi "github.com/openshift/origin/pkg/deploy/api"
	deploycmd "github.com/openshift/origin/pkg/deploy/cmd"
	userapi "github.com/openshift/origin/pkg/user/api"
	authenticationreaper "github.com/openshift/origin/pkg/user/reaper"
)

type ring1Factory struct {
	clientAccessFactory      ClientAccessFactory
	kubeObjectMappingFactory kcmdutil.ObjectMappingFactory
}

func NewObjectMappingFactory(clientAccessFactory ClientAccessFactory) kcmdutil.ObjectMappingFactory {
	return &ring1Factory{
		clientAccessFactory:      clientAccessFactory,
		kubeObjectMappingFactory: kcmdutil.NewObjectMappingFactory(clientAccessFactory),
	}
}

func (f *ring1Factory) Object() (meta.RESTMapper, runtime.ObjectTyper) {
	return f.kubeObjectMappingFactory.Object()
}

func (f *ring1Factory) UnstructuredObject() (meta.RESTMapper, runtime.ObjectTyper, error) {
	return f.kubeObjectMappingFactory.UnstructuredObject()
}

func (f *ring1Factory) ClientForMapping(mapping *meta.RESTMapping) (resource.RESTClient, error) {
	if latest.OriginKind(mapping.GroupVersionKind) {
		cfg, err := f.clientAccessFactory.ClientConfig()
		if err != nil {
			return nil, err
		}
		if err := client.SetOpenShiftDefaults(cfg); err != nil {
			return nil, err
		}
		cfg.APIPath = "/apis"
		if mapping.GroupVersionKind.Group == kapi.GroupName {
			cfg.APIPath = "/oapi"
		}
		gv := mapping.GroupVersionKind.GroupVersion()
		cfg.GroupVersion = &gv
		return restclient.RESTClientFor(cfg)
	}
	return f.kubeObjectMappingFactory.ClientForMapping(mapping)
}

func (f *ring1Factory) UnstructuredClientForMapping(mapping *meta.RESTMapping) (resource.RESTClient, error) {
	if latest.OriginKind(mapping.GroupVersionKind) {
		cfg, err := f.clientAccessFactory.ClientConfig()
		if err != nil {
			return nil, err
		}
		if err := client.SetOpenShiftDefaults(cfg); err != nil {
			return nil, err
		}
		cfg.APIPath = "/apis"
		if mapping.GroupVersionKind.Group == kapi.GroupName {
			cfg.APIPath = "/oapi"
		}
		gv := mapping.GroupVersionKind.GroupVersion()
		cfg.ContentConfig = dynamic.ContentConfig()
		cfg.GroupVersion = &gv
		return restclient.RESTClientFor(cfg)
	}
	return f.kubeObjectMappingFactory.UnstructuredClientForMapping(mapping)
}

func (f *ring1Factory) Describer(mapping *meta.RESTMapping) (kubectl.Describer, error) {
	if latest.OriginKind(mapping.GroupVersionKind) {
		oClient, kClient, err := f.clientAccessFactory.Clients()
		if err != nil {
			return nil, fmt.Errorf("unable to create client %s: %v", mapping.GroupVersionKind.Kind, err)
		}

		mappingVersion := mapping.GroupVersionKind.GroupVersion()
		cfg, err := f.clientAccessFactory.ClientConfigForVersion(&mappingVersion)
		if err != nil {
			return nil, fmt.Errorf("unable to load a client %s: %v", mapping.GroupVersionKind.Kind, err)
		}

		describer, ok := describe.DescriberFor(mapping.GroupVersionKind.GroupKind(), oClient, kClient, cfg.Host)
		if !ok {
			return nil, fmt.Errorf("no description has been implemented for %q", mapping.GroupVersionKind.Kind)
		}
		return describer, nil
	}
	return f.kubeObjectMappingFactory.Describer(mapping)
}

func (f *ring1Factory) LogsForObject(object, options runtime.Object) (*restclient.Request, error) {
	switch t := object.(type) {
	case *deployapi.DeploymentConfig:
		dopts, ok := options.(*deployapi.DeploymentLogOptions)
		if !ok {
			return nil, errors.New("provided options object is not a DeploymentLogOptions")
		}
		oc, _, err := f.clientAccessFactory.Clients()
		if err != nil {
			return nil, err
		}
		return oc.DeploymentLogs(t.Namespace).Get(t.Name, *dopts), nil
	case *buildapi.Build:
		bopts, ok := options.(*buildapi.BuildLogOptions)
		if !ok {
			return nil, errors.New("provided options object is not a BuildLogOptions")
		}
		if bopts.Version != nil {
			return nil, errors.New("cannot specify a version and a build")
		}
		oc, _, err := f.clientAccessFactory.Clients()
		if err != nil {
			return nil, err
		}
		return oc.BuildLogs(t.Namespace).Get(t.Name, *bopts), nil
	case *buildapi.BuildConfig:
		bopts, ok := options.(*buildapi.BuildLogOptions)
		if !ok {
			return nil, errors.New("provided options object is not a BuildLogOptions")
		}
		oc, _, err := f.clientAccessFactory.Clients()
		if err != nil {
			return nil, err
		}
		builds, err := oc.Builds(t.Namespace).List(kapi.ListOptions{})
		if err != nil {
			return nil, err
		}
		builds.Items = buildapi.FilterBuilds(builds.Items, buildapi.ByBuildConfigPredicate(t.Name))
		if len(builds.Items) == 0 {
			return nil, fmt.Errorf("no builds found for %q", t.Name)
		}
		if bopts.Version != nil {
			// If a version has been specified, try to get the logs from that build.
			desired := buildutil.BuildNameForConfigVersion(t.Name, int(*bopts.Version))
			return oc.BuildLogs(t.Namespace).Get(desired, *bopts), nil
		}
		sort.Sort(sort.Reverse(buildapi.BuildSliceByCreationTimestamp(builds.Items)))
		return oc.BuildLogs(t.Namespace).Get(builds.Items[0].Name, *bopts), nil
	default:
		return f.kubeObjectMappingFactory.LogsForObject(object, options)
	}
}

func (f *ring1Factory) Scaler(mapping *meta.RESTMapping) (kubectl.Scaler, error) {
	if deployapi.IsKindOrLegacy("DeploymentConfig", mapping.GroupVersionKind.GroupKind()) {
		oc, kc, err := f.clientAccessFactory.Clients()
		if err != nil {
			return nil, err
		}
		return deploycmd.NewDeploymentConfigScaler(oc, kc), nil
	}
	return f.kubeObjectMappingFactory.Scaler(mapping)
}

func (f *ring1Factory) Reaper(mapping *meta.RESTMapping) (kubectl.Reaper, error) {
	gk := mapping.GroupVersionKind.GroupKind()
	switch {
	case deployapi.IsKindOrLegacy("DeploymentConfig", gk):
		oc, kc, err := f.clientAccessFactory.Clients()
		if err != nil {
			return nil, err
		}
		return deploycmd.NewDeploymentConfigReaper(oc, kc), nil
	case authorizationapi.IsKindOrLegacy("Role", gk):
		oc, _, err := f.clientAccessFactory.Clients()
		if err != nil {
			return nil, err
		}
		return authorizationreaper.NewRoleReaper(oc, oc), nil
	case authorizationapi.IsKindOrLegacy("ClusterRole", gk):
		oc, _, err := f.clientAccessFactory.Clients()
		if err != nil {
			return nil, err
		}
		return authorizationreaper.NewClusterRoleReaper(oc, oc, oc), nil
	case userapi.IsKindOrLegacy("User", gk):
		oc, kc, err := f.clientAccessFactory.Clients()
		if err != nil {
			return nil, err
		}
		return authenticationreaper.NewUserReaper(
			client.UsersInterface(oc),
			client.GroupsInterface(oc),
			client.ClusterRoleBindingsInterface(oc),
			client.RoleBindingsNamespacer(oc),
			client.OAuthClientAuthorizationsInterface(oc),
			kc.Core(),
		), nil
	case userapi.IsKindOrLegacy("Group", gk):
		oc, kc, err := f.clientAccessFactory.Clients()
		if err != nil {
			return nil, err
		}
		return authenticationreaper.NewGroupReaper(
			client.GroupsInterface(oc),
			client.ClusterRoleBindingsInterface(oc),
			client.RoleBindingsNamespacer(oc),
			kc.Core(),
		), nil
	case buildapi.IsKindOrLegacy("BuildConfig", gk):
		oc, _, err := f.clientAccessFactory.Clients()
		if err != nil {
			return nil, err
		}
		return buildcmd.NewBuildConfigReaper(oc), nil
	}
	return f.kubeObjectMappingFactory.Reaper(mapping)
}

func (f *ring1Factory) HistoryViewer(mapping *meta.RESTMapping) (kubectl.HistoryViewer, error) {
	if deployapi.IsKindOrLegacy("DeploymentConfig", mapping.GroupVersionKind.GroupKind()) {
		oc, kc, err := f.clientAccessFactory.Clients()
		if err != nil {
			return nil, err
		}
		return deploycmd.NewDeploymentConfigHistoryViewer(oc, kc), nil
	}
	return f.kubeObjectMappingFactory.HistoryViewer(mapping)
}

func (f *ring1Factory) Rollbacker(mapping *meta.RESTMapping) (kubectl.Rollbacker, error) {
	if deployapi.IsKindOrLegacy("DeploymentConfig", mapping.GroupVersionKind.GroupKind()) {
		oc, _, err := f.clientAccessFactory.Clients()
		if err != nil {
			return nil, err
		}
		return deploycmd.NewDeploymentConfigRollbacker(oc), nil
	}
	return f.kubeObjectMappingFactory.Rollbacker(mapping)
}

func (f *ring1Factory) StatusViewer(mapping *meta.RESTMapping) (kubectl.StatusViewer, error) {
	oc, _, err := f.clientAccessFactory.Clients()
	if err != nil {
		return nil, err
	}

	if deployapi.IsKindOrLegacy("DeploymentConfig", mapping.GroupVersionKind.GroupKind()) {
		return deploycmd.NewDeploymentConfigStatusViewer(oc), nil
	}
	return f.kubeObjectMappingFactory.StatusViewer(mapping)
}

func (f *ring1Factory) AttachablePodForObject(object runtime.Object) (*kapi.Pod, error) {
	switch t := object.(type) {
	case *deployapi.DeploymentConfig:
		_, kc, err := f.clientAccessFactory.Clients()
		if err != nil {
			return nil, err
		}
		selector := labels.SelectorFromSet(t.Spec.Selector)
		f := func(pods []*kapi.Pod) sort.Interface { return sort.Reverse(controller.ActivePods(pods)) }
		pod, _, err := kcmdutil.GetFirstPod(kc.Core(), t.Namespace, selector, 1*time.Minute, f)
		return pod, err
	default:
		return f.kubeObjectMappingFactory.AttachablePodForObject(object)
	}
}

// PrinterForMapping returns a printer suitable for displaying the provided resource type.
// Requires that printer flags have been added to cmd (see AddPrinterFlags).
func (f *ring1Factory) PrinterForMapping(cmd *cobra.Command, mapping *meta.RESTMapping, withNamespace bool) (kubectl.ResourcePrinter, error) {
	/*
		// TODO FIX ME. COPIED FROM KUBE AS PART OF THE COPY/PASTE FOR
		// PrinterForMapping
		if latest.OriginKind(mapping.GroupVersionKind) {
			printer, ok, err := kcmdutil.PrinterForCommand(cmd)
			if err != nil {
				return nil, err
			}
			if ok && mapping != nil {
				printer = kubectl.NewVersionedPrinter(printer, mapping.ObjectConvertor, mapping.GroupVersionKind.GroupVersion())
			} else {
				// Some callers do not have "label-columns" so we can't use the GetFlagStringSlice() helper
				columnLabel, err := cmd.Flags().GetStringSlice("label-columns")
				if err != nil {
					columnLabel = []string{}
				}
				printer, err = f.Printer(mapping, kubectl.PrintOptions{
					NoHeaders:          kcmdutil.GetFlagBool(cmd, "no-headers"),
					WithNamespace:      withNamespace,
					Wide:               kcmdutil.GetWideFlag(cmd),
					ShowAll:            kcmdutil.GetFlagBool(cmd, "show-all"),
					ShowLabels:         kcmdutil.GetFlagBool(cmd, "show-labels"),
					AbsoluteTimestamps: isWatch(cmd),
					ColumnLabels:       columnLabel,
				})
				if err != nil {
					return nil, err
				}
				printer = maybeWrapSortingPrinter(cmd, printer)
			}

			return printer, nil
		}
	*/

	return f.kubeObjectMappingFactory.PrinterForMapping(cmd, mapping, withNamespace)
}

func (f *ring1Factory) Validator(validate bool, cacheDir string) (validation.Schema, error) {
	return f.kubeObjectMappingFactory.Validator(validate, cacheDir)
}

func (f *ring1Factory) SwaggerSchema(gvk unversioned.GroupVersionKind) (*swagger.ApiDeclaration, error) {
	if !latest.OriginKind(gvk) {
		return f.kubeObjectMappingFactory.SwaggerSchema(gvk)
	}
	// TODO: we need to register the OpenShift API under the Kube group, and start returning the OpenShift
	// group from the scheme.
	oc, _, err := f.clientAccessFactory.Clients()
	if err != nil {
		return nil, err
	}
	return f.OriginSwaggerSchema(oc.RESTClient, gvk.GroupVersion())
}

// OriginSwaggerSchema returns a swagger API doc for an Origin schema under the /oapi prefix.
func (f *ring1Factory) OriginSwaggerSchema(client *restclient.RESTClient, version unversioned.GroupVersion) (*swagger.ApiDeclaration, error) {
	if version.Empty() {
		return nil, fmt.Errorf("groupVersion cannot be empty")
	}
	body, err := client.Get().AbsPath("/").Suffix("swaggerapi", "oapi", version.Version).Do().Raw()
	if err != nil {
		return nil, err
	}
	var schema swagger.ApiDeclaration
	err = json.Unmarshal(body, &schema)
	if err != nil {
		return nil, fmt.Errorf("got '%s': %v", string(body), err)
	}
	return &schema, nil
}

// useDiscoveryRESTMapper checks the server version to see if its recent enough to have
// enough discovery information avaiable to reliably build a RESTMapper.  If not, use the
// hardcoded mapper in this client (legacy behavior)
func useDiscoveryRESTMapper(serverVersion string) bool {
	serverSemVer, err := semver.Parse(serverVersion[1:])
	if err != nil {
		return false
	}
	if serverSemVer.LT(semver.MustParse("1.3.0")) {
		return false
	}
	return true
}
