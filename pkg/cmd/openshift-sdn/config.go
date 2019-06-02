package openshift_sdn

import (
	"bytes"
	"io/ioutil"
	"path"
	"strings"

	legacyconfigv1 "github.com/openshift/api/legacyconfig/v1"
	"github.com/openshift/library-go/pkg/config/helpers"
	configapi "github.com/openshift/origin/pkg/cmd/server/apis/config"
	configv1 "github.com/openshift/origin/pkg/cmd/server/apis/config/v1"
)

func readNodeConfig(filename string) (*configapi.NodeConfig, error) {
	data, err := ioutil.ReadFile(filename)
	if err != nil {
		return nil, err
	}

	uncast, err := helpers.ReadYAMLToInternal(bytes.NewBuffer(data), configapi.InstallLegacy, legacyconfigv1.InstallLegacy, configv1.InstallLegacy, configv1.InstallLegacyExternal)
	if err != nil {
		return nil, err
	}
	return uncast.(*configapi.NodeConfig), nil
}

func readAndResolveNodeConfig(filename string) (*configapi.NodeConfig, error) {
	nodeConfig, err := readNodeConfig(filename)
	if err != nil {
		return nil, err
	}

	if err := resolveNodeConfigPaths(nodeConfig, path.Dir(filename)); err != nil {
		return nil, err
	}

	return nodeConfig, nil
}

func resolveNodeConfigPaths(config *configapi.NodeConfig, base string) error {
	return helpers.ResolvePaths(getNodeFileReferences(config), base)
}

func getNodeFileReferences(config *configapi.NodeConfig) []*string {
	refs := []*string{}

	refs = append(refs, &config.ServingInfo.ServerCert.CertFile)
	refs = append(refs, &config.ServingInfo.ServerCert.KeyFile)
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

func appendFlagsWithFileExtensions(refs []*string, args configapi.ExtendedArguments) []*string {
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
