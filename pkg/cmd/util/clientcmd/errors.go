package clientcmd

import (
	"strings"

	kerrors "github.com/GoogleCloudPlatform/kubernetes/pkg/api/errors"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/client/clientcmd"
)

const (
	unknownReason                     = 0
	noServerFoundReason               = 1
	certificateAuthorityUnknownReason = 2
	configurationInvalidReason        = 3

	certificateAuthorityUnknownMsg = "The server uses a certificate signed by unknown authority. You may need to use the --certificate-authority flag to provide the path to a certificate file for the certificate authority, or --insecure-skip-tls-verify to bypass the certificate check and use insecure connections."
	notConfiguredMsg               = `OpenShift is not configured. You need to run the login command in order to create a default config for your server and credentials:
  oc login
You can also run this command again providing the path to a config file directly, either through the --config flag of the KUBECONFIG environment variable.
`
)

// GetPrettyMessageFor prettifys the message of the provided error
func GetPrettyMessageFor(err error) string {
	if err == nil {
		return ""
	}

	reason := detectReason(err)

	switch reason {
	case noServerFoundReason:
		return notConfiguredMsg

	case certificateAuthorityUnknownReason:
		return certificateAuthorityUnknownMsg
	}

	return err.Error()
}

// IsNoServerFound checks whether the provided error is a 'no server found' error or not
func IsNoServerFound(err error) bool {
	return detectReason(err) == noServerFoundReason
}

// IsConfigurationInvalid checks whether the provided error is a 'invalid configuration' error or not
func IsConfigurationInvalid(err error) bool {
	return detectReason(err) == configurationInvalidReason
}

// IsCertificateAuthorityUnknown checks whether the provided error is a 'certificate authority unknown' error or not
func IsCertificateAuthorityUnknown(err error) bool {
	return detectReason(err) == certificateAuthorityUnknownReason
}

// IsForbidden checks whether the provided error is a 'forbidden' error or not
func IsForbidden(err error) bool {
	return kerrors.IsForbidden(err)
}

func detectReason(err error) int {
	if err != nil {
		switch {
		case strings.Contains(err.Error(), "certificate signed by unknown authority"):
			return certificateAuthorityUnknownReason
		case strings.Contains(err.Error(), "no server defined"):
			return noServerFoundReason
		case clientcmd.IsConfigurationInvalid(err):
			return configurationInvalidReason
		}
	}
	return unknownReason
}
