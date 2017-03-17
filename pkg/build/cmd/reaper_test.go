package cmd

import (
	"fmt"
	"reflect"
	"strings"
	"testing"
	"time"

	kapi "k8s.io/kubernetes/pkg/api"
	kerrors "k8s.io/kubernetes/pkg/api/errors"
	"k8s.io/kubernetes/pkg/api/unversioned"
	"k8s.io/kubernetes/pkg/client/testing/core"
	"k8s.io/kubernetes/pkg/labels"
	"k8s.io/kubernetes/pkg/runtime"
	ktypes "k8s.io/kubernetes/pkg/types"
	"k8s.io/kubernetes/pkg/util/validation"

	buildapi "github.com/openshift/origin/pkg/build/api"
	buildutil "github.com/openshift/origin/pkg/build/util"
	"github.com/openshift/origin/pkg/client/testclient"
)

var (
	configName           = strings.Repeat("a", validation.DNS1123LabelMaxLength)
	longConfigNameA      = strings.Repeat("0", 250) + "a"
	longConfigNameB      = strings.Repeat("0", 250) + "b"
	buildsResource       = unversioned.GroupVersionResource{Group: "", Version: "", Resource: "builds"}
	buildConfigsResource = unversioned.GroupVersionResource{Group: "", Version: "", Resource: "buildconfigs"}
)

func makeBuildConfig(configName string, version int64, deleting bool) *buildapi.BuildConfig {
	ret := &buildapi.BuildConfig{
		ObjectMeta: kapi.ObjectMeta{
			Name:        configName,
			Namespace:   "default",
			Annotations: make(map[string]string),
		},
		Spec: buildapi.BuildConfigSpec{},
		Status: buildapi.BuildConfigStatus{
			LastVersion: version,
		},
	}
	if deleting {
		ret.Annotations[buildapi.BuildConfigPausedAnnotation] = "true"
	}
	return ret
}

func makeBuild(configName string, version int) buildapi.Build {
	return buildapi.Build{
		ObjectMeta: kapi.ObjectMeta{
			Name:        fmt.Sprintf("build-%s-%d", configName, version),
			UID:         ktypes.UID(fmt.Sprintf("build-%s-%d", configName, version)),
			Namespace:   "default",
			Labels:      map[string]string{buildapi.BuildConfigLabel: buildapi.LabelValue(configName)},
			Annotations: map[string]string{buildapi.BuildConfigAnnotation: configName},
		},
	}
}

func makeDeprecatedBuild(configName string, version int) buildapi.Build {
	return buildapi.Build{
		ObjectMeta: kapi.ObjectMeta{
			Name:        fmt.Sprintf("build-%s-%d", configName, version),
			UID:         ktypes.UID(fmt.Sprintf("build-%s-%d", configName, version)),
			Namespace:   "default",
			Labels:      map[string]string{buildapi.BuildConfigLabelDeprecated: buildapi.LabelValue(configName)},
			Annotations: map[string]string{buildapi.BuildConfigAnnotation: configName},
		},
	}
}

func makeBuildList(configName string, version int) *buildapi.BuildList {
	if version%2 != 0 {
		panic("version needs be even")
	}
	list := &buildapi.BuildList{}

	for i := 1; i <= version; i += 2 {
		list.Items = append(list.Items, makeBuild(configName, i))
		list.Items = append(list.Items, makeDeprecatedBuild(configName, i+1))
	}
	return list
}

func newBuildListFake(objects ...runtime.Object) *testclient.Fake {
	fake := testclient.NewSimpleFake(objects...)
	fake.PrependReactor("list", "builds", func(action core.Action) (handled bool, ret runtime.Object, err error) {
		selector := action.(core.ListAction).GetListRestrictions().Labels
		retList := &buildapi.BuildList{}
		for _, obj := range objects {
			list, ok := obj.(*buildapi.BuildList)
			if !ok {
				continue
			}
			for _, build := range list.Items {
				if selector.Matches(labels.Set(build.Labels)) {
					retList.Items = append(retList.Items, build)
				}
			}
		}
		return true, retList, nil
	})
	return fake
}

func actionsAreEqual(a, b core.Action) bool {
	if reflect.DeepEqual(a, b) {
		return true
	}
	// If it's an update action, we will take a better look at the object
	if a.GetVerb() == "update" && b.GetVerb() == "update" &&
		a.GetNamespace() == b.GetNamespace() &&
		a.GetResource() == b.GetResource() &&
		a.GetSubresource() == b.GetSubresource() {
		ret := reflect.DeepEqual(a.(core.UpdateAction).GetObject(), b.(core.UpdateAction).GetObject())
		return ret
	}
	return false
}

func TestStop(t *testing.T) {
	notFoundClient := &testclient.Fake{} //(notFound(), makeBuildList(configName, 2))
	notFoundClient.AddReactor("*", "*", func(action core.Action) (handled bool, ret runtime.Object, err error) {
		return true, nil, kerrors.NewNotFound(buildapi.Resource("BuildConfig"), configName)
	})

	tests := map[string]struct {
		targetBC string
		oc       *testclient.Fake
		expected []core.Action
		err      bool
	}{
		"simple stop": {
			targetBC: configName,
			oc:       newBuildListFake(makeBuildConfig(configName, 0, false)),
			expected: []core.Action{
				core.NewGetAction(buildConfigsResource, "default", configName),
				// Since there are no builds associated with this build config, do not expect an update
				core.NewListAction(buildsResource, "default", kapi.ListOptions{LabelSelector: buildutil.BuildConfigSelector(configName)}),
				core.NewListAction(buildsResource, "default", kapi.ListOptions{LabelSelector: buildutil.BuildConfigSelectorDeprecated(configName)}),
				core.NewDeleteAction(buildConfigsResource, "default", configName),
			},
			err: false,
		},
		"multiple builds": {
			targetBC: configName,
			oc:       newBuildListFake(makeBuildConfig(configName, 4, false), makeBuildList(configName, 4)),
			expected: []core.Action{
				core.NewGetAction(buildConfigsResource, "default", configName),
				core.NewListAction(buildsResource, "default", kapi.ListOptions{LabelSelector: buildutil.BuildConfigSelector(configName)}),
				core.NewListAction(buildsResource, "default", kapi.ListOptions{LabelSelector: buildutil.BuildConfigSelectorDeprecated(configName)}),
				core.NewGetAction(buildConfigsResource, "default", configName),                              // Second GET to enable conflict retry logic
				core.NewUpdateAction(buildConfigsResource, "default", makeBuildConfig(configName, 4, true)), // Because this bc has builds, it is paused
				core.NewDeleteAction(buildsResource, "default", "build-"+configName+"-1"),
				core.NewDeleteAction(buildsResource, "default", "build-"+configName+"-2"),
				core.NewDeleteAction(buildsResource, "default", "build-"+configName+"-3"),
				core.NewDeleteAction(buildsResource, "default", "build-"+configName+"-4"),
				core.NewDeleteAction(buildConfigsResource, "default", configName),
			},
			err: false,
		},
		"long name builds": {
			targetBC: longConfigNameA,
			oc:       newBuildListFake(makeBuildConfig(longConfigNameA, 4, false), makeBuildList(longConfigNameA, 4), makeBuildList(longConfigNameB, 4)),
			expected: []core.Action{
				core.NewGetAction(buildConfigsResource, "default", longConfigNameA),
				core.NewListAction(buildsResource, "default", kapi.ListOptions{LabelSelector: buildutil.BuildConfigSelector(longConfigNameA)}),
				core.NewListAction(buildsResource, "default", kapi.ListOptions{LabelSelector: buildutil.BuildConfigSelectorDeprecated(longConfigNameA)}),
				core.NewGetAction(buildConfigsResource, "default", longConfigNameA),                              // Second GET to enable conflict retry logic
				core.NewUpdateAction(buildConfigsResource, "default", makeBuildConfig(longConfigNameA, 4, true)), // Because this bc has builds, it is paused
				core.NewDeleteAction(buildsResource, "default", "build-"+longConfigNameA+"-1"),
				core.NewDeleteAction(buildsResource, "default", "build-"+longConfigNameA+"-2"),
				core.NewDeleteAction(buildsResource, "default", "build-"+longConfigNameA+"-3"),
				core.NewDeleteAction(buildsResource, "default", "build-"+longConfigNameA+"-4"),
				core.NewDeleteAction(buildConfigsResource, "default", longConfigNameA),
			},
			err: false,
		},
		"no config, no or some builds": {
			targetBC: configName,
			oc:       notFoundClient,
			expected: []core.Action{
				core.NewGetAction(buildConfigsResource, "default", configName),
			},
			err: true,
		},
		"config, no builds": {
			targetBC: configName,
			oc:       testclient.NewSimpleFake(makeBuildConfig(configName, 0, false)),
			expected: []core.Action{
				core.NewGetAction(buildConfigsResource, "default", configName),
				core.NewListAction(buildsResource, "default", kapi.ListOptions{LabelSelector: buildutil.BuildConfigSelector(configName)}),
				core.NewListAction(buildsResource, "default", kapi.ListOptions{LabelSelector: buildutil.BuildConfigSelectorDeprecated(configName)}),
				core.NewDeleteAction(buildConfigsResource, "default", configName),
			},
			err: false,
		},
	}

	for testName, test := range tests {
		reaper := &BuildConfigReaper{oc: test.oc, pollInterval: time.Millisecond, timeout: time.Millisecond}
		err := reaper.Stop("default", test.targetBC, 1*time.Second, nil)

		if !test.err && err != nil {
			t.Errorf("%s: unexpected error: %v", testName, err)
		}
		if test.err && err == nil {
			t.Errorf("%s: expected an error", testName)
		}
		if len(test.oc.Actions()) != len(test.expected) {
			t.Fatalf("%s: unexpected actions: %v, expected %v", testName, test.oc.Actions(), test.expected)
		}
		for j, actualAction := range test.oc.Actions() {
			if !actionsAreEqual(actualAction, test.expected[j]) {
				t.Errorf("%s: unexpected action: %v, expected %v", testName, actualAction, test.expected[j])
			}
		}
	}
}
