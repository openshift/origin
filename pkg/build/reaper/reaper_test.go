package reaper

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
	"k8s.io/kubernetes/pkg/util/validation"

	buildapi "github.com/openshift/origin/pkg/build/api"
	_ "github.com/openshift/origin/pkg/build/api/install"
	buildutil "github.com/openshift/origin/pkg/build/util"
	"github.com/openshift/origin/pkg/client/testclient"
)

var (
	configName = strings.Repeat("a", validation.DNS1123LabelMaxLength)
)

func makeBuildConfig(version int, deleting bool) *buildapi.BuildConfig {
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

func makeBuild(version int) buildapi.Build {
	return buildapi.Build{
		ObjectMeta: kapi.ObjectMeta{
			Name:        fmt.Sprintf("build-%d", version),
			Namespace:   "default",
			Labels:      map[string]string{buildapi.BuildConfigLabel: buildapi.LabelValue(configName)},
			Annotations: map[string]string{buildapi.BuildConfigAnnotation: configName},
		},
	}
}

func makeDeprecatedBuild(version int) buildapi.Build {
	return buildapi.Build{
		ObjectMeta: kapi.ObjectMeta{
			Name:        fmt.Sprintf("build-%d", version),
			Namespace:   "default",
			Labels:      map[string]string{buildapi.BuildConfigLabelDeprecated: buildapi.LabelValue(configName)},
			Annotations: map[string]string{buildapi.BuildConfigAnnotation: configName},
		},
	}
}

func makeBuildList(version int) *buildapi.BuildList {
	if version%2 != 0 {
		panic("version needs be even")
	}
	list := &buildapi.BuildList{}

	for i := 1; i <= version; i += 2 {
		list.Items = append(list.Items, makeBuild(i))
		list.Items = append(list.Items, makeDeprecatedBuild(i+1))
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
		return &(kerrors.NewNotFound(buildapi.Resource("BuildConfig"), configName).(*kerrors.StatusError).ErrStatus)
	}

	tests := map[string]struct {
		oc       *testclient.Fake
		expected []ktestclient.Action
		err      bool
	}{
		"simple stop": {
			oc: newBuildListFake(makeBuildConfig(0, false)),
			expected: []ktestclient.Action{
				ktestclient.NewGetAction("buildconfigs", "default", configName),
				ktestclient.NewUpdateAction("buildconfigs", "default", makeBuildConfig(0, true)),
				ktestclient.NewListAction("builds", "default", kapi.ListOptions{LabelSelector: buildutil.BuildConfigSelector(configName)}),
				ktestclient.NewListAction("builds", "default", kapi.ListOptions{LabelSelector: buildutil.BuildConfigSelectorDeprecated(configName)}),
				ktestclient.NewDeleteAction("buildconfigs", "default", configName),
			},
			err: false,
		},
		"multiple builds": {
			oc: newBuildListFake(makeBuildConfig(4, false), makeBuildList(4)),
			expected: []ktestclient.Action{
				ktestclient.NewGetAction("buildconfigs", "default", configName),
				ktestclient.NewUpdateAction("buildconfigs", "default", makeBuildConfig(4, true)),
				ktestclient.NewListAction("builds", "default", kapi.ListOptions{LabelSelector: buildutil.BuildConfigSelector(configName)}),
				ktestclient.NewDeleteAction("builds", "default", "build-1"),
				ktestclient.NewDeleteAction("builds", "default", "build-3"),
				ktestclient.NewListAction("builds", "default", kapi.ListOptions{LabelSelector: buildutil.BuildConfigSelectorDeprecated(configName)}),
				ktestclient.NewDeleteAction("builds", "default", "build-2"),
				ktestclient.NewDeleteAction("builds", "default", "build-4"),
				ktestclient.NewDeleteAction("buildconfigs", "default", configName),
			},
			err: false,
		},
		"no config, some builds": {
			oc: newBuildListFake(makeBuildList(2)),
			expected: []ktestclient.Action{
				ktestclient.NewGetAction("buildconfigs", "default", configName),
				ktestclient.NewListAction("builds", "default", kapi.ListOptions{LabelSelector: buildutil.BuildConfigSelector(configName)}),
				ktestclient.NewDeleteAction("builds", "default", "build-1"),
				ktestclient.NewListAction("builds", "default", kapi.ListOptions{LabelSelector: buildutil.BuildConfigSelectorDeprecated(configName)}),
				ktestclient.NewDeleteAction("builds", "default", "build-2"),
			},
			err: false,
		},
		"no config, no builds": {
			oc: testclient.NewSimpleFake(notFound()),
			expected: []ktestclient.Action{
				ktestclient.NewGetAction("buildconfigs", "default", configName),
				ktestclient.NewListAction("builds", "default", kapi.ListOptions{LabelSelector: buildutil.BuildConfigSelector(configName)}),
				ktestclient.NewListAction("builds", "default", kapi.ListOptions{LabelSelector: buildutil.BuildConfigSelectorDeprecated(configName)}),
			},
			err: true,
		},
		"config, no builds": {
			oc: testclient.NewSimpleFake(makeBuildConfig(0, false)),
			expected: []ktestclient.Action{
				ktestclient.NewGetAction("buildconfigs", "default", configName),
				ktestclient.NewUpdateAction("buildconfigs", "default", makeBuildConfig(0, true)),
				ktestclient.NewListAction("builds", "default", kapi.ListOptions{LabelSelector: buildutil.BuildConfigSelector(configName)}),
				ktestclient.NewListAction("builds", "default", kapi.ListOptions{LabelSelector: buildutil.BuildConfigSelectorDeprecated(configName)}),
				ktestclient.NewDeleteAction("buildconfigs", "default", configName),
			},
			err: false,
		},
	}

	for testName, test := range tests {
		reaper := &BuildConfigReaper{oc: test.oc, pollInterval: time.Millisecond, timeout: time.Millisecond}
		err := reaper.Stop("default", configName, 1*time.Second, nil)

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
