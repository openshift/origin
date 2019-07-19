package serving

import (
	"fmt"
	"net"
	"strconv"

	genericapiserveroptions "k8s.io/apiserver/pkg/server/options"
	utilflag "k8s.io/component-base/cli/flag"

	configv1 "github.com/openshift/api/config/v1"
)

func ToServingOptions(servingInfo configv1.HTTPServingInfo) (*genericapiserveroptions.SecureServingOptionsWithLoopback, error) {
	host, portString, err := net.SplitHostPort(servingInfo.BindAddress)
	if err != nil {
		return nil, fmt.Errorf("bindAddress is invalid: %v", err)
	}
	port, err := strconv.Atoi(portString)
	if err != nil {
		return nil, fmt.Errorf("bindAddress is invalid: %v", err)
	}
	if t := net.ParseIP(host); t == nil {
		return nil, fmt.Errorf("bindAddress is invalid: %v", "not an IP")
	}

	servingOptions := genericapiserveroptions.NewSecureServingOptions()
	servingOptions.BindAddress = net.ParseIP(host)
	servingOptions.BindPort = port
	servingOptions.BindNetwork = servingInfo.BindNetwork
	servingOptions.ServerCert.CertKey.CertFile = servingInfo.CertFile
	servingOptions.ServerCert.CertKey.KeyFile = servingInfo.KeyFile
	servingOptions.CipherSuites = servingInfo.CipherSuites
	servingOptions.MinTLSVersion = servingInfo.MinTLSVersion

	for _, namedCert := range servingInfo.NamedCertificates {
		genericNamedCertKey := utilflag.NamedCertKey{
			Names:    namedCert.Names,
			CertFile: namedCert.CertFile,
			KeyFile:  namedCert.KeyFile,
		}

		servingOptions.SNICertKeys = append(servingOptions.SNICertKeys, genericNamedCertKey)
	}

	// TODO sort out what we should do here
	//servingOptions.HTTP2MaxStreamsPerConnection = ??

	servingOptionsWithLoopback := servingOptions.WithLoopback()
	return servingOptionsWithLoopback, nil
}
