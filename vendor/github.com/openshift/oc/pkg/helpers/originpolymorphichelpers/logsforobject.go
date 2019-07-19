package originpolymorphichelpers

import (
	"errors"
	"fmt"
	"sort"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/rest"
	"k8s.io/kubernetes/pkg/kubectl/polymorphichelpers"
	"k8s.io/kubernetes/pkg/kubectl/scheme"

	appsv1 "github.com/openshift/api/apps/v1"
	buildv1 "github.com/openshift/api/build/v1"
	appsv1client "github.com/openshift/client-go/apps/clientset/versioned/typed/apps/v1"
	buildv1client "github.com/openshift/client-go/build/clientset/versioned/typed/build/v1"
	ocbuildapihelpers "github.com/openshift/oc/pkg/helpers/build"
	buildmanualclientv1 "github.com/openshift/oc/pkg/helpers/build/client/v1"
	"github.com/openshift/oc/pkg/helpers/originpolymorphichelpers/deploymentconfigs"
)

func NewLogsForObjectFn(delegate polymorphichelpers.LogsForObjectFunc) polymorphichelpers.LogsForObjectFunc {
	return func(restClientGetter genericclioptions.RESTClientGetter, object, options runtime.Object, timeout time.Duration, allContainers bool) ([]rest.ResponseWrapper, error) {
		clientConfig, err := restClientGetter.ToRESTConfig()
		if err != nil {
			return nil, err
		}

		switch t := object.(type) {
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
			return []rest.ResponseWrapper{deploymentconfigs.NewRolloutLogClient(appsClient.RESTClient(), t.Namespace).Logs(t.Name, *dopts)}, nil
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
			return []rest.ResponseWrapper{buildmanualclientv1.NewBuildLogClient(buildClient.RESTClient(), t.Namespace, scheme.Scheme).Logs(t.Name, *bopts)}, nil
		case *buildv1.BuildConfig:
			bopts, ok := options.(*buildv1.BuildLogOptions)
			if !ok {
				return nil, errors.New("provided options object is not a v1.BuildLogOptions")
			}
			buildClient, err := buildv1client.NewForConfig(clientConfig)
			if err != nil {
				return nil, err
			}
			logClient := buildmanualclientv1.NewBuildLogClient(buildClient.RESTClient(), t.Namespace, scheme.Scheme)
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
				desired := ocbuildapihelpers.BuildNameForConfigVersion(t.Name, int(*bopts.Version))
				// TODO: support allContainers flag
				return []rest.ResponseWrapper{logClient.Logs(desired, *bopts)}, nil
			}
			sort.Sort(sort.Reverse(ocbuildapihelpers.BuildSliceByCreationTimestamp(filteredInternalBuildItems)))
			// TODO: support allContainers flag
			return []rest.ResponseWrapper{logClient.Logs(filteredInternalBuildItems[0].Name, *bopts)}, nil

		default:
			return delegate(restClientGetter, object, options, timeout, allContainers)
		}
	}
}
