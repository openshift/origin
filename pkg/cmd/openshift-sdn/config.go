package openshift_sdn

import (
	"bytes"
	"io/ioutil"
	"path"
	"strings"

	legacyconfigv1 "github.com/openshift/api/legacyconfig/v1"
	"github.com/openshift/library-go/pkg/config/helpers"
)

func readNodeConfig(filename string) (*legacyconfigv1.NodeConfig, error) {
	data, err := ioutil.ReadFile(filename)
	if err != nil {
		return nil, err
	}

	uncast, err := helpers.ReadYAML(bytes.NewBuffer(data), legacyconfigv1.InstallLegacy)
	if err != nil {
		return nil, err
	}

	ret := uncast.(*legacyconfigv1.NodeConfig)
	// at this point defaults need to be set
	setDefaults_NodeConfig(ret)

	return ret, nil
}

func readAndResolveNodeConfig(filename string) (*legacyconfigv1.NodeConfig, error) {
	nodeConfig, err := readNodeConfig(filename)
	if err != nil {
		return nil, err
	}

	if err := resolveNodeConfigPaths(nodeConfig, path.Dir(filename)); err != nil {
		return nil, err
	}

	return nodeConfig, nil
}

func resolveNodeConfigPaths(config *legacyconfigv1.NodeConfig, base string) error {
	return helpers.ResolvePaths(getNodeFileReferences(config), base)
}

func getNodeFileReferences(config *legacyconfigv1.NodeConfig) []*string {
	refs := []*string{}

	refs = append(refs, &config.ServingInfo.CertInfo.CertFile)
	refs = append(refs, &config.ServingInfo.CertInfo.KeyFile)
	refs = append(refs, &config.ServingInfo.ClientCA)
	for i := range config.ServingInfo.NamedCertificates {
		refs = append(refs, &config.ServingInfo.NamedCertificates[i].CertFile)
		refs = append(refs, &config.ServingInfo.NamedCertificates[i].KeyFile)
	}

	refs = append(refs, &config.DNSRecursiveResolvConf)

	refs = append(refs, &config.MasterKubeConfig)

	refs = append(refs, &config.VolumeDirectory)

	if config.PodManifestConfig != nil {
		refs = append(refs, &config.PodManifestConfig.Path)
	}

	refs = appendFlagsWithFileExtensions(refs, config.KubeletArguments)

	return refs
}

func appendFlagsWithFileExtensions(refs []*string, args legacyconfigv1.ExtendedArguments) []*string {
	for key, s := range args {
		if len(s) == 0 {
			continue
		}
		if !strings.HasSuffix(key, "-file") && !strings.HasSuffix(key, "-dir") {
			continue
		}
		for i := range s {
			refs = append(refs, &s[i])
		}
	}
	return refs
}

func setDefaults_NodeConfig(obj *legacyconfigv1.NodeConfig) {
	if obj.MasterClientConnectionOverrides == nil {
		obj.MasterClientConnectionOverrides = &legacyconfigv1.ClientConnectionOverrides{
			// historical values
			QPS:   10.0,
			Burst: 20,
		}
	}
	setDefaults_ClientConnectionOverrides(obj.MasterClientConnectionOverrides)

	// Defaults/migrations for NetworkConfig
	if len(obj.NetworkConfig.NetworkPluginName) == 0 {
		obj.NetworkConfig.NetworkPluginName = obj.DeprecatedNetworkPluginName
	}
	if obj.NetworkConfig.MTU == 0 {
		obj.NetworkConfig.MTU = 1450
	}
	if len(obj.IPTablesSyncPeriod) == 0 {
		obj.IPTablesSyncPeriod = "30s"
	}

	// Auth cache defaults
	if len(obj.AuthConfig.AuthenticationCacheTTL) == 0 {
		obj.AuthConfig.AuthenticationCacheTTL = "5m"
	}
	if obj.AuthConfig.AuthenticationCacheSize == 0 {
		obj.AuthConfig.AuthenticationCacheSize = 1000
	}
	if len(obj.AuthConfig.AuthorizationCacheTTL) == 0 {
		obj.AuthConfig.AuthorizationCacheTTL = "5m"
	}
	if obj.AuthConfig.AuthorizationCacheSize == 0 {
		obj.AuthConfig.AuthorizationCacheSize = 1000
	}

	// EnableUnidling by default
	if obj.EnableUnidling == nil {
		v := true
		obj.EnableUnidling = &v
	}
}

// SetDefaults_ClientConnectionOverrides defaults a client connection to the pre-1.3 settings of
// being JSON only. Callers must explicitly opt-in to Protobuf support in 1.3+.
func setDefaults_ClientConnectionOverrides(overrides *legacyconfigv1.ClientConnectionOverrides) {
	if len(overrides.AcceptContentTypes) == 0 {
		overrides.AcceptContentTypes = "application/json"
	}
	if len(overrides.ContentType) == 0 {
		overrides.ContentType = "application/json"
	}
}
