package v1

import (
	"github.com/GoogleCloudPlatform/kubernetes/pkg/conversion"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/util"
	newer "github.com/openshift/origin/pkg/cmd/server/api"
)

func init() {
	err := newer.Scheme.AddConversionFuncs(
		func(in *ServingInfo, out *newer.ServingInfo, s conversion.Scope) error {
			out.BindAddress = in.BindAddress
			out.ClientCA = in.ClientCA
			out.ServerCert.CertFile = in.CertFile
			out.ServerCert.KeyFile = in.KeyFile
			return nil
		},
		func(in *newer.ServingInfo, out *ServingInfo, s conversion.Scope) error {
			out.BindAddress = in.BindAddress
			out.ClientCA = in.ClientCA
			out.CertFile = in.ServerCert.CertFile
			out.KeyFile = in.ServerCert.KeyFile
			return nil
		},
		func(in *RemoteConnectionInfo, out *newer.RemoteConnectionInfo, s conversion.Scope) error {
			out.URL = in.URL
			out.CA = in.CA
			out.ClientCert.CertFile = in.CertFile
			out.ClientCert.KeyFile = in.KeyFile
			return nil
		},
		func(in *newer.RemoteConnectionInfo, out *RemoteConnectionInfo, s conversion.Scope) error {
			out.URL = in.URL
			out.CA = in.CA
			out.CertFile = in.ClientCert.CertFile
			out.KeyFile = in.ClientCert.KeyFile
			return nil
		},
		func(in *RequestHeaderIdentityProvider, out *newer.RequestHeaderIdentityProvider, s conversion.Scope) error {
			if err := s.DefaultConvert(in, out, conversion.IgnoreMissingFields); err != nil {
				return err
			}
			out.Headers = util.NewStringSet(in.HeadersSlice...)
			return nil
		},
		func(in *newer.RequestHeaderIdentityProvider, out *RequestHeaderIdentityProvider, s conversion.Scope) error {
			if err := s.DefaultConvert(in, out, conversion.IgnoreMissingFields); err != nil {
				return err
			}
			out.HeadersSlice = in.Headers.List()
			return nil
		},
	)
	if err != nil {
		// If one of the conversion functions is malformed, detect it immediately.
		panic(err)
	}
}
