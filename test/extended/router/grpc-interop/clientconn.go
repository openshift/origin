package grpc_interop

import (
	"crypto/tls"
	"crypto/x509"
	"errors"
	"net"
	"strconv"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

type DialParams struct {
	UseTLS   bool
	CertData []byte
	Host     string
	Port     int
	Insecure bool
}

func Dial(cfg DialParams) (*grpc.ClientConn, error) {
	var opts []grpc.DialOption

	if cfg.UseTLS {
		tlsConfig := &tls.Config{
			InsecureSkipVerify: cfg.Insecure,
		}
		if len(cfg.CertData) > 0 {
			rootCAs, err := x509.SystemCertPool()
			if err != nil {
				return nil, err
			}
			if rootCAs == nil {
				rootCAs = x509.NewCertPool()
			}
			if ok := rootCAs.AppendCertsFromPEM(cfg.CertData); !ok {
				return nil, errors.New("failed to append certs")
			}
			tlsConfig.RootCAs = rootCAs
		}
		opts = append(opts, grpc.WithTransportCredentials(credentials.NewTLS(tlsConfig)))
	} else {
		opts = append(opts, grpc.WithInsecure())
	}

	return grpc.Dial(net.JoinHostPort(cfg.Host, strconv.Itoa(cfg.Port)), append(opts, grpc.WithBlock())...)
}
