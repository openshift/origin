package grpc_interop

import (
	"context"
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
	// Target is the actual IP you want to dial instead of resolving hostname
	Target string
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

	if cfg.Target != "" {
		dialer := func(ctx context.Context, addr string) (net.Conn, error) {
			_, port, err := net.SplitHostPort(addr)
			if err != nil {
				return nil, errors.New("Failed to split addr")
			}
			// Connect to targetIP:port regardless of hostname
			dest := net.JoinHostPort(cfg.Target, port)
			return (&net.Dialer{}).DialContext(ctx, "tcp", dest)
		}
		opts = append(opts, grpc.WithContextDialer(dialer))
	}

	return grpc.Dial(net.JoinHostPort(cfg.Host, strconv.Itoa(cfg.Port)), append(opts, grpc.WithBlock())...)
}
