package cryptoextensions

import (
	"encoding/asn1"
)

// oid is a helper function for concatenating OIDs
func oid(o asn1.ObjectIdentifier, extra ...int) asn1.ObjectIdentifier {
	return asn1.ObjectIdentifier(append(append([]int{}, o...), extra...))
}

var (
	// RedHatOID is the IANA assigned ObjectIdentifier for Red Hat Inc.
	RedHatOID = asn1.ObjectIdentifier{1, 3, 6, 1, 4, 1, 2312}
	// OpenShiftOID is the Red Hat assigned OID arc for OpenShift.
	OpenShiftOID = oid(RedHatOID, 17)
)
