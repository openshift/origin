package create

import (
	"context"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/ghodss/yaml"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/restmapper"

	"github.com/openshift/library-go/pkg/assets"
)

// CreateOptions allow to specify additional create options.
type CreateOptions struct {
	// Filters allows to filter which files we will read from disk.
	// Multiple filters can be specified, in that case only files matching all filters will be returned.
	Filters []assets.FileInfoPredicate

	// Verbose if true will print out extra messages for debugging
	Verbose bool

	// StdErr allows to override the standard error output for printing verbose messages.
	// If not set, os.StdErr is used.
	StdErr io.Writer
}

// EnsureManifestsCreated ensures that all resource manifests from the specified directory are created.
// This function will try to create remaining resources in the manifest list after error is occurred.
// This function will keep retrying creation until no errors are reported or the timeout is hit.
// Pass the context to indicate how much time you are willing to wait until all resources are created.
func EnsureManifestsCreated(ctx context.Context, manifestDir string, restConfig *rest.Config, options CreateOptions) error {
	client, dc, err := newClientsFn(restConfig)
	if err != nil {
		return err
	}

	manifests, err := load(manifestDir, options)
	if err != nil {
		return err
	}

	if options.Verbose && options.StdErr == nil {
		options.StdErr = os.Stderr
	}

	// Default QPS in client (when not specified) is 5 requests/per second
	// This specifies the interval between "create-all-resources", no need to make this configurable.
	interval := 200 * time.Millisecond

	// Retry creation until no errors are returned or the timeout is hit.
	var (
		lastCreateError      error
		retryCount           int
		mapper               meta.RESTMapper
		needDiscoveryRefresh bool = true
	)
	err = wait.PollImmediateUntil(interval, func() (bool, error) {
		retryCount++
		// If we get rest mapper error, we need to pull updated discovery info from API server
		if needDiscoveryRefresh {
			mapper, err = fetchLatestDiscoveryInfoFn(dc)
			if err != nil {
				if options.Verbose {
					fmt.Fprintf(options.StdErr, "[#%d] failed to fetch discovery: %s\n", retryCount, err)
				}
				return false, nil
			}
		}
		err, needDiscoveryRefresh = create(ctx, manifests, client, mapper, options)
		if err == nil {
			lastCreateError = nil
			return true, nil
		}
		if ctx.Err() == nil || lastCreateError == nil {
			lastCreateError = err
		}
		if options.Verbose {
			fmt.Fprintf(options.StdErr, "[#%d] %s\n", retryCount, err)
		}
		return false, nil
	}, ctx.Done())

	// Return the last observed set of errors from the create process instead of timeout error.
	if lastCreateError != nil {
		return lastCreateError
	}

	return err
}

// allow to override in unit test
var newClientsFn = newClients

func newClients(config *rest.Config) (dynamic.Interface, *discovery.DiscoveryClient, error) {
	client, err := dynamic.NewForConfig(config)
	if err != nil {
		return nil, nil, err
	}

	// TODO: We can use cacheddiscovery.NewMemCacheClient(dc) and then call .Invalidate() instead of fetchLatestDiscoveryInfo.
	// It will require more work in unit test though.
	dc, err := discovery.NewDiscoveryClientForConfig(config)
	if err != nil {
		return nil, nil, err
	}

	return client, dc, nil
}

// allow to override in unit test
var fetchLatestDiscoveryInfoFn = fetchLatestDiscoveryInfo

func fetchLatestDiscoveryInfo(dc *discovery.DiscoveryClient) (meta.RESTMapper, error) {
	gr, err := restmapper.GetAPIGroupResources(dc)
	if err != nil {
		return nil, err
	}
	return restmapper.NewDiscoveryRESTMapper(gr), nil
}

// create will attempt to create all manifests provided using dynamic client.
// It will mutate the manifests argument in case the create succeeded for given manifest. When all manifests are successfully created the resulting
// manifests argument should be empty.
func create(ctx context.Context, manifests map[string]*unstructured.Unstructured, client dynamic.Interface, mapper meta.RESTMapper, options CreateOptions) (error, bool) {
	sortedManifestPaths := []string{}
	for key := range manifests {
		sortedManifestPaths = append(sortedManifestPaths, key)
	}
	sort.Strings(sortedManifestPaths)

	// Record all errors for the given manifest path (so when we report errors, users can see what manifest failed).
	errs := map[string]error{}

	// In case we fail to find a rest-mapping for the resource, force to fetch the updated discovery on next run.
	reloadDiscovery := false

	for _, path := range sortedManifestPaths {
		select {
		case <-ctx.Done():
			return ctx.Err(), false
		default:
		}

		gvk := manifests[path].GetObjectKind().GroupVersionKind()
		mappings, err := mapper.RESTMapping(gvk.GroupKind(), gvk.Version)
		if err != nil {
			errs[path] = fmt.Errorf("unable to get REST mapping: %v", err)
			reloadDiscovery = true
			continue
		}

		if mappings.Scope.Name() == meta.RESTScopeNameRoot {
			_, err = client.Resource(mappings.Resource).Create(manifests[path], metav1.CreateOptions{})
		} else {
			_, err = client.Resource(mappings.Resource).Namespace(manifests[path].GetNamespace()).Create(manifests[path], metav1.CreateOptions{})
		}

		// Resource already exists means we already succeeded
		// This should never happen as we remove already created items from the manifest list, unless the resource existed beforehand.
		if kerrors.IsAlreadyExists(err) {
			if options.Verbose {
				fmt.Fprintf(options.StdErr, "Skipped %s as it already exists\n", mappings.Resource.String())
			}
			delete(manifests, path)
			continue
		}

		if err != nil {
			if options.Verbose {
				fmt.Fprintf(options.StdErr, "Failed to create %s: %v\n", mappings.Resource.String(), err)
			}
			errs[path] = fmt.Errorf("failed to create: %v", err)
			continue
		}

		if options.Verbose {
			fmt.Fprintf(options.StdErr, "Created %s\n", mappings.Resource.String())
		}

		// Creation succeeded lets remove the manifest from the list to avoid creating it second time
		delete(manifests, path)
	}

	return formatErrors("failed to create some manifests", errs), reloadDiscovery
}

func formatErrors(prefix string, errors map[string]error) error {
	if len(errors) == 0 {
		return nil
	}
	aggregatedErrMessages := []string{}
	keys := []string{}
	for key := range errors {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	for _, k := range keys {
		aggregatedErrMessages = append(aggregatedErrMessages, fmt.Sprintf("%q: %v", k, errors[k]))
	}
	return fmt.Errorf("%s:\n%s", prefix, strings.Join(aggregatedErrMessages, "\n"))
}

func load(assetsDir string, options CreateOptions) (map[string]*unstructured.Unstructured, error) {
	manifests := map[string]*unstructured.Unstructured{}
	manifestsBytesMap, err := assets.LoadFilesRecursively(assetsDir, options.Filters...)
	if err != nil {
		return nil, err
	}

	errs := map[string]error{}
	for manifestPath, manifestBytes := range manifestsBytesMap {
		manifestJSON, err := yaml.YAMLToJSON(manifestBytes)
		if err != nil {
			errs[manifestPath] = fmt.Errorf("unable to convert asset %q from YAML to JSON: %v", manifestPath, err)
			continue
		}
		manifestObj, err := runtime.Decode(unstructured.UnstructuredJSONScheme, manifestJSON)
		if err != nil {
			errs[manifestPath] = fmt.Errorf("unable to decode asset %q: %v", manifestPath, err)
			continue
		}
		manifestUnstructured, ok := manifestObj.(*unstructured.Unstructured)
		if !ok {
			errs[manifestPath] = fmt.Errorf("unable to convert asset %q to unstructed", manifestPath)
			continue
		}
		manifests[manifestPath] = manifestUnstructured
	}

	return manifests, formatErrors("failed to load some manifests", errs)
}
