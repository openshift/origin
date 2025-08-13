package util

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/ghodss/yaml"
	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	configv1client "github.com/openshift/client-go/config/clientset/versioned"
	authorizationapi "k8s.io/api/authorization/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/apitesting"
	kapierrs "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/uuid"
	"k8s.io/apimachinery/pkg/util/wait"
	quotav1 "k8s.io/apiserver/pkg/quota/v1"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/kubernetes"
	k8sclient "k8s.io/client-go/kubernetes"
	batchv1client "k8s.io/client-go/kubernetes/typed/batch/v1"
	corev1client "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/rest"
	e2e "k8s.io/kubernetes/test/e2e/framework"
	e2epod "k8s.io/kubernetes/test/e2e/framework/pod"
	e2eoutput "k8s.io/kubernetes/test/e2e/framework/pod/output"
	"k8s.io/kubernetes/test/e2e/framework/skipper"
	"k8s.io/kubernetes/test/e2e/framework/statefulset"
	"k8s.io/kubernetes/test/utils/image"

	buildv1 "github.com/openshift/api/build/v1"
	configv1 "github.com/openshift/api/config/v1"
	imagev1 "github.com/openshift/api/image/v1"
	operatorv1 "github.com/openshift/api/operator/v1"
	securityv1 "github.com/openshift/api/security/v1"
	buildv1clienttyped "github.com/openshift/client-go/build/clientset/versioned/typed/build/v1"
	clientconfigv1 "github.com/openshift/client-go/config/clientset/versioned"
	configclient "github.com/openshift/client-go/config/clientset/versioned/typed/config/v1"
	imagev1typedclient "github.com/openshift/client-go/image/clientset/versioned/typed/image/v1"
	"github.com/openshift/library-go/pkg/build/naming"
	"github.com/openshift/library-go/pkg/git"
	"github.com/openshift/library-go/pkg/image/imageutil"

	"github.com/openshift/origin/test/extended/testdata"
	utilimage "github.com/openshift/origin/test/extended/util/image"
)

// WaitForInternalRegistryHostname waits for the internal registry hostname to be made available to the cluster.
func WaitForInternalRegistryHostname(oc *CLI) (string, error) {
	ctx := context.Background()

	e2e.Logf("Waiting up to 2 minutes for the internal registry hostname to be published")
	var registryHostname string
	foundOCMLogs := false
	isOCMProgressing := true
	podLogs := map[string]string{}
	controlPlaneTopology, cpErr := GetControlPlaneTopology(oc)
	o.Expect(cpErr).NotTo(o.HaveOccurred())
	testImageStreamName := ""
	if *controlPlaneTopology == configv1.ExternalTopologyMode {
		is := &imagev1.ImageStream{}
		is.GenerateName = "internal-registry-test"
		is, err := oc.AdminImageClient().ImageV1().ImageStreams("openshift").Create(context.Background(), is, metav1.CreateOptions{})
		if err != nil {
			e2e.Logf("Error creating internal registry test imagestream: %v", err)
			return "", err
		}
		testImageStreamName = is.Name
		defer func() {
			err := oc.AdminImageClient().ImageV1().ImageStreams("openshift").Delete(context.Background(), is.Name, metav1.DeleteOptions{})
			if err != nil {
				e2e.Logf("Failed to cleanup internal-registry-test imagestream")
			}
		}()
	}
	err := wait.Poll(2*time.Second, 2*time.Minute, func() (bool, error) {
		imageConfig, err := oc.AsAdmin().AdminConfigClient().ConfigV1().Images().Get(ctx, "cluster", metav1.GetOptions{})
		if err != nil {
			if kapierrs.IsNotFound(err) {
				e2e.Logf("Image config object not found")
				return false, nil
			}
			e2e.Logf("Error accessing image config object: %#v", err)
			return false, err
		}
		if imageConfig == nil {
			e2e.Logf("Image config object nil")
			return false, nil
		}
		registryHostname = imageConfig.Status.InternalRegistryHostname
		if len(registryHostname) == 0 {
			e2e.Logf("Internal Registry Hostname is not set in image config object")
			return false, nil
		}

		if len(testImageStreamName) > 0 {
			is, err := oc.AdminImageClient().ImageV1().ImageStreams("openshift").Get(context.Background(), testImageStreamName, metav1.GetOptions{})
			if err != nil {
				e2e.Logf("Failed to fetch test imagestream openshift/%s: %v", testImageStreamName, err)
				return false, err
			}
			if len(is.Status.DockerImageRepository) == 0 {
				return false, nil
			}
			imgRef, err := imageutil.ParseDockerImageReference(is.Status.DockerImageRepository)
			if err != nil {
				e2e.Logf("Failed to parse dockerimage repository in test imagestream (%s): %v", is.Status.DockerImageRepository, err)
				return false, err
			}
			if imgRef.Registry != registryHostname {
				return false, nil
			}
			return true, nil
		}

		// verify that the OCM config's internal registry hostname matches
		// the image config's internal registry hostname
		ocm, err := oc.AdminOperatorClient().OperatorV1().OpenShiftControllerManagers().Get(ctx, "cluster", metav1.GetOptions{})
		if err != nil {
			if kapierrs.IsNotFound(err) {
				return false, nil
			}
			return false, err
		}
		observedConfig := map[string]interface{}{}
		err = json.Unmarshal(ocm.Spec.ObservedConfig.Raw, &observedConfig)
		if err != nil {
			return false, nil
		}
		internalRegistryHostnamePath := []string{"dockerPullSecret", "internalRegistryHostname"}
		currentRegistryHostname, _, err := unstructured.NestedString(observedConfig, internalRegistryHostnamePath...)
		if err != nil {
			e2e.Logf("error procesing observed config %#v", err)
			return false, nil
		}
		if currentRegistryHostname != registryHostname {
			e2e.Logf("OCM observed config hostname %s does not match image config hostname %s", currentRegistryHostname, registryHostname)
			return false, nil
		}
		// check pod logs for messages around image config's internal registry hostname has been observed and
		// and that the build controller was started after that observation
		pods, err := oc.AdminKubeClient().CoreV1().Pods("openshift-controller-manager").List(ctx, metav1.ListOptions{})
		if err != nil {
			if kapierrs.IsNotFound(err) {
				return false, nil
			}
			return false, err
		}
		for _, pod := range pods.Items {
			req := oc.AdminKubeClient().CoreV1().Pods("openshift-controller-manager").GetLogs(pod.Name, &corev1.PodLogOptions{})
			readCloser, err := req.Stream(ctx)
			if err == nil {
				b, err := ioutil.ReadAll(readCloser)
				if err == nil {
					podLog := string(b)
					podLogs[pod.Name] = podLog
					scanner := bufio.NewScanner(strings.NewReader(podLog))
					firstLog := false
					for scanner.Scan() {
						line := scanner.Text()
						if strings.Contains(line, "build_controller.go") && strings.Contains(line, "Starting build controller") {
							firstLog = true
							continue
						}
						if firstLog && strings.Contains(line, "build_controller.go") && strings.Contains(line, registryHostname) {
							e2e.Logf("the OCM pod logs indicate the build controller was started after the internal registry hostname has been set in the OCM config")
							foundOCMLogs = true
							break
						}
					}
				}
			} else {
				e2e.Logf("error getting pod logs: %#v", err)
			}
		}
		if !foundOCMLogs {
			e2e.Logf("did not find the sequence in the OCM pod logs around the build controller getting started after the internal registry hostname has been set in the OCM config")
			return false, nil
		}

		if !isOCMProgressing {
			return true, nil
		}
		// now cycle through the OCM operator conditions and make sure the Progressing condition is done
		for _, condition := range ocm.Status.Conditions {
			if condition.Type != operatorv1.OperatorStatusTypeProgressing {
				continue
			}
			if condition.Status != operatorv1.ConditionFalse {
				e2e.Logf("OCM rollout still progressing or in error: %v", condition.Status)
				return false, nil
			}
			e2e.Logf("OCM rollout progressing status reports complete")
			isOCMProgressing = true
			return true, nil
		}
		e2e.Logf("OCM operator progressing condition not present yet")
		return false, nil
	})

	if !foundOCMLogs && *controlPlaneTopology != configv1.ExternalTopologyMode {
		e2e.Logf("dumping OCM pod logs since we never found the internal registry hostname and start build controller sequence")
		for podName, podLog := range podLogs {
			e2e.Logf("pod %s logs:\n%s", podName, podLog)
		}
	}
	if err == wait.ErrWaitTimeout {
		return "", fmt.Errorf("Timed out waiting for Openshift Controller Manager to be rolled out with updated internal registry hostname")
	}
	if err != nil {
		return "", err
	}
	return registryHostname, nil
}

func processScanError(log string) error {
	e2e.Logf("%s", log)
	return fmt.Errorf("%s", log)
}

// getImageStreamObj returns the updated spec for imageStream object
func getImageStreamObj(imageStreamName, imageRef string) *imagev1.ImageStream {
	imageStream := &imagev1.ImageStream{
		ObjectMeta: metav1.ObjectMeta{Name: imageStreamName},
		Spec: imagev1.ImageStreamSpec{
			LookupPolicy:          imagev1.ImageLookupPolicy{Local: true},
			DockerImageRepository: imageRef,
			Tags: []imagev1.TagReference{{
				Name: "latest",
				ImportPolicy: imagev1.TagImportPolicy{
					ImportMode: imagev1.ImportModePreserveOriginal,
				},
			}},
		},
		Status: imagev1.ImageStreamStatus{
			DockerImageRepository: imageRef,
			Tags: []imagev1.NamedTagEventList{{
				Tag: "latest",
			}},
		},
	}
	return imageStream
}

// WaitForImageStreamImport creates & waits for custom ruby imageStream to be available in current namespace
// TODO: To eliminate the dependency on OpenShift Samples Operator in future,
// WaitForImageStreamImport should be a replacement of WaitForOpenShiftNamespaceImageStreams func
func WaitForImageStreamImport(oc *CLI) error {
	ctx := context.Background()
	var registryHostname string

	// TODO: Reference an image from registry.redhat.io
	images := map[string]string{
		"ruby": "registry.access.redhat.com/ubi8/ruby-33",
	}

	// Check to see if we have ImageRegistry enabled
	hasImageRegistry, err := IsCapabilityEnabled(oc, configv1.ClusterVersionCapabilityImageRegistry)
	if err != nil {
		return err
	}
	if hasImageRegistry {
		registryHostname, err = WaitForInternalRegistryHostname(oc)
		if err != nil {
			return err
		}
	}

	// Create custom imageStream using `oc import-image`
	e2e.Logf("waiting for imagestreams to be imported")
	for imageStreamName, imageRef := range images {
		err := CustomImageStream(oc, getImageStreamObj(imageStreamName, imageRef))
		if err != nil {
			e2e.Logf("failed while creating custom imageStream")
			return err
		}

		// Wait for imageRegistry to be ready
		pollErr := wait.PollUntilContextTimeout(ctx, 10*time.Second, 150*time.Second, false, func(context.Context) (bool, error) {
			return checkNamespaceImageStreamImported(ctx, oc, imageStreamName, registryHostname, oc.Namespace())
		})
		// pollErr will be not nil if there was an immediate error, or we timed out.
		if pollErr == nil {
			return nil
		}
		DumpImageStream(oc, oc.Namespace(), imageStreamName)
		return pollErr
	}
	return nil
}

// WaitForOpenShiftNamespaceImageStreams waits for the standard set of imagestreams to be imported
func WaitForOpenShiftNamespaceImageStreams(oc *CLI) error {
	ctx := context.Background()
	images := []string{"nodejs", "perl", "php", "python", "mysql", "postgresql", "jenkins"}

	hasSamplesOperator, err := IsCapabilityEnabled(oc, configv1.ClusterVersionCapabilityOpenShiftSamples)
	if err != nil {
		return err
	}
	// Check to see if we have ImageRegistry and SamplesOperator enabled
	hasImageRegistry, err := IsCapabilityEnabled(oc, configv1.ClusterVersionCapabilityImageRegistry)
	if err != nil {
		return err
	}

	// Check to see if SamplesOperator managementState is Removed
	out, err := oc.AsAdmin().Run("get").Args("configs.samples.operator.openshift.io", "cluster", "-o", "yaml").Output()

	if err != nil {
		e2e.Logf("\n  error on getting samples operator CR: %+v\n%#v\n", err, out)
	}

	if !hasSamplesOperator || strings.Contains(out, "managementState: Removed") {
		images = []string{"cli", "tools", "tests", "installer"}
	}

	e2e.Logf("waiting for image ecoystem imagestreams to be imported")
	for _, image := range images {
		err := WaitForSamplesImagestream(ctx, oc, image, hasImageRegistry, hasSamplesOperator)
		if err != nil {
			DumpSampleOperator(oc)
			return err
		}
	}
	return nil
}

// WaitForSamplesImagestream waits for an imagestream imported by the samples operator to be imported.
// If the imagestream is managed by the samples operator and has failed to import on install, this
// will retry the import. Note that imagestreams which reference images in the OCP payload are not
// managed by the samples operator, and therefore will not be retried.
//
// This will wait up to 150 seconds for the referenced imagestream to finish importing.
func WaitForSamplesImagestream(ctx context.Context, oc *CLI, imagestream string, imageRegistryEnabled, openshiftSamplesEnabled bool) error {
	var registryHostname string
	var err error

	if imageRegistryEnabled {
		registryHostname, err = WaitForInternalRegistryHostname(oc)
		if err != nil {
			return err
		}
	}

	var retried bool

	// Wait up to 150 seconds for an imagestream to import.
	// Based on a sampling of CI tests, imagestream imports from registry.redhat.io can take up to 2 minutes to complete.
	// Imports which take longer generally indicate that there is a performance regression or outage in the container registry.
	pollErr := wait.Poll(10*time.Second, 150*time.Second, func() (bool, error) {
		if openshiftSamplesEnabled {
			retried, err = retrySamplesImagestreamImportIfNeeded(ctx, oc, imagestream)
			if err != nil {
				return false, err
			}
			if retried {
				return false, nil
			}
		}
		return checkNamespaceImageStreamImported(ctx, oc, imagestream, registryHostname, "openshift")
	})
	// pollErr will be not nil if there was an immediate error, or we timed out.
	if pollErr == nil {
		return nil
	}
	DumpImageStream(oc, "openshift", imagestream)
	// If retried=true at this point, it means that we have repeatedly tried to reimport the imagestream and failed to do so.
	// This could be an indicator that the Red Hat Container Registry (registry.redhat.io) is experiencing an outage, since most samples operator imagestream images are hosted there.
	if retried {
		strbuf := bytes.Buffer{}
		strbuf.WriteString("Failed immagestream imports may indicate an issue with the Red Hat Container Registry (registry.redhat.io).\n")
		strbuf.WriteString(" - check status at https://status.redhat.com (catalog.redhat.com) for reported outages\n")
		strbuf.WriteString(" - if no outages are reported there, email Terms-Based-Registry-Team@redhat.com with a report of the error\n")
		strbuf.WriteString("   and prepare to work with the test platform team to get the current set of tokens for CI\n")
		e2e.Logf("%s", strbuf.String())
	}
	return pollErr
}

// CustomImageStream uses the provided imageStreamObj reference to create an imagestream with the given name in the given namespace.
func CustomImageStream(oc *CLI, imageStream *imagev1.ImageStream) error {
	_, err := oc.ImageClient().ImageV1().ImageStreams(oc.Namespace()).Create(context.Background(), imageStream, metav1.CreateOptions{})
	return err
}

// retrySamplesImagestreamImportIfNeeded immediately retries an import for the provided imagestream if:
//
// 1) The imagestream is managed by the samples operator, AND
// 2) The imagestream has failed to import.
//
// This allows the imagestream to be reimported at a faster cadence than what the samples operator currently provides.
// Imagestreams which use images in the OCP payload are not managed by the samples operator and therefore will not be retried.
//
// Returns true if the imagestream import was retried.
func retrySamplesImagestreamImportIfNeeded(ctx context.Context, oc *CLI, imagestream string) (bool, error) {
	// check the samples operator to see about imagestream import status
	samplesOperatorConfig, err := oc.AdminConfigClient().ConfigV1().ClusterOperators().Get(ctx, "openshift-samples", metav1.GetOptions{})
	if err != nil {
		return false, processScanError(fmt.Sprintf("failed to get clusteroperator for samples-operator: %v", err))
	}
	for _, condition := range samplesOperatorConfig.Status.Conditions {
		switch {
		case condition.Type == configv1.OperatorDegraded && condition.Status == configv1.ConditionTrue:
			// if degraded, bail ... unexpected results can ensue
			return false, processScanError(fmt.Sprintf("samples-operator is degraded with reason: %s", condition.Reason))
		case condition.Type == configv1.OperatorProgressing:
			// if the imagestreams for one of our langs above failed, we abort,
			// but if it is for say only EAP streams, we allow
			if condition.Reason == "FailedImageImports" {
				msg := condition.Message
				if strings.Contains(msg, " "+imagestream+" ") || strings.HasSuffix(msg, " "+imagestream) {
					e2e.Logf("samples-operator detected error during imagestream import: %s with message %q", condition.Reason, condition.Message)
					stream, err := oc.AsAdmin().ImageClient().ImageV1().ImageStreams("openshift").Get(ctx, imagestream, metav1.GetOptions{})
					if err != nil {
						return false, processScanError(fmt.Sprintf("failed to get imagestream %s/%s: %v", "openshift", imagestream, err))
					}
					e2e.Logf("manually retrying import for imagestream %s/%s to expedite testing", "openshift", imagestream)
					isi := &imagev1.ImageStreamImport{}
					isi.Name = imagestream
					isi.Namespace = "openshift"
					isi.ResourceVersion = stream.ResourceVersion
					isi.Spec = imagev1.ImageStreamImportSpec{
						Import: true,
						Images: []imagev1.ImageImportSpec{},
					}
					for _, tag := range stream.Spec.Tags {
						if tag.From != nil && tag.From.Kind == "DockerImage" {
							iis := imagev1.ImageImportSpec{}
							iis.From = *tag.From
							iis.To = &corev1.LocalObjectReference{Name: tag.Name}
							isi.Spec.Images = append(isi.Spec.Images, iis)
						}
					}
					_, err = oc.AsAdmin().ImageClient().ImageV1().ImageStreamImports("openshift").Create(ctx, isi, metav1.CreateOptions{})
					if err != nil {
						return false, processScanError(fmt.Sprintf("failed to create imagestream import %s/%s: %v", "openshift", imagestream, err))
					}
					return true, nil
				}
			}
			if condition.Status == configv1.ConditionTrue {
				// updates still in progress ... not "ready"
				e2e.Logf("samples-operator is still progressing without failed imagestream imports.")
			}
		case condition.Type == configv1.OperatorAvailable && condition.Status == configv1.ConditionFalse:
			e2e.Logf("samples-operator is not available")
		}
	}
	return false, nil
}

// checkNamespaceImageStreamImported checks if the provided imagestream has been imported into the specified namespace.
// Returns true if status has been reported on all tags for the imagestream.
func checkNamespaceImageStreamImported(ctx context.Context, oc *CLI, imagestream, registryHostname, namespace string) (bool, error) {
	e2e.Logf("checking imagestream %s/%s", namespace, imagestream)
	is, err := oc.ImageClient().ImageV1().ImageStreams(namespace).Get(ctx, imagestream, metav1.GetOptions{})
	if err != nil {
		return false, processScanError(fmt.Sprintf("failed to get imagestream: %v", err))
	}
	if !strings.HasPrefix(is.Status.DockerImageRepository, registryHostname) {
		e2e.Logf("imagestream repository %s does not match expected host %s", is.Status.DockerImageRepository, registryHostname)
		return false, nil
	}
	for _, tag := range is.Spec.Tags {
		e2e.Logf("checking tag %s for imagestream %s/%s", tag.Name, namespace, imagestream)
		if _, found := imageutil.StatusHasTag(is, tag.Name); !found {
			e2e.Logf("no status for imagestreamtag %s/%s:%s", namespace, imagestream, tag.Name)
			return false, nil
		}
	}
	return true, nil
}

func DumpImageStream(oc *CLI, namespace string, imagestream string) error {
	out, err := oc.AsAdmin().Run("get").Args("is", imagestream, "-n", namespace, "-o", "yaml").Output()
	if err != nil {
		return fmt.Errorf("failed to get imagestream %s/%s: %v", namespace, imagestream, err)
	}
	e2e.Logf("imagestream %s/%s:\n%s\n", namespace, imagestream, out)
	return nil
}

// DumpImageStreams will dump both the openshift namespace and local namespace imagestreams
// as part of debugging when the language imagestreams in the openshift namespace seem to disappear
func DumpImageStreams(oc *CLI) {
	out, err := oc.AsAdmin().Run("get").Args("is", "-n", "openshift", "-o", "yaml").Output()
	if err == nil {
		e2e.Logf("\n  imagestreams in openshift namespace: \n%s\n", out)
	} else {
		e2e.Logf("\n  error on getting imagestreams in openshift namespace: %+v\n%#v\n", err, out)
	}
	out, err = oc.AsAdmin().Run("get").Args("is", "-o", "yaml").Output()
	if err == nil {
		e2e.Logf("\n  imagestreams in dynamic test namespace: \n%s\n", out)
	} else {
		e2e.Logf("\n  error on getting imagestreams in dynamic test namespace: %+v\n%#v\n", err, out)
	}
	ids, err := ListImages()
	if err != nil {
		e2e.Logf("\n  got error on container images %+v\n", err)
	} else {
		for _, id := range ids {
			e2e.Logf(" found local image %s\n", id)
		}
	}
}

func DumpSampleOperator(oc *CLI) {
	out, err := oc.AsAdmin().Run("get").Args("configs.samples.operator.openshift.io", "cluster", "-o", "yaml").Output()
	if err == nil {
		e2e.Logf("\n  samples operator CR: \n%s\n", out)
	} else {
		e2e.Logf("\n  error on getting samples operator CR: %+v\n%#v\n", err, out)
	}
	DumpPodLogsStartingWithInNamespace("cluster-samples-operator", "openshift-cluster-samples-operator", oc)
}

// DumpBuildLogs will dump the latest build logs for a BuildConfig for debug purposes
func DumpBuildLogs(bc string, oc *CLI) {
	buildOutput, err := oc.AsAdmin().Run("logs").Args("-f", "bc/"+bc, "--timestamps").Output()
	if err == nil {
		e2e.Logf("\n\n  build logs : %s\n\n", buildOutput)
	} else {
		e2e.Logf("\n\n  got error on build logs %+v\n\n", err)
	}

	// if we suspect that we are filling up the registry file system, call ExamineDiskUsage / ExaminePodDiskUsage
	// also see if manipulations of the quota around /mnt/openshift-xfs-vol-dir exist in the extended test set up scripts
	ExamineDiskUsage()
	ExaminePodDiskUsage(oc)
}

// DumpBuilds will dump the yaml for every build in the test namespace; remember, pipeline builds
// don't have build pods so a generic framework dump won't cat our pipeline builds objs in openshift
func DumpBuilds(oc *CLI) {
	buildOutput, err := oc.AsAdmin().Run("get").Args("builds", "-o", "yaml").Output()
	if err == nil {
		e2e.Logf("\n\n builds yaml:\n%s\n\n", buildOutput)
	} else {
		e2e.Logf("\n\n got error on build yaml dump: %#v\n\n", err)
	}
}

// DumpBuildConfigs will dump the yaml for every buildconfig in the test namespace
func DumpBuildConfigs(oc *CLI) {
	buildOutput, err := oc.AsAdmin().Run("get").Args("buildconfigs", "-o", "yaml").Output()
	if err == nil {
		e2e.Logf("\n\n buildconfigs yaml:\n%s\n\n", buildOutput)
	} else {
		e2e.Logf("\n\n got error on buildconfig yaml dump: %#v\n\n", err)
	}
}

func GetStatefulSetPods(oc *CLI, setName string) (*corev1.PodList, error) {
	return oc.AdminKubeClient().CoreV1().Pods(oc.Namespace()).List(context.Background(), metav1.ListOptions{LabelSelector: ParseLabelsOrDie(fmt.Sprintf("name=%s", setName)).String()})
}

// DumpPodStates dumps the state of all pods in the CLI's current namespace.
func DumpPodStates(oc *CLI) {
	e2e.Logf("Dumping pod state for namespace %s", oc.Namespace())
	out, err := oc.AsAdmin().Run("get").Args("pods", "-o", "yaml").Output()
	if err != nil {
		e2e.Logf("Error dumping pod states: %v", err)
		return
	}
	e2e.Logf("%s", out)
}

// DumpPodStatesInNamespace dumps the state of all pods in the provided namespace.
func DumpPodStatesInNamespace(namespace string, oc *CLI) {
	e2e.Logf("Dumping pod state for namespace %s", namespace)
	out, err := oc.AsAdmin().Run("get").Args("pods", "-n", namespace, "-o", "yaml").Output()
	if err != nil {
		e2e.Logf("Error dumping pod states: %v", err)
		return
	}
	e2e.Logf("%s", out)
}

// DumpPodLogsStartingWith will dump any pod starting with the name prefix provided
func DumpPodLogsStartingWith(prefix string, oc *CLI) {
	podsToDump := []corev1.Pod{}
	podList, err := oc.AdminKubeClient().CoreV1().Pods(oc.Namespace()).List(context.Background(), metav1.ListOptions{})
	if err != nil {
		e2e.Logf("Error listing pods: %v", err)
		return
	}
	for _, pod := range podList.Items {
		if strings.HasPrefix(pod.Name, prefix) {
			podsToDump = append(podsToDump, pod)
		}
	}
	if len(podsToDump) > 0 {
		DumpPodLogs(podsToDump, oc)
	}
}

// DumpPodLogsStartingWith will dump any pod starting with the name prefix provided
func DumpPodLogsStartingWithInNamespace(prefix, namespace string, oc *CLI) {
	podsToDump := []corev1.Pod{}
	podList, err := oc.AdminKubeClient().CoreV1().Pods(namespace).List(context.Background(), metav1.ListOptions{})
	if err != nil {
		e2e.Logf("Error listing pods: %v", err)
		return
	}
	for _, pod := range podList.Items {
		if strings.HasPrefix(pod.Name, prefix) {
			podsToDump = append(podsToDump, pod)
		}
	}
	if len(podsToDump) > 0 {
		DumpPodLogs(podsToDump, oc)
	}
}

func DumpPodLogs(pods []corev1.Pod, oc *CLI) {
	for _, pod := range pods {
		descOutput, err := oc.AsAdmin().Run("describe").WithoutNamespace().Args("pod/"+pod.Name, "-n", pod.Namespace).Output()
		if err == nil {
			if strings.Contains(descOutput, "BEGIN PRIVATE KEY") {
				// replace private key with XXXXX string
				re := regexp.MustCompile(`BEGIN\s+PRIVATE\s+KEY[\s\S]*END\s+PRIVATE\s+KEY`)
				descOutput = re.ReplaceAllString(descOutput, "XXXXXXXXXXXXXX")
			}
			e2e.Logf("Describing pod %q\n%s\n\n", pod.Name, descOutput)
		} else {
			e2e.Logf("Error retrieving description for pod %q: %v\n\n", pod.Name, err)
		}

		dumpContainer := func(container *corev1.Container) {
			depOutput, err := oc.AsAdmin().Run("logs").WithoutNamespace().Args("pod/"+pod.Name, "-c", container.Name, "-n", pod.Namespace).Output()
			if err == nil {
				e2e.Logf("Log for pod %q/%q\n---->\n%s\n<----end of log for %[1]q/%[2]q\n", pod.Name, container.Name, depOutput)
			} else {
				e2e.Logf("Error retrieving logs for pod %q/%q: %v\n\n", pod.Name, container.Name, err)
			}
		}

		for _, c := range pod.Spec.InitContainers {
			dumpContainer(&c)
		}
		for _, c := range pod.Spec.Containers {
			dumpContainer(&c)
		}
	}
}

// DumpPodsCommand runs the provided command in every pod identified by selector in the provided namespace.
func DumpPodsCommand(c kubernetes.Interface, ns string, selector labels.Selector, cmd string) {
	podList, err := c.CoreV1().Pods(ns).List(context.Background(), metav1.ListOptions{LabelSelector: selector.String()})
	o.Expect(err).NotTo(o.HaveOccurred())

	values := make(map[string]string)
	for _, pod := range podList.Items {
		stdout, err := e2eoutput.RunHostCmdWithRetries(pod.Namespace, pod.Name, cmd, statefulset.StatefulSetPoll, statefulset.StatefulPodTimeout)
		o.Expect(err).NotTo(o.HaveOccurred())
		values[pod.Name] = stdout
	}
	for name, stdout := range values {
		stdout = strings.TrimSuffix(stdout, "\n")
		e2e.Logf("%s: %s", name, strings.Join(strings.Split(stdout, "\n"), fmt.Sprintf("\n%s: ", name)))
	}
}

// DumpConfigMapStates dumps the state of all ConfigMaps in the CLI's current namespace.
func DumpConfigMapStates(oc *CLI) {
	e2e.Logf("Dumping configMap state for namespace %s", oc.Namespace())
	out, err := oc.AsAdmin().Run("get").Args("configmaps", "-o", "yaml").Output()
	if err != nil {
		e2e.Logf("Error dumping configMap states: %v", err)
		return
	}
	e2e.Logf("%s", out)
}

// GetMasterThreadDump will get a golang thread stack dump
func GetMasterThreadDump(oc *CLI) {
	out, err := oc.AsAdmin().Run("get").Args("--raw", "/debug/pprof/goroutine?debug=2").Output()
	if err == nil {
		e2e.Logf("\n\n Master thread stack dump:\n\n%s\n\n", string(out))
		return
	}
	e2e.Logf("\n\n got error on oc get --raw /debug/pprof/goroutine?godebug=2: %v\n\n", err)
}

func PreTestDump() {
	// dump any state we want to know prior to running tests
}

// ExamineDiskUsage will dump df output on the testing system; leveraging this as part of diagnosing
// the registry's disk filling up during external tests on jenkins
func ExamineDiskUsage() {
	// disabling this for now, easier to do it here than everywhere that's calling it.
	return
	/*
				out, err := exec.Command("/bin/df", "-m").Output()
				if err == nil {
					e2e.Logf("\n\n df -m output: %s\n\n", string(out))
				} else {
					e2e.Logf("\n\n got error on df %v\n\n", err)
				}
		                DumpDockerInfo()
	*/
}

// ExaminePodDiskUsage will dump df/du output on registry pod; leveraging this as part of diagnosing
// the registry's disk filling up during external tests on jenkins
func ExaminePodDiskUsage(oc *CLI) {
	// disabling this for now, easier to do it here than everywhere that's calling it.
	return
	/*
		out, err := oc.Run("get").Args("pods", "-o", "json", "-n", "default").Output()
		var podName string
		if err == nil {
			b := []byte(out)
			var list corev1.PodList
			err = json.Unmarshal(b, &list)
			if err == nil {
				for _, pod := range list.Items {
					e2e.Logf("\n\n looking at pod %s \n\n", pod.ObjectMeta.Name)
					if strings.Contains(pod.ObjectMeta.Name, "docker-registry-") && !strings.Contains(pod.ObjectMeta.Name, "deploy") {
						podName = pod.ObjectMeta.Name
						break
					}
				}
			} else {
				e2e.Logf("\n\n got json unmarshal err: %v\n\n", err)
			}
		} else {
			e2e.Logf("\n\n  got error on get pods: %v\n\n", err)
		}
		if len(podName) == 0 {
			e2e.Logf("Unable to determine registry pod name, so we can't examine its disk usage.")
			return
		}

		out, err = oc.Run("exec").Args("-n", "default", podName, "df").Output()
		if err == nil {
			e2e.Logf("\n\n df from registry pod: \n%s\n\n", out)
		} else {
			e2e.Logf("\n\n got error on reg pod df: %v\n", err)
		}
		out, err = oc.Run("exec").Args("-n", "default", podName, "du", "/registry").Output()
		if err == nil {
			e2e.Logf("\n\n du from registry pod: \n%s\n\n", out)
		} else {
			e2e.Logf("\n\n got error on reg pod du: %v\n", err)
		}
	*/
}

// VarSubOnFile reads in srcFile, finds instances of ${key} from the map
// and replaces them with their associated values.
func VarSubOnFile(srcFile string, destFile string, vars map[string]string) error {
	srcData, err := ioutil.ReadFile(srcFile)
	if err == nil {
		srcString := string(srcData)
		for k, v := range vars {
			k = "${" + k + "}"
			srcString = strings.Replace(srcString, k, v, -1) // -1 means unlimited replacements
		}
		err = ioutil.WriteFile(destFile, []byte(srcString), 0644)
	}
	return err
}

// StartBuild executes OC start-build with the specified arguments. StdOut and StdErr from the process
// are returned as separate strings.
func StartBuild(oc *CLI, args ...string) (stdout, stderr string, err error) {
	stdout, stderr, err = oc.Run("start-build").Args(args...).Outputs()
	e2e.Logf("\n\nstart-build output with args %v:\nError>%v\nStdOut>\n%s\nStdErr>\n%s\n\n", args, err, stdout, stderr)
	return stdout, stderr, err
}

var buildPathPattern = regexp.MustCompile(`^build\.build\.openshift\.io/([\w\-\._]+)$`)

type LogDumperFunc func(oc *CLI, br *BuildResult) (string, error)

func NewBuildResult(oc *CLI, build *buildv1.Build) *BuildResult {
	return &BuildResult{
		Oc:        oc,
		BuildName: build.Name,
		BuildPath: "builds/" + build.Name,
	}
}

type BuildResult struct {
	// BuildPath is a resource qualified name (e.g. "build/test-1").
	BuildPath string
	// BuildName is the non-resource qualified name.
	BuildName string
	// StartBuildStdErr is the StdErr output generated by oc start-build.
	StartBuildStdErr string
	// StartBuildStdOut is the StdOut output generated by oc start-build.
	StartBuildStdOut string
	// StartBuildErr is the error, if any, returned by the direct invocation of the start-build command.
	StartBuildErr error
	// The buildconfig which generated this build.
	BuildConfigName string
	// Build is the resource created. May be nil if there was a timeout.
	Build *buildv1.Build
	// BuildAttempt represents that a Build resource was created.
	// false indicates a severe error unrelated to Build success or failure.
	BuildAttempt bool
	// BuildSuccess is true if the build was finshed successfully.
	BuildSuccess bool
	// BuildFailure is true if the build was finished with an error.
	BuildFailure bool
	// BuildCancelled is true if the build was canceled.
	BuildCancelled bool
	// BuildTimeout is true if there was a timeout waiting for the build to finish.
	BuildTimeout bool
	// Alternate log dumper function. If set, this is called instead of 'oc logs'
	LogDumper LogDumperFunc
	// The openshift client which created this build.
	Oc *CLI
}

// DumpLogs sends logs associated with this BuildResult to the GinkgoWriter.
func (t *BuildResult) DumpLogs() {
	e2e.Logf("\n\n*****************************************\n")
	e2e.Logf("Dumping Build Result: %#v\n", *t)

	if t == nil {
		e2e.Logf("No build result available!\n\n")
		return
	}

	desc, err := t.Oc.Run("describe").Args(t.BuildPath).Output()

	e2e.Logf("\n** Build Description:\n")
	if err != nil {
		e2e.Logf("Error during description retrieval: %+v\n", err)
	} else {
		e2e.Logf("%s\n", desc)
	}

	e2e.Logf("\n** Build Logs:\n")

	buildOuput, err := t.Logs()
	if err != nil {
		e2e.Logf("Error during log retrieval: %+v\n", err)
	} else {
		e2e.Logf("%s\n", buildOuput)
	}

	e2e.Logf("\n\n")

	t.dumpRegistryLogs()

	// if we suspect that we are filling up the registry file system, call ExamineDiskUsage / ExaminePodDiskUsage
	// also see if manipulations of the quota around /mnt/openshift-xfs-vol-dir exist in the extended test set up scripts
	/*
		ExamineDiskUsage()
		ExaminePodDiskUsage(t.oc)
		e2e.Logf( "\n\n")
	*/
}

func (t *BuildResult) dumpRegistryLogs() {
	var buildStarted *time.Time
	oc := t.Oc
	e2e.Logf("\n** Registry Logs:\n")

	if t.Build != nil && !t.Build.CreationTimestamp.IsZero() {
		buildStarted = &t.Build.CreationTimestamp.Time
	} else {
		proj, err := oc.ProjectClient().ProjectV1().Projects().Get(context.Background(), oc.Namespace(), metav1.GetOptions{})
		if err != nil {
			e2e.Logf("Failed to get project %s: %v\n", oc.Namespace(), err)
		} else {
			buildStarted = &proj.CreationTimestamp.Time
		}
	}

	if buildStarted == nil {
		e2e.Logf("Could not determine test' start time\n\n\n")
		return
	}

	since := time.Now().Sub(*buildStarted)

	// Changing the namespace on the derived client still changes it on the original client
	// because the kubeFramework field is only copied by reference. Saving the original namespace
	// here so we can restore it when done with registry logs
	// TODO remove the default/docker-registry log retrieval when we are fully migrated to 4.0 for our test env.
	savedNamespace := t.Oc.Namespace()
	oadm := t.Oc.AsAdmin().SetNamespace("default")
	out, err := oadm.Run("logs").Args("dc/docker-registry", "--since="+since.String()).Output()
	if err != nil {
		e2e.Logf("Error during log retrieval: %+v\n", err)
	} else {
		e2e.Logf("%s\n", out)
	}
	oadm = t.Oc.AsAdmin().SetNamespace("openshift-image-registry")
	out, err = oadm.Run("logs").Args("deployment/image-registry", "--since="+since.String()).Output()
	if err != nil {
		e2e.Logf("Error during log retrieval: %+v\n", err)
	} else {
		e2e.Logf("%s\n", out)
	}
	t.Oc.SetNamespace(savedNamespace)

	e2e.Logf("\n\n")
}

// Logs returns the logs associated with this build.
func (t *BuildResult) Logs() (string, error) {
	if t == nil || t.BuildPath == "" {
		return "", fmt.Errorf("Not enough information to retrieve logs for %#v", *t)
	}

	if t.LogDumper != nil {
		return t.LogDumper(t.Oc, t)
	}

	buildOuput, buildErr, err := t.Oc.Run("logs").Args("-f", t.BuildPath, "--timestamps", "--v", "10").Outputs()
	if err != nil {
		return "", fmt.Errorf("Error retrieving logs for build %q: (%s) %v", t.BuildName, buildErr, err)
	}

	return buildOuput, nil
}

// LogsNoTimestamp returns the logs associated with this build.
func (t *BuildResult) LogsNoTimestamp() (string, error) {
	if t == nil || t.BuildPath == "" {
		return "", fmt.Errorf("Not enough information to retrieve logs for %#v", *t)
	}

	if t.LogDumper != nil {
		return t.LogDumper(t.Oc, t)
	}

	buildOuput, buildErr, err := t.Oc.Run("logs").Args("-f", t.BuildPath).Outputs()
	if err != nil {
		return "", fmt.Errorf("Error retrieving logs for build %q: (%s) %v", t.BuildName, buildErr, err)
	}

	return buildOuput, nil
}

// Dumps logs and triggers a Ginkgo assertion if the build did NOT succeed.
func (t *BuildResult) AssertSuccess() *BuildResult {
	if !t.BuildSuccess {
		t.DumpLogs()
	}
	o.ExpectWithOffset(1, t.BuildSuccess).To(o.BeTrue())
	return t
}

// Dumps logs and triggers a Ginkgo assertion if the build did NOT have an error (this will not assert on timeouts)
func (t *BuildResult) AssertFailure() *BuildResult {
	if !t.BuildFailure {
		t.DumpLogs()
	}
	o.ExpectWithOffset(1, t.BuildFailure).To(o.BeTrue())
	return t
}

func StartBuildResult(oc *CLI, args ...string) (result *BuildResult, err error) {
	args = append(args, "-o=name") // ensure that the build name is the only thing send to stdout
	stdout, stderr, err := StartBuild(oc, args...)

	// Usually, with -o=name, we only expect the build path.
	// However, the caller may have added --follow which can add
	// content to stdout. So just grab the first line.
	buildPath := strings.TrimSpace(strings.Split(stdout, "\n")[0])

	result = &BuildResult{
		Build:            nil,
		BuildPath:        buildPath,
		StartBuildStdOut: stdout,
		StartBuildStdErr: stderr,
		StartBuildErr:    nil,
		BuildAttempt:     false,
		BuildSuccess:     false,
		BuildFailure:     false,
		BuildCancelled:   false,
		BuildTimeout:     false,
		Oc:               oc,
	}

	// An error here does not necessarily mean we could not run start-build. For example
	// when --wait is specified, start-build returns an error if the build fails. Therefore,
	// we continue to collect build information even if we see an error.
	result.StartBuildErr = err

	matches := buildPathPattern.FindStringSubmatch(buildPath)
	if len(matches) != 2 {
		return result, fmt.Errorf("Build path output did not match expected format 'build/name' : %q", buildPath)
	}

	result.BuildName = matches[1]

	return result, nil
}

// StartBuildAndWait executes OC start-build with the specified arguments on an existing buildconfig.
// Note that start-build will be run with "-o=name" as a parameter when using this method.
// If no error is returned from this method, it means that the build attempted successfully, NOT that
// the build completed. For completion information, check the BuildResult object.
func StartBuildAndWait(oc *CLI, args ...string) (result *BuildResult, err error) {
	result, err = StartBuildResult(oc, args...)
	if err != nil {
		return result, err
	}
	return result, WaitForBuildResult(oc.BuildClient().BuildV1().Builds(oc.Namespace()), result)
}

// WaitForBuildResult updates result with the state of the build
func WaitForBuildResult(c buildv1clienttyped.BuildInterface, result *BuildResult) error {
	e2e.Logf("Waiting for %s to complete\n", result.BuildName)
	err := WaitForABuild(c, result.BuildName,
		func(b *buildv1.Build) bool {
			result.Build = b
			result.BuildSuccess = CheckBuildSuccess(b)
			return result.BuildSuccess
		},
		func(b *buildv1.Build) bool {
			result.Build = b
			result.BuildFailure = CheckBuildFailed(b)
			return result.BuildFailure
		},
		func(b *buildv1.Build) bool {
			result.Build = b
			result.BuildCancelled = CheckBuildCancelled(b)
			return result.BuildCancelled
		},
	)

	if result.Build == nil {
		// We only abort here if the build progress was unobservable. Only known cause would be severe, non-build related error in WaitForABuild.
		return fmt.Errorf("Severe error waiting for build: %v", err)
	}

	result.BuildAttempt = true
	result.BuildTimeout = !(result.BuildFailure || result.BuildSuccess || result.BuildCancelled)

	e2e.Logf("Done waiting for %s: %#v\n with error: %v\n", result.BuildName, *result, err)
	return nil
}

// WaitForABuild waits for a Build object to match either isOK or isFailed conditions.
func WaitForABuild(c buildv1clienttyped.BuildInterface, name string, isOK, isFailed, isCanceled func(*buildv1.Build) bool) error {
	return WaitForABuildWithTimeout(c, name, 2*time.Minute, 30*time.Minute, isOK, isFailed, isCanceled)
}

// WaitForABuild waits for a Build object to match either isOK or isFailed conditions.
func WaitForABuildWithTimeout(c buildv1clienttyped.BuildInterface, name string, createTimeout, completeTimeout time.Duration, isOK, isFailed, isCanceled func(*buildv1.Build) bool) error {
	if isOK == nil {
		isOK = CheckBuildSuccess
	}
	if isFailed == nil {
		isFailed = CheckBuildFailed
	}
	if isCanceled == nil {
		isCanceled = CheckBuildCancelled
	}

	// wait 2 minutes for build to exist
	err := wait.Poll(1*time.Second, createTimeout, func() (bool, error) {
		if _, err := c.Get(context.Background(), name, metav1.GetOptions{}); err != nil {
			e2e.Logf("attempt to get buildconfig %s failed with error: %s", name, err.Error())
			return false, nil
		}
		return true, nil
	})
	if err == wait.ErrWaitTimeout {
		return fmt.Errorf("Timed out waiting for build %q to be created", name)
	}
	if err != nil {
		return err
	}
	// wait longer for the build to run to completion
	err = wait.Poll(5*time.Second, completeTimeout, func() (bool, error) {
		list, err := c.List(context.Background(), metav1.ListOptions{FieldSelector: fields.Set{"metadata.name": name}.AsSelector().String()})
		if err != nil {
			e2e.Logf("error listing builds: %v", err)
			return false, err
		}
		for i := range list.Items {
			if name == list.Items[i].Name && (isOK(&list.Items[i]) || isCanceled(&list.Items[i])) {
				return true, nil
			}
			if name != list.Items[i].Name {
				return false, fmt.Errorf("While listing builds named %s, found unexpected build %#v", name, list.Items[i])
			}
			if isFailed(&list.Items[i]) {
				return false, fmt.Errorf("The build %q status is %q", name, list.Items[i].Status.Phase)
			}
		}
		return false, nil
	})
	if err != nil {
		e2e.Logf("WaitForABuild returning with error: %v", err)
	}
	if err == wait.ErrWaitTimeout {
		return fmt.Errorf("Timed out waiting for build %q to complete", name)
	}
	return err
}

// CheckBuildSuccess returns true if the build succeeded
func CheckBuildSuccess(b *buildv1.Build) bool {
	return b.Status.Phase == buildv1.BuildPhaseComplete
}

// CheckBuildFailed return true if the build failed
func CheckBuildFailed(b *buildv1.Build) bool {
	return b.Status.Phase == buildv1.BuildPhaseFailed || b.Status.Phase == buildv1.BuildPhaseError
}

// CheckBuildCancelled return true if the build was canceled
func CheckBuildCancelled(b *buildv1.Build) bool {
	return b.Status.Phase == buildv1.BuildPhaseCancelled
}

// WaitForServiceAccount waits until the named service account gets fully
// provisioned. Does not wait for dockercfg secrets
func WaitForServiceAccount(c corev1client.ServiceAccountInterface, name string) error {
	waitFn := func() (bool, error) {
		_, err := c.Get(context.Background(), name, metav1.GetOptions{})
		if err != nil {
			// If we can't access the service accounts, let's wait till the controller
			// create it.
			if kapierrs.IsNotFound(err) || kapierrs.IsForbidden(err) {
				e2e.Logf("Waiting for service account %q to be available: %v (will retry) ...", name, err)
				return false, nil
			}
			return false, fmt.Errorf("Failed to get service account %q: %v", name, err)
		}
		return true, nil
	}
	return wait.Poll(100*time.Millisecond, 3*time.Minute, waitFn)
}

// WaitForServiceAccountWithSecret waits until the named service account gets fully
// provisioned, including dockercfg secrets. We also check if image registry is not enabled,
// the SA will not contain the docker secret and we simply return nil.
func WaitForServiceAccountWithSecret(config configclient.ClusterVersionInterface, c corev1client.ServiceAccountInterface, name string) error {
	cv, err := config.Get(context.Background(), "version", metav1.GetOptions{})
	if err != nil {
		return err
	}
	var found bool
	for _, capability := range cv.Status.Capabilities.EnabledCapabilities {
		if capability == configv1.ClusterVersionCapabilityImageRegistry {
			found = true
			break
		}
	}
	if !found {
		return nil
	}

	waitFn := func() (bool, error) {
		sa, err := c.Get(context.Background(), name, metav1.GetOptions{})
		if err != nil {
			// If we can't access the service accounts, let's wait till the controller
			// create it.
			if kapierrs.IsNotFound(err) || kapierrs.IsForbidden(err) {
				e2e.Logf("Waiting for service account %q to be available: %v (will retry) ...", name, err)
				return false, nil
			}
			return false, fmt.Errorf("Failed to get service account %q: %v", name, err)
		}
		secretNames := []string{}
		var hasDockercfg bool
		for _, s := range sa.ImagePullSecrets {
			if strings.Contains(s.Name, "-dockercfg-") {
				hasDockercfg = true
			}
			secretNames = append(secretNames, s.Name)
		}
		if hasDockercfg {
			return true, nil
		}
		e2e.Logf("Waiting for service account %q secrets (%s) to include dockercfg ...", name, strings.Join(secretNames, ","))
		return false, nil
	}
	return wait.Poll(100*time.Millisecond, 3*time.Minute, waitFn)
}

// WaitForNamespaceSCCAnnotations waits up to 30s for the cluster-policy-controller to add the SCC related
// annotations to the provided namespace.
func WaitForNamespaceSCCAnnotations(c corev1client.CoreV1Interface, name string) error {
	waitFn := func() (bool, error) {
		ns, err := c.Namespaces().Get(context.Background(), name, metav1.GetOptions{})
		if err != nil {
			// it is assumed the project was created prior to calling this, so we
			// do not distinguish not found errors
			return false, err
		}
		if ns.Annotations == nil {
			return false, nil
		}
		for k := range ns.Annotations {
			// annotations to check based off of
			// https://github.com/openshift/cluster-policy-controller/blob/master/pkg/security/controller/namespace_scc_allocation_controller.go#L112
			if k == securityv1.UIDRangeAnnotation {
				return true, nil
			}
		}
		e2e.Logf("namespace %s current annotation set: %#v", name, ns.Annotations)
		return false, nil
	}
	return wait.Poll(time.Duration(250*time.Millisecond), 30*time.Minute, waitFn)
}

// WaitForAnImageStream waits for an ImageStream to fulfill the isOK function
func WaitForAnImageStream(client imagev1typedclient.ImageStreamInterface,
	name string,
	isOK, isFailed func(*imagev1.ImageStream) bool,
) error {
	for {
		list, err := client.List(context.Background(), metav1.ListOptions{FieldSelector: fields.Set{"metadata.name": name}.AsSelector().String()})
		if err != nil {
			return err
		}
		for i := range list.Items {
			if isOK(&list.Items[i]) {
				return nil
			}
			if isFailed(&list.Items[i]) {
				return fmt.Errorf("The image stream %q status is %q",
					name, list.Items[i].Annotations[imagev1.DockerImageRepositoryCheckAnnotation])
			}
		}

		rv := list.ResourceVersion
		w, err := client.Watch(context.Background(), metav1.ListOptions{FieldSelector: fields.Set{"metadata.name": name}.AsSelector().String(), ResourceVersion: rv})
		if err != nil {
			return err
		}
		defer w.Stop()

		for {
			val, ok := <-w.ResultChan()
			if !ok {
				// reget and re-watch
				break
			}
			if e, ok := val.Object.(*imagev1.ImageStream); ok {
				if isOK(e) {
					return nil
				}
				if isFailed(e) {
					return fmt.Errorf("The image stream %q status is %q",
						name, e.Annotations[imagev1.DockerImageRepositoryCheckAnnotation])
				}
			}
		}
	}
}

// WaitForAnImageStreamTag waits until an image stream with given name has non-empty history for given tag.
// Defaults to waiting for 300 seconds
func WaitForAnImageStreamTag(oc *CLI, namespace, name, tag string) error {
	waitTimeout := time.Second * 300
	g.By(fmt.Sprintf("waiting for an is importer to import a tag %s into a stream %s", tag, name))
	start := time.Now()
	c := make(chan error)
	go func() {
		err := WaitForAnImageStream(
			oc.ImageClient().ImageV1().ImageStreams(namespace),
			name,
			func(is *imagev1.ImageStream) bool {
				statusTag, exists := imageutil.StatusHasTag(is, tag)
				if !exists || len(statusTag.Items) == 0 {
					return false
				}
				return true
			},
			func(is *imagev1.ImageStream) bool {
				return time.Now().After(start.Add(waitTimeout))
			})
		c <- err
	}()

	select {
	case e := <-c:
		return e
	case <-time.After(waitTimeout):
		return fmt.Errorf("timed out while waiting of an image stream tag %s/%s:%s", namespace, name, tag)
	}
}

// CheckImageStreamLatestTagPopulated returns true if the imagestream has a ':latest' tag filed
func CheckImageStreamLatestTagPopulated(i *imagev1.ImageStream) bool {
	_, ok := imageutil.StatusHasTag(i, "latest")
	return ok
}

// CheckImageStreamTagNotFound return true if the imagestream update was not successful
func CheckImageStreamTagNotFound(i *imagev1.ImageStream) bool {
	return strings.Contains(i.Annotations[imagev1.DockerImageRepositoryCheckAnnotation], "not") ||
		strings.Contains(i.Annotations[imagev1.DockerImageRepositoryCheckAnnotation], "error")
}

func isUsageSynced(received, expected corev1.ResourceList, expectedIsUpperLimit bool) bool {
	resourceNames := quotav1.ResourceNames(expected)
	masked := quotav1.Mask(received, resourceNames)
	if len(masked) != len(expected) {
		return false
	}
	if expectedIsUpperLimit {
		if le, _ := quotav1.LessThanOrEqual(masked, expected); !le {
			return false
		}
	} else {
		if le, _ := quotav1.LessThanOrEqual(expected, masked); !le {
			return false
		}
	}
	return true
}

// WaitForResourceQuotaSync watches given resource quota until its usage is updated to desired level or a
// timeout occurs. If successful, used quota values will be returned for expected resources. Otherwise an
// ErrWaitTimeout will be returned. If expectedIsUpperLimit is true, given expected usage must compare greater
// or equal to quota's usage, which is useful for expected usage increment. Otherwise expected usage must
// compare lower or equal to quota's usage, which is useful for expected usage decrement.
func WaitForResourceQuotaSync(
	client corev1client.ResourceQuotaInterface,
	name string,
	expectedUsage corev1.ResourceList,
	expectedIsUpperLimit bool,
	timeout time.Duration,
) (corev1.ResourceList, error) {
	startTime := time.Now()
	endTime := startTime.Add(timeout)

	expectedResourceNames := quotav1.ResourceNames(expectedUsage)

	list, err := client.List(context.Background(), metav1.ListOptions{FieldSelector: fields.Set{"metadata.name": name}.AsSelector().String()})
	if err != nil {
		return nil, err
	}

	for i := range list.Items {
		used := quotav1.Mask(list.Items[i].Status.Used, expectedResourceNames)
		if isUsageSynced(used, expectedUsage, expectedIsUpperLimit) {
			return used, nil
		}
	}

	rv := list.ResourceVersion
	w, err := client.Watch(context.Background(), metav1.ListOptions{FieldSelector: fields.Set{"metadata.name": name}.AsSelector().String(), ResourceVersion: rv})
	if err != nil {
		return nil, err
	}
	defer w.Stop()

	for time.Now().Before(endTime) {
		select {
		case val, ok := <-w.ResultChan():
			if !ok {
				// reget and re-watch
				continue
			}
			if rq, ok := val.Object.(*corev1.ResourceQuota); ok {
				used := quotav1.Mask(rq.Status.Used, expectedResourceNames)
				if isUsageSynced(used, expectedUsage, expectedIsUpperLimit) {
					return used, nil
				}
			}
		case <-time.After(endTime.Sub(time.Now())):
			return nil, wait.ErrWaitTimeout
		}
	}
	return nil, wait.ErrWaitTimeout
}

// GetPodNamesByFilter looks up pods that satisfy the predicate and returns their names.
func GetPodNamesByFilter(c corev1client.PodInterface, label labels.Selector, predicate func(corev1.Pod) bool) (podNames []string, err error) {
	podList, err := c.List(context.Background(), metav1.ListOptions{LabelSelector: label.String()})
	if err != nil {
		return nil, err
	}
	for _, pod := range podList.Items {
		if predicate(pod) {
			podNames = append(podNames, pod.Name)
		}
	}
	return podNames, nil
}

func WaitForAJob(c batchv1client.JobInterface, name string, timeout time.Duration) error {
	return wait.Poll(1*time.Second, timeout, func() (bool, error) {
		j, e := c.Get(context.Background(), name, metav1.GetOptions{})
		if e != nil {
			return true, e
		}
		// TODO soltysh: replace this with a function once such exist, currently
		// it's private in the controller
		for _, c := range j.Status.Conditions {
			if (c.Type == batchv1.JobComplete || c.Type == batchv1.JobFailed) && c.Status == corev1.ConditionTrue {
				return true, nil
			}
		}
		return false, nil
	})
}

// WaitForPods waits until given number of pods that match the label selector and
// satisfy the predicate are found
func WaitForPods(c corev1client.PodInterface, label labels.Selector, predicate func(corev1.Pod) bool, count int, timeout time.Duration) ([]string, error) {
	var podNames []string
	err := wait.Poll(1*time.Second, timeout, func() (bool, error) {
		p, e := GetPodNamesByFilter(c, label, predicate)
		if e != nil {
			return true, e
		}
		if len(p) != count {
			return false, nil
		}
		podNames = p
		return true, nil
	})
	return podNames, err
}

// CheckPodIsRunning returns true if the pod is running
func CheckPodIsRunning(pod corev1.Pod) bool {
	return pod.Status.Phase == corev1.PodRunning
}

// CheckPodIsSucceeded returns true if the pod status is "Succdeded"
func CheckPodIsSucceeded(pod corev1.Pod) bool {
	return pod.Status.Phase == corev1.PodSucceeded
}

// CheckPodIsRunning returns true if the pod is running
func CheckPodIsPending(pod corev1.Pod) bool {
	return pod.Status.Phase == corev1.PodPending
}

// CheckPodIsReady returns true if the pod's ready probe determined that the pod is ready.
func CheckPodIsReady(pod corev1.Pod) bool {
	if pod.Status.Phase != corev1.PodRunning {
		return false
	}
	for _, cond := range pod.Status.Conditions {
		if cond.Type != corev1.PodReady {
			continue
		}
		return cond.Status == corev1.ConditionTrue
	}
	return false
}

// CheckPodNoOp always returns true
func CheckPodNoOp(pod corev1.Pod) bool {
	return true
}

// WaitUntilPodIsGone waits until the named Pod will disappear
func WaitUntilPodIsGone(c corev1client.PodInterface, podName string, timeout time.Duration) error {
	return wait.Poll(1*time.Second, timeout, func() (bool, error) {
		_, err := c.Get(context.Background(), podName, metav1.GetOptions{})
		if err != nil {
			if strings.Contains(err.Error(), "not found") {
				return true, nil
			}
			return true, err
		}
		return false, nil
	})
}

// GetDockerImageReference retrieves the full Docker pull spec from the given ImageStream
// and tag
func GetDockerImageReference(c imagev1typedclient.ImageStreamInterface, name, tag string) (string, error) {
	imageStream, err := c.Get(context.Background(), name, metav1.GetOptions{})
	if err != nil {
		return "", err
	}
	isTag, ok := imageutil.StatusHasTag(imageStream, tag)
	if !ok {
		return "", fmt.Errorf("ImageStream %q does not have tag %q", name, tag)
	}
	return isTag.Items[0].DockerImageReference, nil
}

// GetPodForContainer creates a new Pod that runs specified container
func GetPodForContainer(container corev1.Container) *corev1.Pod {
	name := naming.GetPodName("test-pod", string(uuid.NewUUID()))
	return &corev1.Pod{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Pod",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:   name,
			Labels: map[string]string{"name": name},
		},
		Spec: corev1.PodSpec{
			Containers:    []corev1.Container{container},
			RestartPolicy: corev1.RestartPolicyNever,
		},
	}
}

// KubeConfigPath returns the value of KUBECONFIG environment variable
func KubeConfigPath() string {
	// can't use gomega in this method since it is used outside of It()
	return os.Getenv("KUBECONFIG")
}

// StaticConfigManifestDir returns the value of STATIC_CONFIG_MANIFEST_DIR environment variable
// It points to a directory with static manifests, each file is expected to be a single manifest.
// Manifest files can be stored under directory tree.
func StaticConfigManifestDir() string {
	return os.Getenv("STATIC_CONFIG_MANIFEST_DIR")
}

// ArtifactDirPath returns the value of ARTIFACT_DIR environment variable
func ArtifactDirPath() string {
	path := os.Getenv("ARTIFACT_DIR")
	o.Expect(path).NotTo(o.BeNil())
	o.Expect(path).NotTo(o.BeEmpty())
	return path
}

// ArtifactPath returns the absolute path to the fix artifact file
// The path is relative to ARTIFACT_DIR
func ArtifactPath(elem ...string) string {
	return filepath.Join(append([]string{ArtifactDirPath()}, elem...)...)
}

func prefixFixturePath(elem []string) []string {
	switch {
	case len(elem) == 0:
		panic("must specify path")
	case len(elem) > 3 && elem[0] == ".." && elem[1] == ".." && elem[2] == "examples":
		elem = elem[2:]
	case len(elem) > 3 && elem[0] == ".." && elem[1] == ".." && elem[2] == "install":
		elem = elem[2:]
	case len(elem) > 3 && elem[0] == ".." && elem[1] == "integration":
		elem = append([]string{"test"}, elem[1:]...)
	case elem[0] == "testdata":
		elem = append([]string{"test", "extended"}, elem...)
	default:
		panic(fmt.Sprintf("Fixtures must be in test/extended/testdata or examples not %s", path.Join(elem...)))
	}
	return elem
}

// FixturePaths returns the set of paths within the provided fixture directory.
func FixturePaths(elem ...string) []string {
	var paths []string
	elem = prefixFixturePath(elem)
	prefix := path.Join(elem...)
	items, _ := testdata.AssetDir(prefix)
	for _, item := range items {
		paths = append(paths, item)
	}
	return paths
}

var (
	internalFixtureOnce sync.Once
	// callers should use fixtureDirectory() instead
	internalFixtureDir string
)

// fixtureDirectory returns the fixture directory for use within this process.
// It returns true if the current process was the one to initialize the directory.
func fixtureDirectory() (string, bool) {
	// load or allocate fixture directory
	var init bool
	internalFixtureOnce.Do(func() {
		// reuse fixture directories across child processes for efficiency
		internalFixtureDir = os.Getenv("OS_TEST_FIXTURE_DIR")
		if len(internalFixtureDir) == 0 {
			dir, err := ioutil.TempDir("", "fixture-testdata-dir")
			if err != nil {
				panic(err)
			}
			internalFixtureDir = dir
			init = true
		}
	})
	return internalFixtureDir, init
}

// FixturePath returns an absolute path to a fixture file in test/extended/testdata/,
// test/integration/, or examples/. The contents of the path will not exist until the
// test is started.
func FixturePath(elem ...string) string {
	// normalize the element array
	originalElem := elem
	elem = prefixFixturePath(elem)
	relativePath := path.Join(elem...)

	fixtureDir, _ := fixtureDirectory()
	fullPath := path.Join(fixtureDir, relativePath)
	absPath, err := filepath.Abs(fullPath)
	if err != nil {
		panic(err)
	}

	if testsStarted {
		// extract the contents to disk
		if err := restoreFixtureAssets(fixtureDir, relativePath); err != nil {
			panic(err)
		}
	} else {
		// defer extraction of content to a BeforeEach when called before tests start
		g.BeforeEach(func() {
			FixturePath(originalElem...)
		})
	}

	return absPath
}

// restoreFixtureAsset restores an asset under the given directory and post-processes
// any changes required by the test. It hardcodes file modes to 0640 and ensures image
// values are replaced.
func restoreFixtureAsset(dir, name string) error {
	data, err := testdata.Asset(name)
	if err != nil {
		return err
	}
	data, err = utilimage.ReplaceContents(data)
	if err != nil {
		return err
	}
	info, err := testdata.AssetInfo(name)
	if err != nil {
		return err
	}
	err = os.MkdirAll(assetFilePath(dir, filepath.Dir(name)), os.FileMode(0755))
	if err != nil {
		return err
	}
	err = ioutil.WriteFile(assetFilePath(dir, name), data, 0640)
	if err != nil {
		return err
	}
	err = os.Chtimes(assetFilePath(dir, name), info.ModTime(), info.ModTime())
	if err != nil {
		return err
	}
	return nil
}

// restoreFixtureAssets restores an asset under the given directory recursively, changing
// any necessary content. This duplicates a method in testdata but with the additional
// requirements for setting disk permissions and for altering image content.
func restoreFixtureAssets(dir, name string) error {
	children, err := testdata.AssetDir(name)
	// File
	if err != nil {
		return restoreFixtureAsset(dir, name)
	}
	// Dir
	for _, child := range children {
		err = restoreFixtureAssets(dir, filepath.Join(name, child))
		if err != nil {
			return err
		}
	}
	return nil
}

func assetFilePath(dir, name string) string {
	cannonicalName := strings.Replace(name, "\\", "/", -1)
	return filepath.Join(append([]string{dir}, strings.Split(cannonicalName, "/")...)...)
}

// FetchURL grabs the output from the specified url and returns it.
// It will retry once per second for duration retryTimeout if an error occurs during the request.
func FetchURL(oc *CLI, url string, retryTimeout time.Duration) (string, error) {
	ns := oc.KubeFramework().Namespace.Name
	execPod := CreateExecPodOrFail(oc.AdminKubeClient(), ns, string(uuid.NewUUID()))
	defer func() {
		oc.AdminKubeClient().CoreV1().Pods(ns).Delete(context.Background(), execPod.Name, *metav1.NewDeleteOptions(1))
	}()

	var err error
	var response string
	waitFn := func() (bool, error) {
		e2e.Logf("Waiting up to %v to wget %s", retryTimeout, url)
		// cmd := fmt.Sprintf("wget -T 30 -O- %s", url)
		cmd := fmt.Sprintf("curl -vvv %s", url)
		response, err = e2eoutput.RunHostCmd(execPod.Namespace, execPod.Name, cmd)
		if err != nil {
			e2e.Logf("got err: %v, retry until timeout", err)
			return false, nil
		}
		// Need to check output because wget -q might omit the error.
		if strings.TrimSpace(response) == "" {
			e2e.Logf("got empty stdout, retry until timeout")
			return false, nil
		}
		return true, nil
	}
	pollErr := wait.Poll(time.Duration(1*time.Second), retryTimeout, waitFn)
	if pollErr == wait.ErrWaitTimeout {
		return "", fmt.Errorf("Timed out while fetching url %q", url)
	}
	if pollErr != nil {
		return "", pollErr
	}
	return response, nil
}

// ParseLabelsOrDie turns the given string into a label selector or
// panics; for tests or other cases where you know the string is valid.
// TODO: Move this to the upstream labels package.
func ParseLabelsOrDie(str string) labels.Selector {
	ret, err := labels.Parse(str)
	if err != nil {
		panic(fmt.Sprintf("cannot parse '%v': %v", str, err))
	}
	return ret
}

// LaunchWebserverPod launches a pod serving http on port 8080 to act
// as the target for networking connectivity checks.  The ip address
// of the created pod will be returned if the pod is launched
// successfully.
func LaunchWebserverPod(client k8sclient.Interface, namespace, podName, nodeName string) (ip string) {
	containerName := fmt.Sprintf("%s-container", podName)
	port := 8080
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name: podName,
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name:  containerName,
					Image: image.GetE2EImage(image.Agnhost),
					Args:  []string{"netexec", "--http-port", fmt.Sprintf("%d", port)},
					Ports: []corev1.ContainerPort{{ContainerPort: int32(port)}},
				},
			},
			NodeName:      nodeName,
			RestartPolicy: corev1.RestartPolicyNever,
		},
	}
	podClient := client.CoreV1().Pods(namespace)
	_, err := podClient.Create(context.Background(), pod, metav1.CreateOptions{})
	e2e.ExpectNoError(err)
	e2e.ExpectNoError(e2epod.WaitForPodNameRunningInNamespace(context.TODO(), client, podName, namespace))
	createdPod, err := podClient.Get(context.Background(), podName, metav1.GetOptions{})
	e2e.ExpectNoError(err)
	ip = net.JoinHostPort(createdPod.Status.PodIP, strconv.Itoa(port))
	e2e.Logf("Target pod IP:port is %s", ip)
	return
}

func WaitForEndpoint(c kubernetes.Interface, ns, name string) error {
	for t := time.Now(); time.Since(t) < 3*time.Minute; time.Sleep(5 * time.Second) {
		endpoint, err := c.CoreV1().Endpoints(ns).Get(context.Background(), name, metav1.GetOptions{})
		if kapierrs.IsNotFound(err) {
			e2e.Logf("Endpoint %s/%s is not ready yet", ns, name)
			continue
		}
		o.Expect(err).NotTo(o.HaveOccurred())
		if len(endpoint.Subsets) == 0 || len(endpoint.Subsets[0].Addresses) == 0 {
			e2e.Logf("Endpoint %s/%s is not ready yet", ns, name)
			continue
		} else {
			return nil
		}
	}
	return fmt.Errorf("Failed to get endpoints for %s/%s", ns, name)
}

// GetEndpointAddress will return an "ip:port" string for the endpoint.
func GetEndpointAddress(oc *CLI, name string) (string, error) {
	err := WaitForEndpoint(oc.KubeFramework().ClientSet, oc.Namespace(), name)
	if err != nil {
		return "", err
	}
	endpoint, err := oc.KubeClient().CoreV1().Endpoints(oc.Namespace()).Get(context.Background(), name, metav1.GetOptions{})
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%s:%d", endpoint.Subsets[0].Addresses[0].IP, endpoint.Subsets[0].Ports[0].Port), nil
}

// CheckForBuildEvent will poll a build for up to 1 minute looking for an event with
// the specified reason and message template.
func CheckForBuildEvent(client corev1client.CoreV1Interface, build *buildv1.Build, reason, message string) {
	scheme, _ := apitesting.SchemeForOrDie(buildv1.Install)
	var expectedEvent *corev1.Event
	err := wait.PollImmediate(e2e.Poll, 1*time.Minute, func() (bool, error) {
		events, err := client.Events(build.Namespace).Search(scheme, build)
		if err != nil {
			return false, err
		}
		for _, event := range events.Items {
			e2e.Logf("Found event %#v", event)
			if reason == event.Reason {
				expectedEvent = &event
				return true, nil
			}
		}
		return false, nil
	})
	o.ExpectWithOffset(1, err).NotTo(o.HaveOccurred(), "Should be able to get events from the build")
	o.ExpectWithOffset(1, expectedEvent).NotTo(o.BeNil(), "Did not find a %q event on build %s/%s", reason, build.Namespace, build.Name)
	o.ExpectWithOffset(1, expectedEvent.Message).To(o.Equal(fmt.Sprintf(message, build.Namespace, build.Name)))
}

type PodExecutor struct {
	client  *CLI
	podName string
}

// NewPodExecutor returns an executor capable of running commands in a Pod.
func NewPodExecutor(oc *CLI, name, image string) (*PodExecutor, error) {
	out, err := oc.Run("run").Args(name, "--labels", "name="+name, "--image", image, "--restart", "Never", "--command", "--", "/bin/bash", "-c", "sleep infinity").Output()
	if err != nil {
		return nil, fmt.Errorf("error: %v\n(%s)", err, out)
	}
	_, err = WaitForPods(oc.KubeClient().CoreV1().Pods(oc.Namespace()), ParseLabelsOrDie("name="+name), CheckPodIsReady, 1, 3*time.Minute)
	if err != nil {
		return nil, err
	}
	return &PodExecutor{client: oc, podName: name}, nil
}

// Exec executes a single command or a bash script in the running pod. It returns the
// command output and error if the command finished with non-zero status code or the
// command took longer then 3 minutes to run.
func (r *PodExecutor) Exec(script string) (string, error) {
	var out string
	waitErr := wait.PollImmediate(1*time.Second, 3*time.Minute, func() (bool, error) {
		var err error
		out, err = r.client.Run("exec").Args(r.podName, "--", "/bin/bash", "-c", script).Output()
		return true, err
	})
	return out, waitErr
}

func (r *PodExecutor) CopyFromHost(local, remote string) error {
	_, err := r.client.Run("cp").Args(local, fmt.Sprintf("%s:%s", r.podName, remote)).Output()
	return err
}

// RunOneShotCommandPod runs the given command in a pod and waits for completion and log output for the given timeout
// duration, returning the command output or an error.
// TODO: merge with the PodExecutor above
func RunOneShotCommandPod(
	oc *CLI,
	name, image, command string,
	volumeMounts []corev1.VolumeMount,
	volumes []corev1.Volume,
	env []corev1.EnvVar,
	timeout time.Duration,
) (string, []error) {
	errs := []error{}
	cmd := strings.Split(command, " ")
	args := cmd[1:]
	var output string

	pod, err := oc.AdminKubeClient().CoreV1().Pods(oc.Namespace()).Create(context.Background(), newCommandPod(name, image, cmd[0], args,
		volumeMounts, volumes, env), metav1.CreateOptions{})
	if err != nil {
		return "", []error{err}
	}

	// Wait for command completion.
	err = wait.PollImmediate(1*time.Second, timeout, func() (done bool, err error) {
		cmdPod, getErr := oc.AdminKubeClient().CoreV1().Pods(oc.Namespace()).Get(context.Background(), pod.Name, metav1.GetOptions{})
		if getErr != nil {
			e2e.Logf("failed to get pod %q: %v", pod.Name, err)
			return false, nil
		}

		if err := podHasErrored(cmdPod); err != nil {
			e2e.Logf("pod %q errored trying to run the command: %v", pod.Name, err)
			return false, err
		}
		return podHasCompleted(cmdPod), nil
	})
	if err != nil {
		errs = append(errs, fmt.Errorf("error waiting for the pod '%s' to complete: %v", pod.Name, err))
	}

	// Gather pod log output
	err = wait.PollImmediate(1*time.Second, timeout, func() (done bool, err error) {
		logs, logErr := getPodLogs(oc, pod)
		if logErr != nil {
			return false, logErr
		}
		if len(logs) == 0 {
			return false, nil
		}
		output = logs
		return true, nil
	})
	if err != nil {
		errs = append(errs, fmt.Errorf("command pod %s did not complete: %v", pod.Name, err))
	}

	return output, errs
}

func podHasCompleted(pod *corev1.Pod) bool {
	return len(pod.Status.ContainerStatuses) > 0 &&
		pod.Status.ContainerStatuses[0].State.Terminated != nil &&
		pod.Status.ContainerStatuses[0].State.Terminated.Reason == "Completed"
}

func podHasErrored(pod *corev1.Pod) error {
	if len(pod.Status.ContainerStatuses) > 0 &&
		pod.Status.ContainerStatuses[0].State.Terminated != nil &&
		pod.Status.ContainerStatuses[0].State.Terminated.Reason == "Error" {
		return errors.New(pod.Status.ContainerStatuses[0].State.Terminated.Message)
	}
	return nil
}

func getPodLogs(oc *CLI, pod *corev1.Pod) (string, error) {
	reader, err := oc.AdminKubeClient().CoreV1().Pods(oc.Namespace()).GetLogs(pod.Name, &corev1.PodLogOptions{}).Stream(context.Background())
	if err != nil {
		return "", err
	}
	logs, err := ioutil.ReadAll(reader)
	if err != nil {
		return "", err
	}
	return string(logs), nil
}

func newCommandPod(name, image, command string, args []string, volumeMounts []corev1.VolumeMount,
	volumes []corev1.Volume, env []corev1.EnvVar,
) *corev1.Pod {
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Spec: corev1.PodSpec{
			Volumes:       volumes,
			RestartPolicy: corev1.RestartPolicyOnFailure,
			Containers: []corev1.Container{
				{
					Name:                     name,
					Image:                    image,
					Command:                  []string{command},
					Args:                     args,
					VolumeMounts:             volumeMounts,
					ImagePullPolicy:          "Always",
					Env:                      env,
					TerminationMessagePolicy: corev1.TerminationMessageFallbackToLogsOnError,
				},
			},
		},
	}
}

type GitRepo struct {
	baseTempDir  string
	upstream     git.Repository
	upstreamPath string
	repo         git.Repository
	RepoPath     string
}

// AddAndCommit commits a file with its content to local repo
func (r GitRepo) AddAndCommit(file, content string) error {
	dir := filepath.Dir(file)
	if err := os.MkdirAll(filepath.Join(r.RepoPath, dir), 0777); err != nil {
		return err
	}
	if err := ioutil.WriteFile(filepath.Join(r.RepoPath, file), []byte(content), 0666); err != nil {
		return err
	}
	if err := r.repo.Add(r.RepoPath, file); err != nil {
		return err
	}
	if err := r.repo.Commit(r.RepoPath, "added file "+file); err != nil {
		return err
	}
	return nil
}

// Remove performs cleanup of no longer needed directories with local and "remote" git repo
func (r GitRepo) Remove() {
	if r.baseTempDir != "" {
		os.RemoveAll(r.baseTempDir)
	}
}

// NewGitRepo creates temporary test directories with local and "remote" git repo
func NewGitRepo(repoName string) (GitRepo, error) {
	testDir, err := ioutil.TempDir(os.TempDir(), repoName)
	if err != nil {
		return GitRepo{}, err
	}
	repoPath := filepath.Join(testDir, repoName)
	upstreamPath := repoPath + `.git`
	upstream := git.NewRepository()
	if err = upstream.Init(upstreamPath, true); err != nil {
		return GitRepo{baseTempDir: testDir}, err
	}
	repo := git.NewRepository()
	if err = repo.Clone(repoPath, upstreamPath); err != nil {
		return GitRepo{baseTempDir: testDir}, err
	}

	return GitRepo{testDir, upstream, upstreamPath, repo, repoPath}, nil
}

// WaitForUserBeAuthorized waits a minute until the cluster bootstrap roles are available
// and the provided user is authorized to perform the action on the resource.
func WaitForUserBeAuthorized(oc *CLI, user string, attributes *authorizationapi.ResourceAttributes) error {
	sar := &authorizationapi.SubjectAccessReview{
		Spec: authorizationapi.SubjectAccessReviewSpec{
			ResourceAttributes: attributes,
			User:               user,
		},
	}
	return wait.PollImmediate(1*time.Second, 1*time.Minute, func() (bool, error) {
		e2e.Logf("Waiting for user '%s' to be authorized for %v in ns '%s'", user, attributes, oc.Namespace())
		resp, err := oc.AdminKubeClient().AuthorizationV1().SubjectAccessReviews().Create(context.Background(), sar, metav1.CreateOptions{})
		if err == nil && resp != nil && resp.Status.Allowed {
			return true, nil
		}
		if err != nil {
			e2e.Logf("Error creating SubjectAccessReview: %v", err)
		}
		if resp != nil {
			e2e.Logf("SubjectAccessReview.Status: %#v", resp)
		}
		return false, err
	})
}

// GetRouterPodTemplate finds the router pod template across different namespaces,
// helping to mitigate the transition from the default namespace to an operator
// namespace.
func GetRouterPodTemplate(oc *CLI) (*corev1.PodTemplateSpec, string, error) {
	k8sappsclient := oc.AdminKubeClient().AppsV1()
	for _, ns := range []string{"default", "openshift-ingress", "tectonic-ingress"} {
		deploy, err := k8sappsclient.Deployments(ns).Get(context.Background(), "router", metav1.GetOptions{})
		if err == nil {
			return &deploy.Spec.Template, ns, nil
		}
		if !kapierrs.IsNotFound(err) {
			return nil, "", err
		}
		deploy, err = k8sappsclient.Deployments(ns).Get(context.Background(), "router-default", metav1.GetOptions{})
		if err == nil {
			return &deploy.Spec.Template, ns, nil
		}
		if !kapierrs.IsNotFound(err) {
			return nil, "", err
		}
	}
	return nil, "", kapierrs.NewNotFound(schema.GroupResource{Group: "apps", Resource: "deployments"}, "router")
}

func FindRouterImage(oc *CLI) (string, error) {
	exists, err := DoesApiResourceExist(oc.AdminConfig(), "clusteroperators", "config.openshift.io")
	if err != nil {
		return "", err
	}
	if !exists {
		deployment, err := oc.AdminKubeClient().AppsV1().Deployments("openshift-ingress").Get(context.Background(), "router-default", metav1.GetOptions{})
		if err != nil {
			return "", err
		}
		return deployment.Spec.Template.Spec.Containers[0].Image, nil
	}
	configclient := oc.AdminConfigClient().ConfigV1()
	o, err := configclient.ClusterOperators().Get(context.Background(), "ingress", metav1.GetOptions{})
	if err != nil {
		return "", err
	}
	for _, v := range o.Status.Versions {
		if v.Name == "ingress-controller" {
			return v.Version, nil
		}
	}
	return "", fmt.Errorf("expected to find ingress-controller version on clusteroperators/ingress")
}

func IsClusterOperated(oc *CLI) bool {
	configclient := oc.AdminConfigClient().ConfigV1()
	o, err := configclient.Images().Get(context.Background(), "cluster", metav1.GetOptions{})
	if o == nil || err != nil {
		e2e.Logf("Could not find image config object, assuming non-4.0 installed cluster: %v", err)
		return false
	}
	return true
}

var (
	ControlPlaneTopology *configv1.TopologyMode
	controlPlaneMutex    sync.Mutex
)

// GetControlPlaneTopology retrieves the cluster infrastructure TopologyMode
func GetControlPlaneTopology(ocClient *CLI) (*configv1.TopologyMode, error) {
	controlPlaneMutex.Lock()
	defer controlPlaneMutex.Unlock()

	if ControlPlaneTopology == nil {
		infra, err := ocClient.AdminConfigClient().ConfigV1().Infrastructures().Get(context.Background(), "cluster", metav1.GetOptions{})
		if err != nil {
			return nil, fmt.Errorf("failure getting test cluster Infrastructure: %s", err.Error())
		}
		if &infra.Status.ControlPlaneTopology == nil {
			return nil, fmt.Errorf("missing Infrastructure.Status.ControlPlaneTopology")
		}
		ControlPlaneTopology = &infra.Status.ControlPlaneTopology
	}
	return ControlPlaneTopology, nil
}

func GetControlPlaneTopologyFromConfigClient(client *configclient.ConfigV1Client) (*configv1.TopologyMode, error) {
	controlPlaneMutex.Lock()
	defer controlPlaneMutex.Unlock()

	if ControlPlaneTopology == nil {
		infra, err := client.Infrastructures().Get(context.Background(), "cluster", metav1.GetOptions{})
		if err != nil {
			return nil, fmt.Errorf("failure getting test cluster Infrastructure: %s", err.Error())
		}
		if &infra.Status.ControlPlaneTopology == nil {
			return nil, fmt.Errorf("missing Infrastructure.Status.ControlPlaneTopology")
		}
		ControlPlaneTopology = &infra.Status.ControlPlaneTopology
	}
	return ControlPlaneTopology, nil
}

const (
	hypershiftManagementClusterKubeconfigEnvVar = "HYPERSHIFT_MANAGEMENT_CLUSTER_KUBECONFIG"
	hypershiftManagementClusterNamespaceEnvVar  = "HYPERSHIFT_MANAGEMENT_CLUSTER_NAMESPACE"
)

var (
	hypershiftManagementClusterKubeconfig string
	hypershiftManagementClusterNamespace  string
	hypershiftMutex                       sync.Mutex
)

func GetHypershiftManagementClusterConfigAndNamespace() (string, string, error) {
	hypershiftMutex.Lock()
	defer hypershiftMutex.Unlock()

	if hypershiftManagementClusterKubeconfig == "" && hypershiftManagementClusterNamespace != "" {
		return hypershiftManagementClusterKubeconfig, hypershiftManagementClusterNamespace, nil
	}

	kubeconfig, namespace := os.Getenv(hypershiftManagementClusterKubeconfigEnvVar), os.Getenv(hypershiftManagementClusterNamespaceEnvVar)
	if kubeconfig == "" || namespace == "" {
		return "", "", fmt.Errorf("both the %s and the %s env var must be set", hypershiftManagementClusterKubeconfigEnvVar, hypershiftManagementClusterNamespaceEnvVar)
	}

	hypershiftManagementClusterKubeconfig = kubeconfig
	hypershiftManagementClusterNamespace = namespace

	return hypershiftManagementClusterKubeconfig, hypershiftManagementClusterNamespace, nil
}

func SkipIfExternalControlplaneTopology(oc *CLI, reason string) {
	controlPlaneTopology, err := GetControlPlaneTopology(oc)
	o.ExpectWithOffset(1, err).NotTo(o.HaveOccurred())
	if *controlPlaneTopology == configv1.ExternalTopologyMode {
		skipper.Skip(reason)
	}
}

// IsTechPreviewNoUpgrade checks if a cluster is a TechPreviewNoUpgrade cluster
func IsTechPreviewNoUpgrade(ctx context.Context, configClient configv1client.Interface) bool {
	featureGate, err := configClient.ConfigV1().FeatureGates().Get(ctx, "cluster", metav1.GetOptions{})
	if err != nil {
		if kapierrs.IsNotFound(err) {
			return false
		}
		e2e.Failf("could not retrieve feature-gate: %v", err)
	}

	return featureGate.Spec.FeatureSet == configv1.TechPreviewNoUpgrade
}

// IsNoUpgradeFeatureSet checks if a cluster has a non-upgradeable featureset
// such as TechPreviewNoUpgrade or CustomNoUpgrade.
func IsNoUpgradeFeatureSet(oc *CLI) bool {
	featureGate, err := oc.AdminConfigClient().ConfigV1().FeatureGates().Get(context.Background(), "cluster", metav1.GetOptions{})
	if err != nil {
		if kapierrs.IsNotFound(err) {
			return false
		}
		e2e.Failf("could not retrieve feature-gate: %v", err)
	}
	featureSet := featureGate.Spec.FeatureSet
	return (featureSet == configv1.TechPreviewNoUpgrade || featureSet == configv1.CustomNoUpgrade)
}

// DoesApiResourceExist searches the list of ApiResources and returns "true" if a given
// apiResourceName Exists. Valid search strings are for example "cloudprivateipconfigs" or "machines".
func DoesApiResourceExist(config *rest.Config, apiResourceName, group string) (bool, error) {
	discoveryClient, err := discovery.NewDiscoveryClientForConfig(config)
	if err != nil {
		return false, err
	}
	_, allResourceList, err := discoveryClient.ServerGroupsAndResources()
	var groupFailed *discovery.ErrGroupDiscoveryFailed
	if errors.As(err, &groupFailed) {
		for gv, err := range groupFailed.Groups {
			if gv.Group == group {
				return false, err
			}
		}
	} else if err != nil {
		return false, err
	}

	for _, apiResourceList := range allResourceList {
		if groupName(group) != groupName(apiResourceList.GroupVersion) {
			continue
		}
		for _, apiResource := range apiResourceList.APIResources {
			if apiResource.Name == apiResourceName {
				return true, nil
			}
		}
	}

	return false, nil
}

func IsNamespaceExist(kubeClient *kubernetes.Clientset, namespace string) (bool, error) {
	_, err := kubeClient.CoreV1().Namespaces().Get(context.Background(), namespace, metav1.GetOptions{})
	if err != nil {
		if kapierrs.IsNotFound(err) {
			e2e.Logf("%s namespace not found", namespace)
			return false, nil
		}
		return false, err
	}

	return true, nil
}

func IsSelfManagedHA(ctx context.Context, configClient clientconfigv1.Interface) (bool, error) {
	infrastructure, err := configClient.ConfigV1().Infrastructures().Get(ctx, "cluster", metav1.GetOptions{})
	if err != nil {
		return false, nil
	}

	return infrastructure.Status.ControlPlaneTopology == configv1.HighlyAvailableTopologyMode, nil
}

func IsManagedServiceCluster(ctx context.Context, adminClient kubernetes.Interface) (bool, error) {
	_, err := adminClient.CoreV1().Namespaces().Get(ctx, "openshift-backplane", metav1.GetOptions{})
	if err == nil {
		return true, nil
	}

	if !kapierrs.IsNotFound(err) {
		return false, err
	}

	return false, nil
}

func IsSingleNode(ctx context.Context, configClient clientconfigv1.Interface) (bool, error) {
	infrastructure, err := configClient.ConfigV1().Infrastructures().Get(ctx, "cluster", metav1.GetOptions{})
	if err != nil {
		return false, nil
	}

	return infrastructure.Status.ControlPlaneTopology == configv1.SingleReplicaTopologyMode, nil
}

func IsHypershift(ctx context.Context, configClient clientconfigv1.Interface) (bool, error) {
	infrastructure, err := configClient.ConfigV1().Infrastructures().Get(ctx, "cluster", metav1.GetOptions{})
	if err != nil {
		return false, nil
	}

	return infrastructure.Status.ControlPlaneTopology == configv1.ExternalTopologyMode, nil
}

// IsMicroShiftCluster returns "true" if a cluster is MicroShift,
// "false" otherwise. It needs kube-admin client as input.
func IsMicroShiftCluster(kubeClient k8sclient.Interface) (bool, error) {
	// MicroShift cluster contains "microshift-version" configmap in "kube-public" namespace
	cm, err := kubeClient.CoreV1().ConfigMaps("kube-public").Get(context.Background(), "microshift-version", metav1.GetOptions{})
	if err != nil {
		if kapierrs.IsNotFound(err) {
			e2e.Logf("microshift-version configmap not found")
			return false, nil
		}
		e2e.Logf("error accessing microshift-version configmap: %v", err)
		return false, err
	}
	if cm == nil {
		e2e.Logf("microshift-version configmap is nil")
		return false, nil
	}
	e2e.Logf("MicroShift cluster with version: %s", cm.Data["version"])
	return true, nil
}

func IsTwoNodeFencing(ctx context.Context, configClient clientconfigv1.Interface) bool {
	infrastructure, err := configClient.ConfigV1().Infrastructures().Get(ctx, "cluster", metav1.GetOptions{})
	if err != nil {
		return false
	}

	return infrastructure.Status.ControlPlaneTopology == configv1.DualReplicaTopologyMode
}

func groupName(groupVersionName string) string {
	return strings.Split(groupVersionName, "/")[0]
}

type staticObject struct {
	APIVersion, Kind, Namespace, Name string
}

func collectConfigManifestsFromDir(configManifestsDir string) (error, []runtime.Object) {
	objects := []runtime.Object{}
	knownObjects := make(map[staticObject]string)

	err := filepath.Walk(configManifestsDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}

		body, err := os.ReadFile(path)
		if err != nil {
			return err
		}

		object := &metav1.TypeMeta{}
		err = yaml.Unmarshal(body, &object)
		if err != nil {
			return err
		}

		if object.APIVersion == "config.openshift.io/v1" {
			switch object.Kind {
			case "Infrastructure":
				config := &configv1.Infrastructure{}
				err = yaml.Unmarshal(body, &config)
				if err != nil {
					return err
				}
				key := staticObject{APIVersion: object.APIVersion, Kind: object.Kind, Namespace: config.Namespace, Name: config.Name}
				if objPath, exists := knownObjects[key]; exists {
					return fmt.Errorf("object %v duplicated under %v", path, objPath)
				}
				objects = append(objects, config)
				knownObjects[key] = path
			case "Network":
				config := &configv1.Network{}
				err = yaml.Unmarshal(body, &config)
				if err != nil {
					return err
				}
				key := staticObject{APIVersion: object.APIVersion, Kind: object.Kind, Namespace: config.Namespace, Name: config.Name}
				if objPath, exists := knownObjects[key]; exists {
					return fmt.Errorf("object %v duplicated under %v", path, objPath)
				}
				objects = append(objects, config)
				knownObjects[key] = path
			default:
				return fmt.Errorf("unknown 'config.openshift.io/v1' kind: %v", object.Kind)
			}
		} else {
			return fmt.Errorf("unknown apiversion: %v", object.APIVersion)
		}

		return nil
	})

	return err, objects
}

func IsCapabilityEnabled(oc *CLI, cap configv1.ClusterVersionCapability) (bool, error) {
	cv, err := oc.AdminConfigClient().ConfigV1().ClusterVersions().Get(context.Background(), "version", metav1.GetOptions{})
	if err != nil {
		return false, err
	}
	for _, capability := range cv.Status.Capabilities.EnabledCapabilities {
		if capability == cap {
			return true, nil
		}
	}
	return false, nil
}

// AllCapabilitiesEnabled returns true if all of the given capabilities are enabled on the cluster.
func AllCapabilitiesEnabled(oc *CLI, caps ...configv1.ClusterVersionCapability) (bool, error) {
	cv, err := oc.AdminConfigClient().ConfigV1().ClusterVersions().Get(context.Background(), "version", metav1.GetOptions{})
	if err != nil {
		return false, err
	}

	enabledCaps := make(map[configv1.ClusterVersionCapability]struct{}, len(cv.Status.Capabilities.EnabledCapabilities))
	for _, c := range cv.Status.Capabilities.EnabledCapabilities {
		enabledCaps[c] = struct{}{}
	}

	for _, c := range caps {
		if _, found := enabledCaps[c]; !found {
			return false, nil
		}
	}

	return true, nil
}

// SkipIfNotPlatform skip the test if supported platforms are not matched
func SkipIfNotPlatform(oc *CLI, platforms ...configv1.PlatformType) {
	var match bool
	infra, err := oc.AdminConfigClient().ConfigV1().Infrastructures().Get(context.Background(), "cluster", metav1.GetOptions{})
	o.Expect(err).NotTo(o.HaveOccurred())
	for _, platform := range platforms {
		if infra.Status.PlatformStatus.Type == platform {
			match = true
			break
		}
	}
	if !match {
		g.Skip("Skip this test scenario because it is not supported on the " + string(infra.Status.PlatformStatus.Type) + " platform")
	}
}

// SkipIfMissingCapabilities skips the test if any of the given cluster capabilities is not enabled.
func SkipIfMissingCapabilities(oc *CLI, caps ...configv1.ClusterVersionCapability) {
	enabled, err := AllCapabilitiesEnabled(oc, caps...)
	o.Expect(err).NotTo(o.HaveOccurred())
	if !enabled {
		g.Skip(fmt.Sprintf("Skip this test scenario because not all of the following capabilities are enabled: %v", caps))
	}
}

// GetClusterRegion get the cluster's region
func GetClusterRegion(oc *CLI) string {
	region, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("node", `-ojsonpath={.items[].metadata.labels.topology\.kubernetes\.io/region}`).Output()
	o.Expect(err).NotTo(o.HaveOccurred())
	return region
}
