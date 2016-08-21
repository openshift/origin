package podnodeconstraints

import (
	"bytes"
	"fmt"
	"testing"

	admission "k8s.io/kubernetes/pkg/admission"
	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/unversioned"
	"k8s.io/kubernetes/pkg/apis/batch"
	"k8s.io/kubernetes/pkg/apis/extensions"
	"k8s.io/kubernetes/pkg/auth/user"
	"k8s.io/kubernetes/pkg/runtime"
	"k8s.io/kubernetes/pkg/serviceaccount"
	"k8s.io/kubernetes/pkg/util/sets"

	_ "github.com/openshift/origin/pkg/api/install"
	authorizationapi "github.com/openshift/origin/pkg/authorization/api"
	"github.com/openshift/origin/pkg/authorization/authorizer"
	oadmission "github.com/openshift/origin/pkg/cmd/server/admission"
	deployapi "github.com/openshift/origin/pkg/deploy/api"
	"github.com/openshift/origin/pkg/scheduler/admission/podnodeconstraints/api"
	securityapi "github.com/openshift/origin/pkg/security/api"
)

func TestPodNodeConstraints(t *testing.T) {
	ns := kapi.NamespaceDefault
	tests := []struct {
		config           *api.PodNodeConstraintsConfig
		resource         runtime.Object
		kind             unversioned.GroupKind
		groupresource    unversioned.GroupResource
		userinfo         user.Info
		reviewResponse   *authorizationapi.SubjectAccessReviewResponse
		expectedResource string
		expectedErrorMsg string
	}{
		// 0: expect unspecified defaults to not error
		{
			config:           emptyConfig(),
			resource:         defaultPod(),
			userinfo:         serviceaccount.UserInfo("", "", ""),
			reviewResponse:   reviewResponse(false, ""),
			expectedResource: "pods/binding",
			expectedErrorMsg: "",
		},
		// 1: expect nodeSelector to error with user which lacks "pods/binding" access
		{
			config:           testConfig(),
			resource:         nodeSelectorPod(),
			userinfo:         serviceaccount.UserInfo("", "", ""),
			reviewResponse:   reviewResponse(false, ""),
			expectedResource: "pods/binding",
			expectedErrorMsg: "node selection by label(s) [bogus] is prohibited by policy for your role",
		},
		// 2: expect nodeName to fail with user that lacks "pods/binding" access
		{
			config:           testConfig(),
			resource:         nodeNamePod(),
			userinfo:         serviceaccount.UserInfo("herpy", "derpy", ""),
			reviewResponse:   reviewResponse(false, ""),
			expectedResource: "pods/binding",
			expectedErrorMsg: "node selection by nodeName is prohibited by policy for your role",
		},
		// 3: expect nodeName and nodeSelector to fail with user that lacks "pods/binding" access
		{
			config:           testConfig(),
			resource:         nodeNameNodeSelectorPod(),
			userinfo:         serviceaccount.UserInfo("herpy", "derpy", ""),
			reviewResponse:   reviewResponse(false, ""),
			expectedResource: "pods/binding",
			expectedErrorMsg: "node selection by nodeName and label(s) [bogus] is prohibited by policy for your role",
		},
		// 4: expect nodeSelector to succeed with user that has "pods/binding" access
		{
			config:           testConfig(),
			resource:         nodeSelectorPod(),
			userinfo:         serviceaccount.UserInfo("openshift-infra", "daemonset-controller", ""),
			reviewResponse:   reviewResponse(true, ""),
			expectedResource: "pods/binding",
			expectedErrorMsg: "",
		},
		// 5: expect nodeName to succeed with user that has "pods/binding" access
		{
			config:           testConfig(),
			resource:         nodeNamePod(),
			userinfo:         serviceaccount.UserInfo("openshift-infra", "daemonset-controller", ""),
			reviewResponse:   reviewResponse(true, ""),
			expectedResource: "pods/binding",
			expectedErrorMsg: "",
		},
		// 6: expect nil config to bypass admission
		{
			config:           nil,
			resource:         defaultPod(),
			userinfo:         serviceaccount.UserInfo("", "", ""),
			reviewResponse:   reviewResponse(false, ""),
			expectedResource: "pods/binding",
			expectedErrorMsg: "",
		},
	}
	for i, tc := range tests {
		var expectedError error
		errPrefix := fmt.Sprintf("%d", i)
		prc := NewPodNodeConstraints(tc.config)
		prc.(oadmission.WantsAuthorizer).SetAuthorizer(fakeAuthorizer(t))
		err := prc.(oadmission.Validator).Validate()
		if err != nil {
			checkAdmitError(t, err, expectedError, errPrefix)
			continue
		}
		attrs := admission.NewAttributesRecord(tc.resource, nil, kapi.Kind("Pod").WithVersion("version"), ns, "test", kapi.Resource("pods").WithVersion("version"), "", admission.Create, tc.userinfo)
		if tc.expectedErrorMsg != "" {
			expectedError = admission.NewForbidden(attrs, fmt.Errorf(tc.expectedErrorMsg))
		}
		err = prc.Admit(attrs)
		checkAdmitError(t, err, expectedError, errPrefix)
	}
}

func TestPodNodeConstraintsPodUpdate(t *testing.T) {
	ns := kapi.NamespaceDefault
	var expectedError error
	errPrefix := "PodUpdate"
	prc := NewPodNodeConstraints(testConfig())
	prc.(oadmission.WantsAuthorizer).SetAuthorizer(fakeAuthorizer(t))
	err := prc.(oadmission.Validator).Validate()
	if err != nil {
		checkAdmitError(t, err, expectedError, errPrefix)
		return
	}
	attrs := admission.NewAttributesRecord(nodeNamePod(), nodeNamePod(), kapi.Kind("Pod").WithVersion("version"), ns, "test", kapi.Resource("pods").WithVersion("version"), "", admission.Update, serviceaccount.UserInfo("", "", ""))
	err = prc.Admit(attrs)
	checkAdmitError(t, err, expectedError, errPrefix)
}

func TestPodNodeConstraintsNonHandledResources(t *testing.T) {
	ns := kapi.NamespaceDefault
	errPrefix := "ResourceQuotaTest"
	var expectedError error
	prc := NewPodNodeConstraints(testConfig())
	prc.(oadmission.WantsAuthorizer).SetAuthorizer(fakeAuthorizer(t))
	err := prc.(oadmission.Validator).Validate()
	if err != nil {
		checkAdmitError(t, err, expectedError, errPrefix)
		return
	}
	attrs := admission.NewAttributesRecord(resourceQuota(), nil, kapi.Kind("ResourceQuota").WithVersion("version"), ns, "test", kapi.Resource("resourcequotas").WithVersion("version"), "", admission.Create, serviceaccount.UserInfo("", "", ""))
	err = prc.Admit(attrs)
	checkAdmitError(t, err, expectedError, errPrefix)
}

func TestPodNodeConstraintsResources(t *testing.T) {
	ns := kapi.NamespaceDefault
	testconfigs := []struct {
		config         *api.PodNodeConstraintsConfig
		userinfo       user.Info
		reviewResponse *authorizationapi.SubjectAccessReviewResponse
	}{
		{
			config:         testConfig(),
			userinfo:       serviceaccount.UserInfo("", "", ""),
			reviewResponse: reviewResponse(false, ""),
		},
	}
	testresources := []struct {
		resource      func(bool) runtime.Object
		kind          unversioned.GroupKind
		groupresource unversioned.GroupResource
		prefix        string
	}{
		{
			resource:      replicationController,
			kind:          kapi.Kind("ReplicationController"),
			groupresource: kapi.Resource("replicationcontrollers"),
			prefix:        "ReplicationController",
		},
		{
			resource:      deployment,
			kind:          extensions.Kind("Deployment"),
			groupresource: extensions.Resource("deployments"),
			prefix:        "Deployment",
		},
		{
			resource:      replicaSet,
			kind:          extensions.Kind("ReplicaSet"),
			groupresource: extensions.Resource("replicasets"),
			prefix:        "ReplicaSet",
		},
		{
			resource:      job,
			kind:          extensions.Kind("Job"),
			groupresource: extensions.Resource("jobs"),
			prefix:        "Job",
		},
		{
			resource:      job,
			kind:          batch.Kind("Job"),
			groupresource: batch.Resource("jobs"),
			prefix:        "Job",
		},
		{
			resource:      deploymentConfig,
			kind:          deployapi.Kind("DeploymentConfig"),
			groupresource: deployapi.Resource("deploymentconfigs"),
			prefix:        "DeploymentConfig",
		},
		{
			resource:      podTemplate,
			kind:          deployapi.Kind("PodTemplate"),
			groupresource: deployapi.Resource("podtemplates"),
			prefix:        "PodTemplate",
		},
		{
			resource:      podSecurityPolicySubjectReview,
			kind:          securityapi.Kind("PodSecurityPolicySubjectReview"),
			groupresource: securityapi.Resource("podsecuritypolicysubjectreviews"),
			prefix:        "PodSecurityPolicy",
		},
		{
			resource:      podSecurityPolicySelfSubjectReview,
			kind:          securityapi.Kind("PodSecurityPolicySelfSubjectReview"),
			groupresource: securityapi.Resource("podsecuritypolicyselfsubjectreviews"),
			prefix:        "PodSecurityPolicy",
		},
		{
			resource:      podSecurityPolicyReview,
			kind:          securityapi.Kind("PodSecurityPolicyReview"),
			groupresource: securityapi.Resource("podsecuritypolicyreviews"),
			prefix:        "PodSecurityPolicy",
		},
	}
	testparams := []struct {
		nodeselector     bool
		expectedErrorMsg string
		prefix           string
	}{
		{
			nodeselector:     true,
			expectedErrorMsg: "node selection by label(s) [bogus] is prohibited by policy for your role",
			prefix:           "with nodeSelector",
		},
		{
			nodeselector:     false,
			expectedErrorMsg: "",
			prefix:           "without nodeSelector",
		},
	}
	testops := []struct {
		operation admission.Operation
	}{
		{
			operation: admission.Create,
		},
		{
			operation: admission.Update,
		},
	}
	for _, tc := range testconfigs {
		for _, tr := range testresources {
			for _, tp := range testparams {
				for _, top := range testops {
					var expectedError error
					errPrefix := fmt.Sprintf("%s; %s; %s", tr.prefix, tp.prefix, top.operation)
					prc := NewPodNodeConstraints(tc.config)
					prc.(oadmission.WantsAuthorizer).SetAuthorizer(fakeAuthorizer(t))
					err := prc.(oadmission.Validator).Validate()
					if err != nil {
						checkAdmitError(t, err, expectedError, errPrefix)
						continue
					}
					attrs := admission.NewAttributesRecord(tr.resource(tp.nodeselector), nil, tr.kind.WithVersion("version"), ns, "test", tr.groupresource.WithVersion("version"), "", top.operation, tc.userinfo)
					if tp.expectedErrorMsg != "" {
						expectedError = admission.NewForbidden(attrs, fmt.Errorf(tp.expectedErrorMsg))
					}
					err = prc.Admit(attrs)
					checkAdmitError(t, err, expectedError, errPrefix)
				}
			}
		}
	}
}

func emptyConfig() *api.PodNodeConstraintsConfig {
	return &api.PodNodeConstraintsConfig{}
}

func testConfig() *api.PodNodeConstraintsConfig {
	return &api.PodNodeConstraintsConfig{
		NodeSelectorLabelBlacklist: []string{"bogus"},
	}
}

func defaultPod() *kapi.Pod {
	pod := &kapi.Pod{}
	return pod
}

func pod(ns bool) runtime.Object {
	pod := &kapi.Pod{}
	if ns {
		pod.Spec.NodeSelector = map[string]string{"bogus": "frank"}
	}
	return pod
}

func nodeNameNodeSelectorPod() *kapi.Pod {
	pod := &kapi.Pod{}
	pod.Spec.NodeName = "frank"
	pod.Spec.NodeSelector = map[string]string{"bogus": "frank"}
	return pod
}

func nodeNamePod() *kapi.Pod {
	pod := &kapi.Pod{}
	pod.Spec.NodeName = "frank"
	return pod
}

func nodeSelectorPod() *kapi.Pod {
	pod := &kapi.Pod{}
	pod.Spec.NodeSelector = map[string]string{"bogus": "frank"}
	return pod
}

func emptyNodeSelectorPod() *kapi.Pod {
	pod := &kapi.Pod{}
	pod.Spec.NodeSelector = map[string]string{}
	return pod
}

func podSpec(setNodeSelector bool) *kapi.PodSpec {
	ps := &kapi.PodSpec{}
	if setNodeSelector {
		ps.NodeSelector = map[string]string{"bogus": "frank"}
	}
	return ps
}

func podTemplateSpec(setNodeSelector bool) *kapi.PodTemplateSpec {
	pts := &kapi.PodTemplateSpec{}
	if setNodeSelector {
		pts.Spec.NodeSelector = map[string]string{"bogus": "frank"}
	}
	return pts
}

func podTemplate(setNodeSelector bool) runtime.Object {
	pt := &kapi.PodTemplate{}
	pt.Template = *podTemplateSpec(setNodeSelector)
	return pt
}

func replicationController(setNodeSelector bool) runtime.Object {
	rc := &kapi.ReplicationController{}
	rc.Spec.Template = podTemplateSpec(setNodeSelector)
	return rc
}

func deployment(setNodeSelector bool) runtime.Object {
	d := &extensions.Deployment{}
	d.Spec.Template = *podTemplateSpec(setNodeSelector)
	return d
}

func replicaSet(setNodeSelector bool) runtime.Object {
	rs := &extensions.ReplicaSet{}
	rs.Spec.Template = *podTemplateSpec(setNodeSelector)
	return rs
}

func job(setNodeSelector bool) runtime.Object {
	j := &batch.Job{}
	j.Spec.Template = *podTemplateSpec(setNodeSelector)
	return j
}

func resourceQuota() runtime.Object {
	rq := &kapi.ResourceQuota{}
	return rq
}

func deploymentConfig(setNodeSelector bool) runtime.Object {
	dc := &deployapi.DeploymentConfig{}
	dc.Spec.Template = podTemplateSpec(setNodeSelector)
	return dc
}

func podSecurityPolicySubjectReview(setNodeSelector bool) runtime.Object {
	pspsr := &securityapi.PodSecurityPolicySubjectReview{}
	pspsr.Spec.Template.Spec = *podSpec(setNodeSelector)
	return pspsr
}

func podSecurityPolicySelfSubjectReview(setNodeSelector bool) runtime.Object {
	pspssr := &securityapi.PodSecurityPolicySelfSubjectReview{}
	pspssr.Spec.Template.Spec = *podSpec(setNodeSelector)
	return pspssr
}

func podSecurityPolicyReview(setNodeSelector bool) runtime.Object {
	pspr := &securityapi.PodSecurityPolicyReview{}
	pspr.Spec.Template.Spec = *podSpec(setNodeSelector)
	return pspr
}

func checkAdmitError(t *testing.T, err error, expectedError error, prefix string) {
	switch {
	case expectedError == nil && err == nil:
		// continue
	case expectedError != nil && err != nil && err.Error() != expectedError.Error():
		t.Errorf("%s: expected error %q, got: %q", prefix, expectedError.Error(), err.Error())
	case expectedError == nil && err != nil:
		t.Errorf("%s: expected no error, got: %q", prefix, err.Error())
	case expectedError != nil && err == nil:
		t.Errorf("%s: expected error %q, no error recieved", prefix, expectedError.Error())
	}
}

type fakeTestAuthorizer struct {
	t *testing.T
}

func fakeAuthorizer(t *testing.T) authorizer.Authorizer {
	return &fakeTestAuthorizer{
		t: t,
	}
}

func (a *fakeTestAuthorizer) Authorize(ctx kapi.Context, passedAttributes authorizer.Action) (bool, string, error) {
	a.t.Logf("Authorize: ctx: %#v", ctx)
	ui, ok := kapi.UserFrom(ctx)
	if !ok {
		return false, "", fmt.Errorf("No valid UserInfo for Context")
	}
	// User with pods/bindings. permission:
	if ui.GetName() == "system:serviceaccount:openshift-infra:daemonset-controller" {
		return true, "", nil
	}
	// User without pods/bindings. permission:
	return false, "", nil
}

func (a *fakeTestAuthorizer) GetAllowedSubjects(ctx kapi.Context, attributes authorizer.Action) (sets.String, sets.String, error) {
	return nil, nil, nil
}

func reviewResponse(allowed bool, msg string) *authorizationapi.SubjectAccessReviewResponse {
	return &authorizationapi.SubjectAccessReviewResponse{
		Allowed: allowed,
		Reason:  msg,
	}
}

func TestReadConfig(t *testing.T) {
	configStr := `apiVersion: v1
kind: PodNodeConstraintsConfig
nodeSelectorLabelBlacklist:
  - bogus
  - foo
`
	buf := bytes.NewBufferString(configStr)
	config, err := readConfig(buf)
	if err != nil {
		t.Fatalf("unexpected error reading config: %v", err)
	}
	if len(config.NodeSelectorLabelBlacklist) == 0 {
		t.Fatalf("NodeSelectorLabelBlacklist didn't take specified value")
	}
}
