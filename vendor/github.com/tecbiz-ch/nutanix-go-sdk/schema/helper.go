package schema

import (
	"encoding/base64"
	"encoding/json"

	"github.com/google/uuid"
)

type VMRevertRequest struct {
	VMRecoveryPointUUID *string `json:"vm_recovery_point_uuid"`
}

type MetaData struct {
	SSHAuthorizedKeyMap map[string]string `json:"public_keys,omitempty"`
	Hostname            string            `json:"hostname"`
	UUID                string            `json:"uuid"`
	AvailabilityZone    string            `json:"availability_zone,omitempty"`
	Project             string            `json:"project_id,omitempty"`
}

func (m *MetaData) ToBase64() (string, error) {
	if m.UUID == "" {
		uuid, _ := uuid.NewRandom()
		m.UUID = uuid.String()
	}
	j, err := json.Marshal(m)
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(j), nil
}
