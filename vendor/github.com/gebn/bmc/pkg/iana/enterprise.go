package iana

import (
	"fmt"
)

// Enterprise represents an IANA Private Enterprise Number. A list of
// assignments can be found at
// https://www.iana.org/assignments/enterprise-numbers/enterprise-numbers. This
// type does not make any assumptions about the wire format, however typically
// an enterprise number is represented as a 3 or 4 byte uint.
type Enterprise uint32

const (
	// EnterpriseIntel is the enterprise number of Intel Corporation.
	EnterpriseIntel Enterprise = 343

	// EnterpriseDell is the enterprise number of Dell Inc.
	EnterpriseDell Enterprise = 674

	// EnterpriseNvidia is the enterprise number of NVIDIA Corporation.
	EnterpriseNvidia Enterprise = 5703

	// EnterpriseQuanta is the enterprise number of Quanta Computer Inc.
	EnterpriseQuanta Enterprise = 7244

	// EnterpriseSuperMicro is the enterprise number of Super Micro Computer
	// Inc.
	EnterpriseSuperMicro Enterprise = 10876

	// EnterpriseGigaByte is the enterprise number of Giga-Byte Technology Co.,
	// Ltd.
	EnterpriseGigaByte Enterprise = 15370

	// EnterpriseAten is the enterprise number of ATEN International Co., Ltd.
	EnterpriseAten Enterprise = 21317
)

var (
	// enterpriseOrganisations contains a few common Enterprise Numbers along
	// with their official organisation names to handle the majority of cases.
	enterpriseOrganisations = map[Enterprise]string{
		EnterpriseIntel:      "Intel Corporation",
		EnterpriseDell:       "Dell Inc.",
		EnterpriseNvidia:     "NVIDIA Corporation",
		EnterpriseQuanta:     "Quanta Computer Inc.",
		EnterpriseSuperMicro: "Super Micro Computer Inc.",
		EnterpriseGigaByte:   "GIGA-BYTE TECHNOLOGY CO., LTD",
		EnterpriseAten:       "ATEN INTERNATIONAL CO., LTD.",
	}
)

// Organisation returns the official name of the organisation behind a given
// enterprise number, or "Unknown" if it is not recognised.
func (e Enterprise) Organisation() string {
	if name, ok := enterpriseOrganisations[e]; ok {
		return name
	}
	return "Unknown"
}

// String returns the enterprise number, along with the organisation behind it
// if known, e.g. "674(Dell Inc.)".
func (e Enterprise) String() string {
	return fmt.Sprintf("%v(%v)", uint32(e), e.Organisation())
}
