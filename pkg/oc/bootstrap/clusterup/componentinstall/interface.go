package componentinstall

import (
	"fmt"
	"strings"
	"sync"

	utilerrors "k8s.io/apimachinery/pkg/util/errors"

	"github.com/golang/glog"
	"github.com/openshift/origin/pkg/oc/bootstrap/docker/dockerhelper"
)

type Component interface {
	Name() string
	Install(dockerClient dockerhelper.Interface, logdir string) error
}

func InstallComponents(components []Component, dockerClient dockerhelper.Interface, logdir string) error {
	componentNames := []string{}
	errorCh := make(chan error, len(components))
	waitGroupOne := sync.WaitGroup{}
	for i := range components {
		component := components[i]
		componentNames = append(componentNames, fmt.Sprintf("%q", component.Name()))
		glog.V(4).Infof("Installing %q...", component.Name())
		waitGroupOne.Add(1)

		go func() {
			defer waitGroupOne.Done()

			err := component.Install(dockerClient, logdir)
			if err != nil {
				glog.Errorf("Failed to install %q: %v", component.Name(), err)
				errorCh <- err
			}

		}()
	}
	waitGroupOne.Wait()
	glog.Infof("Finished installing %v", strings.Join(componentNames, " "))
	close(errorCh)

	errs := []error{}
	for err := range errorCh {
		errs = append(errs, err)
	}

	return utilerrors.NewAggregate(errs)
}
