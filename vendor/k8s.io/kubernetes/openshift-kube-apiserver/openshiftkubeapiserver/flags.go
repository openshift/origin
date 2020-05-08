package openshiftkubeapiserver

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	configv1 "github.com/openshift/api/config/v1"
	kubecontrolplanev1 "github.com/openshift/api/kubecontrolplane/v1"
	"github.com/openshift/apiserver-library-go/pkg/configflags"
	"github.com/openshift/library-go/pkg/config/helpers"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/sets"
	apiserverv1alpha1 "k8s.io/apiserver/pkg/apis/apiserver/v1alpha1"
)

func ConfigToFlags(kubeAPIServerConfig *kubecontrolplanev1.KubeAPIServerConfig) ([]string, error) {
	args := unmaskArgs(kubeAPIServerConfig.APIServerArguments)

	// TODO this list (and the content below) will be used to drive a config struct and a reflective test matching config to flags
	// these flags are overridden by a patch
	// admission-control
	// authentication-token-webhook-cache-ttl
	// authentication-token-webhook-config-file
	// authorization-mode
	// authorization-policy-file
	// authorization-webhook-cache-authorized-ttl
	// authorization-webhook-cache-unauthorized-ttl
	// authorization-webhook-config-file
	// basic-auth-file
	// enable-aggregator-routing
	// enable-bootstrap-token-auth
	// oidc-client-id
	// oidc-groups-claim
	// oidc-groups-prefix
	// oidc-issuer-url
	// oidc-required-claim
	// oidc-signing-algs
	// oidc-username-claim
	// oidc-username-prefix
	// token-auth-file

	// alsologtostderr - don't know whether to change it
	// apiserver-count - ignored, hopefully we don't have to fix via patch
	// cert-dir - ignored because we set certs

	// these flags were never supported via config
	// cloud-config
	// cloud-provider
	// cloud-provider-gce-lb-src-cidrs
	// contention-profiling
	// default-not-ready-toleration-seconds
	// default-unreachable-toleration-seconds
	// default-watch-cache-size
	// delete-collection-workers
	// deserialization-cache-size
	// enable-garbage-collector
	// etcd-compaction-interval
	// etcd-count-metric-poll-period
	// etcd-servers-overrides
	// experimental-encryption-provider-config
	// feature-gates
	// http2-max-streams-per-connection
	// insecure-bind-address
	// kubelet-timeout
	// log-backtrace-at
	// log-dir
	// log-flush-frequency
	// logtostderr
	// master-service-namespace
	// max-connection-bytes-per-sec
	// profiling
	// request-timeout
	// runtime-config
	// service-account-api-audiences
	// service-account-issuer
	// service-account-key-file
	// service-account-max-token-expiration
	// stderrthreshold
	// storage-versions
	// target-ram-mb
	// v
	// version
	// vmodule
	// watch-cache
	// watch-cache-sizes

	// TODO, we need to set these in order to enable the right admission plugins in each of the servers
	// TODO this is needed for a viable cluster up
	admissionFlags, err := admissionFlags(kubeAPIServerConfig.AdmissionConfig)
	if err != nil {
		return nil, err
	}
	for flag, value := range admissionFlags {
		configflags.SetIfUnset(args, flag, value...)
	}
	if err := writePolicyFile(kubeAPIServerConfig.AuditConfig); err != nil {
		return nil, err
	}

	configflags.SetIfUnset(args, "cors-allowed-origins", kubeAPIServerConfig.CORSAllowedOrigins...)
	configflags.SetIfUnset(args, "etcd-servers", kubeAPIServerConfig.StorageConfig.URLs...)
	configflags.SetIfUnset(args, "service-cluster-ip-range", kubeAPIServerConfig.ServicesSubnet)
	configflags.SetIfUnset(args, "tls-cipher-suites", kubeAPIServerConfig.ServingInfo.CipherSuites...)
	configflags.SetIfUnset(args, "tls-min-version", kubeAPIServerConfig.ServingInfo.MinTLSVersion)
	configflags.SetIfUnset(args, "tls-sni-cert-key", sniCertKeys(kubeAPIServerConfig.ServingInfo.NamedCertificates)...)

	return configflags.ToFlagSlice(args), nil
}

const defaultAuditPolicyFilePath = "openshift.local.audit/policy.yaml"

func writePolicyFile(c configv1.AuditConfig) error {
	if len(c.PolicyConfiguration.Raw) > 0 && string(c.PolicyConfiguration.Raw) != "null" {
		if err := os.MkdirAll(filepath.Dir(defaultAuditPolicyFilePath), 0755); err != nil {
			return err
		}
		if err := ioutil.WriteFile(defaultAuditPolicyFilePath, c.PolicyConfiguration.Raw, 0644); err != nil {
			return err
		}
	}
	return nil
}

func admissionFlags(admissionConfig configv1.AdmissionConfig) (map[string][]string, error) {
	args := map[string][]string{}

	upstreamAdmissionConfig, err := ConvertOpenshiftAdmissionConfigToKubeAdmissionConfig(admissionConfig.PluginConfig)
	if err != nil {
		return nil, err
	}
	configBytes, err := helpers.WriteYAML(upstreamAdmissionConfig, apiserverv1alpha1.AddToScheme)
	if err != nil {
		return nil, err
	}

	tempFile, err := ioutil.TempFile("", "kubeapiserver-admission-config.yaml")
	if err != nil {
		return nil, err
	}
	if _, err := tempFile.Write(configBytes); err != nil {
		return nil, err
	}
	tempFile.Close()

	configflags.SetIfUnset(args, "admission-control-config-file", tempFile.Name())
	configflags.SetIfUnset(args, "disable-admission-plugins", admissionConfig.DisabledAdmissionPlugins...)
	configflags.SetIfUnset(args, "enable-admission-plugins", admissionConfig.EnabledAdmissionPlugins...)

	return args, nil
}

func sniCertKeys(namedCertificates []configv1.NamedCertificate) []string {
	args := []string{}
	for _, nc := range namedCertificates {
		names := ""
		if len(nc.Names) > 0 {
			names = ":" + strings.Join(nc.Names, ",")
		}
		args = append(args, fmt.Sprintf("%s,%s%s", nc.CertFile, nc.KeyFile, names))
	}
	return args
}

func unmaskArgs(args map[string]kubecontrolplanev1.Arguments) map[string][]string {
	ret := map[string][]string{}
	for key, slice := range args {
		for _, val := range slice {
			ret[key] = append(ret[key], val)
		}
	}
	return ret
}

func ConvertOpenshiftAdmissionConfigToKubeAdmissionConfig(in map[string]configv1.AdmissionPluginConfig) (*apiserverv1alpha1.AdmissionConfiguration, error) {
	ret := &apiserverv1alpha1.AdmissionConfiguration{}

	for _, pluginName := range sets.StringKeySet(in).List() {
		kubeConfig := apiserverv1alpha1.AdmissionPluginConfiguration{
			Name: pluginName,
			Path: in[pluginName].Location,
			Configuration: &runtime.Unknown{
				Raw: in[pluginName].Configuration.Raw,
			},
		}

		ret.Plugins = append(ret.Plugins, kubeConfig)
	}

	return ret, nil
}
