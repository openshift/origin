package cmd

import (
	"io/ioutil"
	"reflect"
	"strings"
	"testing"

	"github.com/spf13/pflag"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	kcmd "k8s.io/kubernetes/pkg/kubectl/cmd"

	appsapi "github.com/openshift/origin/pkg/apps/apis/apps"
	buildapi "github.com/openshift/origin/pkg/build/apis/build"
	buildfake "github.com/openshift/origin/pkg/build/generated/internalclientset/fake"
	"github.com/openshift/origin/pkg/oc/cli/util/clientcmd"
)

// TestLogsFlagParity makes sure that our copied flags don't slip during rebases
func TestLogsFlagParity(t *testing.T) {
	kubeCmd := kcmd.NewCmdLogs(nil, ioutil.Discard)
	f := clientcmd.NewFactory(nil)
	originCmd := NewCmdLogs("oc", "logs", f, ioutil.Discard)

	kubeCmd.LocalFlags().VisitAll(func(kubeFlag *pflag.Flag) {
		originFlag := originCmd.LocalFlags().Lookup(kubeFlag.Name)
		if originFlag == nil {
			t.Errorf("missing %v flag", kubeFlag.Name)
			return
		}

		if !reflect.DeepEqual(originFlag, kubeFlag) {
			t.Errorf("flag %v %v does not match %v", kubeFlag.Name, kubeFlag, originFlag)
		}
	})
}

type fakeWriter struct {
	data []byte
}

func (f *fakeWriter) Write(p []byte) (n int, err error) {
	f.data = p
	return len(p), nil
}

func TestRunLogForPipelineStrategy(t *testing.T) {
	bld := buildapi.Build{
		ObjectMeta: metav1.ObjectMeta{
			Name:        "foo-0",
			Namespace:   "foo",
			Annotations: map[string]string{buildapi.BuildJenkinsBlueOceanLogURLAnnotation: "https://foo"},
		},
		Spec: buildapi.BuildSpec{
			CommonSpec: buildapi.CommonSpec{
				Strategy: buildapi.BuildStrategy{
					JenkinsPipelineStrategy: &buildapi.JenkinsPipelineBuildStrategy{},
				},
			},
		},
	}

	fakebc := buildfake.NewSimpleClientset(&bld)
	fakewriter := fakeWriter{}

	testCases := []struct {
		o runtime.Object
	}{
		{
			o: &bld,
		},
		{
			o: &buildapi.BuildConfig{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "foo",
					Name:      "foo",
				},
				Spec: buildapi.BuildConfigSpec{
					CommonSpec: buildapi.CommonSpec{
						Strategy: buildapi.BuildStrategy{
							JenkinsPipelineStrategy: &buildapi.JenkinsPipelineBuildStrategy{},
						},
					},
				},
			},
		},
	}

	for _, tc := range testCases {
		opts := OpenShiftLogsOptions{
			KubeLogOptions: &kcmd.LogsOptions{
				Object:    tc.o,
				Namespace: "foo",
				Out:       &fakewriter,
			},
			Client: fakebc.Build(),
		}
		err := opts.RunLog()
		if err != nil {
			t.Errorf("%#v: RunLog error %v", tc.o, err)
		}
		output := string(fakewriter.data[:])
		if !strings.Contains(output, "https://foo") {
			t.Errorf("%#v: RunLog did not have https://foo, but rather had: %s", tc.o, output)
		}
	}

}

func TestIsPipelineBuild(t *testing.T) {
	testCases := []struct {
		o          runtime.Object
		isPipeline bool
	}{
		{
			o: &buildapi.Build{
				Spec: buildapi.BuildSpec{
					CommonSpec: buildapi.CommonSpec{
						Strategy: buildapi.BuildStrategy{
							JenkinsPipelineStrategy: &buildapi.JenkinsPipelineBuildStrategy{},
						},
					},
				},
			},
			isPipeline: true,
		},
		{
			o: &buildapi.Build{
				Spec: buildapi.BuildSpec{
					CommonSpec: buildapi.CommonSpec{
						Strategy: buildapi.BuildStrategy{
							SourceStrategy: &buildapi.SourceBuildStrategy{},
						},
					},
				},
			},
			isPipeline: false,
		},
		{
			o: &buildapi.BuildConfig{
				Spec: buildapi.BuildConfigSpec{
					CommonSpec: buildapi.CommonSpec{
						Strategy: buildapi.BuildStrategy{
							JenkinsPipelineStrategy: &buildapi.JenkinsPipelineBuildStrategy{},
						},
					},
				},
			},
			isPipeline: true,
		},
		{
			o: &buildapi.BuildConfig{
				Spec: buildapi.BuildConfigSpec{
					CommonSpec: buildapi.CommonSpec{
						Strategy: buildapi.BuildStrategy{
							DockerStrategy: &buildapi.DockerBuildStrategy{},
						},
					},
				},
			},
			isPipeline: false,
		},
		{
			o:          &appsapi.DeploymentConfig{},
			isPipeline: false,
		},
	}

	for _, tc := range testCases {
		isPipeline, _, _, _, _ := isPipelineBuild(tc.o)
		if isPipeline != tc.isPipeline {
			t.Errorf("%#v, unexpected results expected isPipeline %v returned isPipeline %v", tc.o, tc.isPipeline, isPipeline)
		}
	}
}
