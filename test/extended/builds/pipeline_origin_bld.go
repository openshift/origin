package builds

import (
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/validation"

	buildv1 "github.com/openshift/api/build/v1"
)

const (
	localClientPluginSnapshotImageStream = "jenkins-client-plugin-snapshot-test"
	localClientPluginSnapshotImage       = "openshift/" + localClientPluginSnapshotImageStream + ":latest"
	localSyncPluginSnapshotImageStream   = "jenkins-sync-plugin-snapshot-test"
	localSyncPluginSnapshotImage         = "openshift/" + localSyncPluginSnapshotImageStream + ":latest"
	clientLicenseText                    = "About OpenShift Client Jenkins Plugin"
	syncLicenseText                      = "About OpenShift Sync"
	clientPluginName                     = "openshift-client"
	syncPluginName                       = "openshift-sync"
	secretName                           = "secret-to-credential"
	secretCredentialSyncLabel            = "credential.sync.jenkins.openshift.io"
	envVarsPipelineGitRepoBuildConfig    = "test-build-app-pipeline"
)

// BuildConfigSelector returns a label Selector which can be used to find all
// builds for a BuildConfig.
func BuildConfigSelector(name string) labels.Selector {
	return labels.Set{buildv1.BuildConfigLabel: LabelValue(name)}.AsSelector()
}

// LabelValue returns a string to use as a value for the Build
// label in a pod. If the length of the string parameter exceeds
// the maximum label length, the value will be truncated.
func LabelValue(name string) string {
	if len(name) <= validation.DNS1123LabelMaxLength {
		return name
	}
	return name[:validation.DNS1123LabelMaxLength]
}
