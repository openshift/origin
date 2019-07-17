package login

import (
	"crypto/x509"
	"errors"
	"fmt"
	"strings"
)

const (
	unknownReason = iota
	noServerFoundReason
	certificateAuthorityUnknownReason
	certificateHostnameErrorReason
	certificateInvalidReason
	tlsOversizedRecordReason

	certificateAuthorityUnknownMsg = "The server uses a certificate signed by unknown authority. You may need to use the --certificate-authority flag to provide the path to a certificate file for the certificate authority, or --insecure-skip-tls-verify to bypass the certificate check and use insecure connections."
	notConfiguredMsg               = `The client is not configured. You need to run the login command in order to create a default config for your server and credentials:
  oc login
You can also run this command again providing the path to a config file directly, either through the --config flag of the KUBECONFIG environment variable.
`
	tlsOversizedRecordMsg = `Unable to connect to %[2]s using TLS: %[1]s.
Ensure the specified server supports HTTPS.`
)

// GetPrettyMessageForServer prettifys the message of the provided error
func getPrettyMessageForServer(err error, serverName string) string {
	if err == nil {
		return ""
	}

	reason := detectReason(err)

	switch reason {
	case noServerFoundReason:
		return notConfiguredMsg

	case certificateAuthorityUnknownReason:
		return certificateAuthorityUnknownMsg

	case tlsOversizedRecordReason:
		if len(serverName) == 0 {
			serverName = "server"
		}
		return fmt.Sprintf(tlsOversizedRecordMsg, err, serverName)

	case certificateHostnameErrorReason:
		return fmt.Sprintf("The server is using a certificate that does not match its hostname: %s", err)

	case certificateInvalidReason:
		return fmt.Sprintf("The server is using an invalid certificate: %s", err)
	}

	return err.Error()
}

// GetPrettyErrorForServer prettifys the message of the provided error
func getPrettyErrorForServer(err error, serverName string) error {
	return errors.New(getPrettyMessageForServer(err, serverName))
}

func detectReason(err error) int {
	if err != nil {
		switch {
		case strings.Contains(err.Error(), "certificate signed by unknown authority"):
			return certificateAuthorityUnknownReason
		case strings.Contains(err.Error(), "no server defined"):
			return noServerFoundReason
		case strings.Contains(err.Error(), "tls: oversized record received"):
			return tlsOversizedRecordReason
		}
		switch err.(type) {
		case x509.UnknownAuthorityError:
			return certificateAuthorityUnknownReason
		case x509.HostnameError:
			return certificateHostnameErrorReason
		case x509.CertificateInvalidError:
			return certificateInvalidReason
		}
	}
	return unknownReason
}
