package pki

// CertificateType identifies the category of a certificate for profile resolution.
type CertificateType string

const (
	// CertificateTypeSigner identifies certificate authority (CA) certificates
	// that sign other certificates.
	CertificateTypeSigner CertificateType = "signer"

	// CertificateTypeServing identifies TLS server certificates used to serve
	// HTTPS endpoints.
	CertificateTypeServing CertificateType = "serving"

	// CertificateTypeClient identifies client authentication certificates used
	// to authenticate to servers.
	CertificateTypeClient CertificateType = "client"

	// CertificateTypePeer identifies certificates used for both server and client
	// authentication. The resolved key configuration is the stronger of the
	// serving and client configurations.
	CertificateTypePeer CertificateType = "peer"
)
