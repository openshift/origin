package clientcmd

import (
	"fmt"
	"testing"

	"k8s.io/kubernetes/pkg/api"
	kcmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

func TestUpstreamFlagConsistency(t *testing.T) {
	osflags := pflag.NewFlagSet("test", pflag.ExitOnError)
	_ = DefaultClientConfig(osflags)
	kflags := pflag.NewFlagSet("test", pflag.ExitOnError)
	_ = kcmdutil.DefaultClientConfig(kflags)

	// ensure we are consistent with the flags for upstream
	kflags.VisitAll(func(kflag *pflag.Flag) {
		fmt.Println(kflag.Name)
		osflag := osflags.Lookup(kflag.Name)
		if osflag == nil {
			switch kflag.Name {
			case "kubeconfig", "username", "password":
				// we remove kubeconfig, username, password
			default:
				t.Errorf("%s present in Kubernetes default flags but not Origin", kflag.Name)
			}
			return
		}
		switch kflag.Name {
		case "certificate-authority", "client-certificate", "client-key":
			// we add annotations for certificate-authority, client-certificate, client-key
			delete(osflag.Annotations, cobra.BashCompFilenameExt)
		}
		if e, a := kflag.Annotations, osflag.Annotations; !api.Semantic.DeepEqual(e, a) {
			t.Errorf("%s: annotations: expected %v, got %v", kflag.Name, e, a)
		}
		if e, a := kflag.DefValue, osflag.DefValue; e != a {
			t.Errorf("%s: default value: expected %q, got %q", kflag.Name, e, a)
		}
		if e, a := kflag.Deprecated, osflag.Deprecated; e != a {
			t.Errorf("%s: deprecated: expected %q, got %q", kflag.Name, e, a)
		}
		if e, a := kflag.Hidden, osflag.Hidden; e != a {
			t.Errorf("%s: hidden: expected %t, got %t", kflag.Name, e, a)
		}
		if e, a := kflag.NoOptDefVal, osflag.NoOptDefVal; e != a {
			t.Errorf("%s: no opt default value: expected %q, got %q", kflag.Name, e, a)
		}
		// we override shorthand for namespace
		if kflag.Name == "namespace" {
			if e, a := "n", osflag.Shorthand; e != a {
				t.Errorf("%s: shorthand: expected %q, got %q", kflag.Name, e, a)
			}
		} else {
			if e, a := kflag.Shorthand, osflag.Shorthand; e != a {
				t.Errorf("%s: shorthand: expected %q, got %q", kflag.Name, e, a)
			}
		}
		if e, a := kflag.ShorthandDeprecated, osflag.ShorthandDeprecated; e != a {
			t.Errorf("%s: shorthand deprecated: expected %q, got %q", kflag.Name, e, a)
		}
		if e, a := kflag.Usage, osflag.Usage; e != a {
			t.Errorf("%s: usage: expected %q, got %q", kflag.Name, e, a)
		}
	})
}
