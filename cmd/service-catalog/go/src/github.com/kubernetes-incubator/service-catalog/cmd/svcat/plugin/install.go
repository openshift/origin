/*
Copyright 2018 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package plugin

import (
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"syscall"

	"github.com/kubernetes-incubator/service-catalog/cmd/svcat/command"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v2"
)

type installCmd struct {
	*command.Context
	path     string
	svcatCmd *cobra.Command
}

// NewInstallCmd builds a "svcat install plugin" command
func NewInstallCmd(cxt *command.Context) *cobra.Command {
	installCmd := &installCmd{Context: cxt}
	cmd := &cobra.Command{
		Use:   "plugin",
		Short: "Install as a kubectl plugin",
		Example: `
  svcat install plugin
  svcat install plugin --plugins-path /tmp/kube/plugins
`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return installCmd.run(cmd)
		},
	}
	cmd.Flags().StringVarP(&installCmd.path, "plugins-path", "p", "",
		"The installation path. Defaults to KUBECTL_PLUGINS_PATH, if defined, otherwise the plugins directory under the KUBECONFIG dir. In most cases, this is ~/.kube/plugins.")
	cxt.Viper.BindEnv("plugins-path", EnvPluginPath)

	return cmd
}

func (c *installCmd) run(cmd *cobra.Command) error {
	c.svcatCmd = cmd.Root()
	return c.install()
}

func (c *installCmd) install() error {
	installPath := c.getInstallPath()

	err := copyBinary(installPath)
	if err != nil {
		return err
	}

	manifest, err := c.generateManifest()
	if err != nil {
		return err
	}

	err = saveManifest(installPath, manifest)
	if err != nil {
		return err
	}

	fmt.Fprintf(c.Output, "Plugin has been installed to %s. Run kubectl plugin %s --help for help using the plugin.\n",
		installPath, Name)

	return nil
}

func (c *installCmd) getInstallPath() string {
	pluginDir := c.getPluginsDir()
	return filepath.Join(pluginDir, Name)
}

func (c *installCmd) getPluginsDir() string {
	if c.path != "" {
		return c.path
	}

	if kubeconfig := os.Getenv("KUBECONFIG"); kubeconfig != "" {
		kubeDir := filepath.Base(kubeconfig)
		return filepath.Join(kubeDir, "plugins")
	}

	home := getUserHomeDir()
	return filepath.Join(home, ".kube", "plugins")
}

func (c *installCmd) generateManifest() ([]byte, error) {
	m := &Manifest{}
	m.Load(c.svcatCmd)

	contents, err := yaml.Marshal(m)
	if err != nil {
		return nil, fmt.Errorf("could not marshall the generated manifest (%s)", err)
	}

	return contents, nil
}

func copyBinary(installPath string) error {
	err := os.MkdirAll(installPath, 0755)
	if err != nil {
		return fmt.Errorf("could not create installation directory %s (%s)", installPath, err)
	}

	srcBin, err := os.Executable()
	if err != nil {
		return fmt.Errorf("could not retrieve the path to the currently running program (%s)", err)
	}
	binName := Name + getFileExt()

	destBin := filepath.Join(installPath, binName)
	err = copyFile(srcBin, destBin)
	if err != nil {
		return fmt.Errorf("could not copy %s to %s (%s)", srcBin, destBin, err)
	}

	return nil
}

func copyFile(src, dest string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.OpenFile(dest, syscall.O_CREAT|syscall.O_WRONLY, 0755)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, in)
	if err != nil {
		return err
	}
	return out.Close()
}

func saveManifest(installPath string, manifest []byte) error {
	manifestPath := filepath.Join(installPath, "plugin.yaml")
	err := ioutil.WriteFile(manifestPath, []byte(manifest), 0644)
	if err != nil {
		return fmt.Errorf("could not write the plugin manifest to %s (%s)", manifestPath, err)
	}

	return nil
}
