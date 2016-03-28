package emptydir

import (
	"errors"
	"strings"
	"testing"

	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/resource"
)

const expectedDevice = "/dev/sdb2"

func TestParseFSDevice(t *testing.T) {
	tests := map[string]struct {
		dfOutput  string
		expDevice string
		expError  string
	}{
		"happy path": {
			dfOutput:  "Filesystem\n/dev/sdb2",
			expDevice: expectedDevice,
		},
		"happy path multi-token": {
			dfOutput:  "Filesystem\n/dev/sdb2           16444592     8  16444584   1% /var/openshift.local.volumes/",
			expDevice: expectedDevice,
		},
		"invalid tmpfs": {
			dfOutput: "Filesystem\ntmpfs",
			expError: invalidFilesystemError,
		},
		"invalid empty": {
			dfOutput: "",
			expError: unexpectedLineCountError,
		},
		"invalid one line": {
			dfOutput: "Filesystem\n",
			expError: invalidFilesystemError,
		},
		"invalid blank second line": {
			dfOutput: "Filesystem\n\n",
			expError: invalidFilesystemError,
		},
		"invalid too many lines": {
			dfOutput:  "Filesystem\n/dev/sdb2\ntmpfs\nwhatisgoingon",
			expDevice: expectedDevice,
		},
	}
	for name, test := range tests {
		t.Logf("running TestParseFSDevice: %s", name)
		device, err := parseFSDevice(test.dfOutput)
		if test.expDevice != "" && test.expDevice != device {
			t.Errorf("Unexpected filesystem device, expected: %s, got: %s", test.expDevice, device)
		}
		if test.expError != "" && (err == nil || !strings.Contains(err.Error(), test.expError)) {
			t.Errorf("Unexpected filesystem error, expected: %s, got: %s", test.expError, err)
		}
	}
}

// Avoid running actual commands to manage XFS quota:
type mockQuotaCommandRunner struct {
	RunFSDeviceCommandResponse *cmdResponse
	RunFSTypeCommandResponse   *cmdResponse

	RanApplyQuotaFSDevice string
	RanApplyQuota         *resource.Quantity
	RanApplyQuotaFSGroup  int64
}

func (m *mockQuotaCommandRunner) RunFSTypeCommand(dir string) (string, string, error) {
	if m.RunFSTypeCommandResponse != nil {
		return m.RunFSTypeCommandResponse.Stdout, m.RunFSTypeCommandResponse.Stderr, m.RunFSTypeCommandResponse.Error
	}
	return "xfs", "", nil
}

func (m *mockQuotaCommandRunner) RunFSDeviceCommand(dir string) (string, string, error) {
	if m.RunFSDeviceCommandResponse != nil {
		return m.RunFSDeviceCommandResponse.Stdout, m.RunFSDeviceCommandResponse.Stderr, m.RunFSDeviceCommandResponse.Error
	}
	return "Filesystem\n/dev/sdb2", "", nil
}

func (m *mockQuotaCommandRunner) RunApplyQuotaCommand(fsDevice string, quota resource.Quantity, fsGroup int64) (string, string, error) {
	// Store these for assertions in tests:
	m.RanApplyQuotaFSDevice = fsDevice
	m.RanApplyQuota = &quota
	m.RanApplyQuotaFSGroup = fsGroup
	return "", "", nil
}

// Small struct for specifying how we want the various quota command runners to
// respond in tests:
type cmdResponse struct {
	Stdout string
	Stderr string
	Error  error
}

func TestApplyQuota(t *testing.T) {

	var defaultFSGroup int64
	defaultFSGroup = 1000050000

	tests := map[string]struct {
		FSGroupID *int64
		Quota     string

		FSTypeCmdResponse     *cmdResponse
		FSDeviceCmdResponse   *cmdResponse
		ApplyQuotaCmdResponse *cmdResponse

		ExpFSDevice string
		ExpError    string // sub-string to be searched for in error message
		ExpSkipped  bool
	}{
		"happy path": {
			Quota:     "512",
			FSGroupID: &defaultFSGroup,
		},
		"zero quota": {
			Quota:     "0",
			FSGroupID: &defaultFSGroup,
		},
		"invalid filesystem device": {
			Quota:     "512",
			FSGroupID: &defaultFSGroup,
			FSDeviceCmdResponse: &cmdResponse{
				Stdout: "Filesystem\ntmpfs",
				Stderr: "",
				Error:  nil,
			},
			ExpError:   invalidFilesystemError,
			ExpSkipped: true,
		},
		"error checking filesystem device": {
			Quota:     "512",
			FSGroupID: &defaultFSGroup,
			FSDeviceCmdResponse: &cmdResponse{
				Stdout: "",
				Stderr: "no such file or directory",
				Error:  errors.New("no such file or directory"), // Would be exit error in real life
			},
			ExpError:   "no such file or directory",
			ExpSkipped: true,
		},
		"non-xfs filesystem type": {
			Quota:     "512",
			FSGroupID: &defaultFSGroup,
			FSTypeCmdResponse: &cmdResponse{
				Stdout: "ext4",
				Stderr: "",
				Error:  nil,
			},
			ExpError:   "not on an XFS filesystem",
			ExpSkipped: true,
		},
		"error checking filesystem type": {
			Quota:     "512",
			FSGroupID: &defaultFSGroup,
			FSTypeCmdResponse: &cmdResponse{
				Stdout: "",
				Stderr: "no such file or directory",
				Error:  errors.New("no such file or directory"), // Would be exit error in real life
			},
			ExpError:   "unable to check filesystem type",
			ExpSkipped: true,
		},
		// Should result in success, but no quota actually gets applied:
		"no FSGroup": {
			Quota:      "512",
			ExpSkipped: true,
		},
	}

	for name, test := range tests {
		t.Logf("running TestApplyQuota: %s", name)
		quotaApplicator := xfsQuotaApplicator{}
		// Replace the real command runner with our mock:
		mockCmdRunner := mockQuotaCommandRunner{}
		quotaApplicator.cmdRunner = &mockCmdRunner
		fakeDir := "/var/lib/origin/openshift.local.volumes/pods/d71f6949-cb3f-11e5-aedf-989096de63cb"

		// Configure the default happy path command responses if nothing was specified
		// by the test:
		if test.FSTypeCmdResponse == nil {
			// Configure the default happy path response:
			test.FSTypeCmdResponse = &cmdResponse{
				Stdout: "xfs",
				Stderr: "",
				Error:  nil,
			}
		}
		if test.FSDeviceCmdResponse == nil {
			test.FSDeviceCmdResponse = &cmdResponse{
				Stdout: "Filesystem\n/dev/sdb2",
				Stderr: "",
				Error:  nil,
			}
		}

		if test.ApplyQuotaCmdResponse == nil {
			test.ApplyQuotaCmdResponse = &cmdResponse{
				Stdout: "",
				Stderr: "",
				Error:  nil,
			}
		}

		mockCmdRunner.RunFSDeviceCommandResponse = test.FSDeviceCmdResponse
		mockCmdRunner.RunFSTypeCommandResponse = test.FSTypeCmdResponse

		quota := resource.MustParse(test.Quota)
		err := quotaApplicator.Apply(fakeDir, kapi.StorageMediumDefault, &kapi.Pod{}, test.FSGroupID, quota)
		if test.ExpError == "" && !test.ExpSkipped {
			// Expecting success case:
			if mockCmdRunner.RanApplyQuotaFSDevice != "/dev/sdb2" {
				t.Errorf("failed: '%s', expected quota applied to: %s, got: %s", name, "/dev/sdb2", mockCmdRunner.RanApplyQuotaFSDevice)
			}
			if mockCmdRunner.RanApplyQuota.Value() != quota.Value() {
				t.Errorf("failed: '%s', expected quota: %d, got: %d", name, quota.Value(),
					mockCmdRunner.RanApplyQuota.Value())
			}
			if mockCmdRunner.RanApplyQuotaFSGroup != *test.FSGroupID {
				t.Errorf("failed: '%s', expected FSGroup: %d, got: %d", name, test.FSGroupID, mockCmdRunner.RanApplyQuotaFSGroup)
			}
		} else if test.ExpError != "" {
			// Expecting error case:
			if err == nil {
				t.Errorf("failed: '%s', expected error but got none", name)
			} else if !strings.Contains(err.Error(), test.ExpError) {
				t.Errorf("failed: '%s', expected error containing '%s', got: '%s'", name, test.ExpError, err)
			}
		}

		if test.ExpSkipped {
			if mockCmdRunner.RanApplyQuota != nil {
				t.Errorf("failed: '%s', expected error but quota was applied", name)
			}
			if mockCmdRunner.RanApplyQuotaFSGroup != 0 {
				t.Errorf("failed: '%s', expected error but quota was applied", name)
			}
			if mockCmdRunner.RanApplyQuotaFSDevice != "" {
				t.Errorf("failed: '%s', expected error but quota was applied", name)
			}
		}
	}
}
