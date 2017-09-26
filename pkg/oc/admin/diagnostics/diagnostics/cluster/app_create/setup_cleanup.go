package app_create

import (
	"bytes"
	"fmt"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"

	"github.com/openshift/origin/pkg/cmd/server/bootstrappolicy"
	newproject "github.com/openshift/origin/pkg/oc/admin/project"
	appscmd "github.com/openshift/origin/pkg/oc/cli/deploymentconfigs"
)

const podGoneTimeout = 30 // seconds to wait for previous app pods to disappear

func (d *AppCreate) prepareForApp() bool {
	defer func() {
		d.result.PrepDuration = jsonDuration(time.Since(time.Time(d.result.BeginTime)))
	}()
	if !d.setupProject() {
		return false
	}

	// delete any pieces of the app left over from a previous run so they don't get in the way
	d.cleanupApp()
	// ensure that the previous app is gone before creating again
	if !d.waitForPodGone() {
		return false
	}

	return true
}

func (d *AppCreate) setupProject() bool {
	d.out.Info("DCluAC003", fmt.Sprintf("%s: Using project '%s' for diagnostic.", now(), d.project))
	if existing, err := d.KubeClient.Core().Namespaces().Get(d.project, metav1.GetOptions{}); existing != nil && err == nil {
		d.out.Debug("DCluAC004", fmt.Sprintf("%s: Project '%s' already exists.", now(), d.project))
		return true
	}

	buffer := bytes.Buffer{}
	projOpts := &newproject.NewProjectOptions{
		ProjectName:       d.project,
		DisplayName:       "AppCreate diagnostic",
		Description:       "AppCreate diagnostic",
		NodeSelector:      d.nodeSelector,
		ProjectClient:     d.ProjectClient,
		RoleBindingClient: d.RoleBindingClient,
		AdminRole:         bootstrappolicy.AdminRoleName,
		AdminUser:         "",
		Output:            &buffer,
	}
	if err := projOpts.Run(true); err != nil {
		d.out.Error("DCluAC005", err, fmt.Sprintf("%s: Creating project '%s' failed: \n%s\n%v", now(), d.project, buffer.String(), err))
		return false
	}

	return true
}

func (d *AppCreate) cleanup() {
	if !d.keepApp {
		d.cleanupApp()
	}
	if !d.keepProject {
		d.out.Debug("DCluAC041", fmt.Sprintf("%s: Deleting project '%s'.", now(), d.project))
		if err := d.KubeClient.Core().Namespaces().Delete(d.project, nil); err != nil {
			d.out.Warn("DCluAC042", err, fmt.Sprintf("%s: Deleting project '%s' failed: %v", now(), d.project, err))
		} else {
			return
		}
	}
}

// delete all the app components. Errors are listed in debug and ignored, as it is normal for these components
// not to exist and thus lead to an error on delete. If it turns out that other errors occur that we actually
// care about then this can be refined.
func (d *AppCreate) cleanupApp() {
	errs := []error{}
	d.out.Debug("DCluAC043", fmt.Sprintf("%s: Deleting components of app '%s' if present.", now(), d.appName))

	// reap the DC's deployments first
	if err := appscmd.NewDeploymentConfigReaper(d.AppsClient, d.KubeClient).Stop(d.project, d.appName, time.Duration(1)*time.Second, nil); err != nil {
		errs = append(errs, err)
	}

	// then delete the DC, service, and route
	if err := d.AppsClient.Apps().DeploymentConfigs(d.project).Delete(d.appName, nil); err != nil {
		errs = append(errs, err)
	}
	if err := d.KubeClient.Core().Services(d.project).Delete(d.appName, nil); err != nil {
		errs = append(errs, err)
	}
	if err := d.RouteClient.Route().Routes(d.project).Delete(d.appName, nil); err != nil {
		errs = append(errs, err)
	}

	if len(errs) > 0 {
		d.out.Debug("DCluAC044", fmt.Sprintf("%s: Deleting components of app '%s' failed: %v", now(), d.appName, errs))
	}
}

func (d *AppCreate) waitForPodGone() bool {
	d.out.Debug("DCluAC045", fmt.Sprintf("%s: Waiting to ensure any previous pod for '%s' is gone.", now(), d.appName))
	err := wait.PollImmediate(time.Second, time.Duration(podGoneTimeout)*time.Second, func() (bool, error) {
		pods, err := d.KubeClient.Core().Pods(d.project).List(metav1.ListOptions{LabelSelector: d.labelSelector})
		if err == nil && len(pods.Items) == 0 {
			return true, nil
		}
		return false, err
	})
	switch err {
	case nil:
		return true
	case wait.ErrWaitTimeout:
		d.out.Error("DCluAC046", err, fmt.Sprintf("%s: Previous app pod still present after %ds", now(), podGoneTimeout))
	default:
		d.out.Error("DCluAC047", err, fmt.Sprintf("%s: Error while checking for previous app pod:\n%v", now(), err))
	}
	return false
}
