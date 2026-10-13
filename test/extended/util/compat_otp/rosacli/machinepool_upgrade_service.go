package rosacli

import (
	"bytes"

	logger "github.com/openshift/origin/test/extended/util/compat_otp/logext"
	"gopkg.in/yaml.v3"
)

type MachinePoolUpgradeService interface {
	ResourcesCleaner

	ListUpgrades(clusterID string, mpID string) (bytes.Buffer, error)
	ReflectUpgradesList(result bytes.Buffer) (*MachinePoolUpgradeList, error)
	ListAndReflectUpgrades(clusterID string, mpID string) (*MachinePoolUpgradeList, error)

	// Create a manual upgrade. `version`, `scheduleDate` and `scheduleTime` are optional. `schedule*` if provided MUST be both at the same time provided.
	CreateManualUpgrade(clusterID string, mpID string, version string, scheduleDate string, scheduleTime string) (bytes.Buffer, error)
	// Create an automatic upgrade based on the given cron.
	CreateAutomaticUpgrade(clusterID string, mpID string, scheduleCron string) (bytes.Buffer, error)

	DescribeUpgrade(clusterID string, mpID string) (bytes.Buffer, error)
	ReflectUpgradeDescription(result bytes.Buffer) (*MachinePoolUpgradeDescription, error)
	DescribeAndReflectUpgrade(clusterID string, mpID string) (*MachinePoolUpgradeDescription, error)

	DeleteUpgrade(clusterID string, mpID string) (bytes.Buffer, error)

	RetrieveHelpForCreate() (bytes.Buffer, error)
	RetrieveHelpForDescribe() (bytes.Buffer, error)
	RetrieveHelpForList() (bytes.Buffer, error)
	RetrieveHelpForDelete() (bytes.Buffer, error)
}

type machinePoolUpgradeService struct {
	ResourcesService

	machinePools map[string][]string
}

func NewMachinePoolUpgradeService(client *Client) MachinePoolUpgradeService {
	return &machinePoolUpgradeService{
		ResourcesService: ResourcesService{
			client: client,
		},

		machinePools: make(map[string][]string),
	}
}

type MachinePoolUpgrade struct {
	Version string `json:"VERSION,omitempty"`
	Notes   string `json:"NOTES,omitempty"`
}
type MachinePoolUpgradeList struct {
	MachinePoolUpgrades []MachinePoolUpgrade `json:"MachinePoolUpgrades,omitempty"`
}

// Struct for the 'rosa describe upgrades' output for non-hosted-cp clusters
type MachinePoolUpgradeDescription struct {
	ID           string `yaml:"ID,omitempty"`
	ClusterID    string `yaml:"Cluster ID,omitempty"`
	ScheduleType string `yaml:"Schedule Type,omitempty"`
	NextRun      string `yaml:"Next Run,omitempty"`
	UpgradeState string `yaml:"Upgrade State,omitempty"`
	Version      string `yaml:"Version,omitempty"`
}

// List MachinePool upgrades
func (mpus *machinePoolUpgradeService) ListUpgrades(clusterID string, mpID string) (bytes.Buffer, error) {
	listMachinePool := mpus.client.Runner.
		Cmd("list", "upgrades").
		CmdFlags("-c", clusterID, "--machinepool", mpID)
	return listMachinePool.Run()
}

// Pasrse the result of 'rosa list upgrades --machinepool' to MachinePoolList struct
func (mpus *machinePoolUpgradeService) ReflectUpgradesList(result bytes.Buffer) (mpul *MachinePoolUpgradeList, err error) {
	mpul = &MachinePoolUpgradeList{}
	theMap := mpus.client.Parser.TableData.Input(result).Parse().Output()
	for _, machinepoolItem := range theMap {
		mpu := &MachinePoolUpgrade{}
		err = MapStructure(machinepoolItem, mpu)
		if err != nil {
			return
		}
		mpul.MachinePoolUpgrades = append(mpul.MachinePoolUpgrades, *mpu)
	}
	return mpul, err
}

func (mpus *machinePoolUpgradeService) ListAndReflectUpgrades(clusterID string, mpID string) (mpul *MachinePoolUpgradeList, err error) {
	output, err := mpus.ListUpgrades(clusterID, mpID)
	if err != nil {
		return nil, err
	}
	return mpus.ReflectUpgradesList(output)
}

func (mpus *machinePoolUpgradeService) CreateManualUpgrade(clusterID string, mpID string, version string, scheduleDate string, scheduleTime string) (output bytes.Buffer, err error) {
	var flags []string
	if version != "" {
		flags = append(flags, "--version", version)
	}

	if scheduleDate != "" {
		flags = append(flags, "--schedule-date", scheduleDate)
		flags = append(flags, "--schedule-time", scheduleTime)
	}

	return mpus.create(clusterID, mpID, flags...)
}

func (mpus *machinePoolUpgradeService) CreateAutomaticUpgrade(clusterID string, mpID string, scheduleCron string) (output bytes.Buffer, err error) {
	return mpus.create(clusterID, mpID, "--schedule", scheduleCron)
}

func (mpus *machinePoolUpgradeService) create(clusterID string, mpID string, flags ...string) (output bytes.Buffer, err error) {
	output, err = mpus.client.Runner.
		Cmd("upgrade", "machinepool", mpID).
		CmdFlags(append(flags, "-c", clusterID)...).
		Run()
	if err == nil {
		mpus.machinePools[clusterID] = append(mpus.machinePools[clusterID], mpID)
	}
	return
}

// Describe MachinePool
func (mpus *machinePoolUpgradeService) DescribeUpgrade(clusterID string, mpID string) (bytes.Buffer, error) {
	describeMp := mpus.client.Runner.
		Cmd("describe", "upgrade").
		CmdFlags("-c", clusterID, "--machinepool", mpID)
	return describeMp.Run()
}

// Pasrse the result of 'rosa describe upgrade --machinepool' to the RosaClusterDescription struct
func (mpus *machinePoolUpgradeService) ReflectUpgradeDescription(result bytes.Buffer) (*MachinePoolUpgradeDescription, error) {
	theMap, err := mpus.client.Parser.TextData.Input(result).Parse().YamlToMap()
	if err != nil {
		return nil, err
	}
	data, err := yaml.Marshal(&theMap)
	if err != nil {
		return nil, err
	}
	mpud := new(MachinePoolUpgradeDescription)
	err = yaml.Unmarshal(data, mpud)
	return mpud, err
}

func (mpus *machinePoolUpgradeService) DescribeAndReflectUpgrade(clusterID string, mpID string) (*MachinePoolUpgradeDescription, error) {
	output, err := mpus.DescribeUpgrade(clusterID, mpID)
	if err != nil {
		return nil, err
	}
	return mpus.ReflectUpgradeDescription(output)
}

func (mpus *machinePoolUpgradeService) DeleteUpgrade(clusterID string, mpID string) (output bytes.Buffer, err error) {
	output, err = mpus.client.Runner.
		Cmd("delete", "upgrade").
		CmdFlags("-c", clusterID, "--machinepool", mpID, "-y").Run()
	if err == nil {
		mpus.machinePools[clusterID] = RemoveFromStringSlice(mpus.machinePools[clusterID], mpID)
	}
	return
}

// Create MachinePool
func (mpus *machinePoolUpgradeService) RetrieveHelpForCreate() (output bytes.Buffer, err error) {
	return mpus.client.Runner.Cmd("upgrade", "machinepool").CmdFlags("-h").Run()
}

func (mpus *machinePoolUpgradeService) RetrieveHelpForList() (output bytes.Buffer, err error) {
	return mpus.client.Runner.Cmd("list", "upgrades").CmdFlags("-h").Run()
}

func (mpus *machinePoolUpgradeService) RetrieveHelpForDescribe() (output bytes.Buffer, err error) {
	return mpus.client.Runner.Cmd("list", "upgrade").CmdFlags("-h").Run()
}

func (mpus *machinePoolUpgradeService) RetrieveHelpForDelete() (output bytes.Buffer, err error) {
	return mpus.client.Runner.Cmd("list", "upgrade").CmdFlags("-h").Run()
}

func (mpus *machinePoolUpgradeService) CleanResources(clusterID string) (errors []error) {
	var mpsToDel []string
	mpsToDel = append(mpsToDel, mpus.machinePools[clusterID]...)
	for _, mpID := range mpsToDel {
		logger.Infof("Remove remaining machinepool upgrade on '%s'", mpID)
		_, err := mpus.DeleteUpgrade(clusterID, mpID)
		if err != nil {
			errors = append(errors, err)
		}
	}

	return
}
