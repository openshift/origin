package common

import (
	"fmt"
	"reflect"
	"sort"
	"strings"
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	kapi "k8s.io/kubernetes/pkg/apis/core"

	buildapi "github.com/openshift/origin/pkg/build/apis/build"
	buildclient "github.com/openshift/origin/pkg/build/client"
	buildfake "github.com/openshift/origin/pkg/build/generated/internalclientset/fake"
	buildutil "github.com/openshift/origin/pkg/build/util"
)

func mockBuildConfig(name string) buildapi.BuildConfig {
	appName := strings.Split(name, "-")
	successfulBuildsToKeep := int32(2)
	failedBuildsToKeep := int32(3)
	return buildapi.BuildConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-build", appName[0]),
			Namespace: "namespace",
			Labels: map[string]string{
				"app": appName[0],
			},
		},
		Spec: buildapi.BuildConfigSpec{
			SuccessfulBuildsHistoryLimit: &successfulBuildsToKeep,
			FailedBuildsHistoryLimit:     &failedBuildsToKeep,
		},
	}
}

func mockBuild(name string, phase buildapi.BuildPhase, stamp *metav1.Time) buildapi.Build {
	appName := strings.Split(name, "-")
	return buildapi.Build{
		ObjectMeta: metav1.ObjectMeta{
			Name:              name,
			UID:               types.UID(fmt.Sprintf("uid%v", appName[1])),
			Namespace:         "namespace",
			CreationTimestamp: *stamp,
			Labels: map[string]string{
				"app": appName[0],
				buildapi.BuildConfigLabel: fmt.Sprintf("%v-build", appName[0]),
				"buildconfig":             fmt.Sprintf("%v-build", appName[0]),
			},
			Annotations: map[string]string{
				buildapi.BuildConfigLabel: fmt.Sprintf("%v-build", appName[0]),
			},
		},
		Status: buildapi.BuildStatus{
			Phase:          phase,
			StartTimestamp: stamp,
			Config: &kapi.ObjectReference{
				Name:      fmt.Sprintf("%v-build", appName[0]),
				Namespace: "namespace",
			},
		},
	}
}

// Using a multiple of 4 for length will return a list of buildapi.Build objects
// that are evenly split between all four below build phases.
func mockBuildsList(length int) (buildapi.BuildConfig, []buildapi.Build) {
	var builds []buildapi.Build
	buildPhaseList := []buildapi.BuildPhase{buildapi.BuildPhaseComplete, buildapi.BuildPhaseFailed, buildapi.BuildPhaseError, buildapi.BuildPhaseCancelled}
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

func TestHandleBuildPruning(t *testing.T) {
	var objects []runtime.Object
	buildconfig, builds := mockBuildsList(16)

	objects = append(objects, &buildconfig)
	for index := range builds {
		objects = append(objects, &builds[index])
	}

	buildClient := buildfake.NewSimpleClientset(objects...)

	build, err := buildClient.Build().Builds("namespace").Get("myapp-0", metav1.GetOptions{})
	if err != nil {
		t.Errorf("%v", err)
	}

	buildLister := buildclient.NewClientBuildLister(buildClient.Build())
	buildConfigGetter := buildclient.NewClientBuildConfigLister(buildClient.Build())
	buildDeleter := buildclient.NewClientBuildClient(buildClient)

	bcName := buildutil.ConfigNameForBuild(build)
	successfulStartingBuilds, err := buildutil.BuildConfigBuilds(buildLister, build.Namespace, bcName, func(build *buildapi.Build) bool { return build.Status.Phase == buildapi.BuildPhaseComplete })
	sort.Sort(ByCreationTimestamp(successfulStartingBuilds))

	failedStartingBuilds, err := buildutil.BuildConfigBuilds(buildLister, build.Namespace, bcName, func(build *buildapi.Build) bool {
		return (build.Status.Phase == buildapi.BuildPhaseFailed || build.Status.Phase == buildapi.BuildPhaseError || build.Status.Phase == buildapi.BuildPhaseCancelled)
	})
	sort.Sort(ByCreationTimestamp(failedStartingBuilds))

	if len(successfulStartingBuilds)+len(failedStartingBuilds) != 16 {
		t.Errorf("should start with 16 builds, but started with %v instead", len(successfulStartingBuilds)+len(failedStartingBuilds))
	}

	if err := HandleBuildPruning(bcName, build.Namespace, buildLister, buildConfigGetter, buildDeleter); err != nil {
		t.Errorf("error pruning builds: %v", err)
	}

	successfulRemainingBuilds, err := buildutil.BuildConfigBuilds(buildLister, build.Namespace, bcName, func(build *buildapi.Build) bool { return build.Status.Phase == buildapi.BuildPhaseComplete })
	sort.Sort(ByCreationTimestamp(successfulRemainingBuilds))

	failedRemainingBuilds, err := buildutil.BuildConfigBuilds(buildLister, build.Namespace, bcName, func(build *buildapi.Build) bool {
		return (build.Status.Phase == buildapi.BuildPhaseFailed || build.Status.Phase == buildapi.BuildPhaseError || build.Status.Phase == buildapi.BuildPhaseCancelled)
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
