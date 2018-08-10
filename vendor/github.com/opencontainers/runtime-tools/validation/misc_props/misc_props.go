package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"time"

	"github.com/mndrix/tap-go"
	rspecs "github.com/opencontainers/runtime-spec/specs-go"
	"github.com/opencontainers/runtime-tools/specerror"
	"github.com/opencontainers/runtime-tools/validation/util"
	uuid "github.com/satori/go.uuid"
)

func saveConfig(path string, v interface{}) error {
	data, err := json.Marshal(v)
	if err != nil {
		return err
	}

	return ioutil.WriteFile(path, data, 0644)
}

func main() {
	t := tap.New()
	t.Header(0)
	bundleDir, err := util.PrepareBundle()
	if err != nil {
		util.Fatal(err)
	}
	defer os.RemoveAll(bundleDir)
	configFile := filepath.Join(bundleDir, "config.json")

	type extendedSpec struct {
		rspecs.Spec
		Unknown string `json:"unknown,omitempty"`
	}

	containerID := uuid.NewV4().String()
	basicConfig, err := util.GetDefaultGenerator()
	if err != nil {
		util.Fatal(err)
	}
	basicConfig.SetProcessArgs([]string{"true"})
	annotationConfig := basicConfig
	annotationConfig.AddAnnotation(fmt.Sprintf("org.%s", containerID), "")
	invalidConfig := basicConfig
	invalidConfig.SetVersion("invalid")

	cases := []struct {
		eSpec       extendedSpec
		action      util.LifecycleAction
		errExpected bool
		err         error
	}{
		{extendedSpec{Spec: *annotationConfig.Spec()}, util.LifecycleActionCreate | util.LifecycleActionStart | util.LifecycleActionDelete, true, specerror.NewError(specerror.AnnotationsKeyIgnoreUnknown, fmt.Errorf("implementations that are reading/processing this configuration file MUST NOT generate an error if they encounter an unknown annotation key"), rspecs.Version)},
		{extendedSpec{Spec: *basicConfig.Spec(), Unknown: "unknown"}, util.LifecycleActionCreate | util.LifecycleActionStart | util.LifecycleActionDelete, true, specerror.NewError(specerror.ExtensibilityIgnoreUnknownProp, fmt.Errorf("runtimes that are reading or processing this configuration file MUST NOT generate an error if they encounter an unknown property"), rspecs.Version)},
		{extendedSpec{Spec: *invalidConfig.Spec()}, util.LifecycleActionCreate | util.LifecycleActionStart | util.LifecycleActionDelete, false, specerror.NewError(specerror.ValidValues, fmt.Errorf("runtimes that are reading or processing this configuration file MUST generate an error when invalid or unsupported values are encountered"), rspecs.Version)},
	}

	for _, c := range cases {
		config := util.LifecycleConfig{
			BundleDir: bundleDir,
			Actions:   c.action,
			PreCreate: func(r *util.Runtime) error {
				r.SetID(containerID)
				return saveConfig(configFile, c.eSpec)
			},
			PreDelete: func(r *util.Runtime) error {
				util.WaitingForStatus(*r, util.LifecycleStatusCreated|util.LifecycleStatusStopped, time.Second*10, time.Second*1)
				return nil
			},
		}
		err = util.RuntimeLifecycleValidate(config)
		util.SpecErrorOK(t, (err == nil) == c.errExpected, c.err, err)
	}

	t.AutoPlan()
}
