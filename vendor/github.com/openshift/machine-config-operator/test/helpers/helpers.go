package helpers

import (
	"fmt"

	"github.com/clarketm/json"
	ign3types "github.com/coreos/ignition/v2/config/v3_4/types"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	utilrand "k8s.io/apimachinery/pkg/util/rand"

	mcfgv1 "github.com/openshift/machine-config-operator/pkg/apis/machineconfiguration.openshift.io/v1"
)

var (
	// MasterSelector returns a label selector for masters nodes
	MasterSelector = metav1.AddLabelToSelector(&metav1.LabelSelector{}, "node-role/master", "")
	// WorkerSelector returns a label selector for workers nodes
	WorkerSelector = metav1.AddLabelToSelector(&metav1.LabelSelector{}, "node-role/worker", "")
	// InfraSelector returns a label selector for infra nodes
	InfraSelector = metav1.AddLabelToSelector(&metav1.LabelSelector{}, "node-role/infra", "")
)

// StrToPtr returns a pointer to a string
func StrToPtr(s string) *string {
	return &s
}

// BoolToPtr returns a pointer to a bool
func BoolToPtr(b bool) *bool {
	return &b
}

// IntToPtr returns a pointer to an int
func IntToPtr(i int) *int {
	return &i
}

// NewMachineConfig returns a basic machine config with supplied labels, osurl & files added
func NewMachineConfig(name string, labels map[string]string, osurl string, files []ign3types.File) *mcfgv1.MachineConfig {
	return NewMachineConfigExtended(
		name,
		labels,
		nil,
		files,
		[]ign3types.Unit{},
		[]ign3types.SSHAuthorizedKey{},
		[]string{},
		false,
		[]string{},
		"",
		osurl,
	)
}

// NewMachineConfig returns a basic machine config with supplied labels, osurl & files added
func NewMachineConfigWithAnnotation(name string, labels, annotations map[string]string, osurl string, files []ign3types.File) *mcfgv1.MachineConfig {
	return NewMachineConfigExtended(
		name,
		labels,
		annotations,
		files,
		[]ign3types.Unit{},
		[]ign3types.SSHAuthorizedKey{},
		[]string{},
		false,
		[]string{},
		"",
		osurl,
	)
}

// NewMachineConfigExtended returns a more comprehensive machine config
func NewMachineConfigExtended(
	name string,
	labels map[string]string,
	annotations map[string]string,
	files []ign3types.File,
	units []ign3types.Unit,
	sshkeys []ign3types.SSHAuthorizedKey,
	extensions []string,
	fips bool,
	kernelArguments []string,
	kernelType, osurl string,
) *mcfgv1.MachineConfig {
	if labels == nil {
		labels = map[string]string{}
	}
	if annotations == nil {
		annotations = map[string]string{}
	}
	rawIgnition := MarshalOrDie(
		&ign3types.Config{
			Ignition: ign3types.Ignition{
				Version: ign3types.MaxVersion.String(),
			},
			Storage: ign3types.Storage{
				Files: files,
			},
			Systemd: ign3types.Systemd{
				Units: units,
			},
			Passwd: ign3types.Passwd{
				Users: []ign3types.PasswdUser{
					{
						Name:              "core",
						SSHAuthorizedKeys: sshkeys,
					},
				},
			},
		},
	)

	return &mcfgv1.MachineConfig{
		TypeMeta: metav1.TypeMeta{
			APIVersion: mcfgv1.SchemeGroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:        name,
			Labels:      labels,
			Annotations: annotations,
			UID:         types.UID(utilrand.String(5)),
		},
		Spec: mcfgv1.MachineConfigSpec{
			Config: runtime.RawExtension{
				Raw: rawIgnition,
			},
			Extensions:      extensions,
			FIPS:            fips,
			KernelArguments: kernelArguments,
			KernelType:      kernelType,
			OSImageURL:      osurl,
		},
	}
}

// NewMachineConfigPool returns a MCP with supplied mcSelector, nodeSelector and machineconfig
func NewMachineConfigPool(name string, mcSelector, nodeSelector *metav1.LabelSelector, currentMachineConfig string) *mcfgv1.MachineConfigPool {
	return &mcfgv1.MachineConfigPool{
		TypeMeta: metav1.TypeMeta{
			APIVersion: mcfgv1.SchemeGroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
			Labels: map[string]string{
				"machineconfiguration.openshift.io/mco-built-in":                         "",
				fmt.Sprintf("pools.operator.machineconfiguration.openshift.io/%s", name): "",
			},
			UID: types.UID(utilrand.String(5)),
		},
		Spec: mcfgv1.MachineConfigPoolSpec{
			NodeSelector:          nodeSelector,
			MachineConfigSelector: mcSelector,
			Configuration: mcfgv1.MachineConfigPoolStatusConfiguration{
				ObjectReference: corev1.ObjectReference{
					Name: currentMachineConfig,
				},
			},
		},
		Status: mcfgv1.MachineConfigPoolStatus{
			Configuration: mcfgv1.MachineConfigPoolStatusConfiguration{
				ObjectReference: corev1.ObjectReference{
					Name: currentMachineConfig,
				},
			},
			Conditions: []mcfgv1.MachineConfigPoolCondition{
				{
					Type:               mcfgv1.MachineConfigPoolRenderDegraded,
					Status:             corev1.ConditionFalse,
					LastTransitionTime: metav1.Unix(0, 0),
					Reason:             "",
					Message:            "",
				},
			},
		},
	}
}

// CreateMachineConfigFromIgnition returns a MachineConfig object from an Ignition config passed to it
func CreateMachineConfigFromIgnition(ignCfg interface{}) *mcfgv1.MachineConfig {
	return &mcfgv1.MachineConfig{
		Spec: mcfgv1.MachineConfigSpec{
			Config: runtime.RawExtension{
				Raw: MarshalOrDie(ignCfg),
			},
		},
	}
}

// MarshalOrDie returns a marshalled interface or panics
func MarshalOrDie(input interface{}) []byte {
	bytes, err := json.Marshal(input)
	if err != nil {
		panic(err)
	}
	return bytes
}
