package framework

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"k8s.io/apimachinery/pkg/api/errors"

	"github.com/stretchr/testify/require"
	"sigs.k8s.io/controller-runtime/pkg/envtest"

	configv1 "github.com/openshift/api/config/v1"
	apioperatorsv1alpha1 "github.com/openshift/api/operator/v1alpha1"
	mcfgv1 "github.com/openshift/machine-config-operator/pkg/apis/machineconfiguration.openshift.io/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
)

const (
	OpenshiftConfigNamespace string = "openshift-config"

	// TODO: Figure out how to obtain this value programmatically so we don't
	// have to remember to increment it.
	k8sVersion string = "1.26.1"
)

// This is needed because both setup-envtest and the kubebuilder tools assume
// that HOME is set to a writable value. While running locally, this is
// generally a given, it is not in OpenShift CI. OpenShift CI usually defaults
// to "/" as the HOME value, which is not writable.
//
// TODO: Pre-fetch the kubebuilder binaries as part of the build-root process
// so we only have to fetch the kubebuilder tools once.
func overrideHomeDir(t *testing.T) (string, bool) {
	homeDir := os.Getenv("HOME")
	if homeDir != "" && homeDir != "/" {
		t.Logf("HOME env var set to %s, will use as-is", homeDir)
		return "", false
	}

	// For the purposes of this library, we will use the repo root
	// (/go/src/github.com/openshift/machine-config-operator). This is so that we
	// have a predictable HOME value which enables setup-envtest to reuse a
	// kubebuilder tool package across multiple test suites (assuming they run in
	// the same pod).
	overrideHomeDir, err := os.Getwd()
	require.NoError(t, err)

	if homeDir == "/" {
		t.Log("HOME env var set to '/', overriding with", overrideHomeDir)
		return overrideHomeDir, true
	}

	t.Log("HOME env var not set, overriding with", overrideHomeDir)
	return overrideHomeDir, true
}

// Instead of using a couple of ad-hoc shell scripts, envtest helpfully
// includes a setup utility (setup-envtest) that will retrieve the appropriate
// version of the kubebuilder toolchain for a given GOOS / GOARCH and K8s
// version. setup-envtest can also allow the toolchain to be prefetched and
// will cache it. This way, if multiple envtest targets are running in the same
// CI test pod, it will only fetch kubebuilder for the first suite.
func setupEnvTest(t *testing.T) (string, error) {
	setupEnvTestBinPath, err := exec.LookPath("setup-envtest")
	if err != nil {
		return "", fmt.Errorf("setup-envtest not installed, see installation instructions: https://github.com/kubernetes-sigs/controller-runtime/tree/master/tools/setup-envtest")
	}

	homeDir, overrideHomeDir := overrideHomeDir(t)

	if overrideHomeDir {
		os.Setenv("HOME", homeDir)
	}

	cmd := exec.Command(setupEnvTestBinPath, "use", k8sVersion, "-p", "path")
	t.Log("Setting up EnvTest: $", cmd)

	// We want to consume the path of where setup-envtest installed the
	// kubebuilder toolchain. So we capture stdout from setup-envtest (as well as
	// write it to os.Stdout for debugging purposes).
	pathBuffer := bytes.NewBuffer([]byte{})
	cmd.Stdout = io.MultiWriter(pathBuffer, os.Stdout)
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("could not fetch envtest archive: %w", err)
	}

	t.Log("setup-envtest complete!")

	return pathBuffer.String(), nil
}

func NewTestEnv(t *testing.T) *envtest.Environment {
	toolsPath, err := setupEnvTest(t)
	require.NoError(t, err)

	return &envtest.Environment{
		CRDInstallOptions: envtest.CRDInstallOptions{
			Paths: []string{
				filepath.Join("..", "..", "install"),
				filepath.Join("..", "..", "manifests", "controllerconfig.crd.yaml"),
				filepath.Join("..", "..", "vendor", "github.com", "openshift", "api", "config", "v1"),
				filepath.Join("..", "..", "vendor", "github.com", "openshift", "api", "operator", "v1alpha1"),
			},
			CleanUpAfterUse: true,
		},
		BinaryAssetsDirectory: toolsPath,
	}
}

// checkCleanEnvironment checks that all of the resource types that are to be used in this test currently have no items.
// This ensures that no atifacts from previous test runs are interfering with the current test.
func CheckCleanEnvironment(t *testing.T, clientSet *ClientSet) {
	t.Helper()

	ctx := context.Background()

	// ########################################
	// BEGIN: machineconfiguration.openshift.io
	// ########################################
	crcList, err := clientSet.ContainerRuntimeConfigs().List(ctx, metav1.ListOptions{})
	require.NoError(t, err)
	require.Len(t, crcList.Items, 0)

	ccList, err := clientSet.ControllerConfigs().List(ctx, metav1.ListOptions{})
	require.NoError(t, err)
	require.Len(t, ccList.Items, 0)

	kcList, err := clientSet.KubeletConfigs().List(ctx, metav1.ListOptions{})
	require.NoError(t, err)
	require.Len(t, kcList.Items, 0)

	mcpList, err := clientSet.MachineConfigPools().List(ctx, metav1.ListOptions{})
	require.NoError(t, err)
	require.Len(t, mcpList.Items, 0)

	mcList, err := clientSet.MachineConfigs().List(ctx, metav1.ListOptions{})
	require.NoError(t, err)
	require.Len(t, mcList.Items, 0)
	// ######################################
	// END: machineconfiguration.openshift.io
	// ######################################

	// #############
	// BEGIN: "core"
	// #############
	namespaceList, err := clientSet.Namespaces().List(ctx, metav1.ListOptions{})
	require.NoError(t, err)

	// Iterate through each namespace for namespace-scoped objects so we can
	// ensure they've been deleted.
	for _, namespace := range namespaceList.Items {
		namespaceName := namespace.GetName()

		secretList, err := clientSet.Secrets(namespaceName).List(ctx, metav1.ListOptions{})
		require.NoError(t, err)
		require.Len(t, secretList.Items, 0)

		podList, err := clientSet.Pods(namespaceName).List(ctx, metav1.ListOptions{})
		require.NoError(t, err)
		require.Len(t, podList.Items, 0)
	}

	nodeList, err := clientSet.ConfigV1Interface.Nodes().List(ctx, metav1.ListOptions{})
	require.NoError(t, err)
	require.Len(t, nodeList.Items, 0)

	// ###########
	// END: "core"
	// ###########

	// #####################################
	// BEGIN: operator.openshift.io/v1alpha1
	// #####################################
	icspList, err := clientSet.ImageContentSourcePolicies().List(ctx, metav1.ListOptions{})
	require.NoError(t, err)
	require.Len(t, icspList.Items, 0)
	// #####################################
	// END: operator.openshift.io/v1alpha1
	// #####################################

	// #############################
	// BEGIN: config.openshift.io/v1
	// #############################
	imagesList, err := clientSet.ConfigV1Interface.Images().List(ctx, metav1.ListOptions{})
	require.NoError(t, err)
	require.Len(t, imagesList.Items, 0)

	clusterVersionList, err := clientSet.ClusterVersions().List(ctx, metav1.ListOptions{})
	require.NoError(t, err)
	require.Len(t, clusterVersionList.Items, 0)

	featureGateList, err := clientSet.FeatureGates().List(ctx, metav1.ListOptions{})
	require.NoError(t, err)
	require.Len(t, featureGateList.Items, 0)

	nodeConfigList, err := clientSet.ConfigV1Interface.Nodes().List(ctx, metav1.ListOptions{})
	require.NoError(t, err)
	require.Len(t, nodeConfigList.Items, 0)
	// ###########################
	// END: config.openshift.io/v1
	// ###########################
}

// cleanEnvironment is called at the end of the test to ensure that all the resources that were created during the test
// are removed ahead of the next test starting.
func CleanEnvironment(t *testing.T, clientSet *ClientSet) {
	t.Helper()

	ctx := context.Background()

	// ########################################
	// BEGIN: machineconfiguration.openshift.io
	// ########################################
	err := clientSet.ContainerRuntimeConfigs().DeleteCollection(ctx, metav1.DeleteOptions{}, metav1.ListOptions{})
	require.NoError(t, err)

	err = clientSet.ControllerConfigs().DeleteCollection(ctx, metav1.DeleteOptions{}, metav1.ListOptions{})
	require.NoError(t, err)

	// KubeletConfigs must have their finalizers removed
	kcList, err := clientSet.KubeletConfigs().List(ctx, metav1.ListOptions{})
	require.NoError(t, err)
	for _, kc := range kcList.Items {
		if len(kc.Finalizers) > 0 {
			k := kc.DeepCopy()
			k.Finalizers = []string{}
			_, err := clientSet.KubeletConfigs().Update(ctx, k, metav1.UpdateOptions{})
			require.NoError(t, err)
		}
	}

	err = clientSet.KubeletConfigs().DeleteCollection(ctx, metav1.DeleteOptions{}, metav1.ListOptions{})
	require.NoError(t, err)

	err = clientSet.MachineConfigPools().DeleteCollection(ctx, metav1.DeleteOptions{}, metav1.ListOptions{})
	require.NoError(t, err)

	err = clientSet.MachineConfigs().DeleteCollection(ctx, metav1.DeleteOptions{}, metav1.ListOptions{})
	require.NoError(t, err)
	// ######################################
	// END: machineconfiguration.openshift.io
	// ######################################

	// #############
	// BEGIN: "core"
	// #############
	namespaceList, err := clientSet.Namespaces().List(ctx, metav1.ListOptions{})
	require.NoError(t, err)

	// Iterate through each namespace for namespace-scoped objects so we can
	// delete them from all known namespaces.
	for _, namespace := range namespaceList.Items {
		namespaceName := namespace.GetName()

		err = clientSet.Secrets(namespaceName).DeleteCollection(ctx, metav1.DeleteOptions{}, metav1.ListOptions{})
		require.NoError(t, err)

		err = clientSet.Pods(namespaceName).DeleteCollection(ctx, metav1.DeleteOptions{}, metav1.ListOptions{})
		require.NoError(t, err)
	}

	err = clientSet.ConfigV1Interface.Nodes().DeleteCollection(ctx, metav1.DeleteOptions{}, metav1.ListOptions{})
	require.NoError(t, err)
	// ###########
	// END: "core"
	// ###########

	// #####################################
	// BEGIN: operator.openshift.io/v1alpha1
	// #####################################
	err = clientSet.ImageContentSourcePolicies().DeleteCollection(ctx, metav1.DeleteOptions{}, metav1.ListOptions{})
	require.NoError(t, err)
	// #####################################
	// END: operator.openshift.io/v1alpha1
	// #####################################

	// #############################
	// BEGIN: config.openshift.io/v1
	// #############################
	err = clientSet.ConfigV1Interface.Images().DeleteCollection(ctx, metav1.DeleteOptions{}, metav1.ListOptions{})
	require.NoError(t, err)

	err = clientSet.ClusterVersions().DeleteCollection(ctx, metav1.DeleteOptions{}, metav1.ListOptions{})
	require.NoError(t, err)

	err = clientSet.FeatureGates().DeleteCollection(ctx, metav1.DeleteOptions{}, metav1.ListOptions{})
	require.NoError(t, err)

	err = clientSet.ConfigV1Interface.Nodes().DeleteCollection(ctx, metav1.DeleteOptions{}, metav1.ListOptions{})
	require.NoError(t, err)
	// ###########################
	// END: config.openshift.io/v1
	// ###########################
}

func CreateObjects(t *testing.T, clientSet *ClientSet, objs ...runtime.Object) {
	t.Helper()

	ctx := context.Background()

	for _, obj := range objs {
		switch tObj := obj.(type) {
		case *mcfgv1.MachineConfig:
			_, err := clientSet.MachineConfigs().Create(ctx, tObj, metav1.CreateOptions{})
			require.NoError(t, err)
		case *mcfgv1.MachineConfigPool:
			_, err := clientSet.MachineConfigPools().Create(ctx, tObj, metav1.CreateOptions{})
			require.NoError(t, err)
		case *mcfgv1.ControllerConfig:
			// Hack to get the pull secret working for the template controller
			o := tObj.DeepCopy()
			o.Spec.PullSecret = &corev1.ObjectReference{
				Name:      "pull-secret",
				Namespace: OpenshiftConfigNamespace,
			}

			_, err := clientSet.ControllerConfigs().Create(ctx, o, metav1.CreateOptions{})
			require.NoError(t, err)
		case *mcfgv1.ContainerRuntimeConfig:
			_, err := clientSet.ContainerRuntimeConfigs().Create(ctx, tObj, metav1.CreateOptions{})
			require.NoError(t, err)
		case *mcfgv1.KubeletConfig:
			_, err := clientSet.KubeletConfigs().Create(ctx, tObj, metav1.CreateOptions{})
			require.NoError(t, err)
		case *corev1.Secret:
			_, err := clientSet.Secrets(tObj.GetNamespace()).Create(ctx, tObj, metav1.CreateOptions{})
			require.NoError(t, err)
		case *corev1.Pod:
			_, err := clientSet.Pods(tObj.GetNamespace()).Create(ctx, tObj, metav1.CreateOptions{})
			require.NoError(t, err)
		case *corev1.Node:
			_, err := clientSet.CoreV1Interface.Nodes().Create(ctx, tObj, metav1.CreateOptions{})
			require.NoError(t, err)
		case *apioperatorsv1alpha1.ImageContentSourcePolicy:
			_, err := clientSet.ImageContentSourcePolicies().Create(ctx, tObj, metav1.CreateOptions{})
			require.NoError(t, err)
		case *configv1.Image:
			_, err := clientSet.ConfigV1Interface.Images().Create(ctx, tObj, metav1.CreateOptions{})
			require.NoError(t, err)
		case *configv1.FeatureGate:
			originalStatus := tObj.Status
			cObj, err := clientSet.FeatureGates().Create(ctx, tObj, metav1.CreateOptions{})
			if !apierrors.IsAlreadyExists(err) {
				require.NoError(t, err)
			} else {
				// If the test specificed a feature gate, override the existing one.
				cObj, err = clientSet.FeatureGates().Get(ctx, tObj.Name, metav1.GetOptions{})
				require.NoError(t, err)
				cObj.Spec = tObj.Spec
				cObj, err = clientSet.FeatureGates().Update(ctx, cObj, metav1.UpdateOptions{})
				require.NoError(t, err)
			}

			cObj.Status = originalStatus
			_, err = clientSet.FeatureGates().UpdateStatus(ctx, cObj, metav1.UpdateOptions{})
			require.NoError(t, err)
		case *configv1.Node:
			_, err := clientSet.ConfigV1Interface.Nodes().Get(ctx, "cluster", metav1.GetOptions{})
			if errors.IsNotFound(err) {
				_, err = clientSet.ConfigV1Interface.Nodes().Create(ctx, tObj, metav1.CreateOptions{})
			} else {
				clientSet.ConfigV1Interface.Nodes().Update(ctx, tObj, metav1.UpdateOptions{})
			}
			require.NoError(t, err)
		default:
			t.Errorf("Unknown object type %T", obj)
		}
	}
}
