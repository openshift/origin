package common

import (
	"fmt"
	"reflect"
	"sort"
	"strings"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"

	buildv1 "github.com/openshift/api/build/v1"
	buildfake "github.com/openshift/client-go/build/clientset/versioned/fake"
	buildclientv1 "github.com/openshift/client-go/build/clientset/versioned/typed/build/v1"
	buildlisterv1 "github.com/openshift/client-go/build/listers/build/v1"
	sharedbuildutil "github.com/openshift/library-go/pkg/build/buildutil"
	buildutil "github.com/openshift/openshift-controller-manager/pkg/build/buildutil"
)

func mockBuildConfig(name string) buildv1.BuildConfig {
	appName := strings.Split(name, "-")
	successfulBuildsToKeep := int32(2)
	failedBuildsToKeep := int32(3)
	return buildv1.BuildConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-build", appName[0]),
			Namespace: "namespace",
			Labels: map[string]string{
				"app": appName[0],
			},
		},
		Spec: buildv1.BuildConfigSpec{
			SuccessfulBuildsHistoryLimit: &successfulBuildsToKeep,
			FailedBuildsHistoryLimit:     &failedBuildsToKeep,
		},
	}
}

func mockBuild(name string, phase buildv1.BuildPhase, stamp *metav1.Time) buildv1.Build {
	appName := strings.Split(name, "-")
	return buildv1.Build{
		ObjectMeta: metav1.ObjectMeta{
			Name:              name,
			UID:               types.UID(fmt.Sprintf("uid%v", appName[1])),
			Namespace:         "namespace",
			CreationTimestamp: *stamp,
			Labels: map[string]string{
				"app":                    appName[0],
				buildv1.BuildConfigLabel: fmt.Sprintf("%v-build", appName[0]),
				"buildconfig":            fmt.Sprintf("%v-build", appName[0]),
			},
			Annotations: map[string]string{
				buildv1.BuildConfigLabel: fmt.Sprintf("%v-build", appName[0]),
			},
		},
		Status: buildv1.BuildStatus{
			Phase:          phase,
			StartTimestamp: stamp,
			Config: &corev1.ObjectReference{
				Name:      fmt.Sprintf("%v-build", appName[0]),
				Namespace: "namespace",
			},
		},
	}
}

// Using a multiple of 4 for length will return a list of buildv1.Build objects
// that are evenly split between all four below build phases.
func mockBuildsList(length int) (buildv1.BuildConfig, []buildv1.Build) {
	var builds []buildv1.Build
	buildPhaseList := []buildv1.BuildPhase{buildv1.BuildPhaseComplete, buildv1.BuildPhaseFailed, buildv1.BuildPhaseError, buildv1.BuildPhaseCancelled}
	addOrSubtract := []string{"+", "-"}

	j := 0
	for i := 0; i < length; i++ {
		duration, _ := time.ParseDuration(fmt.Sprintf("%v%vh", addOrSubtract[i%2], i))
		startTime := metav1.NewTime(time.Now().Add(duration))
		build := mockBuild(fmt.Sprintf("myapp-%v", i), buildPhaseList[j], &startTime)
		builds = append(builds, build)
		j++
		if j == 4 {
			j = 0
		}
	}

	return mockBuildConfig("myapp"), builds
}

type fakeBuildLister struct {
	client    buildclientv1.BuildsGetter
	namespace string
}

func (l *fakeBuildLister) List(selector labels.Selector) (ret []*buildv1.Build, err error) {
	builds, err := l.client.Builds(l.namespace).List(metav1.ListOptions{LabelSelector: selector.String()})
	if err != nil {
		return nil, err
	}
	result := make([]*buildv1.Build, len(builds.Items))
	for i := range builds.Items {
		result[i] = &builds.Items[i]
	}
	return result, nil
}

func (l *fakeBuildLister) Get(name string) (*buildv1.Build, error) {
	return l.client.Builds(l.namespace).Get(name, metav1.GetOptions{})
}

func (l *fakeBuildLister) Builds(namespace string) buildlisterv1.BuildNamespaceLister {
	return l
}

type fakeBuildConfigLister struct {
	client    buildclientv1.BuildConfigsGetter
	namespace string
}

func (l *fakeBuildConfigLister) List(selector labels.Selector) (ret []*buildv1.BuildConfig, err error) {
	buildConfigs, err := l.client.BuildConfigs(l.namespace).List(metav1.ListOptions{LabelSelector: selector.String()})
	if err != nil {
		return nil, err
	}
	result := make([]*buildv1.BuildConfig, len(buildConfigs.Items))
	for i := range buildConfigs.Items {
		result[i] = &buildConfigs.Items[i]
	}
	return result, nil
}

func (l *fakeBuildConfigLister) Get(name string) (*buildv1.BuildConfig, error) {
	return l.client.BuildConfigs(l.namespace).Get(name, metav1.GetOptions{})
}

func (l *fakeBuildConfigLister) BuildConfigs(namespace string) buildlisterv1.BuildConfigNamespaceLister {
	return l
}

func TestHandleBuildPruning(t *testing.T) {
	var objects []runtime.Object
	buildconfig, builds := mockBuildsList(16)

	objects = append(objects, &buildconfig)
	for index := range builds {
		objects = append(objects, &builds[index])
	}

	buildClient := buildfake.NewSimpleClientset(objects...)

	build, err := buildClient.BuildV1().Builds("namespace").Get("myapp-0", metav1.GetOptions{})
	if err != nil {
		t.Errorf("%v", err)
	}

	bcName := sharedbuildutil.ConfigNameForBuild(build)
	successfulStartingBuilds, err := buildutil.BuildConfigBuilds(buildClient.BuildV1(), build.Namespace, bcName, func(build *buildv1.Build) bool { return build.Status.Phase == buildv1.BuildPhaseComplete })
	sort.Sort(ByCreationTimestamp(successfulStartingBuilds))

	failedStartingBuilds, err := buildutil.BuildConfigBuilds(buildClient.BuildV1(), build.Namespace, bcName, func(build *buildv1.Build) bool {
		return build.Status.Phase == buildv1.BuildPhaseFailed || build.Status.Phase == buildv1.BuildPhaseError || build.Status.Phase == buildv1.BuildPhaseCancelled
	})
	sort.Sort(ByCreationTimestamp(failedStartingBuilds))

	if len(successfulStartingBuilds)+len(failedStartingBuilds) != 16 {
		t.Errorf("should start with 16 builds, but started with %v instead", len(successfulStartingBuilds)+len(failedStartingBuilds))
	}

	buildLister := &fakeBuildLister{client: buildClient.BuildV1(), namespace: "namespace"}
	buildConfigLister := &fakeBuildConfigLister{client: buildClient.BuildV1(), namespace: "namespace"}

	if err := HandleBuildPruning(bcName, build.Namespace, buildLister, buildConfigLister, buildClient.BuildV1()); err != nil {
		t.Errorf("error pruning builds: %v", err)
	}

	successfulRemainingBuilds, err := buildutil.BuildConfigBuilds(buildClient.BuildV1(), build.Namespace, bcName, func(build *buildv1.Build) bool { return build.Status.Phase == buildv1.BuildPhaseComplete })
	sort.Sort(ByCreationTimestamp(successfulRemainingBuilds))

	failedRemainingBuilds, err := buildutil.BuildConfigBuilds(buildClient.BuildV1(), build.Namespace, bcName, func(build *buildv1.Build) bool {
		return build.Status.Phase == buildv1.BuildPhaseFailed || build.Status.Phase == buildv1.BuildPhaseError || build.Status.Phase == buildv1.BuildPhaseCancelled
	})
	sort.Sort(ByCreationTimestamp(failedRemainingBuilds))

	if len(successfulRemainingBuilds)+len(failedRemainingBuilds) != 5 {
		t.Errorf("there should only be 5 builds left, but instead there are %v", len(successfulRemainingBuilds)+len(failedRemainingBuilds))
	}

	if !reflect.DeepEqual(successfulStartingBuilds[:2], successfulRemainingBuilds) {
		t.Errorf("expected the two most recent successful builds should be left, but instead there were %v: %v", len(successfulRemainingBuilds), successfulRemainingBuilds)
	}

	if !reflect.DeepEqual(failedStartingBuilds[:3], failedRemainingBuilds) {
		t.Errorf("expected the three most recent failed builds to be left, but instead there were %v: %v", len(failedRemainingBuilds), failedRemainingBuilds)
	}

}
