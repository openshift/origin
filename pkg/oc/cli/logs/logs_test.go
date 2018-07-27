package logs

import (
	"reflect"
	"strings"
	"testing"

	"github.com/spf13/pflag"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	kcmd "k8s.io/kubernetes/pkg/kubectl/cmd"
	"k8s.io/kubernetes/pkg/kubectl/genericclioptions"

	buildv1 "github.com/openshift/api/build/v1"
	buildfake "github.com/openshift/client-go/build/clientset/versioned/fake"
	appsapi "github.com/openshift/origin/pkg/apps/apis/apps"
	buildapi "github.com/openshift/origin/pkg/build/apis/build"
)

// TestLogsFlagParity makes sure that our copied flags don't slip during rebases
func TestLogsFlagParity(t *testing.T) {
	streams := genericclioptions.NewTestIOStreamsDiscard()
	kubeCmd := kcmd.NewCmdLogs(nil, streams)
	originCmd := NewCmdLogs("oc", "logs", nil, streams)

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
	bld := buildv1.Build{
		ObjectMeta: metav1.ObjectMeta{
			Name:        "foo-0",
			Namespace:   "foo",
			Annotations: map[string]string{buildapi.BuildJenkinsBlueOceanLogURLAnnotation: "https://foo"},
		},
		Spec: buildv1.BuildSpec{
			CommonSpec: buildv1.CommonSpec{
				Strategy: buildv1.BuildStrategy{
					JenkinsPipelineStrategy: &buildv1.JenkinsPipelineBuildStrategy{},
				},
			},
		},
	}

	fakebc := buildfake.NewSimpleClientset(&bld)
	streams, _, out, _ := genericclioptions.NewTestIOStreams()

	testCases := []struct {
		o runtime.Object
	}{
		{
			o: &bld,
		},
		{
			o: &buildv1.BuildConfig{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "foo",
					Name:      "foo",
				},
				Spec: buildv1.BuildConfigSpec{
					CommonSpec: buildv1.CommonSpec{
						Strategy: buildv1.BuildStrategy{
							JenkinsPipelineStrategy: &buildv1.JenkinsPipelineBuildStrategy{},
						},
					},
				},
			},
		},
	}

	for _, tc := range testCases {
		o := &LogsOptions{
			IOStreams: streams,
			KubeLogOptions: &kcmd.LogsOptions{
				IOStreams: streams,
				Object:    tc.o,
				Namespace: "foo",
			},
			Client: fakebc.Build(),
		}
		if err := o.runLogPipeline(); err != nil {
			t.Errorf("%#v: RunLog error %v", tc.o, err)
		}
		if !strings.Contains(out.String(), "https://foo") {
			t.Errorf("%#v: RunLog did not have https://foo, but rather had: %s", tc.o, out.String())
		}
	}

}

func TestIsPipelineBuild(t *testing.T) {
	testCases := []struct {
		o          runtime.Object
		isPipeline bool
	}{
		{
			o: &buildv1.Build{
				Spec: buildv1.BuildSpec{
					CommonSpec: buildv1.CommonSpec{
						Strategy: buildv1.BuildStrategy{
							JenkinsPipelineStrategy: &buildv1.JenkinsPipelineBuildStrategy{},
						},
					},
				},
			},
			isPipeline: true,
		},
		{
			o: &buildv1.Build{
				Spec: buildv1.BuildSpec{
					CommonSpec: buildv1.CommonSpec{
						Strategy: buildv1.BuildStrategy{
							SourceStrategy: &buildv1.SourceBuildStrategy{},
						},
					},
				},
			},
			isPipeline: false,
		},
		{
			o: &buildv1.BuildConfig{
				Spec: buildv1.BuildConfigSpec{
					CommonSpec: buildv1.CommonSpec{
						Strategy: buildv1.BuildStrategy{
							JenkinsPipelineStrategy: &buildv1.JenkinsPipelineBuildStrategy{},
						},
					},
				},
			},
			isPipeline: true,
		},
		{
			o: &buildv1.BuildConfig{
				Spec: buildv1.BuildConfigSpec{
					CommonSpec: buildv1.CommonSpec{
						Strategy: buildv1.BuildStrategy{
							DockerStrategy: &buildv1.DockerBuildStrategy{},
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
