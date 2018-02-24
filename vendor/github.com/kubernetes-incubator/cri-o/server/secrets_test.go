package server

import (
	"testing"
)

const (
	defaultError   = "unable to get host and container dir"
	secretDataPath = "fixtures/secret"
	emptyPath      = "fixtures/secret/empty"
)

func TestGetMountsMap(t *testing.T) {
	testCases := []struct {
		Path, HostDir, CtrDir string
		Error                 string
	}{
		{"", "", "", defaultError},
		{"/tmp:/home/crio", "/tmp", "/home/crio", ""},
		{"crio/logs:crio/logs", "crio/logs", "crio/logs", ""},
		{"/tmp", "", "", defaultError},
	}
	for _, c := range testCases {
		hostDir, ctrDir, err := getMountsMap(c.Path)
		if hostDir != c.HostDir || ctrDir != c.CtrDir || (err != nil && err.Error() != c.Error) {
			t.Errorf("expect: (%v, %v, %v) \n but got: (%v, %v, %v) \n",
				c.HostDir, c.CtrDir, c.Error, hostDir, ctrDir, err)
		}
	}
}

func TestGetHostSecretData(t *testing.T) {
	testCases := []struct {
		Path string
		Want []SecretData
	}{
		{
			"emptyPath",
			[]SecretData{},
		},
		{
			secretDataPath,
			[]SecretData{
				{"testDataA", []byte("secretDataA")},
				{"testDataB", []byte("secretDataB")},
			},
		},
	}
	for _, c := range testCases {
		if secretData, err := getHostSecretData(c.Path); err != nil {
			t.Error(err)
		} else {
			for index, data := range secretData {
				if data.Name != c.Want[index].Name || string(data.Data) != string(c.Want[index].Data) {
					t.Errorf("expect: (%v, %v) \n but got: (%v, %v) \n",
						c.Want[index].Name, string(c.Want[index].Data), data.Name, string(data.Data))
				}
			}
		}
	}
}
