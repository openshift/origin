package originpolymorphichelpers

import (
	"errors"
	"fmt"
	"sort"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/rest"
	"k8s.io/kubernetes/pkg/kubectl/genericclioptions"
	"k8s.io/kubernetes/pkg/kubectl/polymorphichelpers"

	appsv1 "github.com/openshift/api/apps/v1"
	buildv1 "github.com/openshift/api/build/v1"
	appsclient "github.com/openshift/client-go/apps/clientset/versioned"
	appsv1client "github.com/openshift/client-go/apps/clientset/versioned/typed/apps/v1"
	buildv1client "github.com/openshift/client-go/build/clientset/versioned/typed/build/v1"
	appsapi "github.com/openshift/origin/pkg/apps/apis/apps"
	appsmanualclient "github.com/openshift/origin/pkg/apps/client/internalversion"
	appsmanualclientv1 "github.com/openshift/origin/pkg/apps/client/v1"
	buildapi "github.com/openshift/origin/pkg/build/apis/build"
	buildmanualclientv1 "github.com/openshift/origin/pkg/build/client/v1"
	buildutil "github.com/openshift/origin/pkg/build/util"
	ocbuildapihelpers "github.com/openshift/origin/pkg/oc/lib/buildapihelpers"
)

func NewLogsForObjectFn(delegate polymorphichelpers.LogsForObjectFunc) polymorphichelpers.LogsForObjectFunc {
	return func(restClientGetter genericclioptions.RESTClientGetter, object, options runtime.Object, timeout time.Duration, allContainers bool) ([]*rest.Request, error) {
		clientConfig, err := restClientGetter.ToRESTConfig()
		if err != nil {
			return nil, err
		}

		switch t := object.(type) {
		case *appsapi.DeploymentConfig:
			dopts, ok := options.(*appsapi.DeploymentLogOptions)
			if !ok {
				return nil, errors.New("provided options object is not a DeploymentLogOptions")
			}
			appsClient, err := appsclient.NewForConfig(clientConfig)
			if err != nil {
				return nil, err
			}
			// TODO: support allContainers flag
			return []*rest.Request{appsmanualclient.NewRolloutLogClient(appsClient.AppsV1().RESTClient(), t.Namespace).Logs(t.Name, *dopts)}, nil
		case *appsv1.DeploymentConfig:
			dopts, ok := options.(*appsv1.DeploymentLogOptions)
			if !ok {
				return nil, errors.New("provided options object is not a DeploymentLogOptions")
			}
			appsClient, err := appsv1client.NewForConfig(clientConfig)
			if err != nil {
				return nil, err
			}
			// TODO: support allContainers flag
			return []*rest.Request{appsmanualclientv1.NewRolloutLogClient(appsClient.RESTClient(), t.Namespace).Logs(t.Name, *dopts)}, nil
		case *buildv1.Build:
			bopts, ok := options.(*buildv1.BuildLogOptions)
			if !ok {
				return nil, errors.New("provided options object is not a v1.BuildLogOptions")
			}
			if bopts.Version != nil {
				return nil, errors.New("cannot specify a version and a build")
			}
			buildClient, err := buildv1client.NewForConfig(clientConfig)
			if err != nil {
				return nil, err
			}
			// TODO: support allContainers flag
			return []*rest.Request{buildmanualclientv1.NewBuildLogClient(buildClient.RESTClient(), t.Namespace).Logs(t.Name, *bopts)}, nil
		case *buildv1.BuildConfig:
			bopts, ok := options.(*buildv1.BuildLogOptions)
			if !ok {
				return nil, errors.New("provided options object is not a v1.BuildLogOptions")
			}
			buildClient, err := buildv1client.NewForConfig(clientConfig)
			if err != nil {
				return nil, err
			}
			logClient := buildmanualclientv1.NewBuildLogClient(buildClient.RESTClient(), t.Namespace)
			builds, err := buildClient.Builds(t.Namespace).List(metav1.ListOptions{})
			if err != nil {
				return nil, err
			}
			filteredInternalBuildItems := ocbuildapihelpers.FilterBuilds(builds.Items, ocbuildapihelpers.ByBuildConfigPredicate(t.Name))
			if len(filteredInternalBuildItems) == 0 {
				return nil, fmt.Errorf("no builds found for %q", t.Name)
			}
			if bopts.Version != nil {
				// If a version has been specified, try to get the logs from that build.
				desired := buildutil.BuildNameForConfigVersion(t.Name, int(*bopts.Version))
				// TODO: support allContainers flag
				return []*rest.Request{logClient.Logs(desired, *bopts)}, nil
			}
			sort.Sort(sort.Reverse(ocbuildapihelpers.BuildSliceByCreationTimestamp(filteredInternalBuildItems)))
			// TODO: support allContainers flag
			return []*rest.Request{logClient.Logs(filteredInternalBuildItems[0].Name, *bopts)}, nil
		case *buildapi.Build:
			bopts, ok := options.(*buildv1.BuildLogOptions)
			if !ok {
				return nil, errors.New("provided options object is not a BuildLogOptions")
			}
			if bopts.Version != nil {
				return nil, errors.New("cannot specify a version and a build")
			}
			buildClient, err := buildv1client.NewForConfig(clientConfig)
			if err != nil {
				return nil, err
			}
			// TODO: support allContainers flag
			return []*rest.Request{buildmanualclientv1.NewBuildLogClient(buildClient.RESTClient(), t.Namespace).Logs(t.Name, *bopts)}, nil
		case *buildapi.BuildConfig:
			bopts, ok := options.(*buildv1.BuildLogOptions)
			if !ok {
				return nil, errors.New("provided options object is not a BuildLogOptions")
			}
			buildClient, err := buildv1client.NewForConfig(clientConfig)
			if err != nil {
				return nil, err
			}
			logClient := buildmanualclientv1.NewBuildLogClient(buildClient.RESTClient(), t.Namespace)
			builds, err := buildClient.Builds(t.Namespace).List(metav1.ListOptions{})
			if err != nil {
				return nil, err
			}
			filteredInternalBuildItems := ocbuildapihelpers.FilterBuilds(builds.Items, ocbuildapihelpers.ByBuildConfigPredicate(t.Name))
			if len(filteredInternalBuildItems) == 0 {
				return nil, fmt.Errorf("no builds found for %q", t.Name)
			}
			if bopts.Version != nil {
				// If a version has been specified, try to get the logs from that build.
				desired := buildutil.BuildNameForConfigVersion(t.Name, int(*bopts.Version))
				// TODO: support allContainers flag
				return []*rest.Request{logClient.Logs(desired, *bopts)}, nil
			}
			sort.Sort(sort.Reverse(ocbuildapihelpers.BuildSliceByCreationTimestamp(filteredInternalBuildItems)))
			// TODO: support allContainers flag
			return []*rest.Request{logClient.Logs(filteredInternalBuildItems[0].Name, *bopts)}, nil
		default:
			return delegate(restClientGetter, object, options, timeout, allContainers)
		}
	}
}
