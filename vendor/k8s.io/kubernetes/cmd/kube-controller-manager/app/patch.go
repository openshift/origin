package app

import (
	"os"
	"path"
	"time"

	"github.com/golang/glog"

	"k8s.io/client-go/informers"
	"k8s.io/klog"
	"k8s.io/kubernetes/cmd/kube-controller-manager/app/config"
	"k8s.io/kubernetes/cmd/kube-controller-manager/app/options"
	utilflag "k8s.io/kubernetes/pkg/util/flag"
)

var InformerFactoryOverride informers.SharedInformerFactory

func ShimForOpenShift(controllerManagerOptions *options.KubeControllerManagerOptions, controllerManager *config.Config, stopFn func()) error {
	if len(controllerManager.OpenShiftContext.OpenShiftConfig) == 0 {
		return nil
	}

	// TODO this gets removed when no longer take flags and no longer build a recycler template
	openshiftConfig, err := getOpenShiftConfig(controllerManager.OpenShiftContext.OpenShiftConfig)
	if err != nil {
		return err
	}

	// watch files which might be rotated outside of the process and suicide if they do
	watchFiles := []string{
		controllerManager.ComponentConfig.CSRSigningController.ClusterSigningCertFile,
		controllerManager.ComponentConfig.CSRSigningController.ClusterSigningKeyFile,
	}
	if err := WatchForChanges(time.Second*5, func(string) { stopFn() }, watchFiles...); err != nil {
		return err
	}

	// TODO this should be replaced by using a flex volume to inject service serving cert CAs into pods instead of adding it to the sa token
	if err := applyOpenShiftServiceServingCertCAFunc(path.Dir(controllerManager.OpenShiftContext.OpenShiftConfig), openshiftConfig); err != nil {
		return err
	}

	// skip GC on some openshift resources
	// TODO this should be replaced by discovery information in some way
	if err := applyOpenShiftGCConfig(controllerManager); err != nil {
		return err
	}

	// Overwrite the informers, because we have our custom generic informers for quota.
	// TODO update quota to create its own informer like garbage collection
	if informers, err := newInformerFactory(controllerManager.Kubeconfig); err != nil {
		return err
	} else {
		InformerFactoryOverride = informers
	}

	return nil
}

func ShimFlagsForOpenShift(controllerManagerOptions *options.KubeControllerManagerOptions) error {
	if len(controllerManagerOptions.OpenShiftContext.OpenShiftConfig) == 0 {
		return nil
	}

	// TODO this gets removed when no longer take flags and no longer build a recycler template
	openshiftConfig, err := getOpenShiftConfig(controllerManagerOptions.OpenShiftContext.OpenShiftConfig)
	if err != nil {
		return err
	}
	// apply the config based controller manager flags.  They will override.
	// TODO this should be replaced by the installer setting up the flags for us
	if err := applyOpenShiftConfigFlags(controllerManagerOptions, openshiftConfig); err != nil {
		return err
	}

	for name, fs := range controllerManagerOptions.Flags(KnownControllers(), ControllersDisabledByDefault.List()).FlagSets {
		glog.V(1).Infof("FLAGSET: %s", name)
		utilflag.PrintFlags(fs)
	}

	return nil
}

// WatchForChanges stats all files in the given interval and calls react if the time of the file changes.
func WatchForChanges(interval time.Duration, react func(file string), files ...string) error {
	lastModTime := map[string]time.Time{}
	for _, f := range files {
		if f == "" {
			continue
		}
		fi, err := os.Stat(f)
		if err != nil && os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return err
		}
		lastModTime[f] = fi.ModTime()
	}

	go func() {
		for _, f := range files {
			if f == "" {
				continue
			}

			fi, err := os.Stat(f)
			if err != nil && os.IsNotExist(err) {
				if _, ok := lastModTime[f]; ok {
					delete(lastModTime, f)
					react(f)
				}
				continue
			}
			if err != nil {
				klog.Warningf("cannot stat %q: %v", f, err)
				continue
			}
			mt := fi.ModTime()
			if last, ok := lastModTime[f]; !ok || !last.Equal(mt) {
				lastModTime[f] = mt
				react(f)
			}
		}

		time.Sleep(interval)
	}()

	return nil
}
