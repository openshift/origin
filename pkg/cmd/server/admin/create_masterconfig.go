package admin

import (
	"errors"
	"fmt"
	"github.com/spf13/cobra"
	"io"
	"io/ioutil"

	kcmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"

	configapi "github.com/openshift/origin/pkg/cmd/server/api"
	configapilatest "github.com/openshift/origin/pkg/cmd/server/api/latest"
	configutil "github.com/openshift/origin/pkg/cmd/server/util"
)

const MasterConfigCommandName = "create-master-config"

type CreateMasterConfigOptions struct {
	MasterArgs *configutil.MasterArgs

	ConfigFile string
	Output     io.Writer
}

func NewCommandMasterConfig(commandName string, fullName string, out io.Writer) *cobra.Command {
	options := &CreateMasterConfigOptions{Output: out}

	cmd := &cobra.Command{
		Use:   commandName,
		Short: "Create a configuration bundle for a master",
		Run: func(cmd *cobra.Command, args []string) {
			if err := options.Validate(args); err != nil {
				kcmdutil.CheckErr(kcmdutil.UsageError(cmd, err.Error()))
			}

			if err := options.CreateMasterFolder(); err != nil {
				kcmdutil.CheckErr(err)
			}
		},
	}
	options.MasterArgs = configutil.NewDefaultMasterArgs()

	flags := cmd.Flags()

	flags.StringVar(&options.ConfigFile, "master-config", "master-config.yaml", "Path for the master configuration file to create.")

	configutil.BindMasterArgs(options.MasterArgs, flags, "")
	configutil.BindListenArg(options.MasterArgs.ListenArg, flags, "")
	configutil.BindImageFormatArgs(options.MasterArgs.ImageFormatArgs, flags, "")
	configutil.BindKubeConnectionArgs(options.MasterArgs.KubeConnectionArgs, flags, "")
	configutil.BindNetworkArgs(options.MasterArgs.NetworkArgs, flags, "")

	return cmd
}

func (o CreateMasterConfigOptions) Validate(args []string) error {
	if len(args) != 0 {
		return errors.New("no arguments are supported")
	}
	return nil
}

func (o CreateMasterConfigOptions) CreateMasterFolder() error {
	if err := o.MakeMasterConfig(); err != nil {
		return err
	}
	return nil
}

func (o CreateMasterConfigOptions) MakeMasterConfig() error {
	var masterConfig *configapi.MasterConfig
	var err error

	masterConfig, err = o.MasterArgs.BuildSerializeableMasterConfig()
	if err != nil {
		return err
	}

	// Roundtrip the config to v1 and back to ensure proper defaults are set.
	ext, err := configapi.Scheme.ConvertToVersion(masterConfig, "v1")
	if err != nil {
		return err
	}
	internal, err := configapi.Scheme.ConvertToVersion(ext, "")
	if err != nil {
		return err
	}

	content, err := configapilatest.WriteYAML(internal)
	if err != nil {
		return err
	}

	if err := ioutil.WriteFile(o.ConfigFile, content, 0644); err != nil {
		return err
	}

	fmt.Fprintf(o.Output, "Wrote master config to: %s\n", o.ConfigFile)

	return nil
}
