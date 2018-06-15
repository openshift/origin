package clientcmd

import "fmt"

// Deprecated
const OpenShiftKubeConfigFlagName = "config"

var UseOpenShiftKubeConfigValues = false

func NewErrConfigurationMissing() error {
	return fmt.Errorf(`Missing or incomplete configuration info.  Please login or point to an existing, complete config file:

  1. Via the command-line flag --config
  2. Via the KUBECONFIG environment variable
  3. In your home directory as ~/.kube/config

To view or setup config directly use the 'config' command.`)
}
