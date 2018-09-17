package configprocessing

import (
	"net"
	"strconv"

	configv1 "github.com/openshift/api/config/v1"

	genericapiserveroptions "k8s.io/apiserver/pkg/server/options"
	utilflag "k8s.io/apiserver/pkg/util/flag"
)

func ToServingOptions(servingInfo configv1.HTTPServingInfo) (*genericapiserveroptions.SecureServingOptionsWithLoopback, error) {
	host, portString, err := net.SplitHostPort(servingInfo.BindAddress)
	if err != nil {
		return nil, err
	}
	port, err := strconv.Atoi(portString)
	if err != nil {
		return nil, err
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

	servingOptionsWithLoopback := genericapiserveroptions.WithLoopback(servingOptions)
	return servingOptionsWithLoopback, nil
}
