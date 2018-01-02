package describe

import (
	"bytes"
	"io/ioutil"
	"reflect"
	"strings"
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/kubernetes/pkg/api/legacyscheme"
	kapi "k8s.io/kubernetes/pkg/apis/core"
	kprinters "k8s.io/kubernetes/pkg/printers"

	"github.com/openshift/origin/pkg/api"
	appsapi "github.com/openshift/origin/pkg/apps/apis/apps"
	authorizationapi "github.com/openshift/origin/pkg/authorization/apis/authorization"
	buildapi "github.com/openshift/origin/pkg/build/apis/build"
	imageapi "github.com/openshift/origin/pkg/image/apis/image"
	oauthapi "github.com/openshift/origin/pkg/oauth/apis/oauth"
	projectapi "github.com/openshift/origin/pkg/project/apis/project"
	securityapi "github.com/openshift/origin/pkg/security/apis/security"
	templateapi "github.com/openshift/origin/pkg/template/apis/template"
)

// PrinterCoverageExceptions is the list of API types that do NOT have corresponding printers
// If you add something to this list, explain why it doesn't need printing.  waaaa is not a valid
// reason.
var PrinterCoverageExceptions = []reflect.Type{
	reflect.TypeOf(&imageapi.DockerImage{}),         // not a top level resource
	reflect.TypeOf(&imageapi.ImageStreamImport{}),   // normal users don't ever look at these
	reflect.TypeOf(&buildapi.BuildLog{}),            // just a marker type
	reflect.TypeOf(&buildapi.BuildLogOptions{}),     // just a marker type
	reflect.TypeOf(&appsapi.DeploymentRequest{}),    // normal users don't ever look at these
	reflect.TypeOf(&appsapi.DeploymentLog{}),        // just a marker type
	reflect.TypeOf(&appsapi.DeploymentLogOptions{}), // just a marker type

	// these resources can't be "GET"ed, so we probably don't need a printer for them
	reflect.TypeOf(&authorizationapi.SubjectAccessReviewResponse{}),
	reflect.TypeOf(&authorizationapi.ResourceAccessReviewResponse{}),
	reflect.TypeOf(&authorizationapi.SubjectAccessReview{}),
	reflect.TypeOf(&imageapi.ImageSignature{}),
	reflect.TypeOf(&authorizationapi.ResourceAccessReview{}),
	reflect.TypeOf(&authorizationapi.LocalSubjectAccessReview{}),
	reflect.TypeOf(&authorizationapi.LocalResourceAccessReview{}),
	reflect.TypeOf(&authorizationapi.SelfSubjectRulesReview{}),
	reflect.TypeOf(&authorizationapi.SubjectRulesReview{}),
	reflect.TypeOf(&buildapi.BuildLog{}),
	reflect.TypeOf(&buildapi.BinaryBuildRequestOptions{}),
	reflect.TypeOf(&buildapi.BuildRequest{}),
	reflect.TypeOf(&buildapi.BuildLogOptions{}),
	reflect.TypeOf(&securityapi.PodSecurityPolicySubjectReview{}),
	reflect.TypeOf(&securityapi.PodSecurityPolicySelfSubjectReview{}),
	reflect.TypeOf(&securityapi.PodSecurityPolicyReview{}),
	reflect.TypeOf(&oauthapi.OAuthRedirectReference{}),
}

// MissingPrinterCoverageExceptions is the list of types that were missing printer methods when I started
// You should never add to this list
// TODO printers should be added for these types
var MissingPrinterCoverageExceptions = []reflect.Type{
	reflect.TypeOf(&appsapi.DeploymentConfigRollback{}),
	reflect.TypeOf(&imageapi.ImageStreamMapping{}),
	reflect.TypeOf(&projectapi.ProjectRequest{}),
}

func TestPrinterCoverage(t *testing.T) {
	printer := NewHumanReadablePrinter(nil, nil, kprinters.PrintOptions{})

main:
	for _, apiType := range legacyscheme.Scheme.KnownTypes(api.SchemeGroupVersion) {
		if !strings.Contains(apiType.PkgPath(), "github.com/openshift/origin") || strings.Contains(apiType.PkgPath(), "github.com/openshift/origin/vendor/") {
			continue
		}

		ptrType := reflect.PtrTo(apiType)
		for _, exception := range PrinterCoverageExceptions {
			if ptrType == exception {
				continue main
			}
		}
		for _, exception := range MissingPrinterCoverageExceptions {
			if ptrType == exception {
				continue main
			}
		}

		newStructValue := reflect.New(apiType)
		newStruct := newStructValue.Interface()

		if err := printer.PrintObj(newStruct.(runtime.Object), ioutil.Discard); (err != nil) && strings.Contains(err.Error(), "error: unknown type ") {
			t.Errorf("missing printer for %v.  Check pkg/cmd/cli/describe/printer.go", apiType)
		}
	}
}

func TestFormatResourceName(t *testing.T) {
	tests := []struct {
		kind, name string
		want       string
	}{
		{"", "", ""},
		{"", "name", "name"},
		{"kind", "", "kind/"}, // should not happen in practice
		{"kind", "name", "kind/name"},
	}
	for _, tt := range tests {
		if got := formatResourceName(tt.kind, tt.name, true); got != tt.want {
			t.Errorf("formatResourceName(%q, %q) = %q, want %q", tt.kind, tt.name, got, tt.want)
		}
	}
}

func mockRoleBindingRestriction() []*authorizationapi.RoleBindingRestriction {
	return []*authorizationapi.RoleBindingRestriction{
		{
			ObjectMeta: metav1.ObjectMeta{Name: "match-serviceaccount"},
			Spec: authorizationapi.RoleBindingRestrictionSpec{
				ServiceAccountRestriction: &authorizationapi.ServiceAccountRestriction{
					Namespaces: []string{""},
				},
			},
		},
	}
}

func TestPrintRoleBindingRestriction(t *testing.T) {
	buf := bytes.NewBuffer([]byte{})
	rbrs := mockRoleBindingRestriction()

	tests := []struct {
		name        string
		rbr         *authorizationapi.RoleBindingRestriction
		expectedOut string
		expectedErr error
	}{
		{
			name:        "output check",
			rbr:         rbrs[0],
			expectedOut: "match-serviceaccount\tServiceAccount\t/*\n",
		},
	}

	for _, test := range tests {
		if err := printRoleBindingRestriction(test.rbr, buf, kprinters.PrintOptions{}); err != test.expectedErr {
			t.Errorf("error mismatch: expected %v, got %v", test.expectedErr, err)
			continue
		}
		got := buf.String()
		buf.Reset()

		if !strings.Contains(got, test.expectedOut) {
			t.Errorf("unexpected output:\n%s\nexpected to contain: %s", got, test.expectedOut)
			continue
		}
	}
}

func TestPrintImageStream(t *testing.T) {
	buf := bytes.NewBuffer([]byte{})
	streams := mockStreams()

	tests := []struct {
		name        string
		stream      *imageapi.ImageStream
		expectedOut string
		expectedErr error
	}{
		{
			name:        "less than three tags",
			stream:      streams[0],
			expectedOut: "latest,other",
		},
		{
			name:        "three tags",
			stream:      streams[1],
			expectedOut: "third,latest,other",
		},
		{
			name:        "more than three tags",
			stream:      streams[2],
			expectedOut: "another,third,latest + 1 more...",
		},
	}

	for _, test := range tests {
		if err := printImageStream(test.stream, buf, kprinters.PrintOptions{}); err != test.expectedErr {
			t.Errorf("error mismatch: expected %v, got %v", test.expectedErr, err)
			continue
		}
		got := buf.String()
		buf.Reset()

		if !strings.Contains(got, test.expectedOut) {
			t.Errorf("unexpected output:\n%s\nexpected to contain: %s", got, test.expectedOut)
			continue
		}
	}

}

func mockStreams() []*imageapi.ImageStream {
	return []*imageapi.ImageStream{
		{
			ObjectMeta: metav1.ObjectMeta{Name: "less-than-three-tags"},
			Status: imageapi.ImageStreamStatus{
				Tags: map[string]imageapi.TagEventList{
					"other": {
						Items: []imageapi.TagEvent{
							{
								DockerImageReference: "other-ref",
								Created:              metav1.Date(2015, 9, 4, 13, 52, 0, 0, time.UTC),
								Image:                "other-image",
							},
						},
					},
					"latest": {
						Items: []imageapi.TagEvent{
							{
								DockerImageReference: "latest-ref",
								Created:              metav1.Date(2015, 9, 4, 13, 53, 0, 0, time.UTC),
								Image:                "latest-image",
							},
						},
					},
				},
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{Name: "three-tags"},
			Status: imageapi.ImageStreamStatus{
				Tags: map[string]imageapi.TagEventList{
					"other": {
						Items: []imageapi.TagEvent{
							{
								DockerImageReference: "other-ref",
								Created:              metav1.Date(2015, 9, 4, 13, 52, 0, 0, time.UTC),
								Image:                "other-image",
							},
						},
					},
					"latest": {
						Items: []imageapi.TagEvent{
							{
								DockerImageReference: "latest-ref",
								Created:              metav1.Date(2015, 9, 4, 13, 53, 0, 0, time.UTC),
								Image:                "latest-image",
							},
						},
					},
					"third": {
						Items: []imageapi.TagEvent{
							{
								DockerImageReference: "third-ref",
								Created:              metav1.Date(2015, 9, 4, 13, 54, 0, 0, time.UTC),
								Image:                "third-image",
							},
						},
					},
				},
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{Name: "more-than-three-tags"},
			Status: imageapi.ImageStreamStatus{
				Tags: map[string]imageapi.TagEventList{
					"other": {
						Items: []imageapi.TagEvent{
							{
								DockerImageReference: "other-ref",
								Created:              metav1.Date(2015, 9, 4, 13, 52, 0, 0, time.UTC),
								Image:                "other-image",
							},
						},
					},
					"latest": {
						Items: []imageapi.TagEvent{
							{
								DockerImageReference: "latest-ref",
								Created:              metav1.Date(2015, 9, 4, 13, 53, 0, 0, time.UTC),
								Image:                "latest-image",
							},
						},
					},
					"third": {
						Items: []imageapi.TagEvent{
							{
								DockerImageReference: "third-ref",
								Created:              metav1.Date(2015, 9, 4, 13, 54, 0, 0, time.UTC),
								Image:                "third-image",
							},
						},
					},
					"another": {
						Items: []imageapi.TagEvent{
							{
								DockerImageReference: "another-ref",
								Created:              metav1.Date(2015, 9, 4, 13, 55, 0, 0, time.UTC),
								Image:                "another-image",
							},
						},
					},
				},
			},
		},
	}
}

func TestPrintTemplate(t *testing.T) {
	tests := []struct {
		template templateapi.Template
		want     string
	}{
		{
			templateapi.Template{
				ObjectMeta: metav1.ObjectMeta{
					Name: "name",
					Annotations: map[string]string{
						"description": "description",
					},
				},
				Parameters: []templateapi.Parameter{{}},
				Objects:    []runtime.Object{&kapi.Pod{}},
			},
			"name\tdescription\t1 (1 blank)\t1\n",
		},
		{
			templateapi.Template{
				ObjectMeta: metav1.ObjectMeta{
					Name: "long",
					Annotations: map[string]string{
						"description": "the long description of this template is way way way way way way way way way way way way way too long",
					},
				},
				Parameters: []templateapi.Parameter{},
				Objects:    []runtime.Object{},
			},
			"long\tthe long description of this template is way way way way way way way way way...\t0 (all set)\t0\n",
		},
		{
			templateapi.Template{
				ObjectMeta: metav1.ObjectMeta{
					Name: "multiline",
					Annotations: map[string]string{
						"description": "Once upon a time\nthere was a template\nwith multiple\nlines\n",
					},
				},
				Parameters: []templateapi.Parameter{},
				Objects:    []runtime.Object{},
			},
			"multiline\tOnce upon a time...\t0 (all set)\t0\n",
		},
		{
			templateapi.Template{
				ObjectMeta: metav1.ObjectMeta{
					Name: "trailingnewline",
					Annotations: map[string]string{
						"description": "Next line please\n",
					},
				},
				Parameters: []templateapi.Parameter{},
				Objects:    []runtime.Object{},
			},
			"trailingnewline\tNext line please...\t0 (all set)\t0\n",
		},
		{
			templateapi.Template{
				ObjectMeta: metav1.ObjectMeta{
					Name: "longmultiline",
					Annotations: map[string]string{
						"description": "12345678901234567890123456789012345678901234567890123456789012345678901234567890123\n0",
					},
				},
				Parameters: []templateapi.Parameter{},
				Objects:    []runtime.Object{},
			},
			"longmultiline\t12345678901234567890123456789012345678901234567890123456789012345678901234567...\t0 (all set)\t0\n",
		},
	}

	for i, test := range tests {
		buf := bytes.NewBuffer([]byte{})
		err := printTemplate(&test.template, buf, kprinters.PrintOptions{})
		if err != nil {
			t.Errorf("[%d] unexpected error: %v", i, err)
			continue
		}
		got := buf.String()
		if got != test.want {
			t.Errorf("[%d] expected %q, got %q", i, test.want, got)
			continue
		}
	}
}
