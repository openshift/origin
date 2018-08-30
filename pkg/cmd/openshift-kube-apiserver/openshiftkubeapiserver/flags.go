package openshiftkubeapiserver

import (
	"fmt"
	"net"
	"sort"

	"io/ioutil"

	"bytes"

	configapi "github.com/openshift/origin/pkg/cmd/server/apis/config"
	configapilatest "github.com/openshift/origin/pkg/cmd/server/apis/config/latest"
	originadmission "github.com/openshift/origin/pkg/cmd/server/origin/admission"
	"k8s.io/apimachinery/pkg/runtime"
)

func ConfigToFlags(kubeAPIServerConfig *configapi.KubeAPIServerConfig) ([]string, error) {
	args := map[string][]string{}
	for key, slice := range kubeAPIServerConfig.APIServerArguments {
		for _, val := range slice {
			args[key] = append(args[key], val)
		}
	}

	host, portString, err := net.SplitHostPort(kubeAPIServerConfig.ServingInfo.BindAddress)
	if err != nil {
		return nil, err
	}

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
	// service-account-lookup
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
	// service-account-signing-key-file
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
	admissionFlags, err := admissionFlags(kubeAPIServerConfig.AdmissionPluginConfig)
	if err != nil {
		return nil, err
	}
	for flag, value := range admissionFlags {
		setIfUnset(args, flag, value...)
	}
	setIfUnset(args, "allow-privileged", "true")
	setIfUnset(args, "anonymous-auth", "false")
	setIfUnset(args, "authorization-mode", "RBAC", "Node") // overridden later, but this runs the poststarthook for bootstrapping RBAC
	for flag, value := range auditFlags(kubeAPIServerConfig) {
		setIfUnset(args, flag, value...)
	}
	setIfUnset(args, "bind-address", host)
	setIfUnset(args, "client-ca-file", kubeAPIServerConfig.ServingInfo.ClientCA)
	setIfUnset(args, "cors-allowed-origins", kubeAPIServerConfig.CORSAllowedOrigins...)
	setIfUnset(args, "enable-logs-handler", "false")
	setIfUnset(args, "enable-swagger-ui", "true")
	setIfUnset(args, "endpoint-reconciler-type", "lease")
	setIfUnset(args, "etcd-cafile", kubeAPIServerConfig.EtcdClientInfo.CA)
	setIfUnset(args, "etcd-certfile", kubeAPIServerConfig.EtcdClientInfo.ClientCert.CertFile)
	setIfUnset(args, "etcd-keyfile", kubeAPIServerConfig.EtcdClientInfo.ClientCert.KeyFile)
	setIfUnset(args, "etcd-prefix", kubeAPIServerConfig.StoragePrefix)
	setIfUnset(args, "etcd-servers", kubeAPIServerConfig.EtcdClientInfo.URLs...)
	setIfUnset(args, "insecure-port", "0")
	setIfUnset(args, "kubelet-certificate-authority", kubeAPIServerConfig.KubeletClientInfo.CA)
	setIfUnset(args, "kubelet-client-certificate", kubeAPIServerConfig.KubeletClientInfo.ClientCert.CertFile)
	setIfUnset(args, "kubelet-client-key", kubeAPIServerConfig.KubeletClientInfo.ClientCert.KeyFile)
	setIfUnset(args, "kubelet-https", "true")
	setIfUnset(args, "kubelet-preferred-address-types", "Hostname", "InternalIP", "ExternalIP")
	setIfUnset(args, "kubelet-read-only-port", "0")
	setIfUnset(args, "kubernetes-service-node-port", "0")
	setIfUnset(args, "max-mutating-requests-inflight", fmt.Sprintf("%d", kubeAPIServerConfig.ServingInfo.MaxRequestsInFlight/2))
	setIfUnset(args, "max-requests-inflight", fmt.Sprintf("%d", kubeAPIServerConfig.ServingInfo.MaxRequestsInFlight))
	setIfUnset(args, "min-request-timeout", fmt.Sprintf("%d", kubeAPIServerConfig.ServingInfo.RequestTimeoutSeconds))
	setIfUnset(args, "proxy-client-cert-file", kubeAPIServerConfig.AggregatorConfig.ProxyClientInfo.CertFile)
	setIfUnset(args, "proxy-client-key-file", kubeAPIServerConfig.AggregatorConfig.ProxyClientInfo.KeyFile)
	setIfUnset(args, "requestheader-allowed-names", kubeAPIServerConfig.AuthConfig.RequestHeader.ClientCommonNames...)
	setIfUnset(args, "requestheader-client-ca-file", kubeAPIServerConfig.AuthConfig.RequestHeader.ClientCA)
	setIfUnset(args, "requestheader-extra-headers-prefix", kubeAPIServerConfig.AuthConfig.RequestHeader.ExtraHeaderPrefixes...)
	setIfUnset(args, "requestheader-group-headers", kubeAPIServerConfig.AuthConfig.RequestHeader.GroupHeaders...)
	setIfUnset(args, "requestheader-username-headers", kubeAPIServerConfig.AuthConfig.RequestHeader.UsernameHeaders...)
	setIfUnset(args, "secure-port", portString)
	setIfUnset(args, "service-cluster-ip-range", kubeAPIServerConfig.ServicesSubnet)
	setIfUnset(args, "service-node-port-range", kubeAPIServerConfig.ServicesNodePortRange)
	setIfUnset(args, "storage-backend", "etcd3")
	setIfUnset(args, "storage-media-type", "application/vnd.kubernetes.protobuf")
	setIfUnset(args, "tls-cert-file", kubeAPIServerConfig.ServingInfo.ServerCert.CertFile)
	setIfUnset(args, "tls-cipher-suites", kubeAPIServerConfig.ServingInfo.CipherSuites...)
	setIfUnset(args, "tls-min-version", kubeAPIServerConfig.ServingInfo.MinTLSVersion)
	setIfUnset(args, "tls-private-key-file", kubeAPIServerConfig.ServingInfo.ServerCert.KeyFile)
	// TODO re-enable SNI for cluster up
	// tls-sni-cert-key
	setIfUnset(args, "secure-port", portString)

	var keys []string
	for key := range args {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	var arguments []string
	for _, key := range keys {
		for _, token := range args[key] {
			arguments = append(arguments, fmt.Sprintf("--%s=%v", key, token))
		}
	}
	return arguments, nil
}

// currently for cluster up, audit is just broken.
// TODO fix this
func auditFlags(kubeAPIServerConfig *configapi.KubeAPIServerConfig) map[string][]string {
	args := map[string][]string{}
	for key, slice := range kubeAPIServerConfig.APIServerArguments {
		for _, val := range slice {
			args[key] = append(args[key], val)
		}
	}

	return args
}

func setIfUnset(cmdLineArgs map[string][]string, key string, value ...string) {
	if _, ok := cmdLineArgs[key]; !ok {
		cmdLineArgs[key] = value
	}
}

func admissionFlags(admissionPluginConfig map[string]*configapi.AdmissionPluginConfig) (map[string][]string, error) {
	args := map[string][]string{}

	forceOn := []string{}
	forceOff := []string{}
	pluginConfig := map[string]*configapi.AdmissionPluginConfig{}
	for pluginName, origConfig := range admissionPluginConfig {
		config := origConfig.DeepCopy()
		if len(config.Location) > 0 {
			content, err := ioutil.ReadFile(config.Location)
			if err != nil {
				return nil, err
			}
			// if the config isn't a DefaultAdmissionConfig, then assume we're enabled (we were called out after all)
			// if the config *is* a DefaultAdmissionConfig and it explicitly said to disable us, we are disabled
			obj, err := configapilatest.ReadYAML(bytes.NewBuffer(content))
			// if we can't read it, let the plugin deal with it
			// if nothing was there, let the plugin deal with it
			if err != nil || obj == nil {
				forceOn = append(forceOn, pluginName)
				config.Location = ""
				config.Configuration = &runtime.Unknown{Raw: content}
				pluginConfig[pluginName] = config
				continue
			}

			if defaultConfig, ok := obj.(*configapi.DefaultAdmissionConfig); !ok {
				forceOn = append(forceOn, pluginName)
				config.Location = ""
				config.Configuration = &runtime.Unknown{Raw: content}
				pluginConfig[pluginName] = config

			} else if defaultConfig.Disable {
				forceOff = append(forceOff, pluginName)

			} else {
				forceOn = append(forceOn, pluginName)
			}

			continue
		}
		// if it wasn't a DefaultAdmissionConfig object, let the plugin deal with it
		if defaultConfig, ok := config.Configuration.(*configapi.DefaultAdmissionConfig); !ok {
			forceOn = append(forceOn, pluginName)
			pluginConfig[pluginName] = config

		} else if defaultConfig.Disable {
			forceOff = append(forceOff, pluginName)

		} else {
			forceOn = append(forceOn, pluginName)
		}

	}
	upstreamAdmissionConfig, err := originadmission.ConvertOpenshiftAdmissionConfigToKubeAdmissionConfig(pluginConfig)
	if err != nil {
		return nil, err
	}
	configBytes, err := configapilatest.WriteYAML(upstreamAdmissionConfig)
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

	setIfUnset(args, "admission-control-config-file", tempFile.Name())
	setIfUnset(args, "disable-admission-plugins", forceOff...)
	setIfUnset(args, "enable-admission-plugins", forceOn...)

	return args, nil
}
