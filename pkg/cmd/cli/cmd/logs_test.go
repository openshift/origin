package cmd

import (
	"io/ioutil"
	"reflect"
	"testing"

	"github.com/spf13/pflag"

	kcmd "k8s.io/kubernetes/pkg/kubectl/cmd"

	"github.com/openshift/origin/pkg/cmd/util/clientcmd"
)

// TestFlagParity makes sure that our copied flags don't slip during rebases
func TestFlagParity(t *testing.T) {
	kubeCmd := kcmd.NewCmdLog(nil, ioutil.Discard)
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
