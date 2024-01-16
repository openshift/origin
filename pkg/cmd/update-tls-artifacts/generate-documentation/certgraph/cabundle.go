package certgraph

import (
	"fmt"
	"strings"

	"github.com/openshift/library-go/pkg/certs/cert-inspection/certgraphapi"
	"k8s.io/apimachinery/pkg/util/sets"
)

type CABundleCoordinates struct {
	CABundle certgraphapi.CertificateAuthorityBundle
}

func (c CABundleCoordinates) String() string {
	return c.CABundle.Name
}

func (c CABundleCoordinates) Name() string {
	return c.CABundle.Name
}

type caBundleSource struct {
	coordinates CABundleCoordinates
	note        string
	sources     []string
}

func (c caBundleSource) Name() string {
	return "ca-bundle/" + c.coordinates.Name()
}

func (r *caBundleSource) GetCABundle() *certgraphapi.CertificateAuthorityBundle {
	return &r.coordinates.CABundle
}

func (r *caBundleSource) GetCertKeyPair() *certgraphapi.CertKeyPair {
	return nil
}

func (s *caBundleSource) Add(resources Resources) Resource {
	resources.Add(s)
	return s
}

func (s *caBundleSource) From(source string) Resource {
	s.sources = append(s.sources, source)
	return s
}

func (s *caBundleSource) Note(note string) Resource {
	s.note = note
	return s
}

func (s *caBundleSource) String() string {
	return fmt.Sprintf("%v%s", s.coordinates, s.note)
}

func (s *caBundleSource) GetNote() string {
	return s.note
}

func (s *caBundleSource) SourceNames() []string {
	if len(s.sources) > 0 {
		return s.sources
	}

	certCommonNames := []string{}
	for _, cert := range s.coordinates.CABundle.Spec.CertificateMetadata {
		certCommonNames = append(certCommonNames, "cert/"+cert.CertIdentifier.CommonName)
	}

	return certCommonNames
}

// strings to hide from display names for ease of reading.
var CertNamesToHide = sets.NewString(
	"ACCVRAIZ1",
	"Actalis Authentication Root CA",
	"AffirmTrust Commercial",
	"AffirmTrust Networking",
	"AffirmTrust Premium",
	"AffirmTrust Premium ECC",
	"Amazon Root CA 1",
	"Amazon Root CA 2",
	"Amazon Root CA 3",
	"Amazon Root CA 4",
	"Atos TrustedRoot 2011",
	"Autoridad de Certificacion Firmaprofesional CIF A62634068",
	"Baltimore CyberTrust Root",
	"Buypass Class 2 Root CA",
	"Buypass Class 3 Root CA",
	"CA Disig Root R2",
	"CFCA EV ROOT",
	"COMODO Certification Authority",
	"COMODO ECC Certification Authority",
	"COMODO RSA Certification Authority",
	"Certigna",
	"Certigna Root CA",
	"Certum Trusted Network CA",
	"Certum Trusted Network CA 2",
	"Chambers of Commerce Root - 2008",
	"AAA Certificate Services",
	"Cybertrust Global Root",
	"D-TRUST Root Class 3 CA 2 2009",
	"D-TRUST Root Class 3 CA 2 EV 2009",
	"DST Root CA X3",
	"DigiCert Assured ID Root CA",
	"DigiCert Assured ID Root G2",
	"DigiCert Assured ID Root G3",
	"DigiCert Global Root CA",
	"DigiCert Global Root G2",
	"DigiCert Global Root G3",
	"DigiCert High Assurance EV Root CA",
	"DigiCert Trusted Root G4",
	"E-Tugra Certification Authority",
	"EC-ACC",
	"EE Certification Centre Root CA",
	"Entrust.net Certification Authority (2048)",
	"Entrust Root Certification Authority",
	"Entrust Root Certification Authority - EC1",
	"Entrust Root Certification Authority - G2",
	"Entrust Root Certification Authority - G4",
	"GDCA TrustAUTH R5 ROOT",
	"GTS Root R1",
	"GTS Root R2",
	"GTS Root R3",
	"GTS Root R4",
	"GeoTrust Global CA",
	"GeoTrust Primary Certification Authority",
	"GeoTrust Primary Certification Authority - G2",
	"GeoTrust Primary Certification Authority - G3",
	"GeoTrust Universal CA",
	"GeoTrust Universal CA 2",
	"GlobalSign",
	"GlobalSign",
	"GlobalSign Root CA",
	"GlobalSign",
	"GlobalSign",
	"GlobalSign",
	"Global Chambersign Root - 2008",
	"Go Daddy Root Certificate Authority - G2",
	"Hellenic Academic and Research Institutions ECC RootCA 2015",
	"Hellenic Academic and Research Institutions RootCA 2011",
	"Hellenic Academic and Research Institutions RootCA 2015",
	"Hongkong Post Root CA 1",
	"Hongkong Post Root CA 3",
	"ISRG Root X1",
	"IdenTrust Commercial Root CA 1",
	"IdenTrust Public Sector Root CA 1",
	"Izenpe.com",
	"LuxTrust Global Root 2",
	"Microsec e-Szigno Root CA 2009",
	"NetLock Arany (Class Gold) Főtanúsítvány",
	"Network Solutions Certificate Authority",
	"OISTE WISeKey Global Root GA CA",
	"OISTE WISeKey Global Root GB CA",
	"OISTE WISeKey Global Root GC CA",
	"QuoVadis Root Certification Authority",
	"QuoVadis Root CA 1 G3",
	"QuoVadis Root CA 2",
	"QuoVadis Root CA 2 G3",
	"QuoVadis Root CA 3",
	"QuoVadis Root CA 3 G3",
	"SSL.com EV Root Certification Authority ECC",
	"SSL.com EV Root Certification Authority RSA R2",
	"SSL.com Root Certification Authority ECC",
	"SSL.com Root Certification Authority RSA",
	"SZAFIR ROOT CA2",
	"SecureSign RootCA11",
	"SecureTrust CA",
	"Secure Global CA",
	"Sonera Class2 CA",
	"Staat der Nederlanden EV Root CA",
	"Staat der Nederlanden Root CA - G3",
	"Starfield Root Certificate Authority - G2",
	"Starfield Services Root Certificate Authority - G2",
	"SwissSign Gold CA - G2",
	"SwissSign Silver CA - G2",
	"T-TeleSec GlobalRoot Class 2",
	"T-TeleSec GlobalRoot Class 3",
	"TUBITAK Kamu SM SSL Kok Sertifikasi - Surum 1",
	"TWCA Global Root CA",
	"TWCA Root Certification Authority",
	"TeliaSonera Root CA v1",
	"TrustCor ECA-1",
	"TrustCor RootCert CA-1",
	"TrustCor RootCert CA-2",
	"UCA Extended Validation Root",
	"UCA Global G2 Root",
	"USERTrust ECC Certification Authority",
	"USERTrust RSA Certification Authority",
	"VeriSign Class 3 Public Primary Certification Authority - G4",
	"VeriSign Class 3 Public Primary Certification Authority - G5",
	"VeriSign Universal Root Certification Authority",
	"VeriSign Class 3 Public Primary Certification Authority - G3",
	"XRamp Global Certification Authority",
	"emSign ECC Root CA - C3",
	"emSign ECC Root CA - G3",
	"emSign Root CA - C1",
	"emSign Root CA - G1",
	"thawte Primary Root CA",
	"thawte Primary Root CA - G2",
	"thawte Primary Root CA - G3",
	"||",
)

func shrinkName(in string) string {
	out := in
	for _, curr := range CertNamesToHide.List() {
		out = strings.ReplaceAll(out, curr, "")
	}
	return out
}
