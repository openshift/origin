package rosacli

import (
	"bytes"
	"os"

	logger "github.com/openshift/origin/test/extended/util/compat_otp/logext"
	"gopkg.in/yaml.v3"
)

type TuningConfigService interface {
	ResourcesCleaner

	CreateTuningConfig(clusterID string, tcName string, specContent string, flags ...string) (bytes.Buffer, error)
	EditTuningConfig(clusterID string, tcName string, flags ...string) (bytes.Buffer, error)
	DeleteTuningConfig(clusterID string, tcName string) (bytes.Buffer, error)

	ListTuningConfigs(clusterID string) (bytes.Buffer, error)
	ReflectTuningConfigList(result bytes.Buffer) (mpl *TuningConfigList, err error)
	ListTuningConfigsAndReflect(clusterID string) (*TuningConfigList, error)

	DescribeTuningConfig(clusterID string, tcID string) (bytes.Buffer, error)
	ReflectTuningConfigDescription(result bytes.Buffer) (npd *TuningConfigDescription, err error)
	DescribeTuningConfigAndReflect(clusterID string, tcID string) (*TuningConfigDescription, error)
}

type tuningConfigService struct {
	ResourcesService

	tuningConfigs map[string][]string
}

func NewTuningConfigService(client *Client) TuningConfigService {
	return &tuningConfigService{
		ResourcesService: ResourcesService{
			client: client,
		},
		tuningConfigs: make(map[string][]string),
	}
}

type TuningConfig struct {
	ID   string `json:"ID,omitempty"`
	Name string `json:"NAME,omitempty"`
}

type TuningConfigList struct {
	TuningConfigs []TuningConfig `json:"TuningConfigs,omitempty"`
}

// Struct for the 'rosa describe cluster' output
type TuningConfigDescription struct {
	Name string `yaml:"Name,omitempty"`
	ID   string `yaml:"ID,omitempty"`
	Spec string `yaml:"Spec,omitempty"`
}

func (tcs *tuningConfigService) CreateTuningConfig(clusterID string, tcName string, specContent string, flags ...string) (output bytes.Buffer, err error) {
	specPath, err := CreateTempFileWithContent(specContent)
	defer os.Remove(specPath)
	if err != nil {
		return *bytes.NewBufferString(""), err
	}
	output, err = tcs.client.Runner.
		Cmd("create", "tuning-config").
		CmdFlags(append(flags, "-c", clusterID, "--name", tcName, "--spec-path", specPath)...).
		Run()
	if err == nil {
		tcs.tuningConfigs[clusterID] = append(tcs.tuningConfigs[clusterID], tcName)
	}
	return
}

func (tcs *tuningConfigService) EditTuningConfig(clusterID string, tcID string, flags ...string) (bytes.Buffer, error) {
	combflags := append([]string{"-c", clusterID}, flags...)
	return tcs.client.Runner.
		Cmd("edit", "tuning-configs", tcID).
		CmdFlags(combflags...).
		Run()
}

func (tcs *tuningConfigService) DeleteTuningConfig(clusterID string, tcName string) (output bytes.Buffer, err error) {
	output, err = tcs.client.Runner.
		Cmd("delete", "tuning-configs", tcName).
		CmdFlags("-c", clusterID, "-y").
		Run()
	if err == nil {
		tcs.tuningConfigs[clusterID] = RemoveFromStringSlice(tcs.tuningConfigs[clusterID], tcName)
	}
	return
}

func (tcs *tuningConfigService) ListTuningConfigs(clusterID string) (bytes.Buffer, error) {
	list := tcs.client.Runner.Cmd("list", "tuning-configs").CmdFlags("-c", clusterID)
	return list.Run()
}

func (tcs *tuningConfigService) ReflectTuningConfigList(result bytes.Buffer) (tcl *TuningConfigList, err error) {
	tcl = &TuningConfigList{}
	theMap := tcs.client.Parser.TableData.Input(result).Parse().Output()
	for _, tcItem := range theMap {
		tuningConfig := &TuningConfig{}
		err = MapStructure(tcItem, tuningConfig)
		if err != nil {
			return
		}
		tcl.TuningConfigs = append(tcl.TuningConfigs, *tuningConfig)
	}
	return
}

func (tcs *tuningConfigService) ListTuningConfigsAndReflect(clusterID string) (*TuningConfigList, error) {
	output, err := tcs.ListTuningConfigs(clusterID)
	if err != nil {
		return nil, err
	}
	return tcs.ReflectTuningConfigList(output)
}

func (tcs *tuningConfigService) DescribeTuningConfig(clusterID string, tcID string) (bytes.Buffer, error) {
	describe := tcs.client.Runner.
		Cmd("describe", "tuning-configs", tcID).
		CmdFlags("-c", clusterID)

	return describe.Run()
}

func (tcs *tuningConfigService) DescribeTuningConfigAndReflect(clusterID string, tcID string) (*TuningConfigDescription, error) {
	output, err := tcs.DescribeTuningConfig(clusterID, tcID)
	if err != nil {
		return nil, err
	}
	return tcs.ReflectTuningConfigDescription(output)
}

func (tcs *tuningConfigService) ReflectTuningConfigDescription(result bytes.Buffer) (res *TuningConfigDescription, err error) {
	var data []byte
	res = &TuningConfigDescription{}
	theMap, err := tcs.client.Parser.TextData.Input(result).Parse().YamlToMap()
	if err != nil {
		return
	}
	data, err = yaml.Marshal(&theMap)
	if err != nil {
		return
	}
	err = yaml.Unmarshal(data, res)
	return res, err
}

func (tcs *tuningConfigService) CleanResources(clusterID string) (errors []error) {
	var tcsToDel []string
	tcsToDel = append(tcsToDel, tcs.tuningConfigs[clusterID]...)
	for _, tcName := range tcsToDel {
		logger.Infof("Remove remaining tuningconfig '%s'", tcName)
		_, err := tcs.DeleteTuningConfig(clusterID, tcName)
		if err != nil {
			errors = append(errors, err)
		}
	}

	return
}
