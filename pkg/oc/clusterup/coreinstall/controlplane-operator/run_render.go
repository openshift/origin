package controlplaneoperator

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/openshift/origin/pkg/oc/clusterup/docker/openshift"
	"github.com/openshift/origin/pkg/oc/clusterup/docker/run"
	"github.com/openshift/origin/pkg/oc/clusterup/docker/util"
	"github.com/openshift/origin/pkg/oc/lib/errors"
)

type RenderConfig struct {
	OperatorImage string

	// AssetInputDir is the location with certificates and secrets used as input for the operator-render call.
	AssetInputDir string

	// AssetsOutputDir is the location where the operator will generate the static pod manifests, ready to be used with bootkube start.
	AssetsOutputDir string

	// ConfigOutputDir is the location where the operator will create the component config file.
	ConfigOutputDir string
	// ConfigFileName is the component config file name inside ConfigOutputDir.
	ConfigFileName string
	// ConfigOverrides is the location of an component config override file.
	ConfigOverrides string

	// ContainerBinds is location to additional container bind mounts for bootkube containers.
	ContainerBinds []string
}

// Start runs the operator render command.
// The assets like certs and keys are to be stored in AssetsInputDir.
// The assets produced by this commands are stored in AssetsOutputDir.
// The configuration yaml file is stored in ConfigOutputDir, named according to ConfigFileName, with
// default values overridden according to ConfigOverrides.
func (opt *RenderConfig) RunRender(component string, hyperShiftImage, hyperKubeImage string, dockerClient util.Interface, hostIP string) (string, error) {
	imageRunHelper := run.NewRunHelper(util.NewHelper(dockerClient)).New()

	renderCommand := []string{
		"render",
		"--asset-input-dir=/asset-input",
		"--asset-output-dir=/asset-output",
		fmt.Sprintf("--config-output-file=%s", filepath.Join("/config-output", opt.ConfigFileName)),
		fmt.Sprintf("--config-override-file=%s", filepath.Join("/config-input", filepath.Base(opt.ConfigOverrides))),
		fmt.Sprintf("--manifest-image=%s", hyperShiftImage),
		fmt.Sprintf("--manifest-config-host-path=%s", opt.ConfigOutputDir),
		fmt.Sprintf("--manifest-config-file-name=%s", opt.ConfigFileName),
	}

	binds := opt.ContainerBinds
	binds = append(binds, fmt.Sprintf("%s:/asset-input:z", opt.AssetInputDir))
	binds = append(binds, fmt.Sprintf("%s:/asset-output:z", opt.AssetsOutputDir))
	binds = append(binds, fmt.Sprintf("%s:/config-output:z", opt.ConfigOutputDir))
	binds = append(binds, fmt.Sprintf("%s:/config-input:z", filepath.Dir(opt.ConfigOverrides)))

	containerID, exitCode, err := imageRunHelper.Image(opt.OperatorImage).
		Name(component + openshift.OperatorRenderContainerNameSuffix).
		User(fmt.Sprintf("%d:%d", os.Getuid(), os.Getgid())).
		DiscardContainer().
		Bind(binds...).
		Entrypoint("cluster-kube-apiserver-operator").
		Command(renderCommand...).Run()

	if err != nil {
		return "", errors.NewError("operator render exited %d: %v", exitCode, err).WithCause(err)
	}

	return containerID, nil
}
