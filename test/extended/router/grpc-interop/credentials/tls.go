package credentials

import (
	"context"
	"crypto/tls"
	"net"

	"google.golang.org/grpc/credentials"

	credinternal "github.com/openshift/origin/test/extended/router/grpc-interop/credentials/internal/credentials"
)

// NewTLS constructs a TransportCredentials object based on the provided TLS configuration.
// The returned TransportCredentials do not verify the negotiated protocol during the client handshake.
// These credentials are intended for use with gRPC-go clients to bypass the enforced ALPN enablement
// introduced in gRPC-go version 1.67.0 (https://github.com/grpc/grpc-go/pull/7535).
func NewTLS(c *tls.Config, delegatedCreds credentials.TransportCredentials) credentials.TransportCredentials {
	return &tlsCredsNoALPNCheck{
		TransportCredentials: delegatedCreds,
		config:               c,
	}
}

type tlsCredsNoALPNCheck struct {
	credentials.TransportCredentials
	config *tls.Config
}

// ClientHandshake is a direct copy of gRPC's TLS credentials client handshake method
// (google.golang.org/grpc/credentials/tls.go#tlsCreds), with the omission of protocol negotiation checks.
func (c *tlsCredsNoALPNCheck) ClientHandshake(ctx context.Context, authority string, rawConn net.Conn) (net.Conn, credentials.AuthInfo, error) {
	// use local cfg to avoid clobbering ServerName if using multiple endpoints
	cfg := credinternal.CloneTLSConfig(c.config)
	if cfg.ServerName == "" {
		serverName, _, err := net.SplitHostPort(authority)
		if err != nil {
			// If the authority had no host port or if the authority cannot be parsed, use it as-is.
			serverName = authority
		}
		cfg.ServerName = serverName
	}
	conn := tls.Client(rawConn, cfg)
	errChannel := make(chan error, 1)
	go func() {
		errChannel <- conn.Handshake()
		close(errChannel)
	}()
	select {
	case err := <-errChannel:
		if err != nil {
			conn.Close()
			return nil, nil, err
		}
	case <-ctx.Done():
		conn.Close()
		return nil, nil, ctx.Err()
	}

	// NOTE: The negotiated protocol check is removed!

	tlsInfo := credentials.TLSInfo{
		State: conn.ConnectionState(),
		CommonAuthInfo: credentials.CommonAuthInfo{
			SecurityLevel: credentials.PrivacyAndIntegrity,
		},
	}
	id := credinternal.SPIFFEIDFromState(conn.ConnectionState())
	if id != nil {
		tlsInfo.SPIFFEID = id
	}
	return credinternal.WrapSyscallConn(rawConn, conn), tlsInfo, nil
}

func (c *tlsCredsNoALPNCheck) ServerHandshake(rawConn net.Conn) (net.Conn, credentials.AuthInfo, error) {
	return c.TransportCredentials.ServerHandshake(rawConn)
}

func (c *tlsCredsNoALPNCheck) Info() credentials.ProtocolInfo {
	return c.TransportCredentials.Info()
}

func (c *tlsCredsNoALPNCheck) Clone() credentials.TransportCredentials {
	return c.TransportCredentials.Clone()
}

func (c *tlsCredsNoALPNCheck) OverrideServerName(serverNameOverride string) error {
	return c.TransportCredentials.OverrideServerName(serverNameOverride)
}
