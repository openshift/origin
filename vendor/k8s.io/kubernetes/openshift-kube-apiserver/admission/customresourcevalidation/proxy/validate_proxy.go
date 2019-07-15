package proxy

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"

	"k8s.io/apimachinery/pkg/util/sets"

	"k8s.io/apimachinery/pkg/api/validation"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	k8serrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/apiserver/pkg/admission"

	configv1 "github.com/openshift/api/config/v1"
	"k8s.io/kubernetes/openshift-kube-apiserver/admission/customresourcevalidation"
)

const (
	PluginName = "config.openshift.io/ValidateProxy"
	maxRetries = 3
)


// Register registers a plugin
func Register(plugins *admission.Plugins) {
	plugins.Register(PluginName, func(config io.Reader) (admission.Interface, error) {
		return customresourcevalidation.NewValidator(
			map[schema.GroupResource]bool{
				configv1.Resource("proxies"): true,
			},
			map[schema.GroupVersionKind]customresourcevalidation.ObjectValidator{
				configv1.GroupVersion.WithKind("Proxy"): ProxyV1{},
			})
	})
}

func toProxyV1(uncastObj runtime.Object) (*configv1.Proxy, field.ErrorList) {
	if uncastObj == nil {
		return nil, nil
	}

	allErrs := field.ErrorList{}

	obj, ok := uncastObj.(*configv1.Proxy)
	if !ok {
		return nil, append(allErrs,
			field.NotSupported(field.NewPath("kind"), fmt.Sprintf("%T", uncastObj), []string{"Proxy"}),
			field.NotSupported(field.NewPath("apiVersion"), fmt.Sprintf("%T", uncastObj), []string{"config.openshift.io/v1"}))
	}

	return obj, nil
}

type proxyV1 struct {}

func validateProxySpec(spec configv1.ProxySpec) field.ErrorList {
	allErrs := field.ErrorList{}

	if len(spec.HTTPProxy) == 0 && len(spec.HTTPSProxy) == 0  {
		allErrs = append(allErrs, field.Invalid(field.NewPath("spec.HTTPProxy"), spec.HTTPProxy, "must be set"))
	}
	if len(spec.HTTPProxy) != 0 {
		if err := validateURI(spec.HTTPProxy); err != nil {
			allErrs = append(allErrs, field.Invalid(field.NewPath("HTTPProxy"), spec.HTTPProxy, err.Error()))
		}
	}
	if len(spec.HTTPSProxy) != 0 {
		if err := validateURI(spec.HTTPSProxy); err != nil {
			allErrs = append(allErrs, field.Invalid(field.NewPath("HTTPSProxy"), spec.HTTPSProxy, err.Error()))
		}
	}
	if len(spec.ReadinessEndpoints) != 0 {
		for _, endpoint := range spec.ReadinessEndpoints {
			if err := validateURI(endpoint); err != nil {
				allErrs = append(allErrs, field.Invalid(field.NewPath("ReadinessEndpoints"), spec.ReadinessEndpoints, err.Error()))
			} else {
				if err := validateReadinessEndpoint(endpoint); err != nil {
					allErrs = append(allErrs, field.Invalid(field.NewPath("spec.ReadinessEndpoints"), spec.ReadinessEndpoints, err.Error()))
				}
			}
		}
	}
	if len(spec.NoProxy) != 0 {
		for _, v := range strings.Split(spec.NoProxy, ",") {
			v = strings.TrimSpace(v)
			errDomain := validateDomainName(v, false)
			_, _, errCIDR := net.ParseCIDR(v)
			if errDomain != nil && errCIDR != nil {
				allErrs = append(allErrs, field.Invalid(field.NewPath("NoProxy"), v, "must be a CIDR or domain, without wildcard characters and without leading or trailing dots ('.')"))
			}
			noProxy, err := createNoProxy(installConfig, network)
		}
	}

	return allErrs
}

func (proxyV1) ValidateCreate(uncastObj runtime.Object) field.ErrorList {
	obj, allErrs := toProxyV1(uncastObj)
	if len(allErrs) > 0 {
		return allErrs
	}

	allErrs = append(allErrs, validateProxySpec(obj.Spec)...)

	return allErrs
}

func (proxyV1) ValidateUpdate(uncastObj runtime.Object, uncastOldObj runtime.Object) field.ErrorList {
	obj, allErrs := toProxyV1(uncastObj)
	if len(allErrs) > 0 {
		return allErrs
	}
	oldObj, allErrs := toProxyV1(uncastOldObj)
	if len(allErrs) > 0 {
		return allErrs
	}

	allErrs = append(allErrs, validation.ValidateObjectMetaUpdate(&obj.ObjectMeta, &oldObj.ObjectMeta, field.NewPath("metadata"))...)
	allErrs = append(allErrs, validateProxySpec(obj.Spec)...)

	return allErrs
}

// validateReadinessEndpoint validates a proxy readinessendpoint.
func validateReadinessEndpoint(endpoint string) error {
	err := validateReadinessEndpointWithRetries(endpoint, maxRetries)
		if err != nil {
			return err
		}

	return nil
}

// validateReadinessEndpointWithRetries tries to validate the proxy readinessendpoint in a finite loop,
// it returns the last result if it never succeeds.
func validateReadinessEndpointWithRetries(endpoint string, retries int) field.ErrorList {
	var err error
	for i := 0; i < retries; i++ {
		result, output, err = runHTTPReadinessProbe(endpoint)
		if err == nil {
			return nil
		}
	}
	return err
}

// runHTTPReadinessProbe issues an http GET request to endpoint and returns an error
// if a 2XX or 3XX http status code is not returned.
func runHTTPReadinessProbe(endpoint string) error {
	url, err := url.Parse(endpoint)
	if err != nil {
		return err
	}
	timeout := time.Duration(5) * time.Second
	resp, err := http.Get(url.String())
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= http.StatusOK && resp.StatusCode < http.StatusBadRequest {
		return nil
	}

	return fmt.Sprintf("HTTP probe failed with statuscode: %d", resp.StatusCode)
}

// createNoProxy combines user-provided & platform-specific values to create a comma-separated
// list of unique NO_PROXY values. Platform values are: serviceCIDR, podCIDR, localhost,
// 127.0.0.1, api.clusterdomain, api-int.clusterdomain, etcd-idx.clusterdomain
// If platform is not vSphere or None add 169.254.169.254 to the list of NO_PROXY
// address. We should not proxy the instance metadata services:
// https://docs.openstack.org/nova/latest/user/metadata.html
// https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/ec2-instance-metadata.html
// https://docs.microsoft.com/en-us/azure/virtual-machines/windows/instance-metadata-service
// https://cloud.google.com/compute/docs/storing-retrieving-metadata
func createNoProxy(spec *configv1.ProxySpec) (string, error) {
	// TODO: Create client with configv1 scheme registered.
	// Retrieve the cluster infrastructure config.
	infraConfig := &configv1.Infrastructure{}
	err = kubeClient.Get(context.TODO(), types.NamespacedName{Name: "cluster"}, infraConfig)
	if err != nil {
		return "", err
	}
	apiServerURL, err := url.Parse(getAPIServerURL(infraConfig.Status.APIServerURL))
	if err != nil {
		return "", errors.New("failed parsing API server URL")
	}
	internalAPIServer, err := url.Parse(getInternalAPIServerURL(infraConfig.Status.APIServerInternalURL))
	if err != nil {
		return "", err
	}
	// TODO: Create client with configv1 scheme registered.
	// Retrieve the cluster network config.
	netConfig := &configv1.Network{}
	err = kubeClient.Get(context.TODO(), types.NamespacedName{Name: "cluster"}, netConfig)
	if err != nil {
		return "", errors.New("failed to get %q Network resource", netConfig.Name)
	}
	// TODO: Add nodes due to services being exposed using nodePorts.
	//  If so, which node address type?
	set := sets.NewString(
		"127.0.0.1",
		"localhost",
		netConfig.Status.ServiceNetwork[0],
		apiServerURL.Hostname(),
		internalAPIServer.Hostname(),
	)
	platform := infraConfig.Status.PlatformStatus.Type

	if platform != configv1.AWSPlatformType && platform != configv1.NonePlatformType {
		set.Insert("169.254.169.254")
	}

	//TODO: Get etcd server count from api (machineconfigpool/master?)
	for i := 0; i < 3; i++ {
		etcdHost := fmt.Sprintf("etcd-%d.%s", i, infraConfig.Status.EtcdDiscoveryDomain)
		set.Insert(etcdHost)
	}

	for _, clusterNetwork := range netConfig.Status.ClusterNetwork {
		set.Insert(clusterNetwork.CIDR)
	}

	for _, userValue := range strings.Split(spec.NoProxy, ",") {
		set.Insert(userValue)
	}

	return strings.Join(set.List(), ","), nil
}

// validateDomainName checks if the given string is a valid domain name and returns an error if not.
func validateDomainName(v string, acceptTrailingDot bool) error {
	if acceptTrailingDot {
		v = strings.TrimSuffix(v, ".")
	}
	return validateSubdomain(v)
}

// validateSubdomain checks if the given string is a valid subdomain name and returns an error if not.
func validateSubdomain(v string) error {
	validationMessages := validation.IsDNS1123Subdomain(v)
	if len(validationMessages) == 0 {
		return nil
	}

	errs := make([]error, len(validationMessages))
	for i, m := range validationMessages {
		errs[i] = errors.New(m)
	}
	return k8serrors.NewAggregate(errs)
}

// validateURI validates if the URI is a valid absolute URI.
func validateURI(uri string) error {
	parsed, err := url.Parse(uri)
	if err != nil {
		return err
	}
	if !parsed.IsAbs() {
		return fmt.Errorf("invalid URI %q (no scheme)", uri)
	}
	return nil
}

func getAPIServerURL(ic *types.InstallConfig) string {
	return fmt.Sprintf("https://api.%s:6443", ic.ClusterDomain())
}

func getInternalAPIServerURL(ic *types.InstallConfig) string {
	return fmt.Sprintf("https://api-int.%s:6443", ic.ClusterDomain())
}

func getEtcdDiscoveryDomain(ic *types.InstallConfig) string {
	return ic.ClusterDomain()
}

// ClusterDomain returns the DNS domain that all records for a cluster must belong to.
func (c *InstallConfig) ClusterDomain() string {
	return fmt.Sprintf("%s.%s", c.ObjectMeta.Name, c.BaseDomain)
}
