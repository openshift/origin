package cmd

import (
	"io/ioutil"
	"reflect"
	"testing"

	"github.com/spf13/pflag"

	kcmd "k8s.io/kubernetes/pkg/kubectl/cmd"

	"github.com/openshift/origin/pkg/cmd/util/clientcmd"
)

// TestLogsFlagParity makes sure that our copied flags don't slip during rebases
func TestLogsFlagParity(t *testing.T) {
	kubeCmd := kcmd.NewCmdLogs(nil, ioutil.Discard)
	f := clientcmd.NewFactory(nil)
	originCmd := NewCmdLogs("oc", "logs", f, ioutil.Discard)

	kubeCmd.LocalFlags().VisitAll(func(kubeFlag *pflag.Flag) {
		originFlag := originCmd.LocalFlags().Lookup(kubeFlag.Name)
		if originFlag == nil {
			t.Errorf("missing %v flag", kubeFlag.Name)
			return
		}

		if !reflect.DeepEqual(originFlag, kubeFlag) {
			t.Errorf("flag %v %v does not match %v", kubeFlag.Name, kubeFlag, originFlag)
		}
	})
}
