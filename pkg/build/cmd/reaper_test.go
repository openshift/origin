package cmd

import (
	"fmt"
	"reflect"
	"strings"
	"testing"
	"time"

	kapi "k8s.io/kubernetes/pkg/api"
	kerrors "k8s.io/kubernetes/pkg/api/errors"
	ktestclient "k8s.io/kubernetes/pkg/client/unversioned/testclient"
	"k8s.io/kubernetes/pkg/labels"
	"k8s.io/kubernetes/pkg/runtime"
	ktypes "k8s.io/kubernetes/pkg/types"
	"k8s.io/kubernetes/pkg/util/validation"

	buildapi "github.com/openshift/origin/pkg/build/api"
	_ "github.com/openshift/origin/pkg/build/api/install"
	buildutil "github.com/openshift/origin/pkg/build/util"
	"github.com/openshift/origin/pkg/client/testclient"
)

var (
	configName      = strings.Repeat("a", validation.DNS1123LabelMaxLength)
	longConfigNameA = strings.Repeat("0", 250) + "a"
	longConfigNameB = strings.Repeat("0", 250) + "b"
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
			Name:        fmt.Sprintf("build-%d", version),
			UID:         ktypes.UID(fmt.Sprintf("build-%d", version)),
			Namespace:   "default",
			Labels:      map[string]string{buildapi.BuildConfigLabel: buildapi.LabelValue(configName)},
			Annotations: map[string]string{buildapi.BuildConfigAnnotation: configName},
		},
	}
}

func makeDeprecatedBuild(configName string, version int) buildapi.Build {
	return buildapi.Build{
		ObjectMeta: kapi.ObjectMeta{
			Name:        fmt.Sprintf("build-%d", version),
			UID:         ktypes.UID(fmt.Sprintf("build-%d", version)),
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
	fake.PrependReactor("list", "builds", func(action ktestclient.Action) (handled bool, ret runtime.Object, err error) {
		selector := action.(ktestclient.ListAction).GetListRestrictions().Labels
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

func actionsAreEqual(a, b ktestclient.Action) bool {
	if reflect.DeepEqual(a, b) {
		return true
	}
	// If it's an update action, we will take a better look at the object
	if a.GetVerb() == "update" && b.GetVerb() == "update" &&
		a.GetNamespace() == b.GetNamespace() &&
		a.GetResource() == b.GetResource() &&
		a.GetSubresource() == b.GetSubresource() {
		ret := reflect.DeepEqual(a.(ktestclient.UpdateAction).GetObject(), b.(ktestclient.UpdateAction).GetObject())
		return ret
	}
	return false
}

func TestStop(t *testing.T) {
	notFound := func() runtime.Object {
		return &(kerrors.NewNotFound(buildapi.Resource("BuildConfig"), configName).ErrStatus)
	}

	tests := map[string]struct {
		targetBC string
		oc       *testclient.Fake
		expected []ktestclient.Action
		err      bool
	}{
		"simple stop": {
			targetBC: configName,
			oc:       newBuildListFake(makeBuildConfig(configName, 0, false)),
			expected: []ktestclient.Action{
				ktestclient.NewGetAction("buildconfigs", "default", configName),
				// Since there are no builds associated with this build config, do not expect an update
				ktestclient.NewListAction("builds", "default", kapi.ListOptions{LabelSelector: buildutil.BuildConfigSelector(configName)}),
				ktestclient.NewListAction("builds", "default", kapi.ListOptions{LabelSelector: buildutil.BuildConfigSelectorDeprecated(configName)}),
				ktestclient.NewDeleteAction("buildconfigs", "default", configName),
			},
			err: false,
		},
		"multiple builds": {
			targetBC: configName,
			oc:       newBuildListFake(makeBuildConfig(configName, 4, false), makeBuildList(configName, 4)),
			expected: []ktestclient.Action{
				ktestclient.NewGetAction("buildconfigs", "default", configName),
				ktestclient.NewListAction("builds", "default", kapi.ListOptions{LabelSelector: buildutil.BuildConfigSelector(configName)}),
				ktestclient.NewListAction("builds", "default", kapi.ListOptions{LabelSelector: buildutil.BuildConfigSelectorDeprecated(configName)}),
				ktestclient.NewGetAction("buildconfigs", "default", configName),                              // Second GET to enable conflict retry logic
				ktestclient.NewUpdateAction("buildconfigs", "default", makeBuildConfig(configName, 4, true)), // Because this bc has builds, it is paused
				ktestclient.NewDeleteAction("builds", "default", "build-1"),
				ktestclient.NewDeleteAction("builds", "default", "build-2"),
				ktestclient.NewDeleteAction("builds", "default", "build-3"),
				ktestclient.NewDeleteAction("builds", "default", "build-4"),
				ktestclient.NewDeleteAction("buildconfigs", "default", configName),
			},
			err: false,
		},
		"long name builds": {
			targetBC: longConfigNameA,
			oc:       newBuildListFake(makeBuildConfig(longConfigNameA, 4, false), makeBuildList(longConfigNameA, 4), makeBuildList(longConfigNameB, 4)),
			expected: []ktestclient.Action{
				ktestclient.NewGetAction("buildconfigs", "default", longConfigNameA),
				ktestclient.NewListAction("builds", "default", kapi.ListOptions{LabelSelector: buildutil.BuildConfigSelector(longConfigNameA)}),
				ktestclient.NewListAction("builds", "default", kapi.ListOptions{LabelSelector: buildutil.BuildConfigSelectorDeprecated(longConfigNameA)}),
				ktestclient.NewGetAction("buildconfigs", "default", longConfigNameA),                              // Second GET to enable conflict retry logic
				ktestclient.NewUpdateAction("buildconfigs", "default", makeBuildConfig(longConfigNameA, 4, true)), // Because this bc has builds, it is paused
				ktestclient.NewDeleteAction("builds", "default", "build-1"),
				ktestclient.NewDeleteAction("builds", "default", "build-2"),
				ktestclient.NewDeleteAction("builds", "default", "build-3"),
				ktestclient.NewDeleteAction("builds", "default", "build-4"),
				ktestclient.NewDeleteAction("buildconfigs", "default", longConfigNameA),
			},
			err: false,
		},
		"no config, no or some builds": {
			targetBC: configName,
			oc:       testclient.NewSimpleFake(notFound(), makeBuildList(configName, 2)),
			expected: []ktestclient.Action{
				ktestclient.NewGetAction("buildconfigs", "default", configName),
			},
			err: true,
		},
		"config, no builds": {
			targetBC: configName,
			oc:       testclient.NewSimpleFake(makeBuildConfig(configName, 0, false)),
			expected: []ktestclient.Action{
				ktestclient.NewGetAction("buildconfigs", "default", configName),
				ktestclient.NewListAction("builds", "default", kapi.ListOptions{LabelSelector: buildutil.BuildConfigSelector(configName)}),
				ktestclient.NewListAction("builds", "default", kapi.ListOptions{LabelSelector: buildutil.BuildConfigSelectorDeprecated(configName)}),
				ktestclient.NewDeleteAction("buildconfigs", "default", configName),
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
